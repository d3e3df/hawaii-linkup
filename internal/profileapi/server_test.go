package profileapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hawaii-linkup/internal/adapter/profilehttp"
	"hawaii-linkup/internal/domain"
)

func TestServerModesAndRecovery(t *testing.T) {
	dependency, err := NewServer([]domain.User{{ID: "u1", Active: true}}, ModeOK, 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(dependency.Handler())
	t.Cleanup(server.Close)
	client, err := profilehttp.NewClient(profilehttp.Config{
		BaseURL:               server.URL,
		ClientTimeout:         50 * time.Millisecond,
		DialTimeout:           time.Second,
		ResponseHeaderTimeout: time.Second,
		MaxResponseBytes:      128,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.Get(context.Background(), "u1"); err != nil {
		t.Fatalf("ok: %v", err)
	}
	tests := []struct {
		mode Mode
		want error
	}{
		{ModeMissing, profilehttp.ErrNotFound},
		{ModeError, profilehttp.ErrUnavailable},
		{ModeSlow, profilehttp.ErrTimeout},
		{ModeLarge, profilehttp.ErrResponseTooLarge},
	}
	for _, tt := range tests {
		setMode(t, server.URL, tt.mode)
		_, err := client.Get(context.Background(), "u1")
		if !errors.Is(err, tt.want) {
			t.Fatalf("mode %s: error=%v, want %v", tt.mode, err, tt.want)
		}
	}
	setMode(t, server.URL, ModeOK)
	profile, err := client.Get(context.Background(), "u1")
	if err != nil || profile.ID != "u1" {
		t.Fatalf("recovery: profile=%+v, err=%v", profile, err)
	}
}

func setMode(t *testing.T, baseURL string, mode Mode) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, baseURL+"/_admin/mode/"+string(mode), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set mode %s: status %d", mode, resp.StatusCode)
	}
}

func TestServerRejectsInvalidMode(t *testing.T) {
	dependency, err := NewServer(nil, ModeOK, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/_admin/mode/bad", nil)
	resp := httptest.NewRecorder()
	dependency.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, body=%s", resp.Code, resp.Body.String())
	}
}

func TestServerValidatesSettings(t *testing.T) {
	if _, err := NewServer(nil, Mode("bad"), time.Second); err == nil {
		t.Fatal("expected invalid mode error")
	}
	if _, err := NewServer(nil, ModeOK, 0); err == nil {
		t.Fatal("expected invalid delay error")
	}
	if _, err := NewServer([]domain.User{{ID: ""}}, ModeOK, time.Second); err == nil {
		t.Fatal("expected empty id error")
	}
}

func ExampleServer() {
	dependency, _ := NewServer([]domain.User{{ID: "u1", Active: true}}, ModeOK, time.Second)
	_ = dependency.Handler()
	fmt.Println("profile API handler created")
	// Output: profile API handler created
}
