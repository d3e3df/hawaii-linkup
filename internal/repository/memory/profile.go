package memory

import (
	"context"
	"fmt"

	domainerrors "hawaii-linkup/internal/domain/errors"
	domain "hawaii-linkup/internal/domain/models"
)

func (s *Store) UpdateProfile(ctx context.Context, userID string, expectedVersion int, update domain.ProfileUpdate) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, user := range s.users {
		if user.ID != userID {
			continue
		}
		if user.Version != expectedVersion {
			return domain.User{}, fmt.Errorf("memory: profile version %d: %w", user.Version, domainerrors.ErrVersionConflict)
		}
		user.Name = update.Name
		user.Age = update.Age
		user.City = update.City
		user.Interests = append([]string{}, update.Interests...)
		user.Version++
		s.users[index] = cloneUser(user)
		return cloneUser(user), nil
	}
	return domain.User{}, domainerrors.ErrNotFound
}
