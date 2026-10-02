package feed

import (
	"errors"
	"reflect"
	"testing"

	"hawaii-linkup/internal/domain"
)

type stubSource struct {
	users      []domain.User
	blocked    []string
	liked      []string
	usersErr   error
	blockedErr error
	likedErr   error
	calls      int
}

func (s *stubSource) ListUsers() ([]domain.User, error) {
	s.calls++
	return s.users, s.usersErr
}

func (s *stubSource) BlockedIDs(userID string) ([]string, error) {
	s.calls++
	return s.blocked, s.blockedErr
}

func (s *stubSource) LikedIDs(userID string) ([]string, error) {
	s.calls++
	return s.liked, s.likedErr
}

var errStorage = errors.New("storage is down")

func TestService_Feed_HappyPath(t *testing.T) {
	svc := NewService(&stubSource{users: testUsers(), blocked: []string{"a"}, liked: []string{"d"}})

	got, err := svc.Feed("me", 10)

	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if !reflect.DeepEqual(ids(got), []string{"b"}) {
		t.Fatalf("got %v, want [b]", ids(got))
	}
}

func TestService_Feed_EmptyResultIsNotError(t *testing.T) {
	svc := NewService(&stubSource{users: testUsers(), blocked: []string{"a", "b"}, liked: []string{"d"}})

	got, err := svc.Feed("me", 10)

	if err != nil {
		t.Fatalf("пустой результат не должен быть ошибкой: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ожидался пустой результат, got %v", ids(got))
	}
}

func TestService_Feed_DependencyErrors(t *testing.T) {
	tests := []struct {
		name string
		stub *stubSource
	}{
		{"ListUsers", &stubSource{usersErr: errStorage}},
		{"BlockedIDs", &stubSource{blockedErr: errStorage}},
		{"LikedIDs", &stubSource{likedErr: errStorage}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(tt.stub)

			got, err := svc.Feed("me", 10)

			if !errors.Is(err, errStorage) {
				t.Fatalf("причина ошибки потеряна: %v", err)
			}
			if got != nil {
				t.Fatalf("при ошибке результат должен быть nil, got %v", got)
			}
		})
	}
}

func TestService_Feed_InvalidRequest(t *testing.T) {
	tests := []struct {
		name        string
		requesterID string
		limit       int
	}{
		{"пустой ID", "", 10},
		{"limit 0", "me", 0},
		{"limit больше максимума", "me", MaxLimit + 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubSource{users: testUsers()}
			svc := NewService(stub)

			_, err := svc.Feed(tt.requesterID, tt.limit)

			if !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("ожидалась ErrInvalidRequest, got %v", err)
			}
			if stub.calls != 0 {
				t.Fatalf("хранилище не должно вызываться, вызовов: %d", stub.calls)
			}
		})
	}
}
