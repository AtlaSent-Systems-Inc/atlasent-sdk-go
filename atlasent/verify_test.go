package atlasent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyPermitHappy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1-verify-permit" {
			t.Errorf("path = %q, want /v1-verify-permit", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["decision_id"] != "dec-ok" {
			t.Errorf("decision_id = %v, want dec-ok", body["decision_id"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"verified":    true,
			"outcome":     "verified",
			"permit_hash": "h-perm",
			"timestamp":   "2026-04-18T00:00:01Z",
		})
	}))
	defer srv.Close()
	c, _ := New("k", WithBaseURL(srv.URL))
	resp, err := c.VerifyPermit(context.Background(), VerifyPermitRequest{PermitID: "dec-ok"})
	if err != nil {
		t.Fatalf("VerifyPermit: %v", err)
	}
	if !resp.Verified || resp.PermitHash != "h-perm" {
		t.Fatalf("unexpected response %+v", resp)
	}
}

func TestVerifyPermitNotVerifiedIsData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"verified": false,
			"outcome":  "expired",
		})
	}))
	defer srv.Close()
	c, _ := New("k", WithBaseURL(srv.URL))
	resp, err := c.VerifyPermit(context.Background(), VerifyPermitRequest{PermitID: "dec-x"})
	if err != nil {
		t.Fatalf("VerifyPermit: %v", err)
	}
	if resp.Verified {
		t.Fatal("want verified=false")
	}
}

// Legacy server shape: only {valid, outcome}. Adapter derives verified.
func TestVerifyPermitNormalizesLegacyShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"valid":   true,
			"outcome": "allow",
		})
	}))
	defer srv.Close()
	c, _ := New("k", WithBaseURL(srv.URL))
	resp, err := c.VerifyPermit(context.Background(), VerifyPermitRequest{PermitID: "x"})
	if err != nil {
		t.Fatalf("VerifyPermit: %v", err)
	}
	if !resp.Verified {
		t.Error("adapter did not derive verified=true from valid=true")
	}
	if resp.PermitHash != "" {
		t.Errorf("legacy server sends no permit_hash — want empty, got %q", resp.PermitHash)
	}
}
