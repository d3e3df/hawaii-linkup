package httpapi

import "hawaii-linkup/internal/domain"

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type userResponse struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Age       int      `json:"age"`
	City      string   `json:"city"`
	Interests []string `json:"interests"`
}

type discoveryResponse struct {
	Items  []userResponse `json:"items"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

type likeResponse struct {
	Outcome    string `json:"outcome"`
	Changed    bool   `json:"changed"`
	Likes      int    `json:"likes"`
	Matched    bool   `json:"matched"`
	MatchedNow bool   `json:"matched_now"`
}

type pairResponse struct {
	TargetID  string `json:"target_id"`
	LikedByMe bool   `json:"liked_by_me"`
	LikedMe   bool   `json:"liked_me"`
	Likes     int    `json:"likes"`
	Matched   bool   `json:"matched"`
}

func newDiscoveryResponse(users []domain.User, limit, offset int) discoveryResponse {
	items := make([]userResponse, 0, len(users))
	for _, user := range users {
		items = append(items, userResponse{
			ID:        user.ID,
			Name:      user.Name,
			Age:       user.Age,
			City:      user.City,
			Interests: user.Interests,
		})
	}
	return discoveryResponse{Items: items, Limit: limit, Offset: offset}
}
