package atlasent

import (
	"testing"
	"time"
)

func TestMemoryCacheGetSet(t *testing.T) {
	cache := NewMemoryCache(2)
	cache.Set("a", Decision{Allowed: true, PolicyID: "p1"}, time.Minute)

	got, ok := cache.Get("a")
	if !ok {
		t.Fatal("miss on freshly set key")
	}
	if !got.Allowed || got.PolicyID != "p1" {
		t.Fatalf("wrong decision: %+v", got)
	}
}

func TestMemoryCacheExpiry(t *testing.T) {
	cache := NewMemoryCache(4)
	now := time.Unix(1000, 0)
	cache.now = func() time.Time { return now }

	cache.Set("k", Decision{Allowed: true}, 10*time.Second)
	now = now.Add(5 * time.Second)
	if _, ok := cache.Get("k"); !ok {
		t.Fatal("entry should still be live at 5s")
	}
	now = now.Add(6 * time.Second)
	if _, ok := cache.Get("k"); ok {
		t.Fatal("entry should have expired at 11s")
	}
}

func TestMemoryCacheLRUEviction(t *testing.T) {
	cache := NewMemoryCache(2)
	cache.Set("a", Decision{Allowed: true}, time.Minute)
	cache.Set("b", Decision{Allowed: true}, time.Minute)
	// Touch "a" so "b" is LRU.
	if _, ok := cache.Get("a"); !ok {
		t.Fatal("a should be cached")
	}
	cache.Set("c", Decision{Allowed: true}, time.Minute)

	if _, ok := cache.Get("b"); ok {
		t.Fatal("b should have been evicted as LRU")
	}
	if _, ok := cache.Get("a"); !ok {
		t.Fatal("a should still be cached")
	}
	if _, ok := cache.Get("c"); !ok {
		t.Fatal("c should be cached")
	}
	if cache.Len() != 2 {
		t.Fatalf("want len=2, got %d", cache.Len())
	}
}

func TestMemoryCacheZeroTTLNoInsert(t *testing.T) {
	cache := NewMemoryCache(4)
	cache.Set("a", Decision{Allowed: true}, 0)
	if _, ok := cache.Get("a"); ok {
		t.Fatal("zero TTL should not insert")
	}
}

func TestCacheKeyStable(t *testing.T) {
	req := CheckRequest{
		Principal: Principal{ID: "u1", Groups: []string{"g"}},
		Action:    "read",
		Resource:  Resource{ID: "r1", Type: "doc"},
	}
	if cacheKey(req) != cacheKey(req) {
		t.Fatal("cacheKey not stable across calls")
	}
	alt := req
	alt.Action = "write"
	if cacheKey(req) == cacheKey(alt) {
		t.Fatal("cacheKey collision between read/write")
	}
}
