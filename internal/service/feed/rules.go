package feed

import "hawaii-linkup/internal/domain"

// BuildFeed формирует ленту кандидатов для requesterID
func BuildFeed(users []domain.User, requesterID string, excluded []string, limit int) []domain.User {
	skip := make(map[string]bool, len(excluded))
	for _, id := range excluded {
		skip[id] = true
	}

	result := []domain.User{}
	for _, u := range users {
		if len(result) >= limit {
			break
		}
		if u.ID == requesterID || !u.Active || skip[u.ID] {
			continue
		}
		result = append(result, u)
	}
	return result
}
