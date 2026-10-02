package interaction

import (
	"errors"
	"testing"

	"hawaii-linkup/internal/domain"
)

type fakeRepo struct {
	users   map[string]domain.User
	likes   map[string]bool
	getErr  error
	hasErr  error
	saveErr error
	saved   []string
}

func newFake() *fakeRepo {
	return &fakeRepo{
		users: map[string]domain.User{
			"a": activeUser("a"),
			"b": activeUser("b"),
			"x": inactiveUser("x"),
		},
		likes: map[string]bool{},
	}
}

func (f *fakeRepo) GetUser(id string) (domain.User, error) {
	if f.getErr != nil {
		return domain.User{}, f.getErr
	}
	u, ok := f.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (f *fakeRepo) HasLiked(from, to string) (bool, error) {
	if f.hasErr != nil {
		return false, f.hasErr
	}
	return f.likes[from+"->"+to], nil
}

func (f *fakeRepo) SaveLike(from, to string) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.likes[from+"->"+to] = true
	f.saved = append(f.saved, from+"->"+to)
	return nil
}

var errStorage = errors.New("storage is down")

func TestService_Like_Pending(t *testing.T) {
	repo := newFake()
	svc := NewService(repo)

	got, err := svc.Like("a", "b")

	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if got != OutcomePending {
		t.Fatalf("got %s, want %s", got, OutcomePending)
	}
	if len(repo.saved) != 1 || repo.saved[0] != "a->b" {
		t.Fatalf("лайк не сохранён: %v", repo.saved)
	}
}

func TestService_Like_Match(t *testing.T) {
	repo := newFake()
	repo.likes["b->a"] = true
	svc := NewService(repo)

	got, err := svc.Like("a", "b")

	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if got != OutcomeMatch {
		t.Fatalf("got %s, want %s", got, OutcomeMatch)
	}
}

func TestService_Like_RejectedDoesNotSave(t *testing.T) {
	repo := newFake()
	svc := NewService(repo)

	got, err := svc.Like("a", "x")

	if err != nil {
		t.Fatalf("отказ не должен быть ошибкой: %v", err)
	}
	if got != OutcomeRejected {
		t.Fatalf("got %s, want %s", got, OutcomeRejected)
	}
	if len(repo.saved) != 0 {
		t.Fatalf("при отказе лайк не должен сохраняться: %v", repo.saved)
	}
}

func TestService_Like_UserNotFound(t *testing.T) {
	svc := NewService(newFake())

	_, err := svc.Like("a", "unknown")

	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ожидалась domain.ErrNotFound, got %v", err)
	}
}

func TestService_Like_DependencyErrors(t *testing.T) {
	tests := []struct {
		name string
		edit func(f *fakeRepo)
	}{
		{"GetUser", func(f *fakeRepo) { f.getErr = errStorage }},
		{"HasLiked", func(f *fakeRepo) { f.hasErr = errStorage }},
		{"SaveLike", func(f *fakeRepo) { f.saveErr = errStorage }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFake()
			tt.edit(repo)
			svc := NewService(repo)

			got, err := svc.Like("a", "b")

			if !errors.Is(err, errStorage) {
				t.Fatalf("причина ошибки потеряна: %v", err)
			}
			if got != "" {
				t.Fatalf("при ошибке не должно быть результата, got %s", got)
			}
		})
	}
}

func TestService_Like_EmptyID(t *testing.T) {
	svc := NewService(newFake())

	_, err := svc.Like("a", "")

	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("ожидалась ErrInvalidRequest, got %v", err)
	}
}
