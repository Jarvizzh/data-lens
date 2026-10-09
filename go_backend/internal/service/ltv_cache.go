package service

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// CachePrefixLtvList LTV 报表列表缓存统一前缀
	CachePrefixLtvList = "ltv:list:"
	// CachePrefixDailyDist 每日充值分布 Singleflight 并发单飞统一前缀
	CachePrefixDailyDist = "daily_dist:"
)

// BuildLtvListCacheKey 统一构建 LTV 报表缓存 Key: "ltv:list:{platform}:{userID}"
func BuildLtvListCacheKey(platformCode string, userID int64) string {
	p := strings.ToLower(strings.TrimSpace(platformCode))
	if p == "" {
		p = "all"
	}
	return fmt.Sprintf("%s%s:%d", CachePrefixLtvList, p, userID)
}

// BuildDailyDistKey 统一构建每日充值分布并发单飞 Key: "daily_dist:{platform}:{userID}"
func BuildDailyDistKey(platformCode string, userID int64) string {
	p := strings.ToLower(strings.TrimSpace(platformCode))
	if p == "" {
		p = "all"
	}
	return fmt.Sprintf("%s%s:%d", CachePrefixDailyDist, p, userID)
}

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
		if k, ok := key.(string); ok && strings.HasPrefix(k, CachePrefixLtvList) && strings.HasSuffix(k, suffix) {
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
