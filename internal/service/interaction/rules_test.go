package interaction

import (
	"reflect"
	"testing"

	"hawaii-linkup/internal/domain"
)

func activeUser(id string) domain.User {
	return domain.User{ID: id, Name: id, Age: 25, Interests: []string{"music"}, Active: true}
}

func inactiveUser(id string) domain.User {
	u := activeUser(id)
	u.Active = false
	return u
}

func TestDecide_Pending(t *testing.T) {
	got := Decide(activeUser("a"), activeUser("b"), false)

	if got != OutcomePending {
		t.Fatalf("got %s, want %s", got, OutcomePending)
	}
}

func TestDecide_MatchWhenReverseLikeExists(t *testing.T) {
	got := Decide(activeUser("a"), activeUser("b"), true)

	if got != OutcomeMatch {
		t.Fatalf("got %s, want %s", got, OutcomeMatch)
	}
}

func TestDecide_Rejected(t *testing.T) {
	tests := []struct {
		name string
		from domain.User
		to   domain.User
	}{
		{"сам себе", activeUser("a"), activeUser("a")},
		{"from неактивен", inactiveUser("a"), activeUser("b")},
		{"to неактивен", activeUser("a"), inactiveUser("b")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(tt.from, tt.to, true)

			if got != OutcomeRejected {
				t.Fatalf("got %s, want %s", got, OutcomeRejected)
			}
		})
	}
}

func TestDecide_DoesNotModifyInput(t *testing.T) {
	from, to := activeUser("a"), activeUser("b")

	_ = Decide(from, to, true)

	if !reflect.DeepEqual(from, activeUser("a")) || !reflect.DeepEqual(to, activeUser("b")) {
		t.Fatal("входные данные изменились")
	}
}
