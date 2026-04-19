package bundle

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Bundle is a raw, verified policy snapshot. Payload is
// engine-specific; ETag + Version identify it for cache hits and
// rollback diagnostics.
type Bundle struct {
	Payload []byte
	ETag    string
	Version string
}

// Syncer fetches the current Bundle from somewhere. The canonical
// implementation is [HTTPSyncer]; tests and on-disk deployments can
// provide their own.
type Syncer interface {
	// Fetch returns the current bundle. If the remote reports
	// unchanged (HTTP 304 for HTTPSyncer), the Syncer returns
	// (nil, false, nil). An error propagates; callers should keep
	// serving the previously-loaded bundle and retry on the next
	// interval.
	Fetch(ctx context.Context) (*Bundle, bool, error)
}

// HTTPSyncerConfig configures an HTTP-backed Syncer.
type HTTPSyncerConfig struct {
	// URL is the GET endpoint that returns `{payload, signature,
	// version}` as JSON. Required.
	URL string
	// APIKey is sent as Bearer auth on every fetch. Required when
	// the bundle endpoint is authenticated (the hosted PDP always
	// requires it).
	APIKey string
	// PublicKey is the Ed25519 verify key for the PDP's signing
	// keypair. A signature that does not verify against this key
	// causes Fetch to return an error and the current bundle to
	// stay in place. Required; there is no "skip verify" switch.
	PublicKey ed25519.PublicKey
	// HTTPClient overrides the default http.Client.
	HTTPClient *http.Client
}

// HTTPSyncer is a [Syncer] that pulls `{payload, signature}` JSON from
// a hosted endpoint, verifies Ed25519 over the payload, and honors
// ETag for conditional GETs.
type HTTPSyncer struct {
	cfg    HTTPSyncerConfig
	client *http.Client
	etag   string
}

// NewHTTPSyncer validates cfg and returns a ready Syncer.
func NewHTTPSyncer(cfg HTTPSyncerConfig) (*HTTPSyncer, error) {
	if cfg.URL == "" {
		return nil, errors.New("bundle: URL is required")
	}
	if len(cfg.PublicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("bundle: PublicKey must be %d bytes, got %d",
			ed25519.PublicKeySize, len(cfg.PublicKey))
	}
	c := cfg.HTTPClient
	if c == nil {
		c = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPSyncer{cfg: cfg, client: c}, nil
}

// bundleResponse is the wire shape returned by the PDP's bundle endpoint.
// Signature is a base64-encoded Ed25519 signature over the raw payload
// bytes (after base64 decoding). Version is a human-readable label for
// logs.
type bundleResponse struct {
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
	Version   string `json:"version"`
}

// Fetch implements Syncer.
func (s *HTTPSyncer) Fetch(ctx context.Context) (*Bundle, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.URL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("bundle: build request: %w", err)
	}
	if s.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)
	}
	if s.etag != "" {
		req.Header.Set("If-None-Match", s.etag)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("bundle: transport: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return nil, false, nil
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<14))
		return nil, false, fmt.Errorf("bundle: %s: %s", resp.Status, bytes.TrimSpace(body))
	}

	buf, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, false, fmt.Errorf("bundle: read body: %w", err)
	}
	var br bundleResponse
	if err := json.Unmarshal(buf, &br); err != nil {
		return nil, false, fmt.Errorf("bundle: parse response: %w", err)
	}
	payload, err := base64.StdEncoding.DecodeString(br.Payload)
	if err != nil {
		return nil, false, fmt.Errorf("bundle: decode payload: %w", err)
	}
	sig, err := base64.StdEncoding.DecodeString(br.Signature)
	if err != nil {
		return nil, false, fmt.Errorf("bundle: decode signature: %w", err)
	}
	if !ed25519.Verify(s.cfg.PublicKey, payload, sig) {
		return nil, false, errors.New("bundle: signature verification failed")
	}

	s.etag = resp.Header.Get("ETag")
	return &Bundle{
		Payload: payload,
		ETag:    s.etag,
		Version: br.Version,
	}, true, nil
}

