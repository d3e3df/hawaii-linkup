//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "hawaii-linkup/internal/domain/models"
	"hawaii-linkup/internal/service/block"
	"hawaii-linkup/internal/service/eligibility"
	"hawaii-linkup/internal/service/reaction"
)

func TestBlockDeactivatesMatchAndRejectsReaction(t *testing.T) {
	store, reactions := newReactionService(t)
	blocks := block.NewService(store)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := reactions.Like(ctx, "u2", "u4"); err != nil {
		t.Fatal(err)
	}
	if _, err := reactions.Like(ctx, "u4", "u2"); err != nil {
		t.Fatal(err)
	}
	first, err := blocks.Block(ctx, "u2", "u4")
	if err != nil || !first.Changed || !first.Blocked || !first.MatchDeactivated {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	repeated, err := blocks.Block(ctx, "u2", "u4")
	if err != nil || repeated.Changed || !repeated.Blocked || repeated.MatchDeactivated {
		t.Fatalf("repeat=%+v err=%v", repeated, err)
	}
	pair, err := store.LoadPairContext(ctx, domain.NewPairKey("u2", "u4"))
	if err != nil || pair.LikeCount() != 2 || pair.HasActiveMatch() {
		t.Fatalf("pair=%+v err=%v", pair, err)
	}
	_, err = reactions.Like(ctx, "u4", "u2")
	assertBlockedReaction(t, err)
}

func TestBlockRollsBackAfterIntermediateInsert(t *testing.T) {
	store, reactions := newReactionService(t)
	blocks := block.NewService(store)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := reactions.Like(ctx, "u2", "u4"); err != nil {
		t.Fatal(err)
	}
	if _, err := reactions.Like(ctx, "u4", "u2"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, `CREATE FUNCTION fail_match_update() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'forced match update failure'; END;
	$$`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, "CREATE TRIGGER fail_match BEFORE UPDATE ON matches FOR EACH ROW EXECUTE FUNCTION fail_match_update()"); err != nil {
		t.Fatal(err)
	}
	if _, err := blocks.Block(ctx, "u2", "u4"); err == nil {
		t.Fatal("expected forced match update failure")
	}
	verification, err := store.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer verification.Release()
	var insertedBlock, activeMatch bool
	if err := verification.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM blocks WHERE blocker_id = 'u2' AND blocked_id = 'u4')").Scan(&insertedBlock); err != nil {
		t.Fatal(err)
	}
	if err := verification.QueryRow(ctx, "SELECT active FROM matches WHERE low_id = 'u2' AND high_id = 'u4'").Scan(&activeMatch); err != nil {
		t.Fatal(err)
	}
	if insertedBlock || !activeMatch {
		t.Fatalf("block=%t active match=%t", insertedBlock, activeMatch)
	}
}

func TestConcurrentLikeAndBlockSerializeOnPairLock(t *testing.T) {
	t.Run("like first creates then block deactivates match", func(t *testing.T) {
		store, reactions := newReactionService(t)
		blocks := block.NewService(store)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := reactions.Like(ctx, "u4", "u2"); err != nil {
			t.Fatal(err)
		}
		barrier := newTestBarrier()
		store.afterPairLock = barrier.hook
		defer barrier.releaseNow()
		type likeResponse struct {
			result reaction.Result
			err    error
		}
		likeDone := make(chan likeResponse, 1)
		blockDone := make(chan struct {
			result block.Result
			err    error
		}, 1)
		go func() {
			result, err := reactions.Like(ctx, "u2", "u4")
			likeDone <- likeResponse{result, err}
		}()
		waitSignal(t, barrier.entered)
		go func() {
			result, err := blocks.Block(ctx, "u4", "u2")
			blockDone <- struct {
				result block.Result
				err    error
			}{result, err}
		}()
		waitForLockWaiter(t, store)
		barrier.releaseNow()
		liked := <-likeDone
		blocked := <-blockDone
		if liked.err != nil || !liked.result.Matched || !liked.result.MatchedNow {
			t.Fatalf("like=%+v err=%v", liked.result, liked.err)
		}
		if blocked.err != nil || !blocked.result.Changed || !blocked.result.MatchDeactivated {
			t.Fatalf("block=%+v err=%v", blocked.result, blocked.err)
		}
		pair, err := store.LoadPairContext(ctx, domain.NewPairKey("u2", "u4"))
		if err != nil || pair.LikeCount() != 2 || pair.HasActiveMatch() {
			t.Fatalf("pair=%+v err=%v", pair, err)
		}
	})

	t.Run("block first rejects waiting like", func(t *testing.T) {
		store, reactions := newReactionService(t)
		blocks := block.NewService(store)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := reactions.Like(ctx, "u2", "u4"); err != nil {
			t.Fatal(err)
		}
		barrier := newTestBarrier()
		store.afterPairLock = barrier.hook
		defer barrier.releaseNow()
		blockDone := make(chan struct {
			result block.Result
			err    error
		}, 1)
		likeDone := make(chan error, 1)
		go func() {
			result, err := blocks.Block(ctx, "u4", "u2")
			blockDone <- struct {
				result block.Result
				err    error
			}{result, err}
		}()
		waitSignal(t, barrier.entered)
		go func() {
			_, err := reactions.Like(ctx, "u4", "u2")
			likeDone <- err
		}()
		waitForLockWaiter(t, store)
		barrier.releaseNow()
		blocked := <-blockDone
		if blocked.err != nil || !blocked.result.Changed || blocked.result.MatchDeactivated {
			t.Fatalf("block=%+v err=%v", blocked.result, blocked.err)
		}
		assertBlockedReaction(t, <-likeDone)
		pair, err := store.LoadPairContext(ctx, domain.NewPairKey("u2", "u4"))
		if err != nil || pair.LikeCount() != 1 || pair.HasActiveMatch() {
			t.Fatalf("pair=%+v err=%v", pair, err)
		}
	})
}

func assertBlockedReaction(t *testing.T, err error) {
	t.Helper()
	var notAllowed *reaction.NotAllowedError
	if !errors.As(err, &notAllowed) || notAllowed.Reason != eligibility.ReasonBlocked {
		t.Fatalf("error=%v, want blocked reaction", err)
	}
}
