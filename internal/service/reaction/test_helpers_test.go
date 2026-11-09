package reaction

import (
	"context"

	"hawaii-linkup/internal/domain"
)

type profileProviderFunc func(ctx context.Context, id string) (domain.User, error)

func (f profileProviderFunc) Get(ctx context.Context, id string) (domain.User, error) {
	return f(ctx, id)
}

func newTestService(storage *fakeStorage) *Service {
	return NewService(storage, profileProviderFunc(func(_ context.Context, id string) (domain.User, error) {
		return storage.GetUser(id)
	}))
}
