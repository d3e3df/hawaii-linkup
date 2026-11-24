package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	domainerrors "hawaii-linkup/internal/domain/errors"
	domain "hawaii-linkup/internal/domain/models"
)

type Store struct {
	pool          *pgxpool.Pool
	afterPairLock func(context.Context) error
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (s *Store) GetUserContext(ctx context.Context, id string) (domain.User, error) {
	return getUser(ctx, s.pool, id)
}

func getUser(ctx context.Context, querier rowQuerier, id string) (domain.User, error) {
	var user domain.User
	err := querier.QueryRow(ctx,
		"SELECT id, name, age, city, interests, active, version FROM users WHERE id = $1", id,
	).Scan(&user.ID, &user.Name, &user.Age, &user.City, &user.Interests, &user.Active, &user.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, fmt.Errorf("postgres: user %s: %w", id, domainerrors.ErrNotFound)
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("postgres: get user: %w", databaseError(err))
	}
	return user, nil
}

func (s *Store) ListUsersContext(ctx context.Context) ([]domain.User, error) {
	rows, err := s.pool.Query(ctx, "SELECT id, name, age, city, interests, active, version FROM users ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("postgres: list users: %w", databaseError(err))
	}
	defer rows.Close()
	users := make([]domain.User, 0)
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(&user.ID, &user.Name, &user.Age, &user.City, &user.Interests, &user.Active, &user.Version); err != nil {
			return nil, fmt.Errorf("postgres: scan user: %w", databaseError(err))
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: read users: %w", databaseError(err))
	}
	return users, nil
}

func (s *Store) BlockedIDsContext(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT blocked_id FROM blocks WHERE blocker_id = $1
		UNION SELECT blocker_id FROM blocks WHERE blocked_id = $1 ORDER BY 1`, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list blocks: %w", databaseError(err))
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("postgres: scan block: %w", databaseError(err))
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: read blocks: %w", databaseError(err))
	}
	return ids, nil
}

func (s *Store) LikedIDsContext(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, "SELECT target_id FROM reactions WHERE actor_id = $1 ORDER BY target_id", userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list reactions: %w", databaseError(err))
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("postgres: scan reaction: %w", databaseError(err))
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: read reactions: %w", databaseError(err))
	}
	return ids, nil
}

func (s *Store) LoadPairContext(ctx context.Context, key domain.PairKey) (domain.Pair, error) {
	return loadPair(ctx, s.pool, key)
}

func loadPair(ctx context.Context, querier rowQuerier, key domain.PairKey) (domain.Pair, error) {
	if !key.Valid() {
		return domain.Pair{}, fmt.Errorf("postgres: %w: %+v", domainerrors.ErrInvalidPairKey, key)
	}
	pair := domain.NewPair(key)
	if err := querier.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM reactions WHERE actor_id = $1 AND target_id = $2)",
		key.Low, key.High,
	).Scan(&pair.LowLiked); err != nil {
		return domain.Pair{}, fmt.Errorf("postgres: load low reaction: %w", databaseError(err))
	}
	if err := querier.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM reactions WHERE actor_id = $1 AND target_id = $2)",
		key.High, key.Low,
	).Scan(&pair.HighLiked); err != nil {
		return domain.Pair{}, fmt.Errorf("postgres: load high reaction: %w", databaseError(err))
	}
	var matchActive bool
	err := querier.QueryRow(ctx,
		"SELECT active FROM matches WHERE low_id = $1 AND high_id = $2",
		key.Low, key.High,
	).Scan(&matchActive)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.Pair{}, fmt.Errorf("postgres: load match: %w", databaseError(err))
	}
	if err == nil {
		pair.MatchState = domain.MatchStateInactive
		if matchActive {
			pair.MatchState = domain.MatchStateActive
		}
	}
	return pair, nil
}

func (s *Store) UpdatePairProtected(
	ctx context.Context,
	key domain.PairKey,
	actorID, targetID string,
	update func(current domain.Pair, actor, target domain.User, blocked bool) (domain.Pair, error),
) (domain.Pair, error) {
	if !validPairMembers(key, actorID, targetID) {
		return domain.Pair{}, fmt.Errorf("postgres: %w: users outside pair", domainerrors.ErrInvalidPairKey)
	}
	if update == nil {
		return domain.Pair{}, errors.New("postgres: nil pair update")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return domain.Pair{}, fmt.Errorf("postgres: begin pair update: %w", databaseError(err))
	}
	defer rollback(ctx, tx)
	if _, _, err := loadUsers(ctx, tx, actorID, targetID); err != nil {
		return domain.Pair{}, err
	}
	if err := s.lockPair(ctx, tx, key); err != nil {
		return domain.Pair{}, err
	}
	actor, target, err := loadUsers(ctx, tx, actorID, targetID)
	if err != nil {
		return domain.Pair{}, err
	}
	current, err := loadPair(ctx, tx, key)
	if err != nil {
		return domain.Pair{}, err
	}
	blocked, err := pairBlocked(ctx, tx, key)
	if err != nil {
		return domain.Pair{}, err
	}
	next, err := update(current, actor, target, blocked)
	if err != nil {
		return domain.Pair{}, err
	}
	next, err = applyPairTransition(ctx, tx, key, actorID, targetID, current, next)
	if err != nil {
		return domain.Pair{}, err
	}
	if err := commit(ctx, tx); err != nil {
		return domain.Pair{}, err
	}
	return next, nil
}

func validPairMembers(key domain.PairKey, actorID, targetID string) bool {
	return key.Valid() && (actorID == key.Low || actorID == key.High) && (targetID == key.Low || targetID == key.High)
}

func (s *Store) lockPair(ctx context.Context, tx pgx.Tx, key domain.PairKey) error {
	if _, err := tx.Exec(ctx,
		"INSERT INTO pairs (low_id, high_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		key.Low, key.High,
	); err != nil {
		return fmt.Errorf("postgres: ensure pair: %w", databaseError(err))
	}
	var lockedLow string
	if err := tx.QueryRow(ctx,
		"SELECT low_id FROM pairs WHERE low_id = $1 AND high_id = $2 FOR UPDATE",
		key.Low, key.High,
	).Scan(&lockedLow); err != nil {
		return fmt.Errorf("postgres: lock pair: %w", databaseError(err))
	}
	if s.afterPairLock != nil {
		if err := s.afterPairLock(ctx); err != nil {
			return fmt.Errorf("postgres: pair lock test point: %w", err)
		}
	}
	return nil
}

func applyPairTransition(ctx context.Context, tx pgx.Tx, key domain.PairKey, actorID, targetID string, current, next domain.Pair) (domain.Pair, error) {
	if next.Key != key || !likesPreserved(current, next) {
		return domain.Pair{}, fmt.Errorf("postgres: %w: invalid pair transition", domainerrors.ErrInvalidPairKey)
	}
	if !current.HasLiked(actorID) && next.HasLiked(actorID) {
		if _, err := tx.Exec(ctx,
			"INSERT INTO reactions (actor_id, target_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
			actorID, targetID,
		); err != nil {
			return domain.Pair{}, fmt.Errorf("postgres: insert reaction: %w", databaseError(err))
		}
	}
	if next.IsMatch() && !current.HasActiveMatch() {
		if _, err := tx.Exec(ctx,
			`INSERT INTO matches (low_id, high_id, active) VALUES ($1, $2, true)
			ON CONFLICT (low_id, high_id) DO UPDATE SET active = true WHERE matches.active = false`,
			key.Low, key.High,
		); err != nil {
			return domain.Pair{}, fmt.Errorf("postgres: create match: %w", databaseError(err))
		}
		next.MatchState = domain.MatchStateActive
	}
	return next, nil
}

func loadUsers(ctx context.Context, tx pgx.Tx, actorID, targetID string) (domain.User, domain.User, error) {
	actor, err := getUser(ctx, tx, actorID)
	if err != nil {
		return domain.User{}, domain.User{}, err
	}
	target, err := getUser(ctx, tx, targetID)
	if err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			return domain.User{}, domain.User{}, fmt.Errorf("%w: %w", domainerrors.ErrTargetNotFound, err)
		}
		return domain.User{}, domain.User{}, err
	}
	return actor, target, nil
}

func likesPreserved(current, next domain.Pair) bool {
	return (!current.LowLiked || next.LowLiked) && (!current.HighLiked || next.HighLiked)
}

func commit(ctx context.Context, tx pgx.Tx) error {
	if err := tx.Commit(ctx); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01") {
			return fmt.Errorf("postgres: commit: %w", domainerrors.ErrTransactionRetryable)
		}
		return fmt.Errorf("postgres: commit: %w: %w", domainerrors.ErrCommitUnconfirmed, err)
	}
	return nil
}

func databaseError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01") {
		return fmt.Errorf("%w: %w", domainerrors.ErrTransactionRetryable, err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", domainerrors.ErrDatabaseTimeout, err)
	}
	return err
}
