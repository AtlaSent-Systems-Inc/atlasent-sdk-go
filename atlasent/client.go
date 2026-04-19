package atlasent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Version is the SDK release string. Exposed for consumers that want to log
// or report which version is in use.
const Version = "0.3.0"

const (
	defaultBaseURL = "https://api.atlasent.io"
	defaultTimeout = 5 * time.Second
	userAgent      = "atlasent-sdk-go/" + Version
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

	retry           RetryPolicy
	cache           Cache
	cacheDefaultTTL time.Duration
	observer        Observer
	breaker         *breaker
	enricher        ContextEnricher
	local           LocalEvaluator
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the AtlaSent API endpoint (useful for staging or
// self-hosted deployments).
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithHTTPClient swaps the underlying HTTP client. The provided client should
// set its own Timeout; otherwise the default 5s is kept.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// WithFailOpen flips the default fail-closed behavior. Use with caution.
func WithFailOpen() Option { return func(c *Client) { c.FailClosed = false } }

// New returns a Client authenticated with the given API key.
func New(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("atlasent: apiKey is required")
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

// httpResult carries the parsed response plus retry metadata.
type httpResult struct {
	status     int
	retryAfter time.Duration
	body       []byte
}

// doJSON performs a single HTTP round trip and returns the raw result. It
// does not retry or decode the body. Transport failures become *APIError
// with KindTransport.
func (c *Client) doJSON(ctx context.Context, path string, in any) (*httpResult, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return nil, &APIError{Kind: KindValidation, Cause: fmt.Errorf("marshal request: %w", err)}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, &APIError{Kind: KindTransport, Cause: fmt.Errorf("build request: %w", err)}
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &APIError{Kind: KindTransport, Cause: err}
	}
	defer resp.Body.Close()

	buf, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, &APIError{Kind: KindTransport, Cause: fmt.Errorf("read body: %w", err)}
	}
	return &httpResult{
		status:     resp.StatusCode,
		retryAfter: parseRetryAfter(resp.Header),
		body:       buf,
	}, nil
}

// postJSON runs doJSON with the configured retry policy and decodes a
// successful 2xx body into out. attempts is the number of HTTP round trips
// actually performed.
func (c *Client) postJSON(ctx context.Context, path string, in, out any) (attempts int, err error) {
	if c.breaker != nil && !c.breaker.allow() {
		return 0, breakerError()
	}
	if c.breaker != nil {
		defer func() {
			if err != nil {
				c.breaker.onFailure()
			}
		}()
	}
	max := c.retry.MaxAttempts
	if max < 1 {
		max = 1
	}
	var lastErr error
	for attempt := 1; attempt <= max; attempt++ {
		attempts = attempt
		res, err := c.doJSON(ctx, path, in)
		if err != nil {
			// Transport error: retry if we have attempts left.
			lastErr = err
			if attempt == max {
				return attempts, lastErr
			}
			if werr := sleepCtx(ctx, c.retry.backoffFor(attempt, jitterRand)); werr != nil {
				return attempts, werr
			}
			continue
		}
		if res.status >= 200 && res.status < 300 {
			if c.breaker != nil {
				c.breaker.onSuccess()
			}
			if out == nil {
				return attempts, nil
			}
			if err := json.Unmarshal(res.body, out); err != nil {
				return attempts, &APIError{Kind: KindServer, Status: res.status, Cause: fmt.Errorf("decode response: %w", err)}
			}
			return attempts, nil
		}
		lastErr = &APIError{
			Kind:   classifyHTTP(res.status),
			Status: res.status,
			Body:   string(bytes.TrimSpace(res.body)),
		}
		if !retryableStatus(res.status) || attempt == max {
			return attempts, lastErr
		}
		wait := res.retryAfter
		if wait == 0 {
			wait = c.retry.backoffFor(attempt, jitterRand)
		}
		if werr := sleepCtx(ctx, wait); werr != nil {
			return attempts, werr
		}
	}
	return attempts, lastErr
}
