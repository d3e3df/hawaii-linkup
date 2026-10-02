package eligibility

import (
	"errors"
	"testing"

	"hawaii-linkup/internal/domain"
)

type stubRepo struct {
	users      map[string]domain.User
	blocked    bool
	liked      bool
	getErr     error
	blockedErr error
	likedErr   error
}

func (s *stubRepo) GetUser(id string) (domain.User, error) {
	if s.getErr != nil {
		return domain.User{}, s.getErr
	}
	u, ok := s.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (s *stubRepo) IsBlocked(a, b string) (bool, error) {
	return s.blocked, s.blockedErr
}

func (s *stubRepo) HasLiked(from, to string) (bool, error) {
	return s.liked, s.likedErr
}

func newStub() *stubRepo {
	return &stubRepo{users: map[string]domain.User{
		"a": activeUser("a"),
		"b": activeUser("b"),
	}}
}

var errStorage = errors.New("storage is down")

func TestService_CanLike_HappyPath(t *testing.T) {
	svc := NewService(newStub())

	got, err := svc.CanLike("a", "b")

	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if !got.Allowed {
		t.Fatalf("ожидалось разрешение, got %+v", got)
	}
}

func TestService_CanLike_DeniedIsNotError(t *testing.T) {
	stub := newStub()
	stub.blocked = true
	svc := NewService(stub)

	got, err := svc.CanLike("a", "b")

	if err != nil {
		t.Fatalf("отказ не должен быть ошибкой: %v", err)
	}
	if got.Allowed || got.Reason != ReasonBlocked {
		t.Fatalf("got %+v, want отказ blocked", got)
	}
}

func TestService_CanLike_UserNotFound(t *testing.T) {
	svc := NewService(newStub())

	_, err := svc.CanLike("a", "unknown")

	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ожидалась domain.ErrNotFound, got %v", err)
	}
}

func TestService_CanLike_DependencyErrors(t *testing.T) {
	tests := []struct {
		name string
		edit func(s *stubRepo)
	}{
		{"GetUser", func(s *stubRepo) { s.getErr = errStorage }},
		{"IsBlocked", func(s *stubRepo) { s.blockedErr = errStorage }},
		{"HasLiked", func(s *stubRepo) { s.likedErr = errStorage }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := newStub()
			tt.edit(stub)
			svc := NewService(stub)

			got, err := svc.CanLike("a", "b")

			if !errors.Is(err, errStorage) {
				t.Fatalf("причина ошибки потеряна: %v", err)
			}
			if got.Allowed {
				t.Fatal("при ошибке не должно быть разрешения")
			}
		})
	}
}

func TestService_CanLike_EmptyID(t *testing.T) {
	svc := NewService(newStub())

	_, err := svc.CanLike("", "b")

	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("ожидалась ErrInvalidRequest, got %v", err)
	}
}
