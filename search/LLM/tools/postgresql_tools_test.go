package tools

import (
	"testing"
	"time"
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

	cache.items["expired"] = postgreSQLToolCacheItem{
		value:     "old",
		expiresAt: time.Now().Add(-time.Second),
	}
	if _, ok := cache.get("expired"); ok {
		t.Fatalf("expected expired cache miss")
	}
	if _, exists := cache.items["expired"]; exists {
		t.Fatalf("expected expired cache item to be deleted")
	}
}

func TestPostgreSQLToolCacheDisabledWhenTTLIsZero(t *testing.T) {
	if cache := newPostgreSQLToolCache(0); cache != nil {
		t.Fatalf("expected nil cache when ttl is zero")
	}
}
