package block

import (
	"context"
	"errors"
	"testing"

	domain "hawaii-linkup/internal/domain/models"
)

type fakeStorage struct {
	changed     bool
	deactivated bool
	err         error
}

func (f fakeStorage) BlockUser(_ context.Context, _, _ string, validate func(domain.User, domain.User, bool) error) (bool, bool, error) {
	if err := validate(domain.User{ID: "a", Active: true}, domain.User{ID: "b", Active: true}, false); err != nil {
		return false, false, err
	}
	return f.changed, f.deactivated, f.err
}

func TestBlockResult(t *testing.T) {
	result, err := NewService(fakeStorage{changed: true, deactivated: true}).Block(context.Background(), "a", "b")
	if err != nil || !result.Changed || !result.Blocked || !result.MatchDeactivated {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestBlockChecksIdentity(t *testing.T) {
	_, err := NewService(fakeStorage{}).Block(context.Background(), "", "b")
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error=%v", err)
	}
}

func TestCheckRules(t *testing.T) {
	active := domain.User{ID: "a", Active: true}
	self := active
	inactive := domain.User{ID: "b", Active: false}
	if !errors.Is(Check(active, self, false), ErrSelfBlock) {
		t.Fatal("self block must be rejected")
	}
	if !errors.Is(Check(active, inactive, false), ErrInactive) {
		t.Fatal("inactive profile must be rejected")
	}
	if err := Check(active, inactive, false); err == nil {
		t.Fatal("expected inactive error")
	}
}
