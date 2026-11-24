package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"hawaii-linkup/internal/service/profile"
)

const maxProfileBodyBytes = 64 << 10

type profileRequestError struct {
	status  int
	code    string
	message string
}

func (e *profileRequestError) Error() string { return e.message }

func (h *Handler) getProfile(w http.ResponseWriter, r *http.Request) {
	if !h.validProfileRequest(w, r) {
		return
	}
	user, err := h.profiles.Get(r.Context(), r.PathValue("userID"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, newProfileResponse(user))
}

func (h *Handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	if !h.validProfileRequest(w, r) {
		return
	}
	update, err := decodeProfileUpdate(w, r)
	if err != nil {
		h.writeErrorCode(w, err.status, err.code, err.message)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.commandTimeout)
	defer cancel()
	user, serviceErr := h.profiles.Update(ctx, actorID(r.Context()), r.PathValue("userID"), update)
	if serviceErr != nil {
		h.writeError(w, r, serviceErr)
		return
	}
	writeJSON(w, http.StatusOK, newProfileResponse(user))
}

func (h *Handler) validProfileRequest(w http.ResponseWriter, r *http.Request) bool {
	if !validUserID(r.PathValue("userID")) {
		h.writeErrorCode(w, http.StatusBadRequest, "invalid_id", "Некорректный userID")
		return false
	}
	if _, err := parseQuery(r.URL.RawQuery); err != nil || len(r.URL.Query()) != 0 {
		h.writeErrorCode(w, http.StatusBadRequest, "invalid_query", "Query-параметры запрещены")
		return false
	}
	if r.Method == http.MethodGet && rejectBody(r) != nil {
		h.writeErrorCode(w, http.StatusBadRequest, "body_not_allowed", "Тело запроса запрещено")
		return false
	}
	return true
}

func decodeProfileUpdate(w http.ResponseWriter, r *http.Request) (profile.Update, *profileRequestError) {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		return profile.Update{}, &profileRequestError{http.StatusUnsupportedMediaType, "unsupported_media_type", "Требуется Content-Type: application/json"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxProfileBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body profileUpdateRequest
	if err := decoder.Decode(&body); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return profile.Update{}, &profileRequestError{http.StatusRequestEntityTooLarge, "body_too_large", "Тело профиля превышает 64 KiB"}
		}
		return profile.Update{}, &profileRequestError{http.StatusBadRequest, "invalid_json", "Некорректное JSON-тело"}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return profile.Update{}, &profileRequestError{http.StatusBadRequest, "invalid_json", "Разрешён только один JSON-документ"}
	}
	return profile.Update{Name: body.Name, Age: body.Age, City: body.City, Interests: body.Interests, Version: body.Version}, nil
}
