package cacheredis_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
	"github.com/atlasent-systems-inc/atlasent-sdk-go/cacheredis"
	"github.com/redis/go-redis/v9"
)

func newCache(t *testing.T) (*cacheredis.Cache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return cacheredis.New(rdb), mr
}

func TestRedisCacheSetGet(t *testing.T) {
	c, _ := newCache(t)
	ctx := context.Background()

	want := atlasent.Decision{Allowed: true, PolicyID: "p1", Obligations: []string{"log"}}
	c.Set(ctx, "k", want, time.Minute)

	got, ok := c.Get(ctx, "k")
	if !ok {
		t.Fatal("miss on freshly set key")
	}
	if got.PolicyID != want.PolicyID || !got.Allowed {
		t.Fatalf("wrong decision: %+v", got)
	}
	if len(got.Obligations) != 1 || got.Obligations[0] != "log" {
		t.Fatalf("lost obligations: %+v", got)
	}
}

func TestRedisCacheExpiry(t *testing.T) {
	c, mr := newCache(t)
	ctx := context.Background()

	c.Set(ctx, "k", atlasent.Decision{Allowed: true}, time.Second)
	if _, ok := c.Get(ctx, "k"); !ok {
		t.Fatal("entry should be live")
	}
	mr.FastForward(2 * time.Second)
	if _, ok := c.Get(ctx, "k"); ok {
		t.Fatal("entry should have expired")
	}
}

func TestRedisCacheZeroTTLNoop(t *testing.T) {
	c, _ := newCache(t)
	ctx := context.Background()

	c.Set(ctx, "k", atlasent.Decision{Allowed: true}, 0)
	if _, ok := c.Get(ctx, "k"); ok {
		t.Fatal("zero TTL should not write")
	}
}

func TestRedisCacheKeyPrefix(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := cacheredis.New(rdb, cacheredis.WithKeyPrefix("custom:"))

	c.Set(context.Background(), "k", atlasent.Decision{Allowed: true}, time.Minute)
	if !mr.Exists("custom:k") {
		t.Fatal("want key custom:k to exist")
	}
}
