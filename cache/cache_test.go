package cache

import (
	"sync"
	"testing"
	"time"
)

func TestCacheTTL(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	cache := NewWithClock[string](time.Hour, func() time.Time { return now })
	cache.Set("key", "value")

	if got, ok := cache.Get("key"); !ok || got != "value" {
		t.Fatalf("Get() = %q, %v", got, ok)
	}
	now = now.Add(time.Hour)
	if _, ok := cache.Get("key"); ok {
		t.Fatal("到期边界仍然命中")
	}
}

func TestCacheConcurrentAccess(t *testing.T) {
	cache := New[int](time.Hour)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(value int) {
			defer wg.Done()
			cache.Set("key", value)
			cache.Get("key")
		}(i)
	}
	wg.Wait()
	if _, ok := cache.Get("key"); !ok {
		t.Fatal("并发写入后缓存未命中")
	}
}
