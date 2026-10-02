package interaction

import (
	"errors"
	"fmt"

	"hawaii-linkup/internal/domain"
)

var ErrInvalidRequest = errors.New("invalid request")

// Repository - операции, которые нужны сервису от хранилища
type Repository interface {
	GetUser(id string) (domain.User, error)
	HasLiked(from, to string) (bool, error)
	SaveLike(from, to string) error
}

// Service выполняет лайк: читает данные, применяет Decide и сохраняет лайк
type Service struct {
	repo Repository
}

// NewService принимает зависимость извне
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Like обрабатывает лайк fromID -> toID и возвращает результат
// При OutcomeRejected лайк не сохраняется
func (s *Service) Like(fromID, toID string) (Outcome, error) {
	if fromID == "" || toID == "" {
		return "", fmt.Errorf("%w: пустой ID пользователя", ErrInvalidRequest)
	}

	from, err := s.repo.GetUser(fromID)
	if err != nil {
		return "", fmt.Errorf("interaction: получение пользователя %s: %w", fromID, err)
	}
	to, err := s.repo.GetUser(toID)
	if err != nil {
		return "", fmt.Errorf("interaction: получение пользователя %s: %w", toID, err)
	}
	reverseLike, err := s.repo.HasLiked(toID, fromID)
	if err != nil {
		return "", fmt.Errorf("interaction: проверка встречного лайка: %w", err)
	}

	outcome := Decide(from, to, reverseLike)
	if outcome == OutcomeRejected {
		return outcome, nil
	}

	if err := s.repo.SaveLike(fromID, toID); err != nil {
		return "", fmt.Errorf("interaction: сохранение лайка: %w", err)
	}
	return outcome, nil
}
