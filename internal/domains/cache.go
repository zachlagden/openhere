package domains

import (
	"sync"
	"time"
)

const (
	foundTTL      = time.Minute
	missingTTL    = 10 * time.Second
	maxCachedKeys = 4096
)

type cacheEntry struct {
	target    Target
	serves    bool
	expiresAt time.Time
}

type cache struct {
	now func() time.Time

	mu         sync.Mutex
	entries    map[string]cacheEntry
	generation uint64
}

func newCache(now func() time.Time) *cache {
	return &cache{now: now, entries: map[string]cacheEntry{}}
}

func (c *cache) get(host string) (cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[host]
	if !ok || !c.now().Before(entry.expiresAt) {
		return cacheEntry{}, false
	}
	return entry, true
}

func (c *cache) currentGeneration() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generation
}

func (c *cache) put(host string, target Target, serves bool, generation uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.generation != generation {
		return
	}
	now := c.now()
	if len(c.entries) >= maxCachedKeys {
		c.makeRoom(now)
	}
	ttl := missingTTL
	if serves {
		ttl = foundTTL
	}
	c.entries[host] = cacheEntry{target: target, serves: serves, expiresAt: now.Add(ttl)}
}

func (c *cache) invalidate() {
	c.mu.Lock()
	c.entries = map[string]cacheEntry{}
	c.generation++
	c.mu.Unlock()
}

func (c *cache) makeRoom(now time.Time) {
	for key, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
	for key := range c.entries {
		if len(c.entries) < maxCachedKeys {
			return
		}
		delete(c.entries, key)
	}
}
