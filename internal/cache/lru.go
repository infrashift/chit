package cache

import (
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
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
	c, err := lru.New[K, entry[V]](size)
	if err != nil {
		return nil, err
	}
	return &LRU[K, V]{cache: c, ttl: ttl}, nil
}

// Get retrieves a value from the cache. Returns the value and true if found
// and not expired; zero value and false otherwise.
func (c *LRU[K, V]) Get(key K) (V, bool) {
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
	c.cache.Add(key, entry[V]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	})
}

// Remove removes a key from the cache.
func (c *LRU[K, V]) Remove(key K) {
	c.cache.Remove(key)
}

// Len returns the number of entries in the cache.
func (c *LRU[K, V]) Len() int {
	return c.cache.Len()
}

// Purge clears all entries from the cache.
func (c *LRU[K, V]) Purge() {
	c.cache.Purge()
}
