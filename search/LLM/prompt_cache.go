package llm

import (
	"fmt"
	"hash/crc32"
)

const (
	promptCacheShardCount   = 8
	promptCacheKeyMaxLength = 64
)

// Keep each workflow on one shard while distributing concurrent requests;
// OpenAI recommends approximately 15 requests per minute per cache key.
func BuildShardedPromptCacheKey(prefix string, shardValue string) string {
	shard := crc32.ChecksumIEEE([]byte(shardValue)) % promptCacheShardCount
	shardSuffix := fmt.Sprintf(":shard=%d", shard)
	if len(prefix)+len(shardSuffix) <= promptCacheKeyMaxLength {
		return prefix + shardSuffix
	}

	// Preserve the readable stage prefix and hash the truncated model/effort suffix.
	hashSuffix := fmt.Sprintf(":h=%08x", crc32.ChecksumIEEE([]byte(prefix)))
	prefix = prefix[:promptCacheKeyMaxLength-len(hashSuffix)-len(shardSuffix)]
	return prefix + hashSuffix + shardSuffix
}
