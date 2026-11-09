package feed

import (
	"errors"
	"reflect"
	"testing"

	"hawaii-linkup/internal/domain"
)

func pageUsers() []domain.User {
	return []domain.User{
		{ID: "me", Active: true, Interests: []string{"go"}},
		{ID: "a", Active: true, Interests: []string{"go"}},
		{ID: "b", Active: true, Interests: []string{"music"}},
		{ID: "c", Active: true, Interests: []string{"Go"}},
		{ID: "d", Active: true, Interests: []string{"go"}},
		{ID: "e", Active: true, Interests: []string{"go"}},
	}
}

func TestService_PageFiltersBeforePagination(t *testing.T) {
	source := &stubSource{users: pageUsers()}
	svc := NewService(source)

	first, err := svc.Page("me", Query{Interest: " go ", Limit: 2})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	second, err := svc.Page("me", Query{Interest: "go", Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if !reflect.DeepEqual(ids(first), []string{"a", "c"}) || !reflect.DeepEqual(ids(second), []string{"d", "e"}) {
		t.Fatalf("pages: first=%v, second=%v", ids(first), ids(second))
	}
}

func TestService_PageEmptyAndBeyondEndAreArrays(t *testing.T) {
	svc := NewService(&stubSource{users: pageUsers()})

	empty, err := svc.Page("me", Query{Interest: "missing", Limit: 2})
	if err != nil {
		t.Fatalf("empty page: %v", err)
	}
	beyond, err := svc.Page("me", Query{Interest: "go", Limit: 2, Offset: 100})
	if err != nil {
		t.Fatalf("beyond end: %v", err)
	}
	if empty == nil || beyond == nil || len(empty) != 0 || len(beyond) != 0 {
		t.Fatalf("empty=%v, beyond=%v; want non-nil empty slices", empty, beyond)
	}
}

func TestService_PageInvalidInputDoesNotCallStorage(t *testing.T) {
	tests := []Query{
		{Limit: 0},
		{Limit: MaxLimit + 1},
		{Limit: -1},
		{Limit: 2, Offset: -1},
	}
	for _, query := range tests {
		source := &stubSource{users: pageUsers()}
		_, err := NewService(source).Page("me", query)
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("query %+v: expected ErrInvalidRequest, got %v", query, err)
		}
		if source.calls != 0 {
			t.Fatalf("query %+v caused %d storage calls", query, source.calls)
		}
	}
}

func TestBuildPage_DoesNotModifyInput(t *testing.T) {
	users := pageUsers()
	want := pageUsers()
	_ = BuildPage(users, "me", nil, Query{Interest: "go", Limit: 2, Offset: 1})
	if !reflect.DeepEqual(users, want) {
		t.Fatalf("input changed: %v", users)
	}
}
