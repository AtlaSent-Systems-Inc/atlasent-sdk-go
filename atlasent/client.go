package atlasent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// Version is the SDK version reported in the User-Agent header. Set by
// goreleaser via -ldflags at build time; "dev" in source trees.
var Version = "dev"

// DefaultBaseURL is the production AtlaSent API endpoint.
const DefaultBaseURL = "https://api.atlasent.io"

// Client is the AtlaSent API client. It is safe for concurrent use.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	userAgent  string

	// FailClosed controls the fallback when the PDP is unreachable. When
	// true (the default), transport failures yield a denying Decision; when
	// false, they yield an allowing one. Guard always blocks fn on denied
	// decisions regardless of this flag.
	FailClosed bool

	cache           Cache
	cacheDefaultTTL time.Duration
	observer        Observer
	retry           RetryPolicy
	rng             *rand.Rand
}

// Option configures a Client at construction. Options are applied in order.
type Option func(*Client)

// WithBaseURL overrides the API base URL. Trailing slashes are trimmed.
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient installs a custom *http.Client. Use it to plug in a custom
// transport (mTLS, tracing wrappers, etc.).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithTimeout sets the per-request timeout on the default HTTP client. No-op
// if WithHTTPClient has been used.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if c.httpClient != nil {
			c.httpClient.Timeout = d
		}
	}
}

// WithUserAgent overrides the default User-Agent of the form
// "atlasent-sdk-go/<version> <go-version>".
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// WithFailOpen flips the Client from fail-closed (default) to fail-open.
// Use only in paths where availability outranks correctness.
func WithFailOpen() Option {
	return func(c *Client) { c.FailClosed = false }
}

// New constructs a Client. apiKey is required; it is sent as
// "Authorization: Bearer <key>" on every request.
func New(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("atlasent: apiKey is required")
	}
	c := &Client{
		apiKey:          apiKey,
		baseURL:         DefaultBaseURL,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		userAgent:       defaultUserAgent(),
		FailClosed:      true,
		cacheDefaultTTL: 5 * time.Minute,
		rng:             rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

func defaultUserAgent() string {
	gv := "unknown"
	if bi, ok := debug.ReadBuildInfo(); ok {
		gv = bi.GoVersion
	}
	return fmt.Sprintf("atlasent-sdk-go/%s %s", Version, gv)
}

// postJSON sends body as JSON to path and unmarshals the success response
// into out. Retries retryable failures per the configured RetryPolicy.
// attempts is the total HTTP attempts (1 on first-try success).
func (c *Client) postJSON(ctx context.Context, path string, body, out any) (attempts int, err error) {
	return c.doJSON(ctx, http.MethodPost, path, body, out)
}

func (c *Client) getJSON(ctx context.Context, path string, out any) (attempts int, err error) {
	return c.doJSON(ctx, http.MethodGet, path, nil, out)
}

func (c *Client) doJSON(ctx context.Context, method, path string, body, out any) (int, error) {
	maxAttempts := c.retry.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("atlasent: marshal: %w", err)
		}
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
		if err != nil {
			return attempt, fmt.Errorf("atlasent: new request: %w", err)
		}
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("User-Agent", c.userAgent)

		res, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("atlasent: do request: %w", err)
			if attempt == maxAttempts {
				return attempt, lastErr
			}
			if sleepErr := sleepCtx(ctx, c.retry.backoffFor(attempt, c.rng)); sleepErr != nil {
				return attempt, sleepErr
			}
			continue
		}

		b, readErr := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("atlasent: read response: %w", readErr)
			if attempt == maxAttempts || !retryableStatus(res.StatusCode) {
				return attempt, lastErr
			}
			if sleepErr := sleepCtx(ctx, c.retry.backoffFor(attempt, c.rng)); sleepErr != nil {
				return attempt, sleepErr
			}
			continue
		}

		if res.StatusCode >= 400 {
			apiErr := decodeAPIError(res.StatusCode, res.Header, b)
			lastErr = apiErr
			if attempt == maxAttempts || !retryableStatus(res.StatusCode) {
				return attempt, lastErr
			}
			wait := parseRetryAfter(res.Header)
			if wait == 0 {
				wait = c.retry.backoffFor(attempt, c.rng)
			}
			if sleepErr := sleepCtx(ctx, wait); sleepErr != nil {
				return attempt, sleepErr
			}
			continue
		}

		if out != nil && len(b) > 0 {
			if err := json.Unmarshal(b, out); err != nil {
				return attempt, fmt.Errorf("atlasent: unmarshal: %w", err)
			}
		}
		return attempt, nil
	}
	return maxAttempts, lastErr
}

func decodeAPIError(status int, h http.Header, body []byte) *APIError {
	apiErr := &APIError{Status: status, RetryAfter: parseRetryAfter(h)}
	if len(body) > 0 {
		_ = json.Unmarshal(body, apiErr)
	}
	if apiErr.Code == "" {
		apiErr.Code = coarseCodeFor(status)
	}
	if apiErr.Message == "" {
		apiErr.Message = http.StatusText(status)
	}
	return apiErr
}

// coarseCodeFor maps an HTTP status to the cross-SDK coarse error kind. See
// contract/vectors/errors.json for the canonical mapping.
func coarseCodeFor(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "invalid_api_key"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return "bad_request"
	}
	if status >= 500 {
		return "server_error"
	}
	return "http_error"
}

// Evaluate calls POST /v1/evaluate with the OpenAPI-shaped payload and
// returns an EvaluationResult. For idiomatic Go authorization, prefer
// Client.Check + Guard.
func (c *Client) Evaluate(ctx context.Context, payload EvaluationPayload) (*EvaluationResult, error) {
	if payload.RequestID == "" {
		payload.RequestID = newRequestID()
	}
	var result EvaluationResult
	if _, err := c.postJSON(ctx, "/v1/evaluate", payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Authorize is a convenience wrapper: it calls Evaluate and returns
// (allowed, result, err). allowed is true iff outcome == allow.
func (c *Client) Authorize(ctx context.Context, actor Actor, actionType string, target Target) (bool, *EvaluationResult, error) {
	result, err := c.Evaluate(ctx, EvaluationPayload{
		Actor:  actor,
		Action: Action{ID: newRequestID(), Type: actionType},
		Target: target,
	})
	if err != nil {
		return false, nil, err
	}
	return result.Allowed(), result, nil
}

// GetSession calls GET /v1/session.
func (c *Client) GetSession(ctx context.Context) (*Session, error) {
	var s Session
	if _, err := c.getJSON(ctx, "/v1/session", &s); err != nil {
		return nil, err
	}
	return &s, nil
}
