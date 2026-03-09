package tools

import (
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

func (t *TranscriptLookupTool) Execute(arguments json.RawMessage) (string, error) {
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

	cacheKey := contentUnitID + "|" + language
	if value, ok := t.getFromCache(cacheKey); ok {
		if value == "" {
			return "", transcriptNotFoundError(contentUnitID, language)
		}
		return value, nil
	}

	fileUID, err := t.resolveTranscriptFileUID(contentUnitID, language, kiteiMakorType.ID)
	if err != nil {
		return "", err
	}
	if fileUID == "" {
		t.setCache(cacheKey, "")
		return "", transcriptNotFoundError(contentUnitID, language)
	}

	content, err := t.assetsService.Doc2Text(fileUID)
	if err != nil {
		return "", fmt.Errorf("transcript_lookup: doc2text failed for file uid '%s': %w", fileUID, err)
	}

	t.setCache(cacheKey, content)
	return content, nil
}

func transcriptNotFoundError(contentUnitID string, language string) error {
	if language != "" {
		return fmt.Errorf("transcript_lookup: transcript not found for content_unit_id '%s' and language '%s'", contentUnitID, language)
	}
	return fmt.Errorf("transcript_lookup: transcript not found for content_unit_id '%s'", contentUnitID)
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
