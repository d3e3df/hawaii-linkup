package discovery

import (
	"errors"
	"testing"

	"hawaii-linkup/internal/domain"
)

type stubLister struct {
	users []domain.User
	err   error
	calls int
}

func (s *stubLister) ListUsers() ([]domain.User, error) {
	s.calls++
	return s.users, s.err
}

var errStorage = errors.New("storage is down")

func TestService_Find_HappyPath(t *testing.T) {
	svc := NewService(&stubLister{users: testUsers()})

	got, err := svc.Find("me", Criteria{City: "Москва", MinAge: 20, MaxAge: 30, Interests: []string{"music"}})

	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("got %v, want [a]", ids(got))
	}
}

func TestService_Find_EmptyResultIsNotError(t *testing.T) {
	svc := NewService(&stubLister{users: testUsers()})

	got, err := svc.Find("me", Criteria{MinAge: 18, MaxAge: 100, Interests: []string{"chess"}})

	if err != nil {
		t.Fatalf("пустой результат не должен быть ошибкой: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ожидался пустой результат, got %v", ids(got))
	}
}

func TestService_Find_StorageError(t *testing.T) {
	svc := NewService(&stubLister{err: errStorage})

	got, err := svc.Find("me", Criteria{MinAge: 18, MaxAge: 100})

	if !errors.Is(err, errStorage) {
		t.Fatalf("причина ошибки потеряна: %v", err)
	}
	if got != nil {
		t.Fatalf("при ошибке результат должен быть nil, got %v", got)
	}
}

func TestService_Find_InvalidCriteriaDoesNotCallStorage(t *testing.T) {
	stub := &stubLister{users: testUsers()}
	svc := NewService(stub)

	_, err := svc.Find("me", Criteria{MinAge: 30, MaxAge: 20})

	if !errors.Is(err, ErrInvalidCriteria) {
		t.Fatalf("ожидалась ErrInvalidCriteria, got %v", err)
	}
	if stub.calls != 0 {
		t.Fatalf("хранилище не должно вызываться, вызовов: %d", stub.calls)
	}
}

func TestService_Find_EmptyRequester(t *testing.T) {
	svc := NewService(&stubLister{users: testUsers()})

	_, err := svc.Find("", Criteria{MinAge: 18, MaxAge: 100})

	if !errors.Is(err, ErrInvalidCriteria) {
		t.Fatalf("ожидалась ErrInvalidCriteria, got %v", err)
	}
}
