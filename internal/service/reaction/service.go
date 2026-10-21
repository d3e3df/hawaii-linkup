package reaction

import (
	"errors"
	"fmt"

	"hawaii-linkup/internal/domain"
	"hawaii-linkup/internal/service/interaction"
)

var ErrInvalidRequest = errors.New("invalid request")

type Storage interface {
	GetUser(id string) (domain.User, error)
	IsBlocked(a, b string) (bool, error)
	LoadPair(key domain.PairKey) (domain.Pair, error)
	SavePair(p domain.Pair) error
}

// Result — результат команды Like
type Result struct {
	Outcome interaction.Outcome
	Changed bool
	Likes   int
}

type Service struct {
	storage Storage
}

func NewService(storage Storage) *Service {
	return &Service{storage: storage}
}

// Like: actorID лайкает targetID.
func (s *Service) Like(actorID, targetID string) (Result, error) {
	if actorID == "" || targetID == "" {
		return Result{}, fmt.Errorf("%w: пустой ID пользователя", ErrInvalidRequest)
	}

	actor, err := s.storage.GetUser(actorID)
	if err != nil {
		return Result{}, fmt.Errorf("reaction: получение пользователя %s: %w", actorID, err)
	}
	target, err := s.storage.GetUser(targetID)
	if err != nil {
		return Result{}, fmt.Errorf("reaction: получение пользователя %s: %w", targetID, err)
	}
	blocked, err := s.storage.IsBlocked(actorID, targetID)
	if err != nil {
		return Result{}, fmt.Errorf("reaction: проверка блокировки: %w", err)
	}
	pair, err := s.storage.LoadPair(domain.NewPairKey(actorID, targetID))
	if err != nil {
		return Result{}, fmt.Errorf("reaction: загрузка пары: %w", err)
	}

	decision, err := Transition(pair, actor, target, blocked)
	if err != nil {
		return Result{}, err
	}

	if decision.Changed {
		if err := s.storage.SavePair(decision.Pair); err != nil {
			return Result{}, fmt.Errorf("reaction: сохранение пары: %w", err)
		}
	}

	return Result{
		Outcome: decision.Outcome,
		Changed: decision.Changed,
		Likes:   decision.Pair.LikeCount(),
	}, nil
}
