package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hawaii-linkup/internal/adapter/profilehttp"
	domainerrors "hawaii-linkup/internal/domain/errors"
	domain "hawaii-linkup/internal/domain/models"
	"hawaii-linkup/internal/profileapi"
	"hawaii-linkup/internal/repository/memory"
	"hawaii-linkup/internal/service/block"
	"hawaii-linkup/internal/service/eligibility"
	"hawaii-linkup/internal/service/feed"
	"hawaii-linkup/internal/service/profile"
	"hawaii-linkup/internal/service/reaction"
)

type testAPI struct {
	handler    http.Handler
	store      *memory.Store
	profileURL string
}

func newTestAPI(t *testing.T) testAPI {
	t.Helper()
	store := memory.NewDemoStore()
	profiles, err := store.ListUsers()
	if err != nil {
		t.Fatal(err)
	}
	dependency, err := profileapi.NewServer(profiles, profileapi.ModeOK, 250*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	profileServer := httptest.NewServer(dependency.Handler())
	t.Cleanup(profileServer.Close)
	client, err := profilehttp.NewClient(profilehttp.Config{
		BaseURL:               profileServer.URL,
		ClientTimeout:         time.Second,
		DialTimeout:           time.Second,
		ResponseHeaderTimeout: time.Second,
		MaxResponseBytes:      1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(feed.NewService(store), reaction.NewService(store, client), block.NewService(store), profile.NewService(store), store, 75*time.Millisecond, logger)
	return testAPI{handler: handler, store: store, profileURL: profileServer.URL}
}

func request(t *testing.T, handler http.Handler, method, path, actor string, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	if actor != "" {
		req.Header.Set("X-Demo-User", actor)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func jsonRequest(t *testing.T, handler http.Handler, path, actor, body, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("X-Demo-User", actor)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func decodeResponse[T any](t *testing.T, resp *httptest.ResponseRecorder) T {
	t.Helper()
	var value T
	if err := json.NewDecoder(resp.Body).Decode(&value); err != nil {
		t.Fatalf("decode response %q: %v", resp.Body.String(), err)
	}
	return value
}

//nolint:gocognit,gocyclo // Сквозной сценарий намеренно сохраняет порядок пользовательских шагов.
func TestBusinessFlowPagesLikeRepeatInspectAndMatch(t *testing.T) {
	api := newTestAPI(t)

	first := request(t, api.handler, http.MethodGet, "/v1/me/discovery?interest=travel&limit=2&offset=0", "u3", "")
	if first.Code != http.StatusOK {
		t.Fatalf("first page: status=%d body=%s", first.Code, first.Body.String())
	}
	firstPage := decodeResponse[discoveryResponse](t, first)
	if len(firstPage.Items) != 2 || firstPage.Items[0].ID != "u1" || firstPage.Items[1].ID != "u6" {
		t.Fatalf("first page=%+v", firstPage)
	}
	second := request(t, api.handler, http.MethodGet, "/v1/me/discovery?interest=travel&limit=2&offset=2", "u3", "")
	secondPage := decodeResponse[discoveryResponse](t, second)
	if second.Code != http.StatusOK || len(secondPage.Items) != 2 || secondPage.Items[0].ID != "u7" || secondPage.Items[1].ID != "u8" {
		t.Fatalf("second page=%+v, status=%d", secondPage, second.Code)
	}

	initial := request(t, api.handler, http.MethodGet, "/v1/me/pairs/u1", "u3", "")
	initialPair := decodeResponse[pairResponse](t, initial)
	if initial.Code != http.StatusOK || initialPair.Matched || initialPair.Likes != 0 {
		t.Fatalf("initial=%+v, status=%d", initialPair, initial.Code)
	}

	like := request(t, api.handler, http.MethodPut, "/v1/me/likes/u1", "u3", "")
	liked := decodeResponse[likeResponse](t, like)
	if like.Code != http.StatusOK || !liked.Changed || liked.Likes != 1 || liked.Matched || liked.MatchedNow {
		t.Fatalf("like=%+v, status=%d", liked, like.Code)
	}
	repeat := request(t, api.handler, http.MethodPut, "/v1/me/likes/u1", "u3", "")
	repeated := decodeResponse[likeResponse](t, repeat)
	if repeat.Code != http.StatusOK || repeated.Changed || repeated.Likes != 1 {
		t.Fatalf("repeat=%+v, status=%d", repeated, repeat.Code)
	}
	inspect := request(t, api.handler, http.MethodGet, "/v1/me/pairs/u1", "u3", "")
	pair := decodeResponse[pairResponse](t, inspect)
	if !pair.LikedByMe || pair.LikedMe || pair.Likes != 1 || pair.Matched {
		t.Fatalf("inspect=%+v", pair)
	}

	after := request(t, api.handler, http.MethodGet, "/v1/me/discovery?interest=travel&limit=2&offset=0", "u3", "")
	afterPage := decodeResponse[discoveryResponse](t, after)
	if len(afterPage.Items) != 2 || afterPage.Items[0].ID != "u6" || afterPage.Items[1].ID != "u7" {
		t.Fatalf("page after like=%+v", afterPage)
	}
	response := request(t, api.handler, http.MethodPut, "/v1/me/likes/u3", "u1", "")
	matched := decodeResponse[likeResponse](t, response)
	if response.Code != http.StatusOK || matched.Outcome != "match" || !matched.Matched || !matched.MatchedNow || matched.Likes != 2 {
		t.Fatalf("match=%+v, status=%d", matched, response.Code)
	}
}

func TestDiscoveryEmptyPageIsJSONarray(t *testing.T) {
	api := newTestAPI(t)
	resp := request(t, api.handler, http.MethodGet, "/v1/me/discovery?interest=none&limit=2&offset=0", "u3", "")
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"items":[]`) {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type=%q", got)
	}
	if resp.Header().Get("X-Request-ID") == "" {
		t.Fatal("request ID is empty")
	}
}

func TestProfileVersionFlow(t *testing.T) {
	api := newTestAPI(t)
	initialResp := request(t, api.handler, http.MethodGet, "/v1/profiles/u2", "u1", "")
	initial := decodeResponse[profileResponse](t, initialResp)
	if initialResp.Code != http.StatusOK || initial.Version != 1 || initial.ID != "u2" {
		t.Fatalf("initial=%+v status=%d", initial, initialResp.Code)
	}
	body := `{"name":"Борис Обновлённый","age":29,"city":"Москва","interests":["go"],"version":1}`
	updatedResp := jsonRequest(t, api.handler, "/v1/profiles/u2", "u2", body, "application/json; charset=utf-8")
	updated := decodeResponse[profileResponse](t, updatedResp)
	if updatedResp.Code != http.StatusOK || updated.Version != 2 || updated.Name != "Борис Обновлённый" {
		t.Fatalf("updated=%+v status=%d body=%s", updated, updatedResp.Code, updatedResp.Body.String())
	}
	staleResp := jsonRequest(t, api.handler, "/v1/profiles/u2", "u2", body, "application/json")
	if staleResp.Code != http.StatusConflict || decodeResponse[errorResponse](t, staleResp).Code != "version_conflict" {
		t.Fatalf("stale status=%d body=%s", staleResp.Code, staleResp.Body.String())
	}
	persistedResp := request(t, api.handler, http.MethodGet, "/v1/profiles/u2", "u1", "")
	persisted := decodeResponse[profileResponse](t, persistedResp)
	if persisted.Name != updated.Name || persisted.Version != 2 {
		t.Fatalf("persisted=%+v", persisted)
	}
	forbidden := jsonRequest(t, api.handler, "/v1/profiles/u2", "u1", `{"name":"Чужая правка","age":30,"city":"Москва","interests":[],"version":2}`, "application/json")
	if forbidden.Code != http.StatusForbidden || decodeResponse[errorResponse](t, forbidden).Code != "profile_forbidden" {
		t.Fatalf("forbidden status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}
	missing := request(t, api.handler, http.MethodGet, "/v1/profiles/u99", "u1", "")
	if missing.Code != http.StatusNotFound || decodeResponse[errorResponse](t, missing).Code != "profile_not_found" {
		t.Fatalf("missing status=%d body=%s", missing.Code, missing.Body.String())
	}
}

func TestProfileRejectsInvalidJSONAndMedia(t *testing.T) {
	api := newTestAPI(t)
	tests := []struct {
		name        string
		body        string
		contentType string
		wantStatus  int
		wantCode    string
	}{
		{"syntax", `{`, "application/json", 400, "invalid_json"},
		{"unknown field", `{"name":"Имя","age":25,"city":"Москва","interests":[],"version":1,"admin":true}`, "application/json", 400, "invalid_json"},
		{"second document", `{"name":"Имя","age":25,"city":"Москва","interests":[],"version":1}{}`, "application/json", 400, "invalid_json"},
		{"missing interests", `{"name":"Имя","age":25,"city":"Москва","version":1}`, "application/json", 400, "invalid_request"},
		{"old zero version", `{"name":"Имя","age":25,"city":"Москва","interests":[],"version":0}`, "application/json", 400, "invalid_request"},
		{"media type", `{}`, "text/plain", 415, "unsupported_media_type"},
		{"too large", strings.Repeat(" ", maxProfileBodyBytes+1), "application/json", 413, "body_too_large"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp := jsonRequest(t, api.handler, "/v1/profiles/u2", "u2", test.body, test.contentType)
			body := decodeResponse[errorResponse](t, resp)
			if resp.Code != test.wantStatus || body.Code != test.wantCode {
				t.Fatalf("status=%d body=%+v", resp.Code, body)
			}
		})
	}
}

func TestWithdrawDeletesOwnReactionAndAllowsNewLike(t *testing.T) {
	api := newTestAPI(t)
	for _, like := range []struct {
		actor  string
		target string
	}{{"u2", "u4"}, {"u4", "u2"}} {
		resp := request(t, api.handler, http.MethodPut, "/v1/me/likes/"+like.target, like.actor, "")
		if resp.Code != http.StatusOK {
			t.Fatalf("like: status=%d body=%s", resp.Code, resp.Body.String())
		}
	}
	first := request(t, api.handler, http.MethodDelete, "/v1/me/likes/u4", "u2", "")
	assertWithdrawResponse(t, first, withdrawResponse{Changed: true, Likes: 1, MatchDeactivated: true})
	pairResp := request(t, api.handler, http.MethodGet, "/v1/me/pairs/u4", "u2", "")
	pair := decodeResponse[pairResponse](t, pairResp)
	if pair.LikedByMe || !pair.LikedMe || pair.Likes != 1 || pair.Matched {
		t.Fatalf("pair=%+v", pair)
	}
	repeat := request(t, api.handler, http.MethodDelete, "/v1/me/likes/u4", "u2", "")
	assertWithdrawResponse(t, repeat, withdrawResponse{Likes: 1})
	likedAgain := request(t, api.handler, http.MethodPut, "/v1/me/likes/u4", "u2", "")
	liked := decodeResponse[likeResponse](t, likedAgain)
	if likedAgain.Code != http.StatusOK || !liked.Matched || !liked.MatchedNow {
		t.Fatalf("like again=%+v status=%d", liked, likedAgain.Code)
	}
}

func assertWithdrawResponse(t *testing.T, resp *httptest.ResponseRecorder, want withdrawResponse) {
	t.Helper()
	got := decodeResponse[withdrawResponse](t, resp)
	if resp.Code != http.StatusOK || got != want {
		t.Fatalf("response=%+v status=%d, want %+v", got, resp.Code, want)
	}
}

func TestBlockDeactivatesMatchAndIsIdempotent(t *testing.T) {
	api := newTestAPI(t)
	for _, like := range []struct {
		actor  string
		target string
	}{{"u2", "u4"}, {"u4", "u2"}} {
		resp := request(t, api.handler, http.MethodPut, "/v1/me/likes/"+like.target, like.actor, "")
		if resp.Code != http.StatusOK {
			t.Fatalf("like %s -> %s: status=%d body=%s", like.actor, like.target, resp.Code, resp.Body.String())
		}
	}
	first := request(t, api.handler, http.MethodPut, "/v1/me/blocks/u4", "u2", "")
	blocked := decodeResponse[blockResponse](t, first)
	if first.Code != http.StatusOK || !blocked.Changed || !blocked.Blocked || !blocked.MatchDeactivated {
		t.Fatalf("first block=%+v status=%d", blocked, first.Code)
	}
	repeat := request(t, api.handler, http.MethodPut, "/v1/me/blocks/u4", "u2", "")
	repeated := decodeResponse[blockResponse](t, repeat)
	if repeat.Code != http.StatusOK || repeated.Changed || !repeated.Blocked || repeated.MatchDeactivated {
		t.Fatalf("repeat block=%+v status=%d", repeated, repeat.Code)
	}
	pairResp := request(t, api.handler, http.MethodGet, "/v1/me/pairs/u4", "u2", "")
	pair := decodeResponse[pairResponse](t, pairResp)
	if pair.Likes != 2 || pair.Matched {
		t.Fatalf("pair after block=%+v", pair)
	}
	like := request(t, api.handler, http.MethodPut, "/v1/me/likes/u4", "u2", "")
	if like.Code != http.StatusConflict || decodeResponse[errorResponse](t, like).Code != "blocked" {
		t.Fatalf("like after block: status=%d body=%s", like.Code, like.Body.String())
	}
}

func TestBlockValidationAndNotFound(t *testing.T) {
	api := newTestAPI(t)
	tests := []struct {
		target     string
		wantStatus int
		wantCode   string
	}{
		{"u2", http.StatusConflict, "self_block"},
		{"u5", http.StatusConflict, "inactive_profile"},
		{"u99", http.StatusNotFound, "target_not_found"},
	}
	for _, test := range tests {
		resp := request(t, api.handler, http.MethodPut, "/v1/me/blocks/"+test.target, "u2", "")
		body := decodeResponse[errorResponse](t, resp)
		if resp.Code != test.wantStatus || body.Code != test.wantCode {
			t.Fatalf("target=%s status=%d body=%+v", test.target, resp.Code, body)
		}
	}
}

func TestDependencyFailuresLeaveFreshPairsEmpty(t *testing.T) {
	api := newTestAPI(t)
	tests := []struct {
		mode       profileapi.Mode
		target     string
		wantStatus int
		wantCode   string
	}{
		{profileapi.ModeMissing, "u2", http.StatusNotFound, "target_not_found"},
		{profileapi.ModeError, "u4", http.StatusBadGateway, "profile_unavailable"},
		{profileapi.ModeSlow, "u6", http.StatusGatewayTimeout, "profile_timeout"},
		{profileapi.ModeLarge, "u7", http.StatusBadGateway, "profile_response_too_large"},
	}
	for _, tt := range tests {
		setProfileMode(t, api.profileURL, tt.mode)
		resp := request(t, api.handler, http.MethodPut, "/v1/me/likes/"+tt.target, "u3", "")
		errorBody := decodeResponse[errorResponse](t, resp)
		if resp.Code != tt.wantStatus || errorBody.Code != tt.wantCode {
			t.Fatalf("mode %s: status=%d body=%+v", tt.mode, resp.Code, errorBody)
		}
		pair, err := api.store.LoadPair(domain.NewPairKey("u3", tt.target))
		if err != nil || pair.LikeCount() != 0 {
			t.Fatalf("mode %s wrote pair %+v, err=%v", tt.mode, pair, err)
		}
	}
	setProfileMode(t, api.profileURL, profileapi.ModeOK)
	resp := request(t, api.handler, http.MethodPut, "/v1/me/likes/u8", "u3", "")
	if resp.Code != http.StatusOK {
		t.Fatalf("recovery: status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func setProfileMode(t *testing.T, baseURL string, mode profileapi.Mode) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, baseURL+"/_admin/mode/"+string(mode), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set mode: status %d", resp.StatusCode)
	}
}

type discoverySpy struct{ calls int }

func (s *discoverySpy) PageContext(context.Context, string, feed.Query) ([]domain.User, error) {
	s.calls++
	return nil, nil
}

type reactionSpy struct {
	calls  int
	result reaction.Result
	err    error
}

func (s *reactionSpy) Like(context.Context, string, string) (reaction.Result, error) {
	s.calls++
	return s.result, s.err
}

func (s *reactionSpy) Withdraw(context.Context, string, string) (reaction.WithdrawResult, error) {
	s.calls++
	return reaction.WithdrawResult{}, s.err
}

type blockSpy struct{ calls int }

func (s *blockSpy) Block(context.Context, string, string) (block.Result, error) {
	s.calls++
	return block.Result{}, nil
}

type profileSpy struct{ calls int }

func (s *profileSpy) Get(context.Context, string) (domain.User, error) {
	s.calls++
	return domain.User{}, nil
}

func (s *profileSpy) Update(context.Context, string, string, profile.Update) (domain.User, error) {
	s.calls++
	return domain.User{}, nil
}

type pairSpy struct{ calls int }

func (s *pairSpy) LoadPairContext(context.Context, domain.PairKey) (domain.Pair, error) {
	s.calls++
	return domain.Pair{}, nil
}

func TestInvalidInputDoesNotCallBusinessServices(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		actor  string
		body   string
	}{
		{"missing identity", http.MethodGet, "/v1/me/discovery", "", ""},
		{"invalid identity", http.MethodGet, "/v1/me/discovery", "bad", ""},
		{"bad limit", http.MethodGet, "/v1/me/discovery?limit=0", "u1", ""},
		{"bad offset", http.MethodGet, "/v1/me/discovery?offset=-1", "u1", ""},
		{"unknown query", http.MethodGet, "/v1/me/discovery?role=admin", "u1", ""},
		{"repeated query", http.MethodGet, "/v1/me/discovery?limit=1&limit=2", "u1", ""},
		{"bad query encoding", http.MethodGet, "/v1/me/discovery?interest=%zz", "u1", ""},
		{"discovery body", http.MethodGet, "/v1/me/discovery", "u1", `{}`},
		{"like id", http.MethodPut, "/v1/me/likes/bad", "u1", ""},
		{"like query", http.MethodPut, "/v1/me/likes/u2?actor=u2", "u1", ""},
		{"like body", http.MethodPut, "/v1/me/likes/u2", "u1", `{}`},
		{"withdraw id", http.MethodDelete, "/v1/me/likes/bad", "u1", ""},
		{"withdraw query", http.MethodDelete, "/v1/me/likes/u2?actor=u2", "u1", ""},
		{"withdraw body", http.MethodDelete, "/v1/me/likes/u2", "u1", `{}`},
		{"block id", http.MethodPut, "/v1/me/blocks/bad", "u1", ""},
		{"block query", http.MethodPut, "/v1/me/blocks/u2?actor=u2", "u1", ""},
		{"block body", http.MethodPut, "/v1/me/blocks/u2", "u1", `{}`},
		{"profile id", http.MethodGet, "/v1/profiles/u01", "u1", ""},
		{"profile query", http.MethodGet, "/v1/profiles/u2?x=1", "u1", ""},
		{"profile body", http.MethodGet, "/v1/profiles/u2", "u1", `{}`},
		{"inspect id", http.MethodGet, "/v1/me/pairs/u01", "u1", ""},
		{"inspect query", http.MethodGet, "/v1/me/pairs/u2?x=1", "u1", ""},
		{"inspect body", http.MethodGet, "/v1/me/pairs/u2", "u1", `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discovery := &discoverySpy{}
			reactions := &reactionSpy{}
			blocks := &blockSpy{}
			profiles := &profileSpy{}
			pairs := &pairSpy{}
			handler := NewHandler(discovery, reactions, blocks, profiles, pairs, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
			resp := request(t, handler, tt.method, tt.path, tt.actor, tt.body)
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
			}
			if discovery.calls != 0 || reactions.calls != 0 || blocks.calls != 0 || profiles.calls != 0 || pairs.calls != 0 {
				t.Fatalf("business calls: discovery=%d reaction=%d blocks=%d profiles=%d pairs=%d", discovery.calls, reactions.calls, blocks.calls, profiles.calls, pairs.calls)
			}
		})
	}
}

func TestWrappedErrorsKeepStatusAndCode(t *testing.T) {
	tests := []struct {
		err        error
		wantStatus int
		wantCode   string
	}{
		{fmt.Errorf("outer: %w", profilehttp.ErrNotFound), 404, "target_not_found"},
		{fmt.Errorf("outer: %w", profilehttp.ErrUnavailable), 502, "profile_unavailable"},
		{fmt.Errorf("outer: %w", profilehttp.ErrTimeout), 504, "profile_timeout"},
		{fmt.Errorf("outer: %w", profilehttp.ErrInvalidResponse), 502, "profile_invalid_response"},
		{fmt.Errorf("outer: %w", profilehttp.ErrResponseTooLarge), 502, "profile_response_too_large"},
		{fmt.Errorf("outer: %w", domainerrors.ErrNotFound), 404, "actor_not_found"},
		{fmt.Errorf("outer: %w", domainerrors.ErrTargetNotFound), 404, "target_not_found"},
		{fmt.Errorf("outer: %w", domainerrors.ErrDatabaseTimeout), 503, "database_timeout"},
		{fmt.Errorf("outer: %w", domainerrors.ErrTransactionRetryable), 503, "transaction_retryable"},
		{fmt.Errorf("outer: %w", domainerrors.ErrCommitUnconfirmed), 503, "commit_unconfirmed"},
		{fmt.Errorf("outer: %w", profile.ErrNotFound), 404, "profile_not_found"},
		{fmt.Errorf("outer: %w", profile.ErrForbidden), 403, "profile_forbidden"},
		{fmt.Errorf("outer: %w", domainerrors.ErrVersionConflict), 409, "version_conflict"},
		{fmt.Errorf("outer: %w", &reaction.NotAllowedError{Reason: eligibility.ReasonSelf}), 409, "self_like"},
		{fmt.Errorf("outer: %w", &reaction.NotAllowedError{Reason: eligibility.ReasonBlocked}), 409, "blocked"},
		{fmt.Errorf("outer: %w", &reaction.NotAllowedError{Reason: eligibility.ReasonInactive}), 409, "inactive_profile"},
		{fmt.Errorf("outer: %w", errors.New("storage failed")), 500, "internal_error"},
	}
	for _, tt := range tests {
		status, code, _ := classifyError(tt.err)
		if status != tt.wantStatus || code != tt.wantCode {
			t.Fatalf("error %v: status=%d code=%s", tt.err, status, code)
		}
	}
}

func TestLikeBusinessRulesReturnConflict(t *testing.T) {
	api := newTestAPI(t)
	for _, target := range []string{"u3", "u5"} {
		resp := request(t, api.handler, http.MethodPut, "/v1/me/likes/"+target, "u3", "")
		if resp.Code != http.StatusConflict {
			t.Fatalf("target %s: status=%d body=%s", target, resp.Code, resp.Body.String())
		}
	}
}

func TestHealthAndUnknownRoute(t *testing.T) {
	api := newTestAPI(t)
	health := request(t, api.handler, http.MethodGet, "/healthz", "", "")
	if health.Code != http.StatusOK || !bytes.Contains(health.Body.Bytes(), []byte(`"status":"ok"`)) {
		t.Fatalf("health=%d %s", health.Code, health.Body.String())
	}
	unknown := request(t, api.handler, http.MethodGet, "/unknown", "", "")
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown status=%d", unknown.Code)
	}
}
