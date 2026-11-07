package profilehttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"hawaii-linkup/internal/domain"
)

var (
	ErrNotFound         = errors.New("profile not found")
	ErrUnavailable      = errors.New("profile service unavailable")
	ErrTimeout          = errors.New("profile service timeout")
	ErrInvalidResponse  = errors.New("invalid profile service response")
	ErrResponseTooLarge = errors.New("profile service response too large")
)

type Config struct {
	BaseURL               string
	ClientTimeout         time.Duration
	DialTimeout           time.Duration
	ResponseHeaderTimeout time.Duration
	MaxResponseBytes      int64
}

type Client struct {
	baseURL          string
	httpClient       *http.Client
	maxResponseBytes int64
}

func NewClient(config Config) (*Client, error) {
	parsed, err := url.Parse(config.BaseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("profile client: invalid base URL %q", config.BaseURL)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("profile client: base URL must not contain query or fragment")
	}
	if config.ClientTimeout <= 0 || config.DialTimeout <= 0 || config.ResponseHeaderTimeout <= 0 {
		return nil, errors.New("profile client: timeouts must be positive")
	}
	if config.MaxResponseBytes <= 0 {
		return nil, errors.New("profile client: response limit must be positive")
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		IdleConnTimeout:       60 * time.Second,
	}
	return &Client{
		baseURL: strings.TrimRight(config.BaseURL, "/"),
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   config.ClientTimeout,
		},
		maxResponseBytes: config.MaxResponseBytes,
	}, nil
}

func (c *Client) Get(ctx context.Context, id string) (domain.User, error) {
	if id == "" {
		return domain.User{}, errors.New("profile client: empty id")
	}
	endpoint := c.baseURL + "/v1/profiles/" + url.PathEscape(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return domain.User{}, fmt.Errorf("profile client: create request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return domain.User{}, fmt.Errorf("profile client: %w: %w", ErrTimeout, err)
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return domain.User{}, fmt.Errorf("profile client: %w: %w", ErrTimeout, err)
		}
		return domain.User{}, fmt.Errorf("profile client: %w: request: %w", ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return domain.User{}, fmt.Errorf("profile client: %w: %s", ErrNotFound, id)
	}
	if resp.StatusCode != http.StatusOK {
		return domain.User{}, fmt.Errorf("profile client: %w: status %d", ErrUnavailable, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, c.maxResponseBytes+1))
	if err != nil {
		return domain.User{}, fmt.Errorf("profile client: %w: read body: %w", ErrUnavailable, err)
	}
	if int64(len(body)) > c.maxResponseBytes {
		return domain.User{}, fmt.Errorf("profile client: %w: limit %d", ErrResponseTooLarge, c.maxResponseBytes)
	}
	profile, err := decodeProfile(body)
	if err != nil {
		return domain.User{}, fmt.Errorf("profile client: %w: %w", ErrInvalidResponse, err)
	}
	if profile.ID != id {
		return domain.User{}, fmt.Errorf("profile client: %w: returned id %q, requested %q", ErrInvalidResponse, profile.ID, id)
	}
	return profile, nil
}

type profileDTO struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Age       int      `json:"age"`
	City      string   `json:"city"`
	Interests []string `json:"interests"`
	Active    *bool    `json:"active"`
}

func decodeProfile(body []byte) (domain.User, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var dto profileDTO
	if err := decoder.Decode(&dto); err != nil {
		return domain.User{}, err
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return domain.User{}, err
	}
	if dto.ID == "" || dto.Active == nil {
		return domain.User{}, errors.New("required fields id and active")
	}
	return domain.User{
		ID:        dto.ID,
		Name:      dto.Name,
		Age:       dto.Age,
		City:      dto.City,
		Interests: dto.Interests,
		Active:    *dto.Active,
	}, nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}
