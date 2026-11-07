package profilehttp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testConfig(baseURL string) Config {
	return Config{
		BaseURL:               baseURL,
		ClientTimeout:         time.Second,
		DialTimeout:           time.Second,
		ResponseHeaderTimeout: time.Second,
		MaxResponseBytes:      1024,
	}
}

func newTestClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	client, err := NewClient(testConfig(server.URL))
	if err != nil {
		server.Close()
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(server.Close)
	return client, server
}

func TestClientGetSuccess(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/v1/profiles/u1" {
			t.Errorf("path=%s", r.URL.EscapedPath())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"u1","name":"Anna","age":25,"city":"Moscow","interests":["go"],"active":true}`)
	}))

	profile, err := client.Get(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if profile.ID != "u1" || !profile.Active || len(profile.Interests) != 1 {
		t.Fatalf("profile=%+v", profile)
	}
}

func TestClientGetStatusAndResponseErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{name: "missing", status: http.StatusNotFound, body: `{}`, want: ErrNotFound},
		{name: "upstream error", status: http.StatusInternalServerError, body: `{}`, want: ErrUnavailable},
		{name: "invalid json", status: http.StatusOK, body: `{`, want: ErrInvalidResponse},
		{name: "multiple values", status: http.StatusOK, body: `{"id":"u1","active":true} {}`, want: ErrInvalidResponse},
		{name: "wrong id", status: http.StatusOK, body: `{"id":"u2","active":true}`, want: ErrInvalidResponse},
		{name: "missing active", status: http.StatusOK, body: `{"id":"u1"}`, want: ErrInvalidResponse},
		{name: "unknown field", status: http.StatusOK, body: `{"id":"u1","active":true,"role":"admin"}`, want: ErrInvalidResponse},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = fmt.Fprint(w, tt.body)
			}))
			_, err := client.Get(context.Background(), "u1")
			if !errors.Is(err, tt.want) {
				t.Fatalf("error=%v, want %v", err, tt.want)
			}
		})
	}
}

func TestClientGetLargeResponse(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, strings.Repeat("x", 1025))
	}))
	_, err := client.Get(context.Background(), "u1")
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("error=%v, want ErrResponseTooLarge", err)
	}
}

func TestClientGetNetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	client, err := NewClient(testConfig(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	server.Close()

	_, err = client.Get(context.Background(), "u1")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error=%v, want ErrUnavailable", err)
	}
}

func TestClientGetTimeout(t *testing.T) {
	started := make(chan struct{})
	client, _ := newTestClient(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Get(ctx, "u1")
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("error=%v, want ErrTimeout", err)
	}
	select {
	case <-started:
	default:
		t.Fatal("handler did not start")
	}
}

func TestClientGetCancellationReachesHandler(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	client, _ := newTestClient(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(canceled)
	}))
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := client.Get(ctx, "u1")
		result <- err
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("client did not return after cancellation")
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("handler did not observe cancellation")
	}
}

func TestNewClientRejectsInvalidConfig(t *testing.T) {
	configs := []Config{
		testConfig("://bad"),
		testConfig("file:///tmp/profile"),
		func() Config { c := testConfig("http://localhost"); c.ClientTimeout = 0; return c }(),
		func() Config { c := testConfig("http://localhost"); c.MaxResponseBytes = 0; return c }(),
	}
	for _, config := range configs {
		if _, err := NewClient(config); err == nil {
			t.Fatalf("expected config error for %+v", config)
		}
	}
}
