package profile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	domainerrors "hawaii-linkup/internal/domain/errors"
	domain "hawaii-linkup/internal/domain/models"
)

var (
	ErrInvalidRequest = errors.New("invalid profile request")
	ErrNotFound       = errors.New("profile not found")
	ErrForbidden      = errors.New("profile edit forbidden")
)

type Update = domain.ProfileUpdate

type Storage interface {
	GetUserContext(ctx context.Context, id string) (domain.User, error)
	UpdateProfile(ctx context.Context, userID string, expectedVersion int, update Update) (domain.User, error)
}

type Service struct {
	storage Storage
}

func NewService(storage Storage) *Service {
	return &Service{storage: storage}
}

func (s *Service) Get(ctx context.Context, userID string) (domain.User, error) {
	if userID == "" {
		return domain.User{}, fmt.Errorf("%w: empty user ID", ErrInvalidRequest)
	}
	user, err := s.storage.GetUserContext(ctx, userID)
	if err != nil {
		return domain.User{}, profileError("get", err)
	}
	return user, nil
}

func (s *Service) Update(ctx context.Context, actorID, userID string, update Update) (domain.User, error) {
	if actorID == "" || userID == "" {
		return domain.User{}, fmt.Errorf("%w: empty user ID", ErrInvalidRequest)
	}
	if actorID != userID {
		return domain.User{}, ErrForbidden
	}
	if strings.TrimSpace(update.Name) == "" || strings.TrimSpace(update.City) == "" || update.Age < 18 || update.Age > 120 || update.Interests == nil || update.Version < 1 {
		return domain.User{}, fmt.Errorf("%w: invalid profile fields", ErrInvalidRequest)
	}
	user, err := s.storage.UpdateProfile(ctx, userID, update.Version, update)
	if err != nil {
		return domain.User{}, profileError("update", err)
	}
	return user, nil
}

func profileError(operation string, err error) error {
	if errors.Is(err, domainerrors.ErrNotFound) {
		return fmt.Errorf("profile %s: %w: %w", operation, ErrNotFound, err)
	}
	return fmt.Errorf("profile %s: %w", operation, err)
}
