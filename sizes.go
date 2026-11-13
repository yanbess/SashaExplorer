package main

import (
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// sizeTTL is how long a folder total stays good. A change deep inside a folder
// does not reach the watcher, so totals cannot be kept forever.
const sizeTTL = 10 * time.Minute

// sizeCache remembers folder totals.
type sizeCache struct {
	mu    sync.Mutex
	items map[string]sizeItem
}

type sizeItem struct {
	bytes int64
	at    time.Time
}

func newSizeCache() *sizeCache {
	return &sizeCache{items: map[string]sizeItem{}}
}

func sizeKey(path string) string {
	return strings.ToLower(filepath.Clean(path))
}

func (c *sizeCache) get(path string, fresh bool) (int64, bool) {
	if fresh {
		return 0, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.items[sizeKey(path)]
	if !ok || time.Since(item.at) > sizeTTL {
		return 0, false
	}
	return item.bytes, true
}

func (c *sizeCache) put(path string, bytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) > 20000 {
		c.items = map[string]sizeItem{}
	}
	c.items[sizeKey(path)] = sizeItem{bytes: bytes, at: time.Now()}
}

// invalidate forgets the totals of a folder and of every folder that holds it,
// because a change inside changes them all.
func (c *sizeCache) invalidate(path string) {
	key := sizeKey(path)
	c.mu.Lock()
	defer c.mu.Unlock()
	for cached := range c.items {
		if cached == key || strings.HasPrefix(key, strings.TrimSuffix(cached, `\`)+`\`) {
			delete(c.items, cached)
		}
	}
}
