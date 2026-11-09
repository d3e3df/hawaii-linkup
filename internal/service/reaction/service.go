package reaction

import (
	"context"
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

type ProfileProvider interface {
	Get(ctx context.Context, id string) (domain.User, error)
}

type Result struct {
	Outcome    interaction.Outcome
	Changed    bool
	Likes      int
	Matched    bool
	MatchedNow bool
}

type Service struct {
	storage  Storage
	profiles ProfileProvider
}

func NewService(storage Storage, profiles ProfileProvider) *Service {
	return &Service{storage: storage, profiles: profiles}
}

func (s *Service) Like(ctx context.Context, actorID, targetID string) (Result, error) {
	if actorID == "" || targetID == "" {
		return Result{}, fmt.Errorf("%w: пустой ID пользователя", ErrInvalidRequest)
	}
	if s.profiles == nil {
		return Result{}, errors.New("reaction: profile provider is nil")
	}

	actor, err := s.storage.GetUser(actorID)
	if err != nil {
		return Result{}, fmt.Errorf("reaction: получение пользователя %s: %w", actorID, err)
	}
	target, err := s.profiles.Get(ctx, targetID)
	if err != nil {
		return Result{}, fmt.Errorf("reaction: получение профиля %s: %w", targetID, err)
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
