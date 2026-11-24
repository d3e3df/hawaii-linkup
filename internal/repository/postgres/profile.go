package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	domainerrors "hawaii-linkup/internal/domain/errors"
	domain "hawaii-linkup/internal/domain/models"
)

func (s *Store) UpdateProfile(ctx context.Context, userID string, expectedVersion int, update domain.ProfileUpdate) (domain.User, error) {
	var user domain.User
	err := s.pool.QueryRow(ctx, `UPDATE users
		SET name = $2, age = $3, city = $4, interests = $5, version = version + 1
		WHERE id = $1 AND version = $6
		RETURNING id, name, age, city, interests, active, version`,
		userID, update.Name, update.Age, update.City, update.Interests, expectedVersion,
	).Scan(&user.ID, &user.Name, &user.Age, &user.City, &user.Interests, &user.Active, &user.Version)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, fmt.Errorf("postgres: update profile: %w", databaseError(err))
	}
	if _, getErr := s.GetUserContext(ctx, userID); getErr != nil {
		return domain.User{}, getErr
	}
	return domain.User{}, fmt.Errorf("postgres: expected version %d: %w", expectedVersion, domainerrors.ErrVersionConflict)
}
