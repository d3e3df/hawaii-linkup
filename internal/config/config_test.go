package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	config, err := load(func(string) string { return "" })
	if err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if config.CommandTimeout != 300*time.Millisecond || config.ProfileMaxResponseBytes != 16384 {
		t.Fatalf("unexpected defaults: %+v", config)
	}
}

func TestLoadEnvironment(t *testing.T) {
	values := map[string]string{
		"LINKUP_ADDR":                     "127.0.0.1:9090",
		"LINKUP_COMMAND_TIMEOUT":          "1s",
		"LINKUP_WRITE_TIMEOUT":            "3s",
		"PROFILE_MAX_RESPONSE_BYTES":      "2048",
		"LINKUP_MAX_HEADER_BYTES":         "4096",
		"PROFILE_DIAL_TIMEOUT":            "2s",
		"PROFILE_RESPONSE_HEADER_TIMEOUT": "2s",
	}
	config, err := load(func(name string) string { return values[name] })
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if config.AppAddress != "127.0.0.1:9090" || config.CommandTimeout != time.Second ||
		config.ProfileMaxResponseBytes != 2048 || config.MaxHeaderBytes != 4096 {
		t.Fatalf("unexpected config: %+v", config)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []map[string]string{
		{"LINKUP_ADDR": "bad"},
		{"PROFILE_BASE_URL": "file:///tmp/profile"},
		{"LINKUP_COMMAND_TIMEOUT": "0"},
		{"LINKUP_COMMAND_TIMEOUT": "bad"},
		{"PROFILE_MAX_RESPONSE_BYTES": "-1"},
		{"LINKUP_MAX_HEADER_BYTES": "0"},
		{"LINKUP_COMMAND_TIMEOUT": "2s", "LINKUP_WRITE_TIMEOUT": "1s"},
	}
	for _, values := range tests {
		_, err := load(func(name string) string { return values[name] })
		if err == nil {
			t.Fatalf("expected error for %v", values)
		}
	}
}

func TestLoadErrorNamesSetting(t *testing.T) {
	_, err := load(func(name string) string {
		if name == "PROFILE_CLIENT_TIMEOUT" {
			return "bad"
		}
		return ""
	})
	if err == nil || !strings.Contains(err.Error(), "PROFILE_CLIENT_TIMEOUT") {
		t.Fatalf("error=%v", err)
	}
}
