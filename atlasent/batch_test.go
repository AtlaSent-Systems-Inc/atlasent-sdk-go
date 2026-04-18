package atlasent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckManyOrderPreserved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req batchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		decs := make([]Decision, len(req.Checks))
		for i, c := range req.Checks {
			decs[i] = Decision{Allowed: c.Resource.ID == "ok", PolicyID: c.Resource.ID}
		}
		_ = json.NewEncoder(w).Encode(batchResponse{Decisions: decs})
	}))
	defer srv.Close()

	c, _ := New("k", WithBaseURL(srv.URL))
	reqs := []CheckRequest{
		{Action: "read", Resource: Resource{ID: "ok", Type: "doc"}},
		{Action: "read", Resource: Resource{ID: "bad", Type: "doc"}},
		{Action: "read", Resource: Resource{ID: "ok", Type: "doc"}},
	}
	decs, err := c.CheckMany(context.Background(), reqs)
	if err != nil {
		t.Fatalf("CheckMany: %v", err)
	}
	if len(decs) != 3 {
		t.Fatalf("want 3 decisions, got %d", len(decs))
	}
	if !decs[0].Allowed || decs[1].Allowed || !decs[2].Allowed {
		t.Fatalf("wrong ordering/values: %+v", decs)
	}
}

func TestCheckManyHonorsCache(t *testing.T) {
	var batchHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		batchHits++
		var req batchRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		decs := make([]Decision, len(req.Checks))
		for i := range decs {
			decs[i] = Decision{Allowed: true, PolicyID: "fresh"}
		}
		_ = json.NewEncoder(w).Encode(batchResponse{Decisions: decs})
	}))
	defer srv.Close()

	cache := NewMemoryCache(8)
	c, _ := New("k", WithBaseURL(srv.URL), WithCache(cache, time.Minute))

	reqs := []CheckRequest{
		{Action: "read", Resource: Resource{ID: "a", Type: "doc"}},
		{Action: "read", Resource: Resource{ID: "b", Type: "doc"}},
	}
	// Pre-seed cache for "a" with a distinguishable decision.
	cache.Set(cacheKey(reqs[0]), Decision{Allowed: true, PolicyID: "cached"}, time.Minute)

	decs, err := c.CheckMany(context.Background(), reqs)
	if err != nil {
		t.Fatalf("CheckMany: %v", err)
	}
	if batchHits != 1 {
		t.Fatalf("want 1 batch call, got %d", batchHits)
	}
	if decs[0].PolicyID != "cached" {
		t.Fatalf("want cached decision for [0], got %+v", decs[0])
	}
	if decs[1].PolicyID != "fresh" {
		t.Fatalf("want fresh decision for [1], got %+v", decs[1])
	}
}

func TestCheckManyFailClosed(t *testing.T) {
	c, _ := New("k", WithBaseURL("http://127.0.0.1:1"))
	reqs := []CheckRequest{
		{Action: "a"},
		{Action: "b"},
	}
	decs, err := c.CheckMany(context.Background(), reqs)
	if err == nil {
		t.Fatal("expected transport error")
	}
	if len(decs) != 2 || decs[0].Allowed || decs[1].Allowed {
		t.Fatalf("want 2 denied decisions, got %+v", decs)
	}
}

func TestCheckManyAllCached(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when every request is cached")
	}))
	defer srv.Close()

	cache := NewMemoryCache(4)
	c, _ := New("k", WithBaseURL(srv.URL), WithCache(cache, time.Minute))

	reqs := []CheckRequest{
		{Action: "a"},
		{Action: "b"},
	}
	for _, r := range reqs {
		cache.Set(cacheKey(r), Decision{Allowed: true}, time.Minute)
	}
	decs, err := c.CheckMany(context.Background(), reqs)
	if err != nil {
		t.Fatalf("CheckMany: %v", err)
	}
	if !decs[0].Allowed || !decs[1].Allowed {
		t.Fatalf("want all allowed from cache, got %+v", decs)
	}
}
