package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"hawaii-linkup/fixtures"
	"hawaii-linkup/internal/config"
	"hawaii-linkup/internal/repository/postgres"
	"hawaii-linkup/migrations"
)

func main() {
	withFixtures := flag.Bool("fixtures", true, "apply idempotent demo fixtures")
	flag.Parse()
	settings, err := config.Load()
	if err != nil {
		log.Fatalf("migrate: config: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	connectCtx, cancelConnect := context.WithTimeout(ctx, settings.DatabaseConnectTimeout)
	defer cancelConnect()
	pool, err := postgres.NewPool(connectCtx, postgres.Config{
		DatabaseURL: settings.DatabaseURL,
		MaxConns:    settings.DatabaseMaxConns,
	})
	if err != nil {
		log.Fatalf("migrate: %v", err)
	}
	defer pool.Close()
	migrationCtx, cancelMigration := context.WithTimeout(ctx, settings.MigrationTimeout)
	defer cancelMigration()
	if err := postgres.Migrate(migrationCtx, pool, migrations.FS, fixtures.FS, *withFixtures); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Printf("migrations applied; fixtures=%t", *withFixtures)
}
