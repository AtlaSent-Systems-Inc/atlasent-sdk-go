package bundle_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
	"github.com/atlasent-systems-inc/atlasent-sdk-go/bundle"
)

// staticEngine is a minimal PolicyEngine used in tests: every CheckRequest
// whose Action matches the payload's allowAction is allowed; everything
// else is "no opinion" (ok=false), falling through to remote.
type staticEngine struct{}

func (staticEngine) Load(payload []byte) (bundle.EngineState, error) {
	var meta struct {
		AllowAction string `json:"allow_action"`
	}
	if err := json.Unmarshal(payload, &meta); err != nil {
		return nil, err
	}
	return &staticState{allow: meta.AllowAction}, nil
}

type staticState struct{ allow string }

func (s *staticState) Evaluate(_ context.Context, req atlasent.CheckRequest) (atlasent.Decision, bool, error) {
	if req.Action == s.allow {
		return atlasent.Decision{Allowed: true, PolicyID: "local"}, true, nil
	}
	return atlasent.Decision{}, false, nil
}

// signedBundleServer serves a single bundle, Ed25519-signed with the
// returned public key. ETag is fixed so a second request with
// If-None-Match returns 304.
func signedBundleServer(t *testing.T, payload []byte) (*httptest.Server, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	sig := ed25519.Sign(priv, payload)
	const etag = `"v1"`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"payload":   base64.StdEncoding.EncodeToString(payload),
			"signature": base64.StdEncoding.EncodeToString(sig),
			"version":   "v1",
		})
	}))
	t.Cleanup(srv.Close)
	return srv, pub
}

func TestManagerServesLocalDecision(t *testing.T) {
	payload := []byte(`{"allow_action":"invoice.read"}`)
	srv, pub := signedBundleServer(t, payload)

	sync, err := bundle.NewHTTPSyncer(bundle.HTTPSyncerConfig{
		URL:       srv.URL,
		PublicKey: pub,
	})
	if err != nil {
		t.Fatalf("NewHTTPSyncer: %v", err)
	}
	mgr, err := bundle.NewManager(sync, staticEngine{}, bundle.WithSyncInterval(time.Hour))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	defer mgr.Close()

	// Covered action → local allow.
	d, ok, err := mgr.Evaluate(context.Background(), atlasent.CheckRequest{
		Principal: atlasent.Principal{ID: "u"},
		Action:    "invoice.read",
		Resource:  atlasent.Resource{ID: "r", Type: "doc"},
	})
	if err != nil || !ok || !d.Allowed {
		t.Fatalf("want ok allow, got ok=%v d=%+v err=%v", ok, d, err)
	}

	// Uncovered action → no opinion (ok=false).
	_, ok, err = mgr.Evaluate(context.Background(), atlasent.CheckRequest{
		Principal: atlasent.Principal{ID: "u"},
		Action:    "invoice.delete",
		Resource:  atlasent.Resource{ID: "r", Type: "doc"},
	})
	if err != nil || ok {
		t.Fatalf("want ok=false, got ok=%v err=%v", ok, err)
	}
}

func TestManagerRejectsBadSignature(t *testing.T) {
	payload := []byte(`{"allow_action":"x"}`)
	_, goodPub := signedBundleServer(t, payload)

	// Build a server that signs with the wrong key.
	_, evilPriv, _ := ed25519.GenerateKey(rand.Reader)
	sig := ed25519.Sign(evilPriv, payload)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"payload":   base64.StdEncoding.EncodeToString(payload),
			"signature": base64.StdEncoding.EncodeToString(sig),
			"version":   "v1",
		})
	}))
	defer srv.Close()

	sync, _ := bundle.NewHTTPSyncer(bundle.HTTPSyncerConfig{
		URL:       srv.URL,
		PublicKey: goodPub,
	})
	if _, err := bundle.NewManager(sync, staticEngine{}); err == nil {
		t.Fatal("expected signature verification to fail")
	}
}

func TestManagerIntegratesWithClient(t *testing.T) {
	payload := []byte(`{"allow_action":"invoice.read"}`)
	srv, pub := signedBundleServer(t, payload)

	sync, _ := bundle.NewHTTPSyncer(bundle.HTTPSyncerConfig{
		URL:       srv.URL,
		PublicKey: pub,
	})
	mgr, err := bundle.NewManager(sync, staticEngine{}, bundle.WithSyncInterval(time.Hour))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	defer mgr.Close()

	var remoteHits atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteHits.Add(1)
		_ = json.NewEncoder(w).Encode(atlasent.Decision{Allowed: false, Reason: "remote said no"})
	}))
	defer remote.Close()

	client, _ := atlasent.New("k",
		atlasent.WithBaseURL(remote.URL),
		atlasent.WithLocalEvaluator(mgr),
	)

	// Covered action — no remote call should happen.
	d, err := client.Check(context.Background(), atlasent.CheckRequest{
		Principal: atlasent.Principal{ID: "u"},
		Action:    "invoice.read",
		Resource:  atlasent.Resource{ID: "r", Type: "doc"},
	})
	if err != nil || !d.Allowed {
		t.Fatalf("local decision should win, got %+v err=%v", d, err)
	}
	if remoteHits.Load() != 0 {
		t.Fatalf("remote should not be hit for covered action, got %d", remoteHits.Load())
	}

	// Uncovered action falls through to remote.
	d, _ = client.Check(context.Background(), atlasent.CheckRequest{
		Principal: atlasent.Principal{ID: "u"},
		Action:    "invoice.delete",
		Resource:  atlasent.Resource{ID: "r", Type: "doc"},
	})
	if d.Allowed {
		t.Fatalf("remote denial should win, got %+v", d)
	}
	if remoteHits.Load() != 1 {
		t.Fatalf("want 1 remote hit, got %d", remoteHits.Load())
	}
}
