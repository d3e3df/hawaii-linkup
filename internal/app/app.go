package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"hawaii-linkup/internal/adapter/profilehttp"
	"hawaii-linkup/internal/config"
	"hawaii-linkup/internal/repository/postgres"
	"hawaii-linkup/internal/service/block"
	"hawaii-linkup/internal/service/feed"
	"hawaii-linkup/internal/service/reaction"
	httpapi "hawaii-linkup/internal/transport/http"
)

type Storage interface {
	feed.UserStorage
	reaction.Storage
	block.Storage
	httpapi.PairStorage
}

func NewServer(settings config.Config, logger *slog.Logger, store Storage) (*http.Server, error) {
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	profiles, err := profilehttp.NewClient(profilehttp.Config{
		BaseURL:               settings.ProfileBaseURL,
		ClientTimeout:         settings.ProfileClientTimeout,
		DialTimeout:           settings.ProfileDialTimeout,
		ResponseHeaderTimeout: settings.ProfileResponseHeaderTimeout,
		MaxResponseBytes:      settings.ProfileMaxResponseBytes,
	})
	if err != nil {
		return nil, err
	}
	handler := httpapi.NewHandler(
		feed.NewService(store),
		reaction.NewService(store, profiles),
		block.NewService(store),
		store,
		settings.CommandTimeout,
		logger,
	)
	return &http.Server{
		Addr:              settings.AppAddress,
		Handler:           handler,
		ReadHeaderTimeout: settings.ReadHeaderTimeout,
		WriteTimeout:      settings.WriteTimeout,
		IdleTimeout:       settings.IdleTimeout,
		MaxHeaderBytes:    settings.MaxHeaderBytes,
	}, nil
}

func Run(ctx context.Context, settings config.Config, logger *slog.Logger) error {
	if err := settings.Validate(); err != nil {
		return err
	}
	connectCtx, cancelConnect := context.WithTimeout(ctx, settings.DatabaseConnectTimeout)
	defer cancelConnect()
	pool, err := postgres.NewPool(connectCtx, postgres.Config{
		DatabaseURL: settings.DatabaseURL,
		MaxConns:    settings.DatabaseMaxConns,
	})
	if err != nil {
		return err
	}
	defer pool.Close()
	store := postgres.NewStore(pool)
	server, err := NewServer(settings, logger, store)
	if err != nil {
		return err
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", settings.AppAddress)
	if err != nil {
		return fmt.Errorf("listen %s: %w", settings.AppAddress, err)
	}
	logger.Info("linkup listening", "address", listener.Addr())
	return serve(ctx, server, listener, settings.ShutdownTimeout, logger)
}

func serve(
	ctx context.Context,
	server *http.Server,
	listener net.Listener,
	shutdownTimeout time.Duration,
	logger *slog.Logger,
) error {
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- server.Serve(listener)
	}()

	select {
	case err := <-serveResult:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed; closing server", "error", err)
		closeErr := server.Close()
		serveErr := <-serveResult
		if closeErr != nil {
			return fmt.Errorf("shutdown: %w; close: %w", err, closeErr)
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("shutdown: %w; serve: %w", err, serveErr)
		}
		return fmt.Errorf("shutdown: %w", err)
	}
	err := <-serveResult
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve after shutdown: %w", err)
	}
	logger.Info("linkup stopped")
	return nil
}
