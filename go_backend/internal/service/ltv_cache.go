package service

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type cacheEntry struct {
	data      interface{}
	timestamp time.Time
}

type LtvMemoryCache struct {
	items sync.Map
	ttl   time.Duration
}

func NewLtvMemoryCache(ttl time.Duration) *LtvMemoryCache {
	return &LtvMemoryCache{
		ttl: ttl,
	}
}

func (c *LtvMemoryCache) Get(key string) (interface{}, bool) {
	val, ok := c.items.Load(key)
	if !ok {
		return nil, false
	}
	entry := val.(cacheEntry)
	if time.Since(entry.timestamp) > c.ttl {
		c.items.Delete(key)
		return nil, false
	}
	return entry.data, true
}

func (c *LtvMemoryCache) Set(key string, data interface{}) {
	c.items.Store(key, cacheEntry{
		data:      data,
		timestamp: time.Now(),
	})
}

func (c *LtvMemoryCache) Delete(key string) {
	c.items.Delete(key)
}

func (c *LtvMemoryCache) InvalidateUser(userID int64) {
	suffix := fmt.Sprintf(":%d", userID)
	c.items.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok && strings.HasSuffix(k, suffix) {
			c.items.Delete(key)
		}
		return true
	})
}

func (c *LtvMemoryCache) Clear() {
	c.items.Range(func(key, value interface{}) bool {
		c.items.Delete(key)
		return true
	})
}

// CleanExpired 清理过期缓存
func (c *LtvMemoryCache) CleanExpired() {
	now := time.Now()
	c.items.Range(func(key, value interface{}) bool {
		entry := value.(cacheEntry)
		if now.Sub(entry.timestamp) > c.ttl {
			c.items.Delete(key)
		}
		return true
	})
}
