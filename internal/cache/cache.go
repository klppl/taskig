package cache

import (
	"strings"
	"sync"
	"time"
)

// Entry stores a cached value with its expiration time.
type entry struct {
	data      any
	expiresAt time.Time
}

// Cache is a thread-safe in-memory key-value store with TTL expiration.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]entry
}

// New creates a new Cache and starts a background cleanup goroutine.
func New() *Cache {
	c := &Cache{
		entries: make(map[string]entry),
	}
	go c.cleanup()
	return c
}

// Get retrieves a cached value. Returns nil, false if not found or expired.
func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.data, true
}

// Set stores a value with the given TTL.
func (c *Cache) Set(key string, data any, ttl time.Duration) {
	c.mu.Lock()
	c.entries[key] = entry{data: data, expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()
}

// InvalidatePrefix removes all entries whose key starts with the given prefix.
func (c *Cache) InvalidatePrefix(prefix string) {
	c.mu.Lock()
	for k := range c.entries {
		if strings.HasPrefix(k, prefix) {
			delete(c.entries, k)
		}
	}
	c.mu.Unlock()
}

// InvalidateKey removes a single entry.
func (c *Cache) InvalidateKey(key string) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

func (c *Cache) cleanup() {
	ticker := time.NewTicker(60 * time.Second)
	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for k, e := range c.entries {
			if now.After(e.expiresAt) {
				delete(c.entries, k)
			}
		}
		c.mu.Unlock()
	}
}
