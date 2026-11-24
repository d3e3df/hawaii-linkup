package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	domain "hawaii-linkup/internal/domain/models"
)

func (s *Store) WithdrawReaction(
	ctx context.Context,
	key domain.PairKey,
	actorID, targetID string,
	validate func(actor, target domain.User, blocked bool) error,
) (domain.Pair, bool, bool, error) {
	if !validPairMembers(key, actorID, targetID) {
		return domain.Pair{}, false, false, errors.New("postgres: invalid withdrawal pair")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return domain.Pair{}, false, false, fmt.Errorf("postgres: begin withdrawal: %w", databaseError(err))
	}
	defer rollback(ctx, tx)
	if _, _, err := loadUsers(ctx, tx, actorID, targetID); err != nil {
		return domain.Pair{}, false, false, err
	}
	if err := s.lockPair(ctx, tx, key); err != nil {
		return domain.Pair{}, false, false, err
	}
	actor, target, err := loadUsers(ctx, tx, actorID, targetID)
	if err != nil {
		return domain.Pair{}, false, false, err
	}
	blocked, err := pairBlocked(ctx, tx, key)
	if err != nil {
		return domain.Pair{}, false, false, err
	}
	if err := validate(actor, target, blocked); err != nil {
		return domain.Pair{}, false, false, err
	}
	pair, err := loadPair(ctx, tx, key)
	if err != nil {
		return domain.Pair{}, false, false, err
	}
	deleted, err := tx.Exec(ctx, "DELETE FROM reactions WHERE actor_id = $1 AND target_id = $2", actorID, targetID)
	if err != nil {
		return domain.Pair{}, false, false, fmt.Errorf("postgres: delete reaction: %w", databaseError(err))
	}
	deactivated, err := tx.Exec(ctx,
		"UPDATE matches SET active = false WHERE low_id = $1 AND high_id = $2 AND active = true",
		key.Low, key.High,
	)
	if err != nil {
		return domain.Pair{}, false, false, fmt.Errorf("postgres: deactivate match: %w", databaseError(err))
	}
	changed := deleted.RowsAffected() > 0
	matchDeactivated := deactivated.RowsAffected() > 0
	if changed {
		pair = pair.WithoutLike(actorID)
	}
	if matchDeactivated || pair.MatchState == domain.MatchStateActive {
		pair.MatchState = domain.MatchStateInactive
	}
	if err := commit(ctx, tx); err != nil {
		return domain.Pair{}, false, false, err
	}
	return pair, changed, matchDeactivated, nil
}
