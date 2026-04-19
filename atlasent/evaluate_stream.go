package atlasent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// StreamEvent is one decoded frame from EvaluateStream. Exactly one of Partial
// or Final is set on success; Err is set on parse failures.
type StreamEvent struct {
	// Partial is an intermediate status update (evaluator hop, cache check,
	// etc.). Useful for UIs; safe to ignore.
	Partial *StreamPartial
	// Final is the terminal evaluation result.
	Final *EvaluationResult
	// Err is non-nil when the frame could not be decoded.
	Err error
}

// StreamPartial is a non-terminal progress update from a streaming evaluator.
type StreamPartial struct {
	Stage   string         `json:"stage"`
	Message string         `json:"message,omitempty"`
	Meta    map[string]any `json:"meta,omitempty"`
}

// EvaluateStream calls POST /v1/evaluate with Accept: text/event-stream and
// yields decoded events on the returned channel until the stream closes or
// ctx is cancelled. The channel is always closed before return.
//
// The server must emit Server-Sent Events (SSE) with data: lines carrying
// one JSON object per frame. Frame shapes:
//
//	{"type":"partial", "stage":"...", "message":"..."}
//	{"type":"final",   "outcome":"allow", ...}
//
// Unknown frame types are ignored.
func (c *Client) EvaluateStream(ctx context.Context, payload EvaluationPayload) (<-chan StreamEvent, error) {
	if payload.RequestID == "" {
		payload.RequestID = newRequestID()
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("atlasent: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/evaluate", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("atlasent: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", c.userAgent)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("atlasent: do request: %w", err)
	}
	if res.StatusCode >= 400 {
		b, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		return nil, decodeAPIError(res.StatusCode, res.Header, b)
	}

	ch := make(chan StreamEvent, 8)
	go func() {
		defer close(ch)
		defer res.Body.Close()
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(make([]byte, 0, 4096), 1<<20)
		for sc.Scan() {
			line := sc.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" || data == "[DONE]" {
				continue
			}
			var envelope struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal([]byte(data), &envelope); err != nil {
				ch <- StreamEvent{Err: fmt.Errorf("atlasent: stream decode: %w", err)}
				continue
			}
			switch envelope.Type {
			case "partial":
				var p StreamPartial
				if err := json.Unmarshal([]byte(data), &p); err != nil {
					ch <- StreamEvent{Err: err}
					continue
				}
				ch <- StreamEvent{Partial: &p}
			case "final":
				var r EvaluationResult
				if err := json.Unmarshal([]byte(data), &r); err != nil {
					ch <- StreamEvent{Err: err}
					continue
				}
				ch <- StreamEvent{Final: &r}
			}
		}
		if err := sc.Err(); err != nil {
			ch <- StreamEvent{Err: fmt.Errorf("atlasent: stream read: %w", err)}
		}
	}()
	return ch, nil
}
