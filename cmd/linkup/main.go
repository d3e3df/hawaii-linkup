package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"hawaii-linkup/internal/app"
	"hawaii-linkup/internal/config"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	settings, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, settings, logger); err != nil {
		logger.Error("application stopped with error", "error", err)
		os.Exit(1)
	}
}
