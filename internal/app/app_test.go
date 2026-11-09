package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hawaii-linkup/internal/config"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func startTestServer(t *testing.T, handler http.Handler, grace time.Duration) (context.CancelFunc, string, <-chan error) {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- serve(ctx, server, listener, grace, testLogger())
	}()
	t.Cleanup(func() {
		cancel()
		_ = server.Close()
	})
	return cancel, "http://" + listener.Addr().String(), result
}

func doRequest(url string) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil && resp.StatusCode != http.StatusNoContent {
		return errors.New("unexpected response status")
	}
	return err
}

func TestShutdownWaitsForActiveRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusNoContent)
	})
	cancel, baseURL, serverResult := startTestServer(t, handler, 2*time.Second)
	responseResult := make(chan error, 1)
	go func() { responseResult <- doRequest(baseURL + "/wait") }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	time.AfterFunc(50*time.Millisecond, func() { close(release) })

	select {
	case err := <-responseResult:
		if err != nil {
			t.Fatalf("active request: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("active request did not finish")
	}
	select {
	case err := <-serverResult:
		if err != nil {
			t.Fatalf("graceful shutdown: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
}

func TestShutdownForcesCloseAfterGracePeriod(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	handler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(canceled)
	})
	cancel, baseURL, serverResult := startTestServer(t, handler, 30*time.Millisecond)
	requestResult := make(chan error, 1)
	go func() { requestResult <- doRequest(baseURL + "/wait") }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	cancel()

	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("active request was not canceled by Close")
	}
	select {
	case err := <-serverResult:
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("shutdown error=%v, want deadline", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop after forced close")
	}
	select {
	case err := <-requestResult:
		if err == nil {
			t.Fatal("client request unexpectedly succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("client did not observe forced close")
	}
}

func TestNewServerValidatesConfig(t *testing.T) {
	settings := config.Default()
	settings.WriteTimeout = settings.CommandTimeout
	if _, err := NewServer(settings, testLogger()); err == nil {
		t.Fatal("expected invalid server config error")
	}
}

func TestNewServerHealth(t *testing.T) {
	server, err := NewServer(config.Default(), testLogger())
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	server.Handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("health status=%d", recorder.Code)
	}
}
