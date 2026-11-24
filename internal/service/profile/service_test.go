package profile

import (
	"context"
	"errors"
	"testing"

	domainerrors "hawaii-linkup/internal/domain/errors"
	domain "hawaii-linkup/internal/domain/models"
)

type fakeStorage struct {
	user            domain.User
	expectedVersion int
	update          Update
	err             error
}

func (f *fakeStorage) GetUserContext(context.Context, string) (domain.User, error) {
	if f.err != nil {
		return domain.User{}, f.err
	}
	return f.user, nil
}

func (f *fakeStorage) UpdateProfile(_ context.Context, _ string, expectedVersion int, update Update) (domain.User, error) {
	f.expectedVersion = expectedVersion
	f.update = update
	if f.err != nil {
		return domain.User{}, f.err
	}
	f.user.Name = update.Name
	f.user.Version++
	return f.user, nil
}

func TestUpdatePassesExpectedVersionWithoutRetry(t *testing.T) {
	storage := &fakeStorage{user: domain.User{ID: "u1", Version: 3}}
	update := Update{Name: "Новое имя", Age: 25, City: "Москва", Interests: []string{}, Version: 3}
	got, err := NewService(storage).Update(context.Background(), "u1", "u1", update)
	if err != nil || got.Version != 4 || storage.expectedVersion != 3 || storage.update.Name != update.Name {
		t.Fatalf("user=%+v storage=%+v err=%v", got, storage, err)
	}
}

func TestUpdateRejectsNonOwnerBeforeStorage(t *testing.T) {
	storage := &fakeStorage{}
	_, err := NewService(storage).Update(context.Background(), "u2", "u1", Update{})
	if !errors.Is(err, ErrForbidden) || storage.expectedVersion != 0 {
		t.Fatalf("err=%v storage=%+v", err, storage)
	}
}

func TestUpdateValidatesFullRepresentation(t *testing.T) {
	valid := Update{Name: "Имя", Age: 25, City: "Москва", Interests: []string{}, Version: 1}
	tests := []func(Update) Update{
		func(u Update) Update { u.Name = " "; return u },
		func(u Update) Update { u.Age = 17; return u },
		func(u Update) Update { u.City = ""; return u },
		func(u Update) Update { u.Interests = nil; return u },
		func(u Update) Update { u.Version = 0; return u },
	}
	for _, change := range tests {
		_, err := NewService(&fakeStorage{}).Update(context.Background(), "u1", "u1", change(valid))
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("error=%v", err)
		}
	}
}

func TestProfileErrorsRemainClassifiable(t *testing.T) {
	service := NewService(&fakeStorage{err: domainerrors.ErrNotFound})
	if _, err := service.Get(context.Background(), "u99"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get error=%v", err)
	}
	storage := &fakeStorage{err: domainerrors.ErrVersionConflict}
	update := Update{Name: "Имя", Age: 25, City: "Москва", Interests: []string{}, Version: 1}
	service = NewService(storage)
	if _, err := service.Update(context.Background(), "u1", "u1", update); !errors.Is(err, domainerrors.ErrVersionConflict) {
		t.Fatalf("version error=%v", err)
	}
}
