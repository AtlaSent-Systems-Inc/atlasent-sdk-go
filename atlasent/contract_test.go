package atlasent_test

// Contract conformance tests. These replay the cross-SDK vectors
// in testdata/contract/vectors/ against a controlled httptest server and
// assert that the Go SDK's wire request matches the vector and its decoded
// output exposes the fields callers depend on.
//
// Vectors are vendored from atlasent-systems-inc/atlasent-sdk at the v1
// contract SHA. Refresh via: go generate ./atlasent/testdata/contract/...
// (see scripts/update-contract-vectors.sh).

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
)

type evaluateVectorFile struct {
	Description string            `json:"description"`
	Vectors     []evaluateVector  `json:"vectors"`
}

type evaluateVector struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	SDKInput    sdkInput        `json:"sdk_input"`
	APIKey      string          `json:"api_key"`
	WireRequest map[string]any  `json:"wire_request"`
	WireResponse json.RawMessage `json:"wire_response"`
	SDKOutput   map[string]any  `json:"sdk_output"`
	SDKError    *sdkError       `json:"sdk_error,omitempty"`
}

type sdkInput struct {
	Agent    string         `json:"agent"`
	Action   string         `json:"action"`
	Context  map[string]any `json:"context,omitempty"`
	PermitID string         `json:"permit_id,omitempty"`
}

type sdkError struct {
	Kind             string `json:"kind"`
	Status           int    `json:"status,omitempty"`
	MessageContains  string `json:"message_contains,omitempty"`
	RetryAfterSeconds int   `json:"retry_after_seconds,omitempty"`
}

func loadEvaluateVectors(t *testing.T) evaluateVectorFile {
	t.Helper()
	path := filepath.Join("testdata", "contract", "vectors", "evaluate.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var f evaluateVectorFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("unmarshal vectors: %v", err)
	}
	return f
}

func loadVerifyVectors(t *testing.T) evaluateVectorFile {
	t.Helper()
	path := filepath.Join("testdata", "contract", "vectors", "verify.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var f evaluateVectorFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("unmarshal vectors: %v", err)
	}
	return f
}

func TestContractEvaluateWireAndOutput(t *testing.T) {
	vectors := loadEvaluateVectors(t)

	for _, v := range vectors.Vectors {
		t.Run(v.Name, func(t *testing.T) {
			var received map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1-evaluate" {
					t.Fatalf("wrong path: %s", r.URL.Path)
				}
				if got := r.Header.Get("Content-Type"); got != "application/json" {
					t.Errorf("Content-Type: want application/json, got %q", got)
				}
				if got := r.Header.Get("Accept"); got != "application/json" {
					t.Errorf("Accept: want application/json, got %q", got)
				}
				if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, "atlasent-sdk-go/") {
					t.Errorf("User-Agent: want atlasent-sdk-go/* prefix, got %q", ua)
				}
				if auth := r.Header.Get("Authorization"); auth != "Bearer "+v.APIKey {
					t.Errorf("Authorization: want Bearer %s, got %q", v.APIKey, auth)
				}
				b, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(b, &received); err != nil {
					t.Fatalf("server decode body: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(v.WireResponse)
			}))
			defer srv.Close()

			base, _ := atlasent.New(v.APIKey, atlasent.WithBaseURL(srv.URL))
			legacy := atlasent.NewLegacy(base)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			out, err := legacy.Evaluate(ctx, v.SDKInput.Agent, v.SDKInput.Action, v.SDKInput.Context)

			if v.SDKError != nil {
				if err == nil {
					t.Fatalf("want error kind=%s, got out=%+v", v.SDKError.Kind, out)
				}
				if v.SDKError.MessageContains != "" && !strings.Contains(err.Error(), v.SDKError.MessageContains) {
					t.Errorf("error %q missing substring %q", err.Error(), v.SDKError.MessageContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}

			// Wire request equality.
			if !mapsEqual(received, v.WireRequest) {
				t.Errorf("wire mismatch\n got:  %s\n want: %s", mustJSON(received), mustJSON(v.WireRequest))
			}

			// SDK output field checks.
			if got := anyStr(v.SDKOutput, "decision"); got != "" && got != out.Decision {
				t.Errorf("decision: want %q, got %q", got, out.Decision)
			}
			if got, ok := v.SDKOutput["permitted"]; ok {
				if bv, _ := got.(bool); bv != out.Permitted {
					t.Errorf("permitted: want %v, got %v", bv, out.Permitted)
				}
			}
			if got := anyStr(v.SDKOutput, "permit_id"); got != "" && got != out.PermitID {
				t.Errorf("permit_id: want %q, got %q", got, out.PermitID)
			}
			if got := anyStr(v.SDKOutput, "reason"); got != out.Reason {
				t.Errorf("reason: want %q, got %q", got, out.Reason)
			}
		})
	}
}

func TestContractVerifyWireAndOutput(t *testing.T) {
	vectors := loadVerifyVectors(t)

	for _, v := range vectors.Vectors {
		t.Run(v.Name, func(t *testing.T) {
			var received map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1-verify-permit" {
					t.Fatalf("wrong path: %s", r.URL.Path)
				}
				b, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(b, &received); err != nil {
					t.Fatalf("server decode body: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(v.WireResponse)
			}))
			defer srv.Close()

			base, _ := atlasent.New(v.APIKey, atlasent.WithBaseURL(srv.URL))
			legacy := atlasent.NewLegacy(base)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			out, err := legacy.VerifyPermit(ctx, v.SDKInput.PermitID, v.SDKInput.Action, v.SDKInput.Agent, v.SDKInput.Context)
			if v.SDKError != nil {
				if err == nil {
					t.Fatalf("want error kind=%s, got out=%+v", v.SDKError.Kind, out)
				}
				if v.SDKError.MessageContains != "" && !strings.Contains(err.Error(), v.SDKError.MessageContains) {
					t.Errorf("error %q missing substring %q", err.Error(), v.SDKError.MessageContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("verify: %v", err)
			}
			if !mapsEqual(received, v.WireRequest) {
				t.Errorf("wire mismatch\n got:  %s\n want: %s", mustJSON(received), mustJSON(v.WireRequest))
			}
			if got, ok := v.SDKOutput["verified"]; ok {
				if bv, _ := got.(bool); bv != out.Verified {
					t.Errorf("verified: want %v, got %v", bv, out.Verified)
				}
			}
		})
	}
}

type errorsVectorFile struct {
	Description string        `json:"description"`
	Vectors     []errorVector `json:"vectors"`
}

type errorVector struct {
	Name            string            `json:"name"`
	HTTPStatus      int               `json:"http_status,omitempty"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	ResponseBody    json.RawMessage   `json:"response_body,omitempty"`
	Transport       string            `json:"transport,omitempty"`
	SDKError        struct {
		Kind              string `json:"kind"`
		Status            int    `json:"status,omitempty"`
		MessageContains   string `json:"message_contains,omitempty"`
		RetryAfterSeconds int    `json:"retry_after_seconds,omitempty"`
	} `json:"sdk_error"`
}

func TestContractErrorMapping(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "contract", "vectors", "errors.json"))
	if err != nil {
		t.Fatalf("read errors.json: %v", err)
	}
	var f errorsVectorFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("parse: %v", err)
	}

	for _, v := range f.Vectors {
		t.Run(v.Name, func(t *testing.T) {
			// Transport-level failures: point at a closed port or use a
			// server that drops the connection.
			if v.Transport != "" {
				c, _ := atlasent.New("k", atlasent.WithBaseURL("http://127.0.0.1:1"),
					atlasent.WithHTTPClient(&http.Client{Timeout: 50 * time.Millisecond}))
				legacy := atlasent.NewLegacy(c)
				_, err := legacy.Evaluate(context.Background(), "a", "b", nil)
				if err == nil {
					t.Fatalf("want transport error")
				}
				return
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, val := range v.ResponseHeaders {
					w.Header().Set(k, val)
				}
				w.WriteHeader(v.HTTPStatus)
				if len(v.ResponseBody) > 0 {
					// Allow string bodies like "oops" as well as JSON.
					if len(v.ResponseBody) > 0 && v.ResponseBody[0] == '"' {
						var s string
						_ = json.Unmarshal(v.ResponseBody, &s)
						_, _ = w.Write([]byte(s))
						return
					}
					_, _ = w.Write(v.ResponseBody)
				}
			}))
			defer srv.Close()

			c, _ := atlasent.New("k", atlasent.WithBaseURL(srv.URL))
			legacy := atlasent.NewLegacy(c)
			_, err := legacy.Evaluate(context.Background(), "a", "b", nil)
			if err == nil {
				t.Fatalf("want error %s", v.SDKError.Kind)
			}

			var apiErr *atlasent.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("want *APIError, got %T: %v", err, err)
			}
			if v.SDKError.Status != 0 && apiErr.Status != v.SDKError.Status {
				t.Errorf("status: want %d, got %d", v.SDKError.Status, apiErr.Status)
			}
			if v.SDKError.Kind != "" && apiErr.Code != v.SDKError.Kind {
				t.Errorf("code: want %s, got %s", v.SDKError.Kind, apiErr.Code)
			}
			if v.SDKError.RetryAfterSeconds > 0 {
				want := time.Duration(v.SDKError.RetryAfterSeconds) * time.Second
				if apiErr.RetryAfter != want {
					t.Errorf("retry_after: want %v, got %v", want, apiErr.RetryAfter)
				}
			}
			if v.SDKError.MessageContains != "" && !strings.Contains(apiErr.Message, v.SDKError.MessageContains) {
				t.Errorf("message %q missing %q", apiErr.Message, v.SDKError.MessageContains)
			}
		})
	}
}

// -----------------------------------------------------------------------
// test helpers
// -----------------------------------------------------------------------

func mapsEqual(a, b map[string]any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	var ax, bx any
	_ = json.Unmarshal(aj, &ax)
	_ = json.Unmarshal(bj, &bx)
	aj2, _ := json.Marshal(ax)
	bj2, _ := json.Marshal(bx)
	return string(aj2) == string(bj2)
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
func anyStr(m map[string]any, k string) string {
	v, _ := m[k].(string)
	return v
}
