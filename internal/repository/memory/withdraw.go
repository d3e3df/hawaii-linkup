package memory

import (
	"context"
	"fmt"

	domainerrors "hawaii-linkup/internal/domain/errors"
	domain "hawaii-linkup/internal/domain/models"
)

func (s *Store) WithdrawReaction(
	ctx context.Context,
	key domain.PairKey,
	actorID, targetID string,
	validate func(actor, target domain.User, blocked bool) error,
) (domain.Pair, bool, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.Pair{}, false, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	actor, err := s.getUser(actorID)
	if err != nil {
		return domain.Pair{}, false, false, err
	}
	target, err := s.getUser(targetID)
	if err != nil {
		return domain.Pair{}, false, false, fmt.Errorf("%w: %w", domainerrors.ErrTargetNotFound, err)
	}
	if err := validate(actor, target, s.isBlocked(key.Low, key.High)); err != nil {
		return domain.Pair{}, false, false, err
	}
	pair := s.loadPair(key)
	changed := pair.HasLiked(actorID)
	deactivated := pair.HasActiveMatch()
	if changed {
		pair = pair.WithoutLike(actorID)
	}
	if deactivated {
		pair.MatchState = domain.MatchStateInactive
	}
	if changed || deactivated {
		s.pairs[key] = pair
	}
	return pair, changed, deactivated, nil
}
