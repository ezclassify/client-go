package ezclassify

import (
	"container/list"
	"context"
	"sync"
	"time"
)

const defaultResponseCacheEntries = 500

type responseCacheEntry struct {
	key     string
	ready   chan struct{}
	value   any
	err     error
	expires time.Time
	waiters int
	cancel  context.CancelFunc
	pending bool
	element *list.Element
}

type responseCache struct {
	mu      sync.Mutex
	entries map[string]*responseCacheEntry
	order   list.List
	limit   int
	// now is the clock entry lifetimes are measured on. It is a field so tests
	// can move it; transport timers such as the request timeout never read it.
	now func() time.Time
}

func newResponseCache(limit int) *responseCache {
	return &responseCache{entries: make(map[string]*responseCacheEntry), limit: limit, now: time.Now}
}

func (c *responseCache) clear() {
	c.mu.Lock()
	c.entries = make(map[string]*responseCacheEntry)
	c.order.Init()
	c.mu.Unlock()
}

func (c *responseCache) remove(entry *responseCacheEntry) {
	if c.entries[entry.key] != entry {
		return
	}
	delete(c.entries, entry.key)
	c.order.Remove(entry.element)
}

func (c *responseCache) load(ctx context.Context, key string, ttl time.Duration, pinned bool, call func(context.Context) (any, bool, error)) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	entry := c.entries[key]
	if entry != nil && !entry.pending && !pinned && !c.now().Before(entry.expires) {
		c.remove(entry)
		entry = nil
	}
	if entry == nil {
		// Preserve request-scoped values without letting the first caller's
		// cancellation abort other callers waiting for the same answer.
		shared, cancel := context.WithCancel(context.WithoutCancel(ctx))
		entry = &responseCacheEntry{key: key, ready: make(chan struct{}), pending: true, cancel: cancel}
		entry.element = c.order.PushFront(entry)
		c.entries[key] = entry
		go func() {
			value, cacheable, err := call(shared)
			c.mu.Lock()
			entry.value, entry.err, entry.pending = value, err, false
			entry.expires = c.now().Add(ttl)
			if err != nil || !cacheable {
				c.remove(entry)
			}
			close(entry.ready)
			c.mu.Unlock()
			cancel()
		}()
		for len(c.entries) > c.limit {
			oldest := c.order.Back().Value.(*responseCacheEntry)
			c.remove(oldest)
		}
	} else {
		c.order.MoveToFront(entry.element)
	}
	if !entry.pending {
		value, err := entry.value, entry.err
		c.mu.Unlock()
		return value, err
	}
	entry.waiters++
	c.mu.Unlock()
	select {
	case <-entry.ready:
		return entry.value, entry.err
	case <-ctx.Done():
		c.mu.Lock()
		if entry.pending {
			entry.waiters--
			if entry.waiters == 0 {
				c.remove(entry)
				entry.cancel()
			}
		}
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}
