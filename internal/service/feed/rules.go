package feed

import (
	"strings"

	"hawaii-linkup/internal/domain"
)

func BuildFeed(users []domain.User, requesterID string, excluded []string, limit int) []domain.User {
	return BuildPage(users, requesterID, excluded, Query{Limit: limit})
}

func BuildPage(users []domain.User, requesterID string, excluded []string, query Query) []domain.User {
	skip := make(map[string]bool, len(excluded))
	for _, id := range excluded {
		skip[id] = true
	}
	interest := strings.ToLower(strings.TrimSpace(query.Interest))
	result := []domain.User{}
	matched := 0
	for _, u := range users {
		if u.ID == requesterID || !u.Active || skip[u.ID] || !hasInterest(u.Interests, interest) {
			continue
		}
		if matched < query.Offset {
			matched++
			continue
		}
		if len(result) >= query.Limit {
			break
		}
		result = append(result, u)
		matched++
	}
	return result
}

func hasInterest(interests []string, wanted string) bool {
	if wanted == "" {
		return true
	}
	for _, interest := range interests {
		if strings.ToLower(strings.TrimSpace(interest)) == wanted {
			return true
		}
	}
	return false
}
