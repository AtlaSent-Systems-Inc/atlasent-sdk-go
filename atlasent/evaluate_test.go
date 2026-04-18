package atlasent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEvaluateAllow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1-evaluate" {
			t.Errorf("path = %q, want /v1-evaluate", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["actor_id"] != "agent-1" {
			t.Errorf("actor_id = %v, want agent-1", body["actor_id"])
		}
		if body["action_type"] != "read_data" {
			t.Errorf("action_type = %v, want read_data", body["action_type"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"permitted":   true,
			"decision_id": "dec-ok",
			"reason":      "",
			"audit_hash":  "h",
			"timestamp":   "2026-04-18T00:00:00Z",
		})
	}))
	defer srv.Close()

	c, _ := New("k", WithBaseURL(srv.URL))
	resp, err := c.Evaluate(context.Background(), EvaluateRequest{
		Agent:  "agent-1",
		Action: "read_data",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !resp.Permitted || resp.DecisionID != "dec-ok" {
		t.Fatalf("unexpected response %+v", resp)
	}
}

func TestEvaluateDenyIsData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"permitted":   false,
			"decision_id": "dec-deny",
			"reason":      "missing change_reason",
		})
	}))
	defer srv.Close()
	c, _ := New("k", WithBaseURL(srv.URL))
	resp, err := c.Evaluate(context.Background(), EvaluateRequest{Agent: "a", Action: "x"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if resp.Permitted {
		t.Fatal("want DENY")
	}
	if resp.Reason != "missing change_reason" {
		t.Errorf("reason = %q", resp.Reason)
	}
}

// Legacy-server shape: only native fields. The wire adapter normalizes.
func TestEvaluateNormalizesLegacyShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"decision":          "allow",
			"permit_token":      "pmt_legacy",
			"audit_entry_hash":  "h_legacy",
			"request_id":        "req_legacy",
		})
	}))
	defer srv.Close()
	c, _ := New("k", WithBaseURL(srv.URL))
	resp, err := c.Evaluate(context.Background(), EvaluateRequest{Agent: "a", Action: "x"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !resp.Permitted {
		t.Error("adapter did not derive permitted=true from decision=allow")
	}
	if resp.DecisionID != "pmt_legacy" {
		t.Errorf("DecisionID = %q, want pmt_legacy", resp.DecisionID)
	}
	if resp.AuditHash != "h_legacy" {
		t.Errorf("AuditHash = %q, want h_legacy", resp.AuditHash)
	}
}

func TestEvaluateMalformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"unrelated":"bar"}`))
	}))
	defer srv.Close()
	c, _ := New("k", WithBaseURL(srv.URL))
	_, err := c.Evaluate(context.Background(), EvaluateRequest{Agent: "a", Action: "x"})
	if err == nil {
		t.Fatal("want error")
	}
	if !IsCode(err, ErrBadResponse) {
		t.Fatalf("want bad_response, got %+v", AsError(err))
	}
}
