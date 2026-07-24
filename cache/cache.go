package cache

import (
	"sync"
	"time"
)

type entry[T any] struct {
	value     T
	expiresAt time.Time
}

// Cache 是并发安全的泛型内存 TTL 缓存。
type Cache[T any] struct {
	mu    sync.RWMutex
	items map[string]entry[T]
	ttl   time.Duration
	now   func() time.Time
}

func New[T any](ttl time.Duration) *Cache[T] {
	return NewWithClock[T](ttl, time.Now)
}

// NewWithClock 创建使用指定时钟的缓存，主要用于确定性的 TTL 测试。
func NewWithClock[T any](ttl time.Duration, now func() time.Time) *Cache[T] {
	if ttl <= 0 {
		panic("缓存 TTL 必须大于 0")
	}
	if now == nil {
		panic("缓存时钟不能为空")
	}
	return &Cache[T]{items: make(map[string]entry[T]), ttl: ttl, now: now}
}

func (c *Cache[T]) Get(key string) (T, bool) {
	c.mu.RLock()
	item, ok := c.items[key]
	if ok && c.now().Before(item.expiresAt) {
		c.mu.RUnlock()
		return item.value, true
	}
	c.mu.RUnlock()

	var zero T
	if !ok {
		return zero, false
	}

	c.mu.Lock()
	if current, exists := c.items[key]; exists && !c.now().Before(current.expiresAt) {
		delete(c.items, key)
	}
	c.mu.Unlock()
	return zero, false
}

func (c *Cache[T]) Set(key string, value T) {
	c.mu.Lock()
	c.items[key] = entry[T]{value: value, expiresAt: c.now().Add(c.ttl)}
	c.mu.Unlock()
}
