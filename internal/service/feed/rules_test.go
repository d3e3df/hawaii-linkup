package feed

import (
	"reflect"
	"slices"
	"testing"

	"hawaii-linkup/internal/domain"
)

func testUsers() []domain.User {
	return []domain.User{
		{ID: "me", Name: "Я", Age: 25, Active: true},
		{ID: "a", Name: "A", Age: 25, Active: true},
		{ID: "b", Name: "B", Age: 26, Active: true},
		{ID: "c", Name: "C", Age: 27, Active: false},
		{ID: "d", Name: "D", Age: 28, Active: true},
	}
}

func ids(users []domain.User) []string {
	result := []string{}
	for _, u := range users {
		result = append(result, u.ID)
	}
	return result
}

func TestBuildFeed_HappyPath(t *testing.T) {
	got := ids(BuildFeed(testUsers(), "me", []string{"b"}, 10))

	want := []string{"a", "d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildFeed_ExcludesExcludedIDs(t *testing.T) {
	got := ids(BuildFeed(testUsers(), "me", []string{"a", "d"}, 10))

	want := []string{"b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildFeed_ExcludesSelfAndInactive(t *testing.T) {
	got := ids(BuildFeed(testUsers(), "me", nil, 10))

	if slices.Contains(got, "me") || slices.Contains(got, "c") {
		t.Fatalf("в ленту попал сам пользователь или неактивный: %v", got)
	}
}

func TestBuildFeed_RespectsLimit(t *testing.T) {
	got := ids(BuildFeed(testUsers(), "me", nil, 2))

	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildFeed_EmptyResult(t *testing.T) {
	got := BuildFeed(testUsers(), "me", []string{"a", "b", "d"}, 10)

	if got == nil || len(got) != 0 {
		t.Fatalf("ожидался пустой (не nil) срез, got %v", got)
	}
}

func TestBuildFeed_DoesNotModifyInput(t *testing.T) {
	users := testUsers()
	excluded := []string{"b"}

	_ = BuildFeed(users, "me", excluded, 1)

	if !reflect.DeepEqual(users, testUsers()) {
		t.Fatal("входной срез пользователей изменился")
	}
	if !reflect.DeepEqual(excluded, []string{"b"}) {
		t.Fatal("срез исключений изменился")
	}
}
