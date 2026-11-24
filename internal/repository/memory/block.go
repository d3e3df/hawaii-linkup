package memory

import (
	"context"
	"fmt"

	domainerrors "hawaii-linkup/internal/domain/errors"
	domain "hawaii-linkup/internal/domain/models"
)

func (s *Store) BlockUser(
	ctx context.Context,
	actorID, targetID string,
	validate func(actor, target domain.User, alreadyBlocked bool) error,
) (bool, bool, error) {
	if err := ctx.Err(); err != nil {
		return false, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	actor, err := s.getUser(actorID)
	if err != nil {
		return false, false, err
	}
	target, err := s.getUser(targetID)
	if err != nil {
		return false, false, fmt.Errorf("%w: %w", domainerrors.ErrTargetNotFound, err)
	}
	alreadyBlocked := s.blocks[actorID][targetID]
	if err := validate(actor, target, alreadyBlocked); err != nil {
		return false, false, err
	}
	if !alreadyBlocked {
		if s.blocks[actorID] == nil {
			s.blocks[actorID] = make(map[string]bool)
		}
		s.blocks[actorID][targetID] = true
	}
	key := domain.NewPairKey(actorID, targetID)
	pair := s.loadPair(key)
	deactivated := pair.HasActiveMatch()
	if deactivated {
		pair.MatchState = domain.MatchStateInactive
		s.pairs[key] = pair
	}
	return !alreadyBlocked, deactivated, nil
}
