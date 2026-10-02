package discovery

import (
	"fmt"

	"hawaii-linkup/internal/domain"
)

type UserStorage interface {
	ListUsers() ([]domain.User, error)
}

type Service struct {
	users UserStorage
}

func NewService(users UserStorage) *Service {
	return &Service{users: users}
}

// Find возвращает кандидатов для requesterID по критериям.
func (s *Service) Find(requesterID string, c Criteria) ([]domain.User, error) {
	if requesterID == "" {
		return nil, fmt.Errorf("%w: пустой requesterID", ErrInvalidCriteria)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}

	users, err := s.users.ListUsers()
	if err != nil {
		return nil, fmt.Errorf("discovery: получение пользователей: %w", err)
	}

	return FilterCandidates(users, requesterID, c), nil
}
