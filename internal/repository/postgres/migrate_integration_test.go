//go:build integration

package postgres

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"hawaii-linkup/fixtures"
	"hawaii-linkup/migrations"
)

func TestMigrateIsVersionedAndRepeatable(t *testing.T) {
	pool := newIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for range 2 {
		if err := Migrate(ctx, pool, migrations.FS, fixtures.FS, true); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	var users, applied int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&users); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&applied); err != nil {
		t.Fatal(err)
	}
	migrationNames, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	if users != 8 || applied != len(migrationNames) {
		t.Fatalf("users=%d migrations=%d, want 8 and %d", users, applied, len(migrationNames))
	}
}

func TestInitialSchemaConstraints(t *testing.T) {
	pool := newIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := Migrate(ctx, pool, migrations.FS, fixtures.FS, true); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	tests := []struct {
		name       string
		sql        string
		wantCode   string
		constraint string
	}{
		{"unique user", "INSERT INTO users SELECT * FROM users WHERE id = 'u1'", "23505", "users_pkey"},
		{"not null name", "INSERT INTO users (id, name, age, city, interests, active) VALUES ('u99', NULL, 20, 'Москва', '{}', true)", "23502", ""},
		{"age check", "INSERT INTO users (id, name, age, city, interests, active) VALUES ('u99', 'Тест', 1, 'Москва', '{}', true)", "23514", "users_age_check"},
		{"canonical pair", "INSERT INTO pairs VALUES ('u2', 'u1')", "23514", "pairs_canonical_order"},
		{"pair foreign key", "INSERT INTO pairs VALUES ('u1', 'u99')", "23503", "pairs_high_id_fkey"},
		{"unique reaction", "INSERT INTO reactions SELECT * FROM reactions WHERE actor_id = 'u1' AND target_id = 'u6'", "23505", "reactions_pkey"},
		{"self reaction", "INSERT INTO reactions VALUES ('u1', 'u1')", "23514", "reactions_distinct_users"},
		{"match foreign key", "INSERT INTO matches VALUES ('u3', 'u4', true)", "23503", "matches_low_id_high_id_fkey"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, test.sql)
			assertPostgresError(t, err, test.wantCode, test.constraint)
		})
	}
}

func assertPostgresError(t *testing.T, err error, code, constraint string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("error=%v, want PostgreSQL %s", err, code)
	}
	if pgErr.Code != code {
		t.Fatalf("SQLSTATE=%s, want %s: %v", pgErr.Code, code, pgErr)
	}
	if constraint != "" && pgErr.ConstraintName != constraint {
		t.Fatalf("constraint=%q, want %q", pgErr.ConstraintName, constraint)
	}
}
