package discovery

import (
	"reflect"
	"slices"
	"testing"

	"hawaii-linkup/internal/domain"
)

func testUsers() []domain.User {
	return []domain.User{
		{ID: "me", Name: "Я", Age: 25, City: "Москва", Interests: []string{"music"}, Active: true},
		{ID: "a", Name: "A", Age: 25, City: "Москва", Interests: []string{"music", "sport"}, Active: true},
		{ID: "b", Name: "B", Age: 26, City: "Казань", Interests: []string{"music"}, Active: true},
		{ID: "c", Name: "C", Age: 27, City: "Москва", Interests: []string{"books"}, Active: true},
		{ID: "d", Name: "D", Age: 24, City: "Москва", Interests: []string{"music"}, Active: false},
		{ID: "e", Name: "E", Age: 40, City: "Москва", Interests: []string{"music"}, Active: true},
	}
}

func ids(users []domain.User) []string {
	result := []string{}
	for _, u := range users {
		result = append(result, u.ID)
	}
	return result
}

func TestFilterCandidates_HappyPath(t *testing.T) {
	c := Criteria{City: "Москва", MinAge: 20, MaxAge: 30, Interests: []string{"music"}}

	got := ids(FilterCandidates(testUsers(), "me", c))

	want := []string{"a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFilterCandidates_ExcludesInactive(t *testing.T) {
	c := Criteria{MinAge: 18, MaxAge: 100}

	got := ids(FilterCandidates(testUsers(), "me", c))

	if slices.Contains(got, "d") {
		t.Fatalf("неактивный пользователь d попал в результат: %v", got)
	}
}

func TestFilterCandidates_ExcludesRequester(t *testing.T) {
	c := Criteria{MinAge: 18, MaxAge: 100}

	got := ids(FilterCandidates(testUsers(), "me", c))

	if slices.Contains(got, "me") {
		t.Fatalf("сам пользователь попал в результат: %v", got)
	}
}

func TestFilterCandidates_AgeBoundsInclusive(t *testing.T) {
	c := Criteria{MinAge: 26, MaxAge: 27}

	got := ids(FilterCandidates(testUsers(), "me", c))

	want := []string{"b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFilterCandidates_EmptyCityMeansAnyCity(t *testing.T) {
	c := Criteria{MinAge: 18, MaxAge: 30, Interests: []string{"music"}}

	got := ids(FilterCandidates(testUsers(), "me", c))

	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFilterCandidates_EmptyResult(t *testing.T) {
	c := Criteria{MinAge: 18, MaxAge: 100, Interests: []string{"chess"}}

	got := FilterCandidates(testUsers(), "me", c)

	if got == nil || len(got) != 0 {
		t.Fatalf("ожидался пустой (не nil) срез, got %v", got)
	}
}

func TestFilterCandidates_DoesNotModifyInput(t *testing.T) {
	users := testUsers()
	c := Criteria{City: "Москва", MinAge: 20, MaxAge: 30, Interests: []string{"music"}}
	usersBefore := testUsers()
	interestsBefore := slices.Clone(c.Interests)

	_ = FilterCandidates(users, "me", c)

	if !reflect.DeepEqual(users, usersBefore) {
		t.Fatal("входной срез пользователей изменился")
	}
	if !reflect.DeepEqual(c.Interests, interestsBefore) {
		t.Fatal("интересы в критериях изменились")
	}
}

func TestCriteria_Validate(t *testing.T) {
	tests := []struct {
		name    string
		c       Criteria
		wantErr bool
	}{
		{"корректные", Criteria{MinAge: 18, MaxAge: 30}, false},
		{"младше 18", Criteria{MinAge: 17, MaxAge: 30}, true},
		{"max меньше min", Criteria{MinAge: 30, MaxAge: 20}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.c.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
