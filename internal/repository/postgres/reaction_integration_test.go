//go:build integration

package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"hawaii-linkup/fixtures"
	domainerrors "hawaii-linkup/internal/domain/errors"
	domain "hawaii-linkup/internal/domain/models"
	"hawaii-linkup/internal/service/feed"
	"hawaii-linkup/internal/service/reaction"
	"hawaii-linkup/migrations"
)

type localProfiles struct {
	store *Store
}

func (p localProfiles) Get(ctx context.Context, id string) (domain.User, error) {
	return p.store.GetUserContext(ctx, id)
}

func newReactionService(t *testing.T) (*Store, *reaction.Service) {
	t.Helper()
	pool := newIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := Migrate(ctx, pool, migrations.FS, fixtures.FS, true); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store := NewStore(pool)
	return store, reaction.NewService(store, localProfiles{store})
}

func TestReactionSuccessRepeatReadAndPersistence(t *testing.T) {
	store, service := newReactionService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	first, err := service.Like(ctx, "u3", "u2")
	if err != nil || !first.Changed || first.Likes != 1 || first.Matched {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	users, err := feed.NewService(store).PageContext(ctx, "u3", feed.Query{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range users {
		if user.ID == "u2" {
			t.Fatal("liked user remains in discovery")
		}
	}
	repeated, err := service.Like(ctx, "u3", "u2")
	if err != nil || repeated.Changed || repeated.Likes != 1 {
		t.Fatalf("repeat=%+v err=%v", repeated, err)
	}
	matched, err := service.Like(ctx, "u2", "u3")
	if err != nil || !matched.Changed || !matched.Matched || !matched.MatchedNow || matched.Likes != 2 {
		t.Fatalf("match=%+v err=%v", matched, err)
	}
	key := domain.NewPairKey("u2", "u3")
	pair, err := NewStore(store.pool).LoadPairContext(ctx, key)
	if err != nil || pair.LikeCount() != 2 || !pair.IsMatch() {
		t.Fatalf("persisted pair=%+v err=%v", pair, err)
	}
	var reactions, matches int
	if err := store.pool.QueryRow(ctx, "SELECT count(*) FROM reactions WHERE (actor_id = 'u2' AND target_id = 'u3') OR (actor_id = 'u3' AND target_id = 'u2')").Scan(&reactions); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, "SELECT count(*) FROM matches WHERE low_id = 'u2' AND high_id = 'u3' AND active").Scan(&matches); err != nil {
		t.Fatal(err)
	}
	if reactions != 2 || matches != 1 {
		t.Fatalf("reactions=%d matches=%d", reactions, matches)
	}
}

func TestReactionRollsBackAfterIntermediateInsert(t *testing.T) {
	store, service := newReactionService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := store.pool.Exec(ctx, "INSERT INTO pairs VALUES ('u3', 'u4') ON CONFLICT DO NOTHING"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, "INSERT INTO reactions VALUES ('u3', 'u4')"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, `CREATE FUNCTION fail_match_insert() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'forced match failure'; END;
	$$`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, "CREATE TRIGGER fail_match BEFORE INSERT ON matches FOR EACH ROW EXECUTE FUNCTION fail_match_insert()"); err != nil {
		t.Fatal(err)
	}
	_, err := service.Like(ctx, "u4", "u3")
	if err == nil {
		t.Fatal("expected forced match failure")
	}
	verification, err := store.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer verification.Release()
	var newReaction, oldReaction, match bool
	if err := verification.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM reactions WHERE actor_id = 'u4' AND target_id = 'u3')").Scan(&newReaction); err != nil {
		t.Fatal(err)
	}
	if err := verification.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM reactions WHERE actor_id = 'u3' AND target_id = 'u4')").Scan(&oldReaction); err != nil {
		t.Fatal(err)
	}
	if err := verification.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM matches WHERE low_id = 'u3' AND high_id = 'u4')").Scan(&match); err != nil {
		t.Fatal(err)
	}
	if newReaction || !oldReaction || match {
		t.Fatalf("new reaction=%t old reaction=%t match=%t", newReaction, oldReaction, match)
	}
}

func TestConcurrentReciprocalReactionsCreateOneMatch(t *testing.T) {
	store, service := newReactionService(t)
	barrier := newTestBarrier()
	store.afterPairLock = barrier.hook
	defer barrier.releaseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	type response struct {
		result reaction.Result
		err    error
	}
	responses := make(chan response, 2)
	go func() {
		result, err := service.Like(ctx, "u2", "u4")
		responses <- response{result, err}
	}()
	waitSignal(t, barrier.entered)
	go func() {
		result, err := service.Like(ctx, "u4", "u2")
		responses <- response{result, err}
	}()
	waitForLockWaiter(t, store)
	barrier.releaseNow()
	changed, matchedNow := 0, 0
	for range 2 {
		response := <-responses
		if response.err != nil {
			t.Fatal(response.err)
		}
		if response.result.Changed {
			changed++
		}
		if response.result.MatchedNow {
			matchedNow++
		}
	}
	if changed != 2 || matchedNow != 1 {
		t.Fatalf("changed=%d matchedNow=%d", changed, matchedNow)
	}
	pair, err := store.LoadPairContext(ctx, domain.NewPairKey("u2", "u4"))
	if err != nil || pair.LikeCount() != 2 || !pair.IsMatch() {
		t.Fatalf("pair=%+v err=%v", pair, err)
	}
}

func TestConcurrentRepeatedReactionIsNoOp(t *testing.T) {
	store, service := newReactionService(t)
	barrier := newTestBarrier()
	store.afterPairLock = barrier.hook
	defer barrier.releaseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	type response struct {
		result reaction.Result
		err    error
	}
	responses := make(chan response, 2)
	for range 2 {
		go func() {
			result, err := service.Like(ctx, "u2", "u3")
			responses <- response{result, err}
		}()
	}
	waitSignal(t, barrier.entered)
	waitForLockWaiter(t, store)
	barrier.releaseNow()
	changed := 0
	for range 2 {
		response := <-responses
		if response.err != nil {
			t.Fatal(response.err)
		}
		if response.result.Changed {
			changed++
		}
	}
	if changed != 1 {
		t.Fatalf("changed=%d, want 1", changed)
	}
	var count int
	if err := store.pool.QueryRow(ctx, "SELECT count(*) FROM reactions WHERE actor_id = 'u2' AND target_id = 'u3'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("reactions=%d", count)
	}
}

func TestPairLockWaitHonorsContextAndReleases(t *testing.T) {
	store, service := newReactionService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := store.pool.Exec(ctx, "INSERT INTO pairs VALUES ('u2', 'u4') ON CONFLICT DO NOTHING"); err != nil {
		t.Fatal(err)
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var low string
	if err := tx.QueryRow(ctx, "SELECT low_id FROM pairs WHERE low_id = 'u2' AND high_id = 'u4' FOR UPDATE").Scan(&low); err != nil {
		t.Fatal(err)
	}
	shortCtx, shortCancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer shortCancel()
	_, err = service.Like(shortCtx, "u2", "u4")
	if !errors.Is(err, domainerrors.ErrDatabaseTimeout) {
		t.Fatalf("error=%v, want database timeout", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	result, err := service.Like(ctx, "u2", "u4")
	if err != nil || !result.Changed {
		t.Fatalf("after release result=%+v err=%v", result, err)
	}
}

type testBarrier struct {
	entered     chan struct{}
	released    chan struct{}
	enteredOnce sync.Once
	releaseOnce sync.Once
}

func newTestBarrier() *testBarrier {
	return &testBarrier{entered: make(chan struct{}), released: make(chan struct{})}
}

func (b *testBarrier) hook(ctx context.Context) error {
	b.enteredOnce.Do(func() { close(b.entered) })
	<-b.released
	return ctx.Err()
}

func (b *testBarrier) releaseNow() {
	b.releaseOnce.Do(func() { close(b.released) })
}

func waitSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal("operation did not reach pair lock")
	}
}

func waitForLockWaiter(t *testing.T, store *Store) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		var waiters int
		err := store.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND pid <> pg_backend_pid()
			AND state = 'active' AND wait_event_type = 'Lock' AND query LIKE '%pairs%'`).Scan(&waiters)
		if err != nil {
			t.Fatal(err)
		}
		if waiters > 0 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("second operation did not wait for pair lock")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
