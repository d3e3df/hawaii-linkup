package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"hawaii-linkup/internal/profileapi"
	"hawaii-linkup/internal/repository/memory"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("profile api: %v", err)
	}
}

func run() error {
	addr := envOrDefault("PROFILE_ADDR", "127.0.0.1:8081")
	mode := profileapi.Mode(envOrDefault("PROFILE_MODE", string(profileapi.ModeOK)))
	slowDelay, err := time.ParseDuration(envOrDefault("PROFILE_SLOW_DELAY", "2s"))
	if err != nil || slowDelay <= 0 {
		return errors.New("PROFILE_SLOW_DELAY must be a positive duration")
	}
	profiles, err := memory.NewDemoStore().ListUsers()
	if err != nil {
		return err
	}
	dependency, err := profileapi.NewServer(profiles, mode, slowDelay)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           dependency.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      slowDelay + 5*time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	log.Printf("profile api listening on %s, mode=%s", addr, mode)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
