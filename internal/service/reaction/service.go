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
	UpdatePair(
		key domain.PairKey,
		otherID string,
		update func(current domain.Pair, blocked bool) (domain.Pair, error),
	) (domain.Pair, error)
}

type Result struct {
	Outcome    interaction.Outcome
	Changed    bool
	Likes      int
	Matched    bool
	MatchedNow bool
}

type Service struct {
	storage Storage
}

func NewService(storage Storage) *Service {
	return &Service{storage: storage}
}

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

	var decision Decision
	key := domain.NewPairKey(actorID, targetID)
	_, err = s.storage.UpdatePair(key, targetID, func(current domain.Pair, blocked bool) (domain.Pair, error) {
		decision, err = Transition(current, actor, target, blocked)
		if err != nil {
			return domain.Pair{}, err
		}
		return decision.Pair, nil
	})
	if err != nil {
		return Result{}, fmt.Errorf("reaction: обновление пары: %w", err)
	}

	return Result{
		Outcome:    decision.Outcome,
		Changed:    decision.Changed,
		Likes:      decision.Pair.LikeCount(),
		Matched:    decision.Pair.IsMatch(),
		MatchedNow: decision.Changed && decision.Pair.IsMatch(),
	}, nil
}
