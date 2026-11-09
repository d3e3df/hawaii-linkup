package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppAddress                   string
	ProfileBaseURL               string
	CommandTimeout               time.Duration
	ProfileClientTimeout         time.Duration
	ProfileDialTimeout           time.Duration
	ProfileResponseHeaderTimeout time.Duration
	ProfileMaxResponseBytes      int64
	ReadHeaderTimeout            time.Duration
	WriteTimeout                 time.Duration
	IdleTimeout                  time.Duration
	MaxHeaderBytes               int
	ShutdownTimeout              time.Duration
}

func Default() Config {
	return Config{
		AppAddress:                   "127.0.0.1:8080",
		ProfileBaseURL:               "http://127.0.0.1:8081",
		CommandTimeout:               300 * time.Millisecond,
		ProfileClientTimeout:         time.Second,
		ProfileDialTimeout:           500 * time.Millisecond,
		ProfileResponseHeaderTimeout: 500 * time.Millisecond,
		ProfileMaxResponseBytes:      16384,
		ReadHeaderTimeout:            5 * time.Second,
		WriteTimeout:                 2 * time.Second,
		IdleTimeout:                  60 * time.Second,
		MaxHeaderBytes:               1 << 20,
		ShutdownTimeout:              5 * time.Second,
	}
}

func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	config := Default()
	config.AppAddress = valueOrDefault(getenv("LINKUP_ADDR"), config.AppAddress)
	config.ProfileBaseURL = valueOrDefault(getenv("PROFILE_BASE_URL"), config.ProfileBaseURL)

	durations := []struct {
		name   string
		target *time.Duration
	}{
		{"LINKUP_COMMAND_TIMEOUT", &config.CommandTimeout},
		{"PROFILE_CLIENT_TIMEOUT", &config.ProfileClientTimeout},
		{"PROFILE_DIAL_TIMEOUT", &config.ProfileDialTimeout},
		{"PROFILE_RESPONSE_HEADER_TIMEOUT", &config.ProfileResponseHeaderTimeout},
		{"LINKUP_READ_HEADER_TIMEOUT", &config.ReadHeaderTimeout},
		{"LINKUP_WRITE_TIMEOUT", &config.WriteTimeout},
		{"LINKUP_IDLE_TIMEOUT", &config.IdleTimeout},
		{"LINKUP_SHUTDOWN_TIMEOUT", &config.ShutdownTimeout},
	}
	for _, setting := range durations {
		if err := applyDuration(getenv(setting.name), setting.name, setting.target); err != nil {
			return Config{}, err
		}
	}
	if err := applyPositiveInt64(getenv("PROFILE_MAX_RESPONSE_BYTES"), "PROFILE_MAX_RESPONSE_BYTES", &config.ProfileMaxResponseBytes); err != nil {
		return Config{}, err
	}
	if raw := getenv("LINKUP_MAX_HEADER_BYTES"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			return Config{}, fmt.Errorf("LINKUP_MAX_HEADER_BYTES must be a positive integer, got %q", raw)
		}
		config.MaxHeaderBytes = value
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func applyDuration(raw, name string, target *time.Duration) error {
	if raw == "" {
		return nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return fmt.Errorf("%s must be a positive duration, got %q", name, raw)
	}
	*target = value
	return nil
}

func applyPositiveInt64(raw, name string, target *int64) error {
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return fmt.Errorf("%s must be a positive integer, got %q", name, raw)
	}
	*target = value
	return nil
}

func (c Config) Validate() error {
	if _, _, err := net.SplitHostPort(c.AppAddress); err != nil {
		return fmt.Errorf("LINKUP_ADDR must be host:port: %w", err)
	}
	parsed, err := url.Parse(c.ProfileBaseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("PROFILE_BASE_URL must be an absolute http(s) URL, got %q", c.ProfileBaseURL)
	}
	durations := []struct {
		name  string
		value time.Duration
	}{
		{"LINKUP_COMMAND_TIMEOUT", c.CommandTimeout},
		{"PROFILE_CLIENT_TIMEOUT", c.ProfileClientTimeout},
		{"PROFILE_DIAL_TIMEOUT", c.ProfileDialTimeout},
		{"PROFILE_RESPONSE_HEADER_TIMEOUT", c.ProfileResponseHeaderTimeout},
		{"LINKUP_READ_HEADER_TIMEOUT", c.ReadHeaderTimeout},
		{"LINKUP_WRITE_TIMEOUT", c.WriteTimeout},
		{"LINKUP_IDLE_TIMEOUT", c.IdleTimeout},
		{"LINKUP_SHUTDOWN_TIMEOUT", c.ShutdownTimeout},
	}
	for _, duration := range durations {
		if duration.value <= 0 {
			return fmt.Errorf("%s must be positive", duration.name)
		}
	}
	if c.ProfileMaxResponseBytes <= 0 || c.MaxHeaderBytes <= 0 {
		return errors.New("response and header limits must be positive")
	}
	if c.WriteTimeout <= c.CommandTimeout {
		return errors.New("LINKUP_WRITE_TIMEOUT must exceed LINKUP_COMMAND_TIMEOUT")
	}
	return nil
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
