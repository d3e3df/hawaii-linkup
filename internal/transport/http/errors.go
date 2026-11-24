package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"hawaii-linkup/internal/adapter/profilehttp"
	domainerrors "hawaii-linkup/internal/domain/errors"
	"hawaii-linkup/internal/service/eligibility"
	"hawaii-linkup/internal/service/feed"
	"hawaii-linkup/internal/service/reaction"
)

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (h *Handler) writeErrorCode(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Code: code, Message: message})
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := classifyError(err)
	if status >= http.StatusInternalServerError {
		h.logger.Error("request failed", "request_id", requestID(r.Context()), "status", status, "error", err)
	}
	h.writeErrorCode(w, status, code, message)
}

func classifyError(err error) (status int, code, message string) {
	var notAllowed *reaction.NotAllowedError
	switch {
	case errors.As(err, &notAllowed):
		switch notAllowed.Reason {
		case eligibility.ReasonSelf:
			return http.StatusConflict, "self_like", "Нельзя лайкнуть самого себя"
		case eligibility.ReasonInactive:
			return http.StatusConflict, "inactive_profile", "Профиль неактивен"
		case eligibility.ReasonBlocked:
			return http.StatusConflict, "blocked", "Взаимодействие заблокировано"
		default:
			return http.StatusConflict, "like_not_allowed", "Действие запрещено текущим состоянием"
		}
	case errors.Is(err, profilehttp.ErrNotFound), errors.Is(err, domainerrors.ErrTargetNotFound):
		return http.StatusNotFound, "target_not_found", "Целевой профиль не найден"
	case errors.Is(err, domainerrors.ErrNotFound):
		return http.StatusNotFound, "actor_not_found", "Текущий пользователь не найден"
	case errors.Is(err, profilehttp.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout, "profile_timeout", "Profile API не ответил вовремя"
	case errors.Is(err, profilehttp.ErrResponseTooLarge):
		return http.StatusBadGateway, "profile_response_too_large", "Ответ Profile API слишком большой"
	case errors.Is(err, profilehttp.ErrInvalidResponse):
		return http.StatusBadGateway, "profile_invalid_response", "Profile API вернул неверный ответ"
	case errors.Is(err, profilehttp.ErrUnavailable):
		return http.StatusBadGateway, "profile_unavailable", "Profile API недоступен"
	case errors.Is(err, feed.ErrInvalidRequest), errors.Is(err, reaction.ErrInvalidRequest):
		return http.StatusBadRequest, "invalid_request", "Некорректный запрос"
	case errors.Is(err, domainerrors.ErrDatabaseTimeout):
		return http.StatusServiceUnavailable, "database_timeout", "Превышен бюджет операции с базой данных"
	case errors.Is(err, domainerrors.ErrTransactionRetryable):
		return http.StatusServiceUnavailable, "transaction_retryable", "Операцию можно повторить позже"
	case errors.Is(err, domainerrors.ErrCommitUnconfirmed):
		return http.StatusServiceUnavailable, "commit_unconfirmed", "Результат операции неизвестен; проверьте состояние"
	default:
		return http.StatusInternalServerError, "internal_error", "Внутренняя ошибка сервера"
	}
}
