package turbo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type cursor struct {
	Token     string
	Timestamp float64
	At        time.Time
}
type stream struct {
	History [16]cursor
	Count   uint64
	Used    time.Time
}

// DeltaCache holds a bounded ring of cursor checkpoints per session/query.
// Forward tokens preserve late arrivals and distinct events at equal timestamps.
type DeltaCache struct {
	mu       sync.Mutex
	streams  map[string]*stream
	locks    [64]chan struct{}
	capacity int
	ttl      time.Duration
}

func NewDeltaCache(capacity int, ttl time.Duration) *DeltaCache {
	c := &DeltaCache{streams: map[string]*stream{}, capacity: capacity, ttl: ttl}
	for i := range c.locks {
		c.locks[i] = make(chan struct{}, 1)
	}
	return c
}

func cacheKey(session string, params []byte) string {
	sum := sha256.Sum256(append([]byte(session+"\x00"), params...))
	return hex.EncodeToString(sum[:])
}

func (c *DeltaCache) lock(ctx context.Context, key string) (func(), error) {
	h := sha256.Sum256([]byte(key))
	ch := c.locks[int(h[0])%len(c.locks)]
	select {
	case ch <- struct{}{}:
		return func() { <-ch }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *DeltaCache) get(key string, now time.Time) cursor {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, s := range c.streams {
		if now.Sub(s.Used) >= c.ttl {
			delete(c.streams, k)
		}
	}
	s := c.streams[key]
	if s == nil {
		return cursor{}
	}
	last := s.History[(s.Count-1)%16]
	// AWS forward tokens expire after 24 hours, even if the stream stays idle.
	if now.Sub(last.At) >= 23*time.Hour {
		delete(c.streams, key)
		return cursor{}
	}
	s.Used = now
	return last
}

func (c *DeltaCache) put(key string, next cursor, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.streams[key]
	if s == nil {
		if len(c.streams) >= c.capacity {
			var oldest string
			var at time.Time
			for k, v := range c.streams {
				if oldest == "" || v.Used.Before(at) {
					oldest, at = k, v.Used
				}
			}
			delete(c.streams, oldest)
		}
		s = &stream{}
		c.streams[key] = s
	}
	if s.Count > 0 {
		last := s.History[(s.Count-1)%16]
		if next.Token == last.Token {
			next.At = last.At
		}
		if next.Timestamp < last.Timestamp {
			next.Timestamp = last.Timestamp
		}
	}
	s.History[s.Count%16] = next
	s.Count++
	s.Used = now
}
