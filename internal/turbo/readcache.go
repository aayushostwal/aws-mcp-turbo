package turbo

import (
	"sync"
	"time"
)

// readCache keeps rendered answers for a few seconds so an agent repeating the
// same read within one turn pays nothing. Entries are bounded and expire.
type readCache struct {
	mu      sync.Mutex
	entries map[string]cached
	max     int
	ttl     time.Duration
}

type cached struct {
	text string
	at   time.Time
}

func newReadCache(max int, ttl time.Duration) *readCache {
	return &readCache{entries: map[string]cached{}, max: max, ttl: ttl}
}

func (c *readCache) get(key string) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || time.Since(e.at) > c.ttl {
		delete(c.entries, key)
		return "", false
	}
	return e.text, true
}

func (c *readCache) put(key, text string) {
	if c == nil || len(text) > 64*1024 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.max {
		now := time.Now()
		for k, e := range c.entries {
			if now.Sub(e.at) > c.ttl {
				delete(c.entries, k)
			}
		}
		for k := range c.entries {
			if len(c.entries) < c.max {
				break
			}
			delete(c.entries, k)
		}
	}
	c.entries[key] = cached{text: text, at: time.Now()}
}

func (c *readCache) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.entries)
}
