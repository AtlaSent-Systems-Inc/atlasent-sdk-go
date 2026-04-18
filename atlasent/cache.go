package atlasent

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

// Cache caches Decisions keyed by a request fingerprint. Implementations must
// be safe for concurrent use. A cache hit skips the PDP entirely, so only
// wire one in if stale-but-fast is acceptable for the hot path.
type Cache interface {
	Get(key string) (Decision, bool)
	Set(key string, dec Decision, ttl time.Duration)
}

// WithCache installs a decision cache on the Client. By default decisions are
// cached for defaultTTL unless the PDP returns a TTLMillis hint on the
// Decision (in which case that wins).
func WithCache(cache Cache, defaultTTL time.Duration) Option {
	return func(c *Client) {
		c.cache = cache
		c.cacheDefaultTTL = defaultTTL
	}
}

// cacheKey returns a stable SHA-256 over the canonical JSON of req. Map
// ordering is not canonical in encoding/json, so callers that rely on the
// cache should keep Context keys stable.
func cacheKey(req CheckRequest) string {
	buf, err := json.Marshal(req)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

// MemoryCache is a bounded in-memory LRU cache with per-entry TTLs. Zero
// value is not usable; call NewMemoryCache.
type MemoryCache struct {
	mu       sync.Mutex
	capacity int
	ll       *list.List
	items    map[string]*list.Element
	now      func() time.Time
}

type cacheEntry struct {
	key      string
	decision Decision
	expires  time.Time
}

// NewMemoryCache returns an LRU cache that holds up to capacity entries.
// When full, the least-recently-used entry is evicted on Set.
func NewMemoryCache(capacity int) *MemoryCache {
	if capacity <= 0 {
		capacity = 1024
	}
	return &MemoryCache{
		capacity: capacity,
		ll:       list.New(),
		items:    make(map[string]*list.Element, capacity),
		now:      time.Now,
	}
}

func (m *MemoryCache) Get(key string) (Decision, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	el, ok := m.items[key]
	if !ok {
		return Decision{}, false
	}
	entry := el.Value.(*cacheEntry)
	if m.now().After(entry.expires) {
		m.ll.Remove(el)
		delete(m.items, key)
		return Decision{}, false
	}
	m.ll.MoveToFront(el)
	return entry.decision, true
}

func (m *MemoryCache) Set(key string, dec Decision, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if el, ok := m.items[key]; ok {
		entry := el.Value.(*cacheEntry)
		entry.decision = dec
		entry.expires = m.now().Add(ttl)
		m.ll.MoveToFront(el)
		return
	}
	entry := &cacheEntry{key: key, decision: dec, expires: m.now().Add(ttl)}
	el := m.ll.PushFront(entry)
	m.items[key] = el
	if m.ll.Len() > m.capacity {
		oldest := m.ll.Back()
		if oldest != nil {
			m.ll.Remove(oldest)
			delete(m.items, oldest.Value.(*cacheEntry).key)
		}
	}
}

// Len returns the current number of entries. Exposed for tests and metrics.
func (m *MemoryCache) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ll.Len()
}
