package reaction

import (
	"context"
	"errors"
	"fmt"

	domain "hawaii-linkup/internal/domain/models"
	"hawaii-linkup/internal/service/eligibility"
	"hawaii-linkup/internal/service/interaction"
)

var ErrInvalidRequest = errors.New("invalid request")

type Storage interface {
	GetUserContext(ctx context.Context, id string) (domain.User, error)
	UpdatePairProtected(
		ctx context.Context,
		key domain.PairKey,
		actorID, targetID string,
		update func(current domain.Pair, actor, target domain.User, blocked bool) (domain.Pair, error),
	) (domain.Pair, error)
	WithdrawReaction(
		ctx context.Context,
		key domain.PairKey,
		actorID, targetID string,
		validate func(actor, target domain.User, blocked bool) error,
	) (pair domain.Pair, changed, matchDeactivated bool, err error)
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

type WithdrawResult struct {
	Changed          bool
	Likes            int
	Matched          bool
	MatchDeactivated bool
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

	actor, err := s.storage.GetUserContext(ctx, actorID)
	if err != nil {
		return Result{}, fmt.Errorf("reaction: получение пользователя %s: %w", actorID, err)
	}
	externalTarget, err := s.profiles.Get(ctx, targetID)
	if err != nil {
		return Result{}, fmt.Errorf("reaction: получение профиля %s: %w", targetID, err)
	}

	key := domain.NewPairKey(actorID, targetID)
	if actorID == targetID {
		_, err := Transition(domain.NewPair(key), actor, externalTarget, false)
		return Result{}, fmt.Errorf("reaction: обновление пары: %w", err)
	}
	var decision Decision
	pair, err := s.storage.UpdatePairProtected(ctx, key, actorID, targetID, func(current domain.Pair, protectedActor, protectedTarget domain.User, blocked bool) (domain.Pair, error) {
		decision, err = Transition(current, protectedActor, protectedTarget, blocked)
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
		Likes:      pair.LikeCount(),
		Matched:    pair.HasActiveMatch(),
		MatchedNow: decision.Changed && pair.HasActiveMatch(),
	}, nil
}

func (s *Service) Withdraw(ctx context.Context, actorID, targetID string) (WithdrawResult, error) {
	if actorID == "" || targetID == "" {
		return WithdrawResult{}, fmt.Errorf("%w: пустой ID пользователя", ErrInvalidRequest)
	}
	key := domain.NewPairKey(actorID, targetID)
	pair, changed, deactivated, err := s.storage.WithdrawReaction(ctx, key, actorID, targetID, func(actor, target domain.User, blocked bool) error {
		check := eligibility.Check(actor, target, blocked, false)
		if !check.Allowed {
			return &NotAllowedError{Reason: check.Reason}
		}
		return nil
	})
	if err != nil {
		return WithdrawResult{}, fmt.Errorf("reaction: отзыв реакции: %w", err)
	}
	return WithdrawResult{
		Changed: changed, Likes: pair.LikeCount(), Matched: pair.HasActiveMatch(), MatchDeactivated: deactivated,
	}, nil
}
