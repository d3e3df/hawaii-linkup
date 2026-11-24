package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	domain "hawaii-linkup/internal/domain/models"
)

func (s *Store) BlockUser(
	ctx context.Context,
	actorID, targetID string,
	validate func(actor, target domain.User, alreadyBlocked bool) error,
) (bool, bool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return false, false, fmt.Errorf("postgres: begin block: %w", databaseError(err))
	}
	defer rollback(ctx, tx)
	actor, target, err := loadUsers(ctx, tx, actorID, targetID)
	if err != nil {
		return false, false, err
	}
	if actorID == targetID {
		return false, false, validate(actor, target, false)
	}
	key := domain.NewPairKey(actorID, targetID)
	if err := s.lockPair(ctx, tx, key); err != nil {
		return false, false, err
	}
	actor, target, err = loadUsers(ctx, tx, actorID, targetID)
	if err != nil {
		return false, false, err
	}
	var alreadyBlocked bool
	if err := tx.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM blocks WHERE blocker_id = $1 AND blocked_id = $2)",
		actorID, targetID,
	).Scan(&alreadyBlocked); err != nil {
		return false, false, fmt.Errorf("postgres: read block: %w", databaseError(err))
	}
	if err := validate(actor, target, alreadyBlocked); err != nil {
		return false, false, err
	}
	if !alreadyBlocked {
		if _, err := tx.Exec(ctx,
			"INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
			actorID, targetID,
		); err != nil {
			return false, false, fmt.Errorf("postgres: insert block: %w", databaseError(err))
		}
	}
	result, err := tx.Exec(ctx,
		"UPDATE matches SET active = false WHERE low_id = $1 AND high_id = $2 AND active = true",
		key.Low, key.High,
	)
	if err != nil {
		return false, false, fmt.Errorf("postgres: deactivate match: %w", databaseError(err))
	}
	if err := commit(ctx, tx); err != nil {
		return false, false, err
	}
	return !alreadyBlocked, result.RowsAffected() > 0, nil
}

func pairBlocked(ctx context.Context, querier rowQuerier, key domain.PairKey) (bool, error) {
	var blocked bool
	if err := querier.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM blocks
		WHERE (blocker_id = $1 AND blocked_id = $2) OR (blocker_id = $2 AND blocked_id = $1)
	)`, key.Low, key.High).Scan(&blocked); err != nil {
		return false, fmt.Errorf("postgres: check block: %w", databaseError(err))
	}
	return blocked, nil
}
