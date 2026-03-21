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
	"github.com/Bnei-Baruch/archive-backend/mdb"
	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
)

const (
	transcriptLookupByUIDWithLanguageQuery = `
SELECT f.uid
FROM files AS f
INNER JOIN content_units AS cu ON f.content_unit_id = cu.id
WHERE f.secure = 0
  AND f.published = TRUE
  AND f.name ~ '.docx?'
  AND f.name NOT LIKE '%tzitutim.do%'
  AND f.name NOT LIKE '%quotation.do%'
  AND f.language NOT IN ('zz', 'xx')
  AND f.content_unit_id IS NOT NULL
  AND cu.type_id != $1
  AND cu.uid = $2
  AND f.language = $3
LIMIT 1`

	transcriptLookupByUIDFallbackQuery = `
SELECT f.uid
FROM files AS f
INNER JOIN content_units AS cu ON f.content_unit_id = cu.id
WHERE f.secure = 0
  AND f.published = TRUE
  AND f.name ~ '.docx?'
  AND f.name NOT LIKE '%tzitutim.do%'
  AND f.name NOT LIKE '%quotation.do%'
  AND f.language NOT IN ('zz', 'xx')
  AND f.content_unit_id IS NOT NULL
  AND cu.type_id != $1
  AND cu.uid = $2
ORDER BY
  CASE
    WHEN $3 <> '' AND f.language = $3 THEN 0
    WHEN f.language = 'en' THEN 1
    WHEN f.language = 'he' THEN 2
    ELSE 3
  END,
  f.uid
LIMIT 1`

	transcriptLookupByMDBIDWithLanguageQuery = `
SELECT f.uid
FROM files AS f
INNER JOIN content_units AS cu ON f.content_unit_id = cu.id
WHERE f.secure = 0
  AND f.published = TRUE
  AND f.name ~ '.docx?'
  AND f.name NOT LIKE '%tzitutim.do%'
  AND f.name NOT LIKE '%quotation.do%'
  AND f.language NOT IN ('zz', 'xx')
  AND f.content_unit_id IS NOT NULL
  AND cu.type_id != $1
  AND cu.id = $2
  AND f.language = $3
LIMIT 1`

	transcriptLookupByMDBIDFallbackQuery = `
SELECT f.uid
FROM files AS f
INNER JOIN content_units AS cu ON f.content_unit_id = cu.id
WHERE f.secure = 0
  AND f.published = TRUE
  AND f.name ~ '.docx?'
  AND f.name NOT LIKE '%tzitutim.do%'
  AND f.name NOT LIKE '%quotation.do%'
  AND f.language NOT IN ('zz', 'xx')
  AND f.content_unit_id IS NOT NULL
  AND cu.type_id != $1
  AND cu.id = $2
ORDER BY
  CASE
    WHEN $3 <> '' AND f.language = $3 THEN 0
    WHEN f.language = 'en' THEN 1
    WHEN f.language = 'he' THEN 2
    ELSE 3
  END,
  f.uid
LIMIT 1`
)

type TranscriptLookupTool struct {
	db            *sql.DB
	assetsService integration.AssetsService

	cacheMu sync.RWMutex
	cache   map[string]string
}

type TranscriptLookupToolArgs struct {
	ContentUnitID string `json:"content_unit_id,omitempty"`
	Language      string `json:"language,omitempty"`
}

func NewTranscriptLookupTool(db *sql.DB, assetsService integration.AssetsService) *TranscriptLookupTool {
	return &TranscriptLookupTool{
		db:            db,
		assetsService: assetsService,
		cache:         map[string]string{},
	}
}

func (t *TranscriptLookupTool) Definition() llm.ReasoningToolDefinition {
	return llm.ReasoningToolDefinition{
		Name:        "transcript_lookup",
		Description: "Retrieve transcript text by content_unit_id.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content_unit_id": map[string]interface{}{
					"type":        "string",
					"description": "Content unit identifier (UID or numeric MDB id).",
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "Preferred transcript language code (optional).",
				},
			},
			"required":             []string{"content_unit_id"},
			"additionalProperties": false,
		},
	}
}

func (t *TranscriptLookupTool) UsageExplanation() string {
	return `Tool: transcript_lookup
	Use this tool to retrieve the text content of a transcript related to some content unit from the CDN using its identifier. This is useful when the highlights of the search results are not sufficient and you need to access the full text of the transcript document for better understanding.
Arguments:
- content_unit_id: required. Content unit UID or numeric MDB id.
- language: optional preferred transcript language code.
Behavior:
- Looks up a public transcript document for the content unit, converts it to plain text with Doc2Text, and returns the text.
- If language is omitted or not found, the tool falls back by language preference.
- If no transcript exists, it returns an acknowledgment message so the agent can continue with other tools.
- If a runtime lookup or text-retrieval error happens, it also returns an acknowledgment message so the agent can continue.`
}

func (t *TranscriptLookupTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if t.db == nil {
		return "", fmt.Errorf("transcript_lookup: db is nil")
	}
	if t.assetsService == nil {
		return "", fmt.Errorf("transcript_lookup: assets service is nil")
	}

	kiteiMakorType, ok := mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_KITEI_MAKOR]
	if !ok {
		return "", fmt.Errorf("transcript_lookup: content type '%s' is unavailable", consts.CT_KITEI_MAKOR)
	}

	args := TranscriptLookupToolArgs{}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return "", fmt.Errorf("transcript_lookup: failed to parse arguments: %w", err)
	}

	contentUnitID := strings.TrimSpace(args.ContentUnitID)
	if contentUnitID == "" {
		return "", fmt.Errorf("transcript_lookup: content_unit_id is required")
	}
	language := strings.ToLower(strings.TrimSpace(args.Language))
	llm.LogIfDeb(ctx, "transcript_lookup: start content_unit_id=%q language=%q", contentUnitID, language)

	cacheKey := contentUnitID + "|" + language
	if value, ok := t.getFromCache(cacheKey); ok {
		llm.LogIfDeb(ctx, "transcript_lookup: cache hit content_unit_id=%q language=%q content_len=%d", contentUnitID, language, len(value))
		return value, nil
	}

	fileUID, err := t.resolveTranscriptFileUID(contentUnitID, language, kiteiMakorType.ID)
	if err != nil {
		llm.LogIfDeb(ctx, "transcript_lookup: lookup fallback content_unit_id=%q language=%q err=%v", contentUnitID, language, err)
		return lookupToolErrorOutput("transcript_lookup", err), nil
	}
	if fileUID == "" {
		content := transcriptNotFoundToolOutput(contentUnitID, language)
		llm.LogIfDeb(ctx, "transcript_lookup: no transcript file found content_unit_id=%q language=%q", contentUnitID, language)
		return content, nil
	}
	llm.LogIfDeb(ctx, "transcript_lookup: resolved file uid content_unit_id=%q language=%q file_uid=%q", contentUnitID, language, fileUID)

	content, err := t.assetsService.Doc2Text(fileUID)
	if err != nil {
		content = doc2TextToolOutput("transcript_lookup", fileUID, err)
		llm.LogIfDeb(ctx, "transcript_lookup: doc2text fallback content_unit_id=%q language=%q file_uid=%q err=%v", contentUnitID, language, fileUID, err)
		return content, nil
	}

	t.setCache(cacheKey, content)
	llm.LogIfDeb(ctx, "transcript_lookup: completed content_unit_id=%q language=%q file_uid=%q content_len=%d", contentUnitID, language, fileUID, len(content))
	return content, nil
}

func (t *TranscriptLookupTool) resolveTranscriptFileUID(contentUnitID string, language string, excludedTypeID int64) (string, error) {
	if numericID, err := strconv.ParseInt(contentUnitID, 10, 64); err == nil {
		fileUID, err := t.resolveByMDBID(numericID, language, excludedTypeID)
		if err != nil {
			return "", err
		}
		if fileUID != "" {
			return fileUID, nil
		}
	}
	return t.resolveByUID(contentUnitID, language, excludedTypeID)
}

func (t *TranscriptLookupTool) resolveByUID(contentUnitUID string, language string, excludedTypeID int64) (string, error) {
	if language != "" {
		fileUID, err := t.queryFileUID(
			transcriptLookupByUIDWithLanguageQuery,
			excludedTypeID,
			contentUnitUID,
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
		transcriptLookupByUIDFallbackQuery,
		excludedTypeID,
		contentUnitUID,
		language,
	)
}

func (t *TranscriptLookupTool) resolveByMDBID(contentUnitID int64, language string, excludedTypeID int64) (string, error) {
	if language != "" {
		fileUID, err := t.queryFileUID(
			transcriptLookupByMDBIDWithLanguageQuery,
			excludedTypeID,
			contentUnitID,
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
		transcriptLookupByMDBIDFallbackQuery,
		excludedTypeID,
		contentUnitID,
		language,
	)
}

func (t *TranscriptLookupTool) queryFileUID(query string, args ...interface{}) (string, error) {
	fileUID := ""
	if err := t.db.QueryRow(query, args...).Scan(&fileUID); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("transcript_lookup: query failed: %w", err)
	}
	return fileUID, nil
}

func (t *TranscriptLookupTool) getFromCache(cacheKey string) (string, bool) {
	t.cacheMu.RLock()
	defer t.cacheMu.RUnlock()
	value, ok := t.cache[cacheKey]
	return value, ok
}

func (t *TranscriptLookupTool) setCache(cacheKey string, value string) {
	t.cacheMu.Lock()
	defer t.cacheMu.Unlock()
	t.cache[cacheKey] = value
}
