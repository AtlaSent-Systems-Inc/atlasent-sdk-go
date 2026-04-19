// Package cacheredis is a Redis-backed implementation of atlasent.Cache.
//
// Use this when multiple SDK instances should share a decision cache
// (typical for horizontally scaled services) or when in-process cache
// doesn't survive restarts.
//
//	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
//	cache := cacheredis.New(rdb)
//	client, _ := atlasent.New(apiKey, atlasent.WithCache(cache, 5*time.Second))
package cacheredis

import (
	"context"
	"encoding/json"
	"time"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
	"github.com/redis/go-redis/v9"
)

// Cache implements atlasent.Cache on top of a Redis client. It is safe for
// concurrent use. All operations are best-effort: transient Redis errors
// degrade to cache misses rather than failing the authorization call.
type Cache struct {
	rdb    redis.Cmdable
	prefix string
}

// Option configures a Cache.
type Option func(*Cache)

// WithKeyPrefix scopes keys in Redis. Default: "atlasent:dec:".
func WithKeyPrefix(p string) Option { return func(c *Cache) { c.prefix = p } }

// New returns a Cache that stores Decisions in Redis via rdb. rdb can be
// any go-redis client (Client, ClusterClient, etc.).
func New(rdb redis.Cmdable, opts ...Option) *Cache {
	c := &Cache{rdb: rdb, prefix: "atlasent:dec:"}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Get implements atlasent.Cache.
func (c *Cache) Get(ctx context.Context, key string) (atlasent.Decision, bool) {
	raw, err := c.rdb.Get(ctx, c.prefix+key).Bytes()
	if err != nil {
		// redis.Nil (miss) or transport error: both treated as miss.
		return atlasent.Decision{}, false
	}
	var d atlasent.Decision
	if err := json.Unmarshal(raw, &d); err != nil {
		return atlasent.Decision{}, false
	}
	return d, true
}

// Set implements atlasent.Cache. TTL <= 0 is a no-op; negative TTLs would
// be interpreted by Redis as "persist forever" which is never the right
// default for an authorization decision.
func (c *Cache) Set(ctx context.Context, key string, dec atlasent.Decision, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	buf, err := json.Marshal(dec)
	if err != nil {
		return
	}
	_ = c.rdb.Set(ctx, c.prefix+key, buf, ttl).Err()
}
