package eligibility

import (
	"reflect"
	"testing"

	"hawaii-linkup/internal/domain"
)

func activeUser(id string) domain.User {
	return domain.User{ID: id, Name: id, Age: 25, Interests: []string{"music"}, Active: true}
}

func TestCheck_HappyPath(t *testing.T) {
	got := Check(activeUser("a"), activeUser("b"), false, false)

	want := Decision{Allowed: true, Reason: ReasonOK}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestCheck_Self(t *testing.T) {
	got := Check(activeUser("a"), activeUser("a"), false, false)

	if got.Allowed || got.Reason != ReasonSelf {
		t.Fatalf("got %+v, want отказ self", got)
	}
}

func TestCheck_InactiveCandidate(t *testing.T) {
	to := activeUser("b")
	to.Active = false

	got := Check(activeUser("a"), to, false, false)

	if got.Allowed || got.Reason != ReasonInactive {
		t.Fatalf("got %+v, want отказ inactive", got)
	}
}

func TestCheck_Blocked(t *testing.T) {
	got := Check(activeUser("a"), activeUser("b"), true, false)

	if got.Allowed || got.Reason != ReasonBlocked {
		t.Fatalf("got %+v, want отказ blocked", got)
	}
}

func TestCheck_AlreadyLiked(t *testing.T) {
	got := Check(activeUser("a"), activeUser("b"), false, true)

	if got.Allowed || got.Reason != ReasonAlreadyLiked {
		t.Fatalf("got %+v, want отказ already_liked", got)
	}
}

func TestCheck_BlockedHasPriorityOverAlreadyLiked(t *testing.T) {
	got := Check(activeUser("a"), activeUser("b"), true, true)

	if got.Reason != ReasonBlocked {
		t.Fatalf("got %+v, want reason blocked", got)
	}
}

func TestCheck_DoesNotModifyInput(t *testing.T) {
	from, to := activeUser("a"), activeUser("b")

	_ = Check(from, to, false, false)

	if !reflect.DeepEqual(from, activeUser("a")) || !reflect.DeepEqual(to, activeUser("b")) {
		t.Fatal("входные данные изменились")
	}
}
