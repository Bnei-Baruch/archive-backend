package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/integration"
	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
)

const (
	sourceLookupByUIDWithLanguageQuery = `
SELECT f.uid
FROM files f
JOIN content_units cu ON cu.id = f.content_unit_id
WHERE cu.published IS TRUE
  AND cu.secure = $1
  AND f.secure = $2
  AND f.published IS TRUE
  AND f.removed_at IS NULL
  AND f.name LIKE '%.doc%'
  AND cu.uid = $3
  AND f.language = $4
LIMIT 1`

	sourceLookupByUIDFallbackQuery = `
SELECT f.uid
FROM files f
JOIN content_units cu ON cu.id = f.content_unit_id
WHERE cu.published IS TRUE
  AND cu.secure = $1
  AND f.secure = $2
  AND f.published IS TRUE
  AND f.removed_at IS NULL
  AND f.name LIKE '%.doc%'
  AND cu.uid = $3
ORDER BY
  CASE
    WHEN $4 <> '' AND f.language = $4 THEN 0
    WHEN f.language = 'en' THEN 1
    WHEN f.language = 'he' THEN 2
    ELSE 3
  END,
  f.uid
LIMIT 1`

	sourceLookupByMDBIDWithLanguageQuery = `
SELECT f.uid
FROM files f
JOIN content_units cu ON cu.id = f.content_unit_id
JOIN sources s ON s.uid = cu.uid
WHERE cu.published IS TRUE
  AND cu.secure = $1
  AND f.secure = $2
  AND f.published IS TRUE
  AND f.removed_at IS NULL
  AND f.name LIKE '%.doc%'
  AND s.id = $3
  AND f.language = $4
LIMIT 1`

	sourceLookupByMDBIDFallbackQuery = `
SELECT f.uid
FROM files f
JOIN content_units cu ON cu.id = f.content_unit_id
JOIN sources s ON s.uid = cu.uid
WHERE cu.published IS TRUE
  AND cu.secure = $1
  AND f.secure = $2
  AND f.published IS TRUE
  AND f.removed_at IS NULL
  AND f.name LIKE '%.doc%'
  AND s.id = $3
ORDER BY
  CASE
    WHEN $4 <> '' AND f.language = $4 THEN 0
    WHEN f.language = 'en' THEN 1
    WHEN f.language = 'he' THEN 2
    ELSE 3
  END,
  f.uid
LIMIT 1`
)

type SourceLookupTool struct {
	db            *sql.DB
	assetsService integration.AssetsService

	cacheMu sync.RWMutex
	cache   map[string]string
}

type SourceLookupToolArgs struct {
	SourceID string `json:"source_id,omitempty"`
	Language string `json:"language,omitempty"`
}

func NewSourceLookupTool(db *sql.DB, assetsService integration.AssetsService) *SourceLookupTool {
	return &SourceLookupTool{
		db:            db,
		assetsService: assetsService,
		cache:         map[string]string{},
	}
}

func (t *SourceLookupTool) Definition() llm.ReasoningToolDefinition {
	return llm.ReasoningToolDefinition{
		Name:        "source_lookup",
		Description: "Retrieve source document text from CDN by source_id.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"source_id": map[string]interface{}{
					"type":        "string",
					"description": "Source identifier (UID or numeric MDB source id).",
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "Preferred language code for the source document (optional).",
				},
			},
			"required":             []string{"source_id"},
			"additionalProperties": false,
		},
	}
}

func (t *SourceLookupTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if t.db == nil {
		return "", fmt.Errorf("source_lookup: db is nil")
	}
	if t.assetsService == nil {
		return "", fmt.Errorf("source_lookup: assets service is nil")
	}

	args := SourceLookupToolArgs{}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return "", fmt.Errorf("source_lookup: failed to parse arguments: %w", err)
	}

	sourceID := strings.TrimSpace(args.SourceID)
	if sourceID == "" {
		return "", fmt.Errorf("source_lookup: source_id is required")
	}
	language := strings.ToLower(strings.TrimSpace(args.Language))
	llm.LogIfDeb(ctx, "source_lookup: start source_id=%q language=%q", sourceID, language)

	cacheKey := sourceID + "|" + language
	if value, ok := t.getFromCache(cacheKey); ok {
		llm.LogIfDeb(ctx, "source_lookup: cache hit source_id=%q language=%q content_len=%d", sourceID, language, len(value))
		return value, nil
	}

	fileUID, err := t.resolveFileUID(sourceID, language)
	if err != nil {
		return "", err
	}
	if fileUID == "" {
		llm.LogIfDeb(ctx, "source_lookup: no file uid found source_id=%q language=%q", sourceID, language)
		t.setCache(cacheKey, "")
		return "", nil
	}
	llm.LogIfDeb(ctx, "source_lookup: resolved file uid source_id=%q language=%q file_uid=%q", sourceID, language, fileUID)

	content, err := t.assetsService.Doc2Text(fileUID)
	if err != nil {
		return "", fmt.Errorf("source_lookup: doc2text failed for file uid '%s': %w", fileUID, err)
	}

	t.setCache(cacheKey, content)
	llm.LogIfDeb(ctx, "source_lookup: completed source_id=%q language=%q file_uid=%q content_len=%d", sourceID, language, fileUID, len(content))
	return content, nil
}

func (t *SourceLookupTool) resolveFileUID(sourceID string, language string) (string, error) {
	if numericID, err := strconv.ParseInt(sourceID, 10, 64); err == nil {
		fileUID, err := t.resolveByMDBID(numericID, language)
		if err != nil {
			return "", err
		}
		if fileUID != "" {
			return fileUID, nil
		}
	}
	return t.resolveByUID(sourceID, language)
}

func (t *SourceLookupTool) resolveByUID(sourceUID string, language string) (string, error) {
	if language != "" {
		fileUID, err := t.queryFileUID(
			sourceLookupByUIDWithLanguageQuery,
			consts.SEC_PUBLIC,
			consts.SEC_PUBLIC,
			sourceUID,
			language,
		)
		if err != nil {
			return "", err
		}
		if fileUID != "" {
			return fileUID, nil
		}
	}
	return t.queryFileUID(
		sourceLookupByUIDFallbackQuery,
		consts.SEC_PUBLIC,
		consts.SEC_PUBLIC,
		sourceUID,
		language,
	)
}

func (t *SourceLookupTool) resolveByMDBID(sourceID int64, language string) (string, error) {
	if language != "" {
		fileUID, err := t.queryFileUID(
			sourceLookupByMDBIDWithLanguageQuery,
			consts.SEC_PUBLIC,
			consts.SEC_PUBLIC,
			sourceID,
			language,
		)
		if err != nil {
			return "", err
		}
		if fileUID != "" {
			return fileUID, nil
		}
	}
	return t.queryFileUID(
		sourceLookupByMDBIDFallbackQuery,
		consts.SEC_PUBLIC,
		consts.SEC_PUBLIC,
		sourceID,
		language,
	)
}

func (t *SourceLookupTool) queryFileUID(query string, args ...interface{}) (string, error) {
	fileUID := ""
	if err := t.db.QueryRow(query, args...).Scan(&fileUID); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("source_lookup: query failed: %w", err)
	}
	return fileUID, nil
}

func (t *SourceLookupTool) getFromCache(cacheKey string) (string, bool) {
	t.cacheMu.RLock()
	defer t.cacheMu.RUnlock()
	value, ok := t.cache[cacheKey]
	return value, ok
}

func (t *SourceLookupTool) setCache(cacheKey string, value string) {
	t.cacheMu.Lock()
	defer t.cacheMu.Unlock()
	t.cache[cacheKey] = value
}
