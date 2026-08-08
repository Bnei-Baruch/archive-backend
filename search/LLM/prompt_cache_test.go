package llm

import (
	"strings"
	"testing"
)

func TestBuildShardedPromptCacheKeyIsStableAndDistributed(t *testing.T) {
	prefix := "reasoning-search:m=gpt-5.6:e=low"
	if first, second := BuildShardedPromptCacheKey(prefix, "session-1"), BuildShardedPromptCacheKey(prefix, "session-1"); first != second {
		t.Fatalf("same session produced different keys: %q and %q", first, second)
	}

	keys := map[string]bool{}
	for _, sessionID := range []string{"session-1", "session-2", "session-3", "session-4", "session-5", "session-6", "session-7", "session-8"} {
		key := BuildShardedPromptCacheKey(prefix, sessionID)
		if !strings.HasPrefix(key, prefix+":shard=") {
			t.Fatalf("unexpected key %q", key)
		}
		keys[key] = true
	}
	if len(keys) < 2 {
		t.Fatalf("expected sessions to be distributed across shards, got %v", keys)
	}
}

func TestBuildShardedPromptCacheKeyLimitsLongKeys(t *testing.T) {
	prefix := "reasoning-search-rapid-classifier:m=gpt-5.6-luna:e=medium"
	key := BuildShardedPromptCacheKey(prefix, "session-1")
	if len(key) > promptCacheKeyMaxLength {
		t.Fatalf("prompt cache key is too long: %d characters in %q", len(key), key)
	}
	if !strings.HasPrefix(key, "reasoning-search-rapid-classifier:") {
		t.Fatalf("stage prefix was not preserved: %q", key)
	}

	other := BuildShardedPromptCacheKey(prefix+"-different", "session-1")
	if key == other {
		t.Fatalf("different long prefixes produced the same key: %q", key)
	}
}
