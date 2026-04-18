// Package atlasent is the Go SDK for AtlaSent execution-time authorization.
//
// An AtlaSent Client is a Policy Decision Point (PDP) client: your application
// asks the client whether a principal is allowed to perform an action on a
// resource, right before the action is executed. The client returns a Decision
// that your code either enforces (Guard, HTTPMiddleware) or inspects directly.
package atlasent

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultBaseURL = "https://api.atlasent.io"
	defaultTimeout = 10 * time.Second
)

// Client talks to the AtlaSent authorization service.
//
// Create one with New and reuse it for the lifetime of the process; it is safe
// for concurrent use.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	// FailClosed controls what happens when the PDP is unreachable or returns
	// a transport error. When true (the default), Check returns a Deny decision
	// so the caller cannot accidentally allow a request. Set false only when
	// availability matters more than correctness.
	FailClosed bool
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the AtlaSent API endpoint (useful for staging or
// self-hosted deployments).
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithHTTPClient swaps the underlying HTTP client. The provided client should
// set its own Timeout; otherwise the default 10s is kept.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// WithFailOpen flips the default fail-closed behavior. Use with caution.
func WithFailOpen() Option { return func(c *Client) { c.FailClosed = false } }

// New returns a Client authenticated with the given API key.
func New(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, &Error{Code: ErrInvalidAPIKey, Message: "apiKey is required"}
	}
	c := &Client{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
		FailClosed: true,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

func userAgent() string { return "atlasent-sdk-go/" + Version }

// newRequestID returns a 32-hex-char correlation ID. Not cryptographically
// meaningful — just needs to be unique per request.
func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (c *Client) postJSON(ctx context.Context, path string, in any, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return &Error{Code: ErrBadRequest, Message: fmt.Sprintf("marshal request: %v", err), Cause: err}
	}
	reqID := newRequestID()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return &Error{Code: ErrNetwork, Message: fmt.Sprintf("build request: %v", err), Cause: err, RequestID: reqID}
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("X-Request-ID", reqID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return mapTransportError(err, reqID)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return buildHTTPError(resp, reqID)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return &Error{
			Code:      ErrBadResponse,
			Status:    resp.StatusCode,
			RequestID: reqID,
			Message:   fmt.Sprintf("decoding response: %v", err),
			Cause:     err,
		}
	}
	return nil
}

func mapTransportError(err error, reqID string) error {
	if err == nil {
		return nil
	}
	// net/url wraps context deadline / canceled in *url.Error; unwrap to spot them.
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Code: ErrTimeout, RequestID: reqID, Message: "request timed out", Cause: err}
	}
	return &Error{Code: ErrNetwork, RequestID: reqID, Message: fmt.Sprintf("transport: %v", err), Cause: err}
}

func buildHTTPError(resp *http.Response, reqID string) *Error {
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<14))
	msg := serverMessage(bodyBytes, resp.StatusCode)
	e := &Error{
		Status:    resp.StatusCode,
		RequestID: reqID,
		Message:   msg,
	}
	switch {
	case resp.StatusCode == 401:
		e.Code = ErrInvalidAPIKey
		if e.Message == "" {
			e.Message = "Invalid API key"
		}
	case resp.StatusCode == 403:
		e.Code = ErrForbidden
		if e.Message == "" {
			e.Message = "Access forbidden — check your API key permissions"
		}
	case resp.StatusCode == 429:
		e.Code = ErrRateLimited
		e.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
		if e.Message == "" {
			e.Message = "Rate limited by AtlaSent API"
		}
	case resp.StatusCode >= 500:
		e.Code = ErrServerError
		if e.Message == "" {
			e.Message = fmt.Sprintf("AtlaSent API returned HTTP %d", resp.StatusCode)
		}
	default:
		e.Code = ErrBadRequest
		if e.Message == "" {
			e.Message = fmt.Sprintf("AtlaSent API returned HTTP %d", resp.StatusCode)
		}
	}
	return e
}

// serverMessage extracts the server's `message` / `reason` field from
// a JSON error body, falling back to the first 500 bytes of raw text.
func serverMessage(body []byte, status int) string {
	_ = status
	if len(body) == 0 {
		return ""
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err == nil {
		for _, key := range []string{"message", "reason"} {
			if v, ok := parsed[key].(string); ok && v != "" {
				return v
			}
		}
	}
	// Not JSON, or no message/reason — fall back to raw text.
	text := string(bytes.TrimSpace(body))
	if len(text) > 500 {
		return text[:500] + "…"
	}
	return text
}

// parseRetryAfter parses a Retry-After header, accepting both
// integer-seconds and HTTP-date formats.
func parseRetryAfter(raw string) time.Duration {
	if raw == "" {
		return 0
	}
	if secs, err := strconv.ParseFloat(raw, 64); err == nil && secs >= 0 {
		return time.Duration(secs * float64(time.Second))
	}
	if t, err := http.ParseTime(raw); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}
