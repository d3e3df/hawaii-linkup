//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	domainerrors "hawaii-linkup/internal/domain/errors"
	"hawaii-linkup/internal/service/profile"
)

func TestProfileVersionConflictPreservesFirstUpdate(t *testing.T) {
	store, _ := newReactionService(t)
	profiles := profile.NewService(store)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	initial, err := profiles.Get(ctx, "u2")
	if err != nil || initial.Version != 1 {
		t.Fatalf("initial=%+v err=%v", initial, err)
	}
	firstUpdate := profile.Update{Name: "Первая правка", Age: 29, City: "Москва", Interests: []string{"go"}, Version: initial.Version}
	first, err := profiles.Update(ctx, "u2", "u2", firstUpdate)
	if err != nil || first.Version != 2 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	staleUpdate := firstUpdate
	staleUpdate.Name = "Старая правка"
	if _, err := profiles.Update(ctx, "u2", "u2", staleUpdate); !errors.Is(err, domainerrors.ErrVersionConflict) {
		t.Fatalf("stale error=%v", err)
	}
	persisted, err := profiles.Get(ctx, "u2")
	if err != nil || persisted.Name != firstUpdate.Name || persisted.Age != 29 || persisted.Version != 2 {
		t.Fatalf("persisted=%+v err=%v", persisted, err)
	}
	if _, err := profiles.Update(ctx, "u3", "u2", profile.Update{Version: 2}); !errors.Is(err, profile.ErrForbidden) {
		t.Fatalf("forbidden error=%v", err)
	}
	if _, err := profiles.Get(ctx, "u99"); !errors.Is(err, profile.ErrNotFound) {
		t.Fatalf("missing error=%v", err)
	}
}

func TestConcurrentProfileUpdatesAllowOnlyOneVersion(t *testing.T) {
	store, _ := newReactionService(t)
	profiles := profile.NewService(store)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	type response struct {
		name string
		err  error
	}
	responses := make(chan response, 2)
	for _, name := range []string{"Правка A", "Правка B"} {
		go func() {
			_, err := profiles.Update(ctx, "u3", "u3", profile.Update{
				Name: name, Age: 31, City: "Казань", Interests: []string{"books"}, Version: 1,
			})
			responses <- response{name, err}
		}()
	}
	succeeded, conflicts := 0, 0
	winner := ""
	for range 2 {
		response := <-responses
		switch {
		case response.err == nil:
			succeeded++
			winner = response.name
		case errors.Is(response.err, domainerrors.ErrVersionConflict):
			conflicts++
		default:
			t.Fatal(response.err)
		}
	}
	if succeeded != 1 || conflicts != 1 {
		t.Fatalf("succeeded=%d conflicts=%d", succeeded, conflicts)
	}
	persisted, err := profiles.Get(ctx, "u3")
	if err != nil || persisted.Name != winner || persisted.Version != 2 {
		t.Fatalf("persisted=%+v winner=%q err=%v", persisted, winner, err)
	}
}
