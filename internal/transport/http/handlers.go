package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	domain "hawaii-linkup/internal/domain/models"
	"hawaii-linkup/internal/service/block"
	"hawaii-linkup/internal/service/feed"
	"hawaii-linkup/internal/service/profile"
	"hawaii-linkup/internal/service/reaction"
)

type DiscoveryService interface {
	PageContext(ctx context.Context, requesterID string, query feed.Query) ([]domain.User, error)
}

type ReactionService interface {
	Like(ctx context.Context, actorID, targetID string) (reaction.Result, error)
	Withdraw(ctx context.Context, actorID, targetID string) (reaction.WithdrawResult, error)
}

type BlockService interface {
	Block(ctx context.Context, actorID, targetID string) (block.Result, error)
}

type ProfileService interface {
	Get(ctx context.Context, userID string) (domain.User, error)
	Update(ctx context.Context, actorID, userID string, update profile.Update) (domain.User, error)
}

type PairStorage interface {
	LoadPairContext(ctx context.Context, key domain.PairKey) (domain.Pair, error)
}

type Handler struct {
	discovery      DiscoveryService
	reactions      ReactionService
	blocks         BlockService
	profiles       ProfileService
	pairs          PairStorage
	commandTimeout time.Duration
	logger         *slog.Logger
}

func NewHandler(
	discovery DiscoveryService,
	reactions ReactionService,
	blocks BlockService,
	profiles ProfileService,
	pairs PairStorage,
	commandTimeout time.Duration,
	logger *slog.Logger,
) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	h := &Handler{
		discovery:      discovery,
		reactions:      reactions,
		blocks:         blocks,
		profiles:       profiles,
		pairs:          pairs,
		commandTimeout: commandTimeout,
		logger:         logger,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.Handle("GET /v1/me/discovery", h.withIdentity(http.HandlerFunc(h.discover)))
	mux.Handle("PUT /v1/me/likes/{targetID}", h.withIdentity(http.HandlerFunc(h.like)))
	mux.Handle("DELETE /v1/me/likes/{targetID}", h.withIdentity(http.HandlerFunc(h.withdraw)))
	mux.Handle("PUT /v1/me/blocks/{targetID}", h.withIdentity(http.HandlerFunc(h.block)))
	mux.Handle("GET /v1/profiles/{userID}", h.withIdentity(http.HandlerFunc(h.getProfile)))
	mux.Handle("PUT /v1/profiles/{userID}", h.withIdentity(http.HandlerFunc(h.updateProfile)))
	mux.Handle("GET /v1/me/pairs/{targetID}", h.withIdentity(http.HandlerFunc(h.inspect)))
	return withRequestID(requestLogger(mux, logger))
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) discover(w http.ResponseWriter, r *http.Request) {
	if rejectBody(r) != nil {
		h.writeErrorCode(w, http.StatusBadRequest, "body_not_allowed", "Тело запроса запрещено")
		return
	}
	values, err := parseQuery(r.URL.RawQuery, "interest", "limit", "offset")
	if err != nil {
		h.writeErrorCode(w, http.StatusBadRequest, "invalid_query", "Некорректные query-параметры")
		return
	}
	interest, limit, offset, err := parsePagination(values)
	if err != nil {
		h.writeErrorCode(w, http.StatusBadRequest, "invalid_query", "Некорректные limit или offset")
		return
	}
	users, err := h.discovery.PageContext(r.Context(), actorID(r.Context()), feed.Query{Interest: interest, Limit: limit, Offset: offset})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, newDiscoveryResponse(users, limit, offset))
}

func (h *Handler) like(w http.ResponseWriter, r *http.Request) {
	targetID := r.PathValue("targetID")
	if !validUserID(targetID) {
		h.writeErrorCode(w, http.StatusBadRequest, "invalid_id", "Некорректный targetID")
		return
	}
	if _, err := parseQuery(r.URL.RawQuery); err != nil || len(r.URL.Query()) != 0 {
		h.writeErrorCode(w, http.StatusBadRequest, "invalid_query", "Query-параметры запрещены")
		return
	}
	if rejectBody(r) != nil {
		h.writeErrorCode(w, http.StatusBadRequest, "body_not_allowed", "Тело запроса запрещено")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.commandTimeout)
	defer cancel()
	result, err := h.reactions.Like(ctx, actorID(r.Context()), targetID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, likeResponse{
		Outcome:    string(result.Outcome),
		Changed:    result.Changed,
		Likes:      result.Likes,
		Matched:    result.Matched,
		MatchedNow: result.MatchedNow,
	})
}

func (h *Handler) withdraw(w http.ResponseWriter, r *http.Request) {
	targetID := r.PathValue("targetID")
	if !validUserID(targetID) {
		h.writeErrorCode(w, http.StatusBadRequest, "invalid_id", "Некорректный targetID")
		return
	}
	if _, err := parseQuery(r.URL.RawQuery); err != nil || len(r.URL.Query()) != 0 {
		h.writeErrorCode(w, http.StatusBadRequest, "invalid_query", "Query-параметры запрещены")
		return
	}
	if rejectBody(r) != nil {
		h.writeErrorCode(w, http.StatusBadRequest, "body_not_allowed", "Тело запроса запрещено")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.commandTimeout)
	defer cancel()
	result, err := h.reactions.Withdraw(ctx, actorID(r.Context()), targetID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, withdrawResponse{
		Changed: result.Changed, Likes: result.Likes, Matched: result.Matched, MatchDeactivated: result.MatchDeactivated,
	})
}

func (h *Handler) block(w http.ResponseWriter, r *http.Request) {
	targetID := r.PathValue("targetID")
	if !validUserID(targetID) {
		h.writeErrorCode(w, http.StatusBadRequest, "invalid_id", "Некорректный targetID")
		return
	}
	if _, err := parseQuery(r.URL.RawQuery); err != nil || len(r.URL.Query()) != 0 {
		h.writeErrorCode(w, http.StatusBadRequest, "invalid_query", "Query-параметры запрещены")
		return
	}
	if rejectBody(r) != nil {
		h.writeErrorCode(w, http.StatusBadRequest, "body_not_allowed", "Тело запроса запрещено")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.commandTimeout)
	defer cancel()
	result, err := h.blocks.Block(ctx, actorID(r.Context()), targetID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, blockResponse{
		Changed:          result.Changed,
		Blocked:          result.Blocked,
		MatchDeactivated: result.MatchDeactivated,
	})
}

func (h *Handler) inspect(w http.ResponseWriter, r *http.Request) {
	targetID := r.PathValue("targetID")
	if !validUserID(targetID) {
		h.writeErrorCode(w, http.StatusBadRequest, "invalid_id", "Некорректный targetID")
		return
	}
	if _, err := parseQuery(r.URL.RawQuery); err != nil || len(r.URL.Query()) != 0 {
		h.writeErrorCode(w, http.StatusBadRequest, "invalid_query", "Query-параметры запрещены")
		return
	}
	if rejectBody(r) != nil {
		h.writeErrorCode(w, http.StatusBadRequest, "body_not_allowed", "Тело запроса запрещено")
		return
	}
	actor := actorID(r.Context())
	pair, err := h.pairs.LoadPairContext(r.Context(), domain.NewPairKey(actor, targetID))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pairResponse{
		TargetID:  targetID,
		LikedByMe: pair.HasLiked(actor),
		LikedMe:   pair.HasLiked(targetID),
		Likes:     pair.LikeCount(),
		Matched:   pair.HasActiveMatch(),
	})
}
