package httpapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
)

var userIDPattern = regexp.MustCompile(`^u[1-9][0-9]*$`)

func validUserID(id string) bool {
	return userIDPattern.MatchString(id)
}

func rejectBody(r *http.Request) error {
	if r.Body == nil || r.Body == http.NoBody {
		return nil
	}
	read, err := io.Copy(io.Discard, io.LimitReader(r.Body, 1))
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}
	if read > 0 {
		return errors.New("request body is not allowed")
	}
	return nil
}

func parseQuery(raw string, allowed ...string) (url.Values, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		known[key] = true
	}
	for key, entries := range values {
		if !known[key] || len(entries) != 1 {
			return nil, fmt.Errorf("unknown or repeated query parameter %q", key)
		}
	}
	return values, nil
}

func parsePagination(values url.Values) (interest string, limit, offset int, err error) {
	limit, offset = 20, 0
	interest = values.Get("interest")
	if raw := values.Get("limit"); raw != "" || values.Has("limit") {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 50 {
			return "", 0, 0, errors.New("limit must be from 1 to 50")
		}
	}
	if raw := values.Get("offset"); raw != "" || values.Has("offset") {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return "", 0, 0, errors.New("offset must be non-negative")
		}
	}
	return interest, limit, offset, nil
}
