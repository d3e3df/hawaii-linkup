package reaction_test

import (
	"errors"
	"slices"
	"testing"

	"hawaii-linkup/internal/domain"
	"hawaii-linkup/internal/repository/memory"
	"hawaii-linkup/internal/service/eligibility"
	"hawaii-linkup/internal/service/feed"
	"hawaii-linkup/internal/service/interaction"
	"hawaii-linkup/internal/service/reaction"
)

type countingStore struct {
	*memory.Store
	saves    int
	failSave bool
}

var errSave = errors.New("save failed")

func (c *countingStore) SavePair(p domain.Pair) error {
	if c.failSave {
		return errSave
	}
	c.saves++
	return c.Store.SavePair(p)
}

func feedIDs(t *testing.T, store *memory.Store, userID string) []string {
	t.Helper()
	users, err := feed.NewService(store).Feed(userID, 10)
	if err != nil {
		t.Fatalf("feed(%s): %v", userID, err)
	}
	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	return ids
}

func checkFeedAfterFirstLike(t *testing.T, store *memory.Store) {
	t.Helper()
	if got := feedIDs(t, store, "u3"); slices.Contains(got, "u2") {
		t.Fatalf("после лайка Борис не должен быть в ленте Веры: %v", got)
	}
	if got := feedIDs(t, store, "u2"); !slices.Contains(got, "u3") {
		t.Fatalf("Борис ещё не ответил, Вера должна быть в его ленте: %v", got)
	}
}

func TestScenario_LikeRepeatResponseRepeat(t *testing.T) {
	store := &countingStore{Store: memory.NewDemoStore()}
	svc := reaction.NewService(store)
	key := domain.NewPairKey("u3", "u2")

	if got := feedIDs(t, store.Store, "u3"); !slices.Contains(got, "u2") {
		t.Fatalf("до лайка Борис должен быть в ленте Веры: %v", got)
	}

	steps := []struct {
		actor, target string
		want          reaction.Result
		wantSaves     int
	}{
		{"u3", "u2", reaction.Result{Outcome: interaction.OutcomePending, Changed: true, Likes: 1}, 1},
		{"u3", "u2", reaction.Result{Outcome: interaction.OutcomePending, Changed: false, Likes: 1}, 1},
		{"u2", "u3", reaction.Result{Outcome: interaction.OutcomeMatch, Changed: true, Likes: 2}, 2},
		{"u2", "u3", reaction.Result{Outcome: interaction.OutcomeMatch, Changed: false, Likes: 2}, 2},
	}
	for i, st := range steps {
		got, err := svc.Like(st.actor, st.target)
		if err != nil {
			t.Fatalf("шаг %d: %v", i+1, err)
		}
		if got != st.want || store.saves != st.wantSaves {
			t.Fatalf("шаг %d: got %+v, saves=%d; want %+v, saves=%d", i+1, got, store.saves, st.want, st.wantSaves)
		}
		pair, _ := store.LoadPair(key)
		if pair.LikeCount() != st.want.Likes {
			t.Fatalf("шаг %d: в хранилище %d лайков, want %d", i+1, pair.LikeCount(), st.want.Likes)
		}

		if i == 0 {
			checkFeedAfterFirstLike(t, store.Store)
		}
	}
	if got := feedIDs(t, store.Store, "u2"); slices.Contains(got, "u3") {
		t.Fatalf("после match Вера не должна быть в ленте Бориса: %v", got)
	}

	d, err := eligibility.NewService(store.Store).CanLike("u3", "u2")
	if err != nil || d.Reason != eligibility.ReasonAlreadyLiked {
		t.Fatalf("CanLike после лайка: %+v, %v; want already_liked", d, err)
	}
}

func TestScenario_NotAllowedLeavesStateUnchanged(t *testing.T) {
	store := &countingStore{Store: memory.NewDemoStore()}

	_, err := reaction.NewService(store).Like("u3", "u5")

	if !errors.Is(err, reaction.ErrNotAllowed) {
		t.Fatalf("ожидался ErrNotAllowed, got %v", err)
	}
	pair, _ := store.LoadPair(domain.NewPairKey("u3", "u5"))
	if store.saves != 0 || pair.LikeCount() != 0 {
		t.Fatalf("отказ не должен ничего записывать: saves=%d, pair=%+v", store.saves, pair)
	}
}

func TestScenario_SaveFailureThenRetry(t *testing.T) {
	store := &countingStore{Store: memory.NewDemoStore(), failSave: true}
	svc := reaction.NewService(store)
	key := domain.NewPairKey("u3", "u2")

	got, err := svc.Like("u3", "u2")

	if !errors.Is(err, errSave) || got != (reaction.Result{}) {
		t.Fatalf("got %+v, %v; want нулевой результат и errSave", got, err)
	}
	if pair, _ := store.LoadPair(key); pair.LikeCount() != 0 {
		t.Fatalf("после отказа записи состояние изменилось: %+v", pair)
	}
	if ids := feedIDs(t, store.Store, "u3"); !slices.Contains(ids, "u2") {
		t.Fatalf("после отказа записи лента изменилась: %v", ids)
	}

	store.failSave = false
	got, err = svc.Like("u3", "u2")

	if err != nil || !got.Changed || got.Likes != 1 {
		t.Fatalf("повтор после устранения отказа: %+v, %v", got, err)
	}
}
