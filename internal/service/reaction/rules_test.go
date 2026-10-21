package reaction

import (
	"errors"
	"reflect"
	"testing"

	"hawaii-linkup/internal/domain"
	"hawaii-linkup/internal/service/interaction"
)

func user(id string) domain.User {
	return domain.User{ID: id, Name: id, Age: 25, Interests: []string{"music"}, Active: true}
}

func emptyPair(a, b string) domain.Pair {
	return domain.NewPair(domain.NewPairKey(a, b))
}

func TestTransition_FirstLikeIsPending(t *testing.T) {
	got, err := Transition(emptyPair("a", "b"), user("a"), user("b"), false)

	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if !got.Changed || got.Outcome != interaction.OutcomePending {
		t.Fatalf("got %+v, want pending с изменением", got)
	}
	if !got.Pair.HasLiked("a") || got.Pair.HasLiked("b") {
		t.Fatalf("неверное направление лайка: %+v", got.Pair)
	}
}

func TestTransition_ResponseIsMatch(t *testing.T) {
	current := emptyPair("a", "b").WithLike("a")

	got, err := Transition(current, user("b"), user("a"), false)

	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if !got.Changed || got.Outcome != interaction.OutcomeMatch || !got.Pair.IsMatch() {
		t.Fatalf("got %+v, want match с изменением", got)
	}
}

func TestTransition_RepeatIsNoop(t *testing.T) {
	tests := []struct {
		name        string
		current     domain.Pair
		actor       string
		target      string
		wantOutcome interaction.Outcome
	}{
		{"повтор в pending", emptyPair("a", "b").WithLike("a"), "a", "b", interaction.OutcomePending},
		{"повтор после match", emptyPair("a", "b").WithLike("a").WithLike("b"), "b", "a", interaction.OutcomeMatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Transition(tt.current, user(tt.actor), user(tt.target), false)

			if err != nil {
				t.Fatalf("повтор не должен быть ошибкой: %v", err)
			}
			if got.Changed {
				t.Fatal("повтор не должен менять состояние")
			}
			if got.Pair != tt.current || got.Outcome != tt.wantOutcome {
				t.Fatalf("got %+v, want пару без изменений и %s", got, tt.wantOutcome)
			}
		})
	}
}

func TestTransition_NotAllowed(t *testing.T) {
	inactive := user("b")
	inactive.Active = false

	tests := []struct {
		name    string
		current domain.Pair
		actor   domain.User
		target  domain.User
		blocked bool
	}{
		{"сам себе", emptyPair("a", "a"), user("a"), user("a"), false},
		{"неактивный кандидат", emptyPair("a", "b"), user("a"), inactive, false},
		{"блокировка", emptyPair("a", "b"), user("a"), user("b"), true},
		{"блокировка важнее повтора", emptyPair("a", "b").WithLike("a"), user("a"), user("b"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Transition(tt.current, tt.actor, tt.target, tt.blocked)

			if !errors.Is(err, ErrNotAllowed) {
				t.Fatalf("ожидался ErrNotAllowed, got %v", err)
			}
			if got != (Decision{}) {
				t.Fatalf("при отказе результат должен быть нулевым, got %+v", got)
			}
		})
	}
}

func TestTransition_PairOfOtherUsers(t *testing.T) {
	_, err := Transition(emptyPair("a", "c"), user("a"), user("b"), false)

	if !errors.Is(err, ErrPairMismatch) {
		t.Fatalf("ожидался ErrPairMismatch, got %v", err)
	}
}

func TestTransition_DoesNotModifyInput(t *testing.T) {
	current := emptyPair("a", "b")
	actor, target := user("a"), user("b")

	_, _ = Transition(current, actor, target, false)

	if current != emptyPair("a", "b") {
		t.Fatal("исходная пара изменилась")
	}
	if !reflect.DeepEqual(actor, user("a")) || !reflect.DeepEqual(target, user("b")) {
		t.Fatal("входные пользователи (вместе со срезом интересов) изменились")
	}
}

func TestTransition_HighSideStartsLowResponds(t *testing.T) {
	first, err := Transition(emptyPair("a", "b"), user("b"), user("a"), false)
	if err != nil || !first.Pair.HasLiked("b") || first.Pair.HasLiked("a") {
		t.Fatalf("первый лайк от b: %+v, %v", first, err)
	}

	second, err := Transition(first.Pair, user("a"), user("b"), false)

	if err != nil || !second.Changed || second.Outcome != interaction.OutcomeMatch {
		t.Fatalf("ответ от a: %+v, %v; want match с изменением", second, err)
	}
}
