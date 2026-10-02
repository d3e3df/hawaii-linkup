package discovery

import (
	"errors"
	"fmt"
	"slices"

	"hawaii-linkup/internal/domain"
)

var ErrInvalidCriteria = errors.New("invalid criteria")

// Criteria - критерии поиска
type Criteria struct {
	City      string
	MinAge    int
	MaxAge    int
	Interests []string
}

// Validate проверяет критерии
func (c Criteria) Validate() error {
	if c.MinAge < 18 {
		return fmt.Errorf("%w: минимальный возраст должен быть не меньше 18", ErrInvalidCriteria)
	}
	if c.MaxAge < c.MinAge {
		return fmt.Errorf("%w: максимальный возраст меньше минимального", ErrInvalidCriteria)
	}
	return nil
}

// FilterCandidates возвращает пользователей, подходящих под все критерии
func FilterCandidates(users []domain.User, requesterID string, c Criteria) []domain.User {
	result := []domain.User{}
	for _, u := range users {
		if matches(u, requesterID, c) {
			result = append(result, u)
		}
	}
	return result
}

func matches(u domain.User, requesterID string, c Criteria) bool {
	if u.ID == requesterID {
		return false
	}
	if !u.Active {
		return false
	}
	if u.Age < c.MinAge || u.Age > c.MaxAge {
		return false
	}
	if c.City != "" && u.City != c.City {
		return false
	}
	if len(c.Interests) > 0 && !hasCommonInterest(u.Interests, c.Interests) {
		return false
	}
	return true
}

func hasCommonInterest(a, b []string) bool {
	for _, x := range a {
		if slices.Contains(b, x) {
			return true
		}
	}
	return false
}
