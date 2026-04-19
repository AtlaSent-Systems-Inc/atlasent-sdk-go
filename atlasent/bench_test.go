package atlasent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func benchServer(b *testing.B) *httptest.Server {
	b.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Decision{Allowed: true, PolicyID: "p"})
	}))
	b.Cleanup(srv.Close)
	return srv
}

func benchReq() CheckRequest {
	return CheckRequest{
		Principal: Principal{ID: "user_alice", Type: "user", Groups: []string{"finance"}},
		Action:    "invoice.read",
		Resource:  Resource{ID: "inv_42", Type: "invoice"},
	}
}

func BenchmarkCheck(b *testing.B) {
	c, _ := New("k", WithBaseURL(benchServer(b).URL))
	req := benchReq()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.Check(ctx, req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCheckCached(b *testing.B) {
	c, _ := New("k",
		WithBaseURL(benchServer(b).URL),
		WithCache(NewMemoryCache(1024), time.Hour),
	)
	req := benchReq()
	ctx := context.Background()
	// Warm.
	if _, err := c.Check(ctx, req); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Check(ctx, req)
	}
}

func BenchmarkCheckMany10(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req batchRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		decs := make([]Decision, len(req.Checks))
		for i := range decs {
			decs[i] = Decision{Allowed: true}
		}
		_ = json.NewEncoder(w).Encode(batchResponse{Decisions: decs})
	}))
	b.Cleanup(srv.Close)

	c, _ := New("k", WithBaseURL(srv.URL))
	reqs := make([]CheckRequest, 10)
	for i := range reqs {
		reqs[i] = benchReq()
		reqs[i].Resource.ID = "inv_" + string(rune('0'+i))
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.CheckMany(ctx, reqs); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGuard(b *testing.B) {
	c, _ := New("k", WithBaseURL(benchServer(b).URL))
	req := benchReq()
	ctx := context.Background()
	fn := func(context.Context) (int, error) { return 42, nil }
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Guard(ctx, c, req, fn); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCacheKey(b *testing.B) {
	req := benchReq()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cacheKey(req)
	}
}

func BenchmarkMemoryCacheGetHit(b *testing.B) {
	cache := NewMemoryCache(1024)
	ctx := context.Background()
	cache.Set(ctx, "k", Decision{Allowed: true}, time.Hour)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(ctx, "k")
	}
}
