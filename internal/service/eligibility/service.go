package eligibility

import (
	"errors"
	"fmt"

	"hawaii-linkup/internal/domain"
)

var ErrInvalidRequest = errors.New("invalid request")

type Repository interface {
	GetUser(id string) (domain.User, error)
	IsBlocked(a, b string) (bool, error)
	HasLiked(from, to string) (bool, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CanLike(fromID, toID string) (Decision, error) {
	if fromID == "" || toID == "" {
		return Decision{}, fmt.Errorf("%w: пустой ID пользователя", ErrInvalidRequest)
	}

	from, err := s.repo.GetUser(fromID)
	if err != nil {
		return Decision{}, fmt.Errorf("eligibility: получение пользователя %s: %w", fromID, err)
	}
	to, err := s.repo.GetUser(toID)
	if err != nil {
		return Decision{}, fmt.Errorf("eligibility: получение пользователя %s: %w", toID, err)
	}
	blocked, err := s.repo.IsBlocked(fromID, toID)
	if err != nil {
		return Decision{}, fmt.Errorf("eligibility: проверка блокировки: %w", err)
	}
	liked, err := s.repo.HasLiked(fromID, toID)
	if err != nil {
		return Decision{}, fmt.Errorf("eligibility: проверка лайка: %w", err)
	}

	return Check(from, to, blocked, liked), nil
}
