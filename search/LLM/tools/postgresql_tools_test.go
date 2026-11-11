package tools

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPostgreSQLToolCacheGetSetAndExpire(t *testing.T) {
	cache := newPostgreSQLToolCache(time.Minute)

	cache.set("key", "value")
	value, ok := cache.get("key")
	if !ok {
		t.Fatalf("expected cache hit")
	}
	if value != "value" {
		t.Fatalf("unexpected cache value: %q", value)
	}

	initialBytes := cache.bytes
	cache.set("expired", "old")
	item := cache.items["expired"]
	item.expiresAt = time.Now().Add(-time.Second)
	cache.items["expired"] = item
	if _, ok := cache.get("expired"); ok {
		t.Fatalf("expected expired cache miss")
	}
	if _, exists := cache.items["expired"]; exists {
		t.Fatalf("expected expired cache item to be deleted")
	}
	require.Equal(t, initialBytes, cache.bytes)
	cache.set("expired", "fresh")
	value, ok = cache.get("expired")
	require.True(t, ok)
	require.Equal(t, "fresh", value)
}

func TestPostgreSQLToolCacheDisabledWhenTTLIsZero(t *testing.T) {
	if cache := newPostgreSQLToolCache(0); cache != nil {
		t.Fatalf("expected nil cache when ttl is zero")
	}
	var cache *postgreSQLToolCache
	cache.set("key", "value")
	_, ok := cache.get("key")
	require.False(t, ok)
}

func TestPostgreSQLToolCacheByteBudget(t *testing.T) {
	cache := newPostgreSQLToolCache(time.Minute)
	require.Equal(t, maxPostgreSQLToolCacheBytes, cache.maxBytes)
	cache.maxBytes = 512
	for i := 0; i < 30; i++ {
		cache.set(fmt.Sprint(i), strings.Repeat("x", 50))
		require.LessOrEqual(t, cache.bytes, cache.maxBytes)
	}
	cache.set("replacement", "old")
	before := cache.bytes
	cache.set("replacement", "new value")
	require.Equal(t, before-len("old")+len("new value"), cache.bytes)
	value, ok := cache.get("replacement")
	require.True(t, ok)
	require.Equal(t, "new value", value)

	previousSize := cache.items["replacement"].sizeBytes
	before = cache.bytes
	cache.set("replacement", strings.Repeat("x", cache.maxBytes))
	_, ok = cache.get("replacement")
	require.False(t, ok)
	require.Equal(t, before-previousSize, cache.bytes)
	cache.set(strings.Repeat("k", cache.maxBytes), "value")
	require.Equal(t, before-previousSize, cache.bytes, "keys must count toward the byte budget")
	for i := 0; i < 30; i++ {
		cache.set(fmt.Sprint(i), "")
		require.LessOrEqual(t, cache.bytes, cache.maxBytes)
	}
	require.LessOrEqual(t, len(cache.items), cache.maxBytes/128)
	bytes := 0
	for _, item := range cache.items {
		bytes += item.sizeBytes
	}
	require.Equal(t, bytes, cache.bytes)
}

func TestPostgreSQLToolCacheConcurrentGetSetAndExpire(t *testing.T) {
	for _, ttl := range []time.Duration{time.Minute, time.Nanosecond} {
		t.Run(ttl.String(), func(t *testing.T) {
			cache := newPostgreSQLToolCache(ttl)
			cache.maxBytes = 1024
			var workers sync.WaitGroup
			for i := 0; i < 8; i++ {
				workers.Add(1)
				go func() {
					defer workers.Done()
					for n := 0; n < 100; n++ {
						key := fmt.Sprint(n % 12)
						cache.set(key, "value")
						cache.get(key)
					}
				}()
			}
			workers.Wait()
			require.GreaterOrEqual(t, cache.bytes, 0)
			require.LessOrEqual(t, cache.bytes, cache.maxBytes)
			bytes := 0
			for _, item := range cache.items {
				bytes += item.sizeBytes
			}
			require.Equal(t, bytes, cache.bytes)
		})
	}
}
