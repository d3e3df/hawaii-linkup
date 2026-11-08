package profileapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"hawaii-linkup/internal/domain"
)

type Mode string

const (
	ModeOK      Mode = "ok"
	ModeMissing Mode = "missing"
	ModeError   Mode = "error"
	ModeSlow    Mode = "slow"
	ModeLarge   Mode = "large"
)

type Server struct {
	mu        sync.RWMutex
	mode      Mode
	slowDelay time.Duration
	profiles  map[string]domain.User
}

func NewServer(profiles []domain.User, mode Mode, slowDelay time.Duration) (*Server, error) {
	if !validMode(mode) {
		return nil, fmt.Errorf("profile api: invalid mode %q", mode)
	}
	if slowDelay <= 0 {
		return nil, errors.New("profile api: slow delay must be positive")
	}
	server := &Server{
		mode:      mode,
		slowDelay: slowDelay,
		profiles:  make(map[string]domain.User, len(profiles)),
	}
	for _, profile := range profiles {
		if profile.ID == "" {
			return nil, errors.New("profile api: empty profile id")
		}
		server.profiles[profile.ID] = profile
	}
	return server, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("PUT /_admin/mode/{mode}", s.setMode)
	mux.HandleFunc("GET /v1/profiles/{id}", s.getProfile)
	return mux
}

func (s *Server) setMode(w http.ResponseWriter, r *http.Request) {
	mode := Mode(r.PathValue("mode"))
	if !validMode(mode) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown mode"})
		return
	}
	s.mu.Lock()
	s.mode = mode
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]Mode{"mode": mode})
}

func (s *Server) getProfile(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mode := s.mode
	s.mu.RUnlock()

	switch mode {
	case ModeMissing:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "profile not found"})
		return
	case ModeError:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "forced failure"})
		return
	case ModeLarge:
		writeJSON(w, http.StatusOK, struct {
			ID      string `json:"id"`
			Active  bool   `json:"active"`
			Padding string `json:"padding"`
		}{ID: r.PathValue("id"), Active: true, Padding: strings.Repeat("x", 64*1024)})
		return
	case ModeSlow:
		timer := time.NewTimer(s.slowDelay)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
		}
	case ModeOK:
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "invalid mode"})
		return
	}

	profile, ok := s.profiles[r.PathValue("id")]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "profile not found"})
		return
	}
	writeJSON(w, http.StatusOK, profileDTO{
		ID:        profile.ID,
		Name:      profile.Name,
		Age:       profile.Age,
		City:      profile.City,
		Interests: profile.Interests,
		Active:    profile.Active,
	})
}

type profileDTO struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Age       int      `json:"age"`
	City      string   `json:"city"`
	Interests []string `json:"interests"`
	Active    bool     `json:"active"`
}

func validMode(mode Mode) bool {
	switch mode {
	case ModeOK, ModeMissing, ModeError, ModeSlow, ModeLarge:
		return true
	default:
		return false
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
