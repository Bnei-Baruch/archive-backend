package tools

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAIQueryDocumentCacheRechecksAccessAndCachesNotFound(t *testing.T) {
	cache := newAIQueryDocumentCache()
	fileUID := "public-file"
	lookups, fetches := 0, 0
	resolve := func() (string, error) { lookups++; return fileUID, nil }
	fetch := func(uid string) (string, error) { fetches++; return "text of " + uid, nil }
	first, err := loadAIQueryDocumentEntry(&cache, "unit|he", resolve, fetch)
	require.NoError(t, err)
	second, err := loadAIQueryDocumentEntry(&cache, "unit|he", resolve, fetch)
	require.NoError(t, err)
	require.Equal(t, first.Content, second.Content)
	require.Equal(t, 2, lookups)
	require.Equal(t, 1, fetches)

	// The file is no longer public. Cached text must not bypass the lookup.
	fileUID = ""
	missing, err := loadAIQueryDocumentEntry(&cache, "unit|he", resolve, fetch)
	require.NoError(t, err)
	require.Empty(t, missing.FileUID)
	require.Empty(t, missing.Content)
	require.Equal(t, 3, lookups)
	require.Equal(t, 1, fetches)

	// Newly published content becomes visible when the negative entry expires.
	fileUID = "new-file"
	missing, err = loadAIQueryDocumentEntry(&cache, "unit|he", resolve, fetch)
	require.NoError(t, err)
	require.Empty(t, missing.Content)
	require.Equal(t, 3, lookups)
	cache.entries["unit|he"].expiresAt = time.Now().Add(-time.Second)
	published, err := loadAIQueryDocumentEntry(&cache, "unit|he", resolve, fetch)
	require.NoError(t, err)
	require.Equal(t, "text of new-file", published.Content)
	require.Equal(t, 4, lookups)
	require.Equal(t, 2, fetches)
}

func TestAIQueryDocumentCacheRefetchesChangedAndExpiredFiles(t *testing.T) {
	cache := newAIQueryDocumentCache()
	fileUID := "first-file"
	fetches := 0
	resolve := func() (string, error) { return fileUID, nil }
	fetch := func(uid string) (string, error) { fetches++; return "text of " + uid, nil }
	_, err := loadAIQueryDocumentEntry(&cache, "unit|he", resolve, fetch)
	require.NoError(t, err)
	fileUID = "replacement-file"
	changed, err := loadAIQueryDocumentEntry(&cache, "unit|he", resolve, fetch)
	require.NoError(t, err)
	require.Equal(t, "text of replacement-file", changed.Content)
	require.Equal(t, 2, fetches)
	cache.entries["unit|he"].expiresAt = time.Now().Add(-time.Second)
	_, err = loadAIQueryDocumentEntry(&cache, "unit|he", resolve, fetch)
	require.NoError(t, err)
	require.Equal(t, 3, fetches)
}

func TestAIQueryDocumentCacheDoesNotCacheErrors(t *testing.T) {
	for _, failure := range []string{"lookup", "fetch", "lookup with cached text"} {
		t.Run(failure, func(t *testing.T) {
			cache := newAIQueryDocumentCache()
			if failure == "lookup with cached text" {
				cache.set("unit", &aiQueryDocumentCacheEntry{FileUID: "file", Content: "cached"})
			}
			fail := true
			lookups, fetches := 0, 0
			resolve := func() (string, error) {
				lookups++
				if fail && strings.HasPrefix(failure, "lookup") {
					return "", errors.New("SQL unavailable")
				}
				return "file", nil
			}
			fetch := func(string) (string, error) {
				fetches++
				if fail {
					return "", errors.New("assets unavailable")
				}
				return "text", nil
			}
			entry, err := loadAIQueryDocumentEntry(&cache, "unit", resolve, fetch)
			require.Error(t, err)
			require.Nil(t, entry)
			fail = false
			entry, err = loadAIQueryDocumentEntry(&cache, "unit", resolve, fetch)
			require.NoError(t, err)
			require.Equal(t, "file", entry.FileUID)
			require.Equal(t, 2, lookups)
		})
	}
}

func TestAIQueryDocumentCacheExpiryAndByteBudget(t *testing.T) {
	cache := newAIQueryDocumentCache()
	require.Equal(t, aiQueryDocumentCacheMaxBytes, cache.maxBytes)
	cache.maxBytes = 512
	before := time.Now()
	value := &aiQueryDocumentCacheEntry{FileUID: "file", Content: "text", Chunks: []string{"chunk"}}
	cache.set("key", value)
	require.True(t, value.expiresAt.IsZero(), "inserting must not mutate a shared entry")
	entry, ok := cache.get("key")
	require.True(t, ok)
	require.WithinDuration(t, before.Add(aiQueryDocumentCacheTTL), entry.expiresAt, time.Second)
	cache.set("key", value)
	require.Equal(t, entry.sizeBytes, cache.bytes, "replacement must not double-count bytes")
	cache.set("missing", &aiQueryDocumentCacheEntry{})
	entry, ok = cache.get("missing")
	require.True(t, ok)
	require.WithinDuration(t, before.Add(aiQueryDocumentNotFoundCacheTTL), entry.expiresAt, time.Second)
	entry.expiresAt = time.Now().Add(-time.Second)
	_, ok = cache.get("missing")
	require.False(t, ok)

	for i := 0; i < 20; i++ {
		cache.set(fmt.Sprint(i), &aiQueryDocumentCacheEntry{})
		require.LessOrEqual(t, cache.bytes, cache.maxBytes)
	}
	require.LessOrEqual(t, len(cache.entries), cache.maxBytes/128, "empty entries must also be bounded")
	bytes := 0
	for _, cached := range cache.entries {
		bytes += cached.sizeBytes
	}
	require.Equal(t, bytes, cache.bytes)
	cache.set("oversized", &aiQueryDocumentCacheEntry{FileUID: "file", Content: strings.Repeat("x", cache.maxBytes)})
	_, ok = cache.get("oversized")
	require.False(t, ok)
	cache.set("chunks", &aiQueryDocumentCacheEntry{FileUID: "file", Chunks: []string{strings.Repeat("x", cache.maxBytes)}})
	_, ok = cache.get("chunks")
	require.False(t, ok, "chunk data must count toward the byte budget")
}

func TestAIQueryDocumentCacheConcurrentAccess(t *testing.T) {
	cache := newAIQueryDocumentCache()
	cache.maxBytes = 1024
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for n := 0; n < 100; n++ {
				key := fmt.Sprint(n % 12)
				cache.set(key, &aiQueryDocumentCacheEntry{FileUID: "file", Content: "text", Chunks: []string{"chunk"}})
				cache.get(key)
			}
		}()
	}
	workers.Wait()
	require.LessOrEqual(t, cache.bytes, cache.maxBytes)
	bytes := 0
	for _, cached := range cache.entries {
		bytes += cached.sizeBytes
	}
	require.Equal(t, bytes, cache.bytes)
}
