package atlasent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ClientOptions configures the AtlaSent client.
type ClientOptions struct {
	APIURL     string
	APIKey     string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// Client is the AtlaSent API client.
type Client struct {
	apiURL     string
	apiKey     string
	httpClient *http.Client
}

// New creates a new Client with the given options.
func New(opts ClientOptions) *Client {
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: opts.Timeout}
	}
	return &Client{
		apiURL:     strings.TrimRight(opts.APIURL, "/"),
		apiKey:     opts.APIKey,
		httpClient: hc,
	}
}

func (c *Client) request(ctx context.Context, method, path string, body, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("atlasent: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.apiURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("atlasent: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AtlaSent-Key", c.apiKey)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("atlasent: do request: %w", err)
	}
	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("atlasent: read response: %w", err)
	}

	if res.StatusCode >= 400 {
		var apiErr APIError
		if jsonErr := json.Unmarshal(b, &apiErr); jsonErr == nil && apiErr.Message != "" {
			apiErr.Status = res.StatusCode
			return &apiErr
		}
		return &APIError{Code: "http_error", Message: res.Status, Status: res.StatusCode}
	}

	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			return fmt.Errorf("atlasent: unmarshal response: %w", err)
		}
	}
	return nil
}

// Evaluate calls POST /v1/evaluate and returns the decision + risk assessment.
func (c *Client) Evaluate(ctx context.Context, payload EvaluationPayload) (*EvaluationResult, error) {
	var result EvaluationResult
	if err := c.request(ctx, http.MethodPost, "/v1/evaluate", payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Authorize is a convenience wrapper that returns true iff decision == "allow".
func (c *Client) Authorize(ctx context.Context, actor Actor, actionType string, target Target) (bool, *EvaluationResult, error) {
	result, err := c.Evaluate(ctx, EvaluationPayload{
		Actor:  actor,
		Action: Action{ID: uuid.NewString(), Type: actionType},
		Target: target,
	})
	if err != nil {
		return false, nil, err
	}
	return result.Decision == DecisionAllow, result, nil
}

// VerifyPermit calls POST /v1/permits/{id}/verify.
func (c *Client) VerifyPermit(ctx context.Context, permitID string) (*Permit, error) {
	var p Permit
	if err := c.request(ctx, http.MethodPost, "/v1/permits/"+permitID+"/verify", nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ConsumePermit calls POST /v1/permits/{id}/consume.
func (c *Client) ConsumePermit(ctx context.Context, permitID string) (*Permit, error) {
	var p Permit
	if err := c.request(ctx, http.MethodPost, "/v1/permits/"+permitID+"/consume", nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// GetSession calls GET /v1/session.
func (c *Client) GetSession(ctx context.Context) (*Session, error) {
	var s Session
	if err := c.request(ctx, http.MethodGet, "/v1/session", nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
