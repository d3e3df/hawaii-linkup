package memory

import (
	"context"
	"fmt"

	domainerrors "hawaii-linkup/internal/domain/errors"
	domain "hawaii-linkup/internal/domain/models"
)

func (s *Store) GetUserContext(ctx context.Context, id string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	return s.GetUser(id)
}

func (s *Store) ListUsersContext(ctx context.Context) ([]domain.User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.ListUsers()
}

func (s *Store) BlockedIDsContext(ctx context.Context, userID string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.BlockedIDs(userID)
}

func (s *Store) LikedIDsContext(ctx context.Context, userID string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.LikedIDs(userID)
}

func (s *Store) LoadPairContext(ctx context.Context, key domain.PairKey) (domain.Pair, error) {
	if err := ctx.Err(); err != nil {
		return domain.Pair{}, err
	}
	return s.LoadPair(key)
}

func (s *Store) UpdatePairProtected(
	ctx context.Context,
	key domain.PairKey,
	actorID, targetID string,
	update func(current domain.Pair, actor, target domain.User, blocked bool) (domain.Pair, error),
) (domain.Pair, error) {
	if err := ctx.Err(); err != nil {
		return domain.Pair{}, err
	}
	if !key.Valid() || actorID != key.Low && actorID != key.High || targetID != key.Low && targetID != key.High {
		return domain.Pair{}, fmt.Errorf("memory: %w: users outside pair", domainerrors.ErrInvalidPairKey)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	actor, err := s.getUser(actorID)
	if err != nil {
		return domain.Pair{}, err
	}
	target, err := s.getUser(targetID)
	if err != nil {
		return domain.Pair{}, fmt.Errorf("%w: %w", domainerrors.ErrTargetNotFound, err)
	}
	current := s.loadPair(key)
	next, err := update(current, actor, target, s.isBlocked(key.Low, key.High))
	if err != nil {
		return domain.Pair{}, err
	}
	if next.Key != key {
		return domain.Pair{}, fmt.Errorf("memory: %w: update changed key", domainerrors.ErrInvalidPairKey)
	}
	if next.IsMatch() && !current.HasActiveMatch() {
		next.MatchState = domain.MatchStateActive
	}
	if next != current {
		s.pairs[key] = next
	}
	return next, nil
}
