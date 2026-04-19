package atlasent

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// AuditEvent is one row of the server-computed audit chain. The hash is
// SHA-256(previous_hash || canonical(payload)); servers guarantee
// monotonically increasing sequence numbers within an org.
type AuditEvent struct {
	ID           string         `json:"id"`
	OrgID        string         `json:"org_id"`
	Type         string         `json:"type"`
	ActorID      string         `json:"actor_id"`
	ResourceType string         `json:"resource_type,omitempty"`
	ResourceID   string         `json:"resource_id,omitempty"`
	Payload      map[string]any `json:"payload,omitempty"`
	Hash         string         `json:"hash"`
	PreviousHash string         `json:"previous_hash"`
	Sequence     int64          `json:"sequence"`
	OccurredAt   time.Time      `json:"occurred_at"`
}

// AuditFilter narrows ListAuditEvents. Zero values are ignored.
type AuditFilter struct {
	Types   []string
	ActorID string
	From    time.Time
	To      time.Time
	Cursor  string
	Limit   int
}

// AuditPage is one page of audit events. NextCursor is empty when the stream
// has been fully consumed.
type AuditPage struct {
	Events     []AuditEvent `json:"events"`
	Total      int          `json:"total"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

// ListAuditEvents calls GET /v1/audit/events with server-side filtering.
// Iterate via repeated calls threading filter.Cursor = prev.NextCursor.
func (c *Client) ListAuditEvents(ctx context.Context, filter AuditFilter) (*AuditPage, error) {
	q := url.Values{}
	for _, t := range filter.Types {
		q.Add("types", t)
	}
	if filter.ActorID != "" {
		q.Set("actor_id", filter.ActorID)
	}
	if !filter.From.IsZero() {
		q.Set("from", filter.From.UTC().Format(time.RFC3339))
	}
	if !filter.To.IsZero() {
		q.Set("to", filter.To.UTC().Format(time.RFC3339))
	}
	if filter.Cursor != "" {
		q.Set("cursor", filter.Cursor)
	}
	if filter.Limit > 0 {
		q.Set("limit", strconv.Itoa(filter.Limit))
	}

	path := "/v1/audit/events"
	if qs := q.Encode(); qs != "" {
		path += "?" + qs
	}
	var page AuditPage
	if _, err := c.getJSON(ctx, path, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// AuditExportRequest configures CreateAuditExport.
type AuditExportRequest struct {
	From   time.Time `json:"from,omitempty"`
	To     time.Time `json:"to,omitempty"`
	Types  []string  `json:"types,omitempty"`
	Format string    `json:"format,omitempty"` // json | pdf | both
}

// AuditExportBundle is the server-signed export envelope returned by
// CreateAuditExport. Clients should verify Signature locally before trusting
// the payload.
type AuditExportBundle struct {
	ExportID         string       `json:"export_id"`
	OrgID            string       `json:"org_id"`
	Events           []AuditEvent `json:"events"`
	ChainHeadHash    string       `json:"chain_head_hash"`
	ChainIntegrityOK bool         `json:"chain_integrity_ok"`
	Signature        string       `json:"signature"`
	SignedAt         time.Time    `json:"signed_at"`
	PDFURL           string       `json:"pdf_url,omitempty"`
}

// CreateAuditExport calls POST /v1/audit/exports and returns a signed
// bundle. format defaults to "both" when empty.
func (c *Client) CreateAuditExport(ctx context.Context, req AuditExportRequest) (*AuditExportBundle, error) {
	if strings.TrimSpace(req.Format) == "" {
		req.Format = "both"
	}
	var out AuditExportBundle
	if _, err := c.postJSON(ctx, "/v1/audit/exports", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AuditVerifyResult is the server-side verification response.
type AuditVerifyResult struct {
	ChainIntegrityOK bool     `json:"chain_integrity_ok"`
	SignatureValid   bool     `json:"signature_valid"`
	TamperedEventIDs []string `json:"tampered_event_ids,omitempty"`
}

// VerifyBundle calls POST /v1/audit/verify to re-check chain integrity of a
// previously exported bundle. Use this during compliance reviews or before
// trusting a forwarded bundle.
func (c *Client) VerifyBundle(ctx context.Context, exportID string) (*AuditVerifyResult, error) {
	if exportID == "" {
		return nil, fmt.Errorf("atlasent: VerifyBundle: exportID is required")
	}
	body := map[string]string{"export_id": exportID}
	var out AuditVerifyResult
	if _, err := c.postJSON(ctx, "/v1/audit/verify", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
