package atlasent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newBatchClient(t *testing.T, answer func(CheckRequest) Decision) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var br batchRequest
		if err := json.NewDecoder(r.Body).Decode(&br); err != nil {
			t.Fatalf("decode: %v", err)
		}
		decs := make([]Decision, len(br.Checks))
		for i, c := range br.Checks {
			decs[i] = answer(c)
		}
		_ = json.NewEncoder(w).Encode(batchResponse{Decisions: decs})
	}))
	t.Cleanup(srv.Close)
	c, _ := New("k", WithBaseURL(srv.URL))
	return c
}

func TestCheckAny_FindsFirstAllow(t *testing.T) {
	c := newBatchClient(t, func(req CheckRequest) Decision {
		return Decision{Allowed: req.Resource.ID == "ok"}
	})
	i, d, err := c.CheckAny(context.Background(), []CheckRequest{
		{Principal: Principal{ID: "u"}, Action: "r", Resource: Resource{ID: "no", Type: "d"}},
		{Principal: Principal{ID: "u"}, Action: "r", Resource: Resource{ID: "ok", Type: "d"}},
		{Principal: Principal{ID: "u"}, Action: "r", Resource: Resource{ID: "no", Type: "d"}},
	})
	if err != nil {
		t.Fatalf("CheckAny: %v", err)
	}
	if i != 1 || !d.Allowed {
		t.Fatalf("want index 1, got i=%d dec=%+v", i, d)
	}
}

func TestCheckAny_AllDenied(t *testing.T) {
	c := newBatchClient(t, func(req CheckRequest) Decision {
		return Decision{Allowed: false, Reason: "nope"}
	})
	_, _, err := c.CheckAny(context.Background(), []CheckRequest{
		{Principal: Principal{ID: "u"}, Action: "r", Resource: Resource{ID: "a", Type: "d"}},
		{Principal: Principal{ID: "u"}, Action: "r", Resource: Resource{ID: "b", Type: "d"}},
	})
	var denied *DeniedError
	if !errors.As(err, &denied) {
		t.Fatalf("want DeniedError, got %v", err)
	}
}

func TestCheckAll_AllAllowed(t *testing.T) {
	c := newBatchClient(t, func(req CheckRequest) Decision {
		return Decision{Allowed: true, PolicyID: "p"}
	})
	decs, err := c.CheckAll(context.Background(), []CheckRequest{
		{Principal: Principal{ID: "u"}, Action: "r", Resource: Resource{ID: "a", Type: "d"}},
		{Principal: Principal{ID: "u"}, Action: "r", Resource: Resource{ID: "b", Type: "d"}},
	})
	if err != nil {
		t.Fatalf("CheckAll: %v", err)
	}
	if len(decs) != 2 || !decs[0].Allowed || !decs[1].Allowed {
		t.Fatalf("want 2 allows, got %+v", decs)
	}
}

func TestCheckAll_OneDenial(t *testing.T) {
	c := newBatchClient(t, func(req CheckRequest) Decision {
		return Decision{Allowed: req.Resource.ID == "ok"}
	})
	_, err := c.CheckAll(context.Background(), []CheckRequest{
		{Principal: Principal{ID: "u"}, Action: "r", Resource: Resource{ID: "ok", Type: "d"}},
		{Principal: Principal{ID: "u"}, Action: "r", Resource: Resource{ID: "no", Type: "d"}},
	})
	var denied *DeniedError
	if !errors.As(err, &denied) {
		t.Fatalf("want DeniedError, got %v", err)
	}
}
