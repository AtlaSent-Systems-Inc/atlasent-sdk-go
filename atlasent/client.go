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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBaseURL = "https://api.atlasent.io"
	defaultTimeout = 5 * time.Second
	userAgent      = "atlasent-sdk-go/0.1"
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

func (c *Client) postJSON(ctx context.Context, path string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("atlasent: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("atlasent: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("atlasent: transport: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<14))
		return fmt.Errorf("atlasent: %s: %s", resp.Status, bytes.TrimSpace(msg))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
