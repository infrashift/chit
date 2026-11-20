package cache

import (
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/infrashift/chit/internal/assert"
)

// entry wraps a cached value with an expiration timestamp.
type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// LRU is a generic, goroutine-safe LRU cache with TTL expiration.
type LRU[K comparable, V any] struct {
	cache *lru.Cache[K, entry[V]]
	ttl   time.Duration
}

// NewLRU creates a new LRU cache with the given size and TTL.
func NewLRU[K comparable, V any](size int, ttl time.Duration) (*LRU[K, V], error) {
	assert.That(ttl >= 0, "NewLRU: negative ttl")
	c, err := lru.New[K, entry[V]](size)
	if err != nil {
		return nil, err
	}
	assert.That(c != nil && size > 0, "NewLRU: no cache for a valid size")
	return &LRU[K, V]{cache: c, ttl: ttl}, nil
}

// Get retrieves a value from the cache. Returns the value and true if found
// and not expired; zero value and false otherwise.
func (c *LRU[K, V]) Get(key K) (V, bool) {
	assert.That(c.cache != nil, "LRU.Get: not built by NewLRU")
	e, ok := c.cache.Get(key)
	if !ok {
		var zero V
		return zero, false
	}
	if time.Now().After(e.expiresAt) {
		c.cache.Remove(key)
		var zero V
		return zero, false
	}
	return e.value, true
}

// Set adds or updates a value in the cache.
func (c *LRU[K, V]) Set(key K, value V) {
	assert.That(c.cache != nil, "LRU.Set: not built by NewLRU")
	assert.That(c.ttl >= 0, "LRU.Set: negative ttl")
	c.cache.Add(key, entry[V]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	})
}

// Remove removes a key from the cache.
func (c *LRU[K, V]) Remove(key K) {
	assert.That(c.cache != nil, "LRU.Remove: not built by NewLRU")
	c.cache.Remove(key)
}
