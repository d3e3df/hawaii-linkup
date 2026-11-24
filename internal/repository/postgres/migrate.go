package postgres

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationLockID int64 = 741852963

func Migrate(ctx context.Context, pool *pgxpool.Pool, migrationFS, fixtureFS fs.FS, withFixtures bool) error {
	if err := applySQLFiles(ctx, pool, migrationFS, true); err != nil {
		return fmt.Errorf("postgres: migrations: %w", err)
	}
	if withFixtures {
		if err := applySQLFiles(ctx, pool, fixtureFS, false); err != nil {
			return fmt.Errorf("postgres: fixtures: %w", err)
		}
	}
	return nil
}

func applySQLFiles(ctx context.Context, pool *pgxpool.Pool, source fs.FS, versioned bool) error {
	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		sql, err := fs.ReadFile(source, name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if err := applySQLFile(ctx, pool, name, string(sql), versioned); err != nil {
			return err
		}
	}
	return nil
}

func applySQLFile(ctx context.Context, pool *pgxpool.Pool, name, sql string, versioned bool) (returnErr error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin %s: %w", name, err)
	}
	defer rollback(ctx, tx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("lock %s: %w", name, err)
	}
	if versioned {
		if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
			name text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
			return fmt.Errorf("create migration table: %w", err)
		}
		var applied bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE name = $1)", name).Scan(&applied); err != nil {
			return fmt.Errorf("check %s: %w", name, err)
		}
		if applied {
			return tx.Commit(ctx)
		}
	}
	if _, err := tx.Exec(ctx, sql); err != nil {
		return fmt.Errorf("apply %s: %w", name, err)
	}
	if versioned {
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (name) VALUES ($1)", name); err != nil {
			return fmt.Errorf("record %s: %w", name, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s: %w", name, err)
	}
	return nil
}

func rollback(parent context.Context, tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 3*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
