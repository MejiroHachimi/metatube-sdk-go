package service

import (
	"container/list"
	"sync"
	"time"
)

type cacheEntry struct {
	key     string
	value   response
	expires time.Time
	size    int64
}
type imageCache struct {
	mu              sync.Mutex
	entries         map[string]*list.Element
	lru             *list.List
	bytes, capacity int64
	ttl             time.Duration
	now             func() time.Time
}

func newImageCache(capacity int64, ttl time.Duration) *imageCache {
	return &imageCache{entries: make(map[string]*list.Element), lru: list.New(), capacity: capacity, ttl: ttl, now: time.Now}
}
func (c *imageCache) remove(e *list.Element) {
	v := e.Value.(cacheEntry)
	delete(c.entries, v.key)
	c.bytes -= v.size
	c.lru.Remove(e)
}
func (c *imageCache) get(key string) (response, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return response{}, false
	}
	v := e.Value.(cacheEntry)
	if !c.now().Before(v.expires) {
		c.remove(e)
		return response{}, false
	}
	c.lru.MoveToFront(e)
	return v.value, true
}
func (c *imageCache) put(key string, v response) {
	size := int64(len(v.body) + len(key) + 256)
	if c.capacity == 0 || size > c.capacity || len(v.body) > 8<<20 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.entries[key]; ok {
		c.remove(old)
	}
	for e := c.lru.Back(); e != nil; {
		prev := e.Prev()
		if !c.now().Before(e.Value.(cacheEntry).expires) {
			c.remove(e)
		}
		e = prev
	}
	for c.bytes+size > c.capacity {
		c.remove(c.lru.Back())
	}
	c.entries[key] = c.lru.PushFront(cacheEntry{key: key, value: v, expires: c.now().Add(c.ttl), size: size})
	c.bytes += size
}
