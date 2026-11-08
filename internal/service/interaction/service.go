package interaction

import (
	"errors"
	"fmt"

	"hawaii-linkup/internal/domain"
)

var ErrInvalidRequest = errors.New("invalid request")

type Repository interface {
	GetUser(id string) (domain.User, error)
	UpdatePair(
		key domain.PairKey,
		otherID string,
		update func(current domain.Pair, blocked bool) (domain.Pair, error),
	) (domain.Pair, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

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

	var outcome Outcome
	key := domain.NewPairKey(fromID, toID)
	_, err = s.repo.UpdatePair(key, toID, func(current domain.Pair, _ bool) (domain.Pair, error) {
		outcome = Decide(from, to, current.HasLiked(toID))
		if outcome == OutcomeRejected {
			return current, nil
		}
		return current.WithLike(fromID), nil
	})
	if err != nil {
		return "", fmt.Errorf("interaction: сохранение лайка: %w", err)
	}
	return outcome, nil
}
