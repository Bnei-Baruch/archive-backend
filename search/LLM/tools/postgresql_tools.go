package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Bnei-Baruch/archive-backend/consts"
	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
)

const (
	defaultPostgreSQLToolLimit    = 25
	maxPostgreSQLToolLimit        = 100
	defaultPostgreSQLToolCacheTTL = 3 * time.Hour
)

const authorMetadataByMDBIDQuery = `
SELECT
	a.id,
	a.code,
	COALESCE(
		(SELECT name FROM author_i18n WHERE author_id = a.id AND language = $1),
		(SELECT name FROM author_i18n WHERE author_id = a.id AND language = 'en'),
		(SELECT name FROM author_i18n WHERE author_id = a.id AND language = 'he'),
		a.name,
		''
	) AS name,
	COALESCE(
		(SELECT full_name FROM author_i18n WHERE author_id = a.id AND language = $1),
		(SELECT full_name FROM author_i18n WHERE author_id = a.id AND language = 'en'),
		(SELECT full_name FROM author_i18n WHERE author_id = a.id AND language = 'he'),
		a.full_name,
		''
	) AS full_name
FROM authors a
WHERE a.id = $2
LIMIT 1`

const authorMetadataByCodeQuery = `
SELECT
	a.id,
	a.code,
	COALESCE(
		(SELECT name FROM author_i18n WHERE author_id = a.id AND language = $1),
		(SELECT name FROM author_i18n WHERE author_id = a.id AND language = 'en'),
		(SELECT name FROM author_i18n WHERE author_id = a.id AND language = 'he'),
		a.name,
		''
	) AS name,
	COALESCE(
		(SELECT full_name FROM author_i18n WHERE author_id = a.id AND language = $1),
		(SELECT full_name FROM author_i18n WHERE author_id = a.id AND language = 'en'),
		(SELECT full_name FROM author_i18n WHERE author_id = a.id AND language = 'he'),
		a.full_name,
		''
	) AS full_name
FROM authors a
WHERE a.code = $2
LIMIT 1`

const sourcesByAuthorQuery = `
SELECT
	s.id,
	s.uid,
	COALESCE(parent.uid, '') AS parent_uid,
	COALESCE(st.name, '') AS source_type,
	COALESCE(
		(SELECT name FROM source_i18n WHERE source_id = s.id AND language = $1),
		(SELECT name FROM source_i18n WHERE source_id = s.id AND language = 'en'),
		(SELECT name FROM source_i18n WHERE source_id = s.id AND language = 'he'),
		''
	) AS name,
	COALESCE(
		(SELECT description FROM source_i18n WHERE source_id = s.id AND language = $1),
		(SELECT description FROM source_i18n WHERE source_id = s.id AND language = 'en'),
		(SELECT description FROM source_i18n WHERE source_id = s.id AND language = 'he'),
		''
	) AS description,
	COALESCE(s.properties->>'year', '') AS year,
	COALESCE(s.properties->>'number', '') AS number,
	COALESCE(s.position, 0) AS position
FROM authors_sources aus
INNER JOIN sources s ON s.id = aus.source_id
LEFT JOIN sources parent ON parent.id = s.parent_id
LEFT JOIN source_types st ON st.id = s.type_id
WHERE aus.author_id = $2
  AND (
    EXISTS (
      SELECT 1
      FROM content_units cu
      WHERE cu.uid = s.uid
        AND cu.secure = $3
        AND cu.published IS TRUE
    )
    OR EXISTS (
      SELECT 1
      FROM sources child
      WHERE child.parent_id = s.id
    )
  )
ORDER BY s.parent_id NULLS FIRST, s.position ASC, s.id ASC
LIMIT $4`

const sourceSelectQuery = `
SELECT
	s.id,
	s.uid,
	COALESCE(parent.uid, '') AS parent_uid,
	COALESCE(st.name, '') AS source_type,
	COALESCE(
		(SELECT name FROM source_i18n WHERE source_id = s.id AND language = $1),
		(SELECT name FROM source_i18n WHERE source_id = s.id AND language = 'en'),
		(SELECT name FROM source_i18n WHERE source_id = s.id AND language = 'he'),
		''
	) AS name,
	COALESCE(
		(SELECT description FROM source_i18n WHERE source_id = s.id AND language = $1),
		(SELECT description FROM source_i18n WHERE source_id = s.id AND language = 'en'),
		(SELECT description FROM source_i18n WHERE source_id = s.id AND language = 'he'),
		''
	) AS description,
	COALESCE(s.properties->>'year', '') AS year,
	COALESCE(s.properties->>'number', '') AS number,
	COALESCE(s.position, 0) AS position,
	EXISTS (
		SELECT 1
		FROM sources child
		WHERE child.parent_id = s.id
	) AS has_children
FROM sources s
LEFT JOIN sources parent ON parent.id = s.parent_id
LEFT JOIN source_types st ON st.id = s.type_id
`

const sourceByMDBIDQuery = sourceSelectQuery + `
WHERE s.id = $2
LIMIT 1`

const sourceByUIDQuery = sourceSelectQuery + `
WHERE s.uid = $2
LIMIT 1`

const sourcesBySourceQuery = sourceSelectQuery + `
WHERE s.parent_id = $2
  AND (
    EXISTS (
      SELECT 1
      FROM content_units cu
      WHERE cu.uid = s.uid
        AND cu.secure = $3
        AND cu.published IS TRUE
    )
    OR EXISTS (
      SELECT 1
      FROM sources child
      WHERE child.parent_id = s.id
    )
  )
ORDER BY s.position ASC, s.id ASC
LIMIT $4`

const availableBooksQuery = `
SELECT
	a.code AS author_code,
	COALESCE(
		(SELECT name FROM author_i18n WHERE author_id = a.id AND language = 'en'),
		a.name,
		''
	) AS author_name_en,
	COALESCE(
		(SELECT name FROM author_i18n WHERE author_id = a.id AND language = 'he'),
		a.name,
		''
	) AS author_name_he,
	s.uid AS source_uid,
	COALESCE(parent.uid, '') AS parent_uid,
	COALESCE(grandparent.uid, '') AS grandparent_uid,
	COALESCE(
		(SELECT name FROM source_i18n WHERE source_id = s.id AND language = 'en'),
		''
	) AS source_name_en,
	COALESCE(
		(SELECT name FROM source_i18n WHERE source_id = s.id AND language = 'he'),
		''
	) AS source_name_he,
	COALESCE(
		(SELECT name FROM source_i18n WHERE source_id = parent.id AND language = 'en'),
		''
	) AS parent_name_en,
	COALESCE(
		(SELECT name FROM source_i18n WHERE source_id = parent.id AND language = 'he'),
		''
	) AS parent_name_he
FROM authors a
INNER JOIN authors_sources aus ON aus.author_id = a.id
INNER JOIN sources s ON s.id = aus.source_id
LEFT JOIN sources parent ON parent.id = s.parent_id
LEFT JOIN sources grandparent ON grandparent.id = parent.parent_id
WHERE (
	EXISTS (
		SELECT 1
		FROM content_units cu
		WHERE cu.uid = s.uid
		  AND cu.secure = $1
		  AND cu.published IS TRUE
	)
	OR EXISTS (
		SELECT 1
		FROM sources child
		WHERE child.parent_id = s.id
	)
)
ORDER BY a.id ASC, s.parent_id NULLS FIRST, s.position ASC, s.id ASC`

const collectionsSelectQuery = `
SELECT
	c.id,
	c.uid,
	COALESCE(ct.name, '') AS content_type,
	COALESCE(
		(SELECT name FROM collection_i18n WHERE collection_id = c.id AND language = $1),
		(SELECT name FROM collection_i18n WHERE collection_id = c.id AND language = 'en'),
		(SELECT name FROM collection_i18n WHERE collection_id = c.id AND language = 'he'),
		''
	) AS name,
	COALESCE(
		(SELECT description FROM collection_i18n WHERE collection_id = c.id AND language = $1),
		(SELECT description FROM collection_i18n WHERE collection_id = c.id AND language = 'en'),
		(SELECT description FROM collection_i18n WHERE collection_id = c.id AND language = 'he'),
		''
	) AS description,
	COALESCE(c.properties->>'film_date', '') AS film_date,
	COALESCE(c.properties->>'start_date', '') AS start_date,
	COALESCE(c.properties->>'end_date', '') AS end_date,
	COALESCE(c.properties->>'source', '') AS source_id,
	COALESCE(c.properties->>'number', '') AS number,
	(
		SELECT COUNT(*)
		FROM collections_content_units ccu
		INNER JOIN content_units cu ON cu.id = ccu.content_unit_id
		WHERE ccu.collection_id = c.id
		  AND cu.secure = $2
		  AND cu.published IS TRUE
	) AS content_units_count
FROM collections c
LEFT JOIN content_types ct ON ct.id = c.type_id
`

const collectionByMDBIDQuery = collectionsSelectQuery + `
WHERE c.secure = $2
  AND c.published IS TRUE
  AND c.id = $3
LIMIT 1`

const collectionByUIDQuery = collectionsSelectQuery + `
WHERE c.secure = $2
  AND c.published IS TRUE
  AND c.uid = $3
LIMIT 1`

const contentUnitsByCollectionQuery = `
SELECT
	cu.id,
	cu.uid,
	COALESCE(ct.name, '') AS content_type,
	COALESCE(
		(SELECT name FROM content_unit_i18n WHERE content_unit_id = cu.id AND language = $1),
		(SELECT name FROM content_unit_i18n WHERE content_unit_id = cu.id AND language = 'en'),
		(SELECT name FROM content_unit_i18n WHERE content_unit_id = cu.id AND language = 'he'),
		''
	) AS name,
	COALESCE(
		(SELECT description FROM content_unit_i18n WHERE content_unit_id = cu.id AND language = $1),
		(SELECT description FROM content_unit_i18n WHERE content_unit_id = cu.id AND language = 'en'),
		(SELECT description FROM content_unit_i18n WHERE content_unit_id = cu.id AND language = 'he'),
		''
	) AS description,
	ccu.name AS name_in_collection,
	ccu.position,
	COALESCE(cu.properties->>'film_date', '') AS film_date,
	COALESCE(cu.properties->>'original_language', '') AS original_language,
	COALESCE(NULLIF(cu.properties->>'duration', ''), '0')::double precision AS duration
FROM collections_content_units ccu
INNER JOIN content_units cu ON cu.id = ccu.content_unit_id
INNER JOIN collections c ON c.id = ccu.collection_id
LEFT JOIN content_types ct ON ct.id = cu.type_id
WHERE ccu.collection_id = $2
  AND c.secure = $3
  AND c.published IS TRUE
  AND cu.secure = $3
  AND cu.published IS TRUE
ORDER BY ccu.position ASC, COALESCE(NULLIF(cu.properties->>'film_date', '')::date, cu.created_at::date) DESC, cu.created_at DESC
LIMIT $4`

const collectionUIDsByContentUnitMDBIDQuery = `
SELECT DISTINCT c.uid
FROM collections c
INNER JOIN collections_content_units ccu ON c.id = ccu.collection_id
INNER JOIN content_units cu ON cu.id = ccu.content_unit_id
WHERE c.secure = $1
  AND c.published IS TRUE
  AND cu.secure = $1
  AND cu.published IS TRUE
  AND cu.id = $2
ORDER BY c.uid ASC
LIMIT 5`

const collectionUIDsByContentUnitUIDQuery = `
SELECT DISTINCT c.uid
FROM collections c
INNER JOIN collections_content_units ccu ON c.id = ccu.collection_id
INNER JOIN content_units cu ON cu.id = ccu.content_unit_id
WHERE c.secure = $1
  AND c.published IS TRUE
  AND cu.secure = $1
  AND cu.published IS TRUE
  AND cu.uid = $2
ORDER BY c.uid ASC
LIMIT 5`

type GetSourcesByAuthorTool struct {
	db    *sql.DB
	cache *postgreSQLToolCache
}

type GetAvailableBooksTool struct {
	db    *sql.DB
	cache *postgreSQLToolCache
}

type GetSourcesBySourceTool struct {
	db    *sql.DB
	cache *postgreSQLToolCache
}

type GetCollectionsTool struct {
	db    *sql.DB
	cache *postgreSQLToolCache
}

type GetContentUnitsByCollectionTool struct {
	db    *sql.DB
	cache *postgreSQLToolCache
}

type postgreSQLToolCache struct {
	mu    sync.RWMutex
	ttl   time.Duration
	items map[string]postgreSQLToolCacheItem
}

type postgreSQLToolCacheItem struct {
	value     string
	expiresAt time.Time
}

type postgreSQLToolRecoverableResult struct {
	Error                  string   `json:"error,omitempty"`
	RetrySuggested         bool     `json:"retry_suggested,omitempty"`
	Guidance               string   `json:"guidance,omitempty"`
	SuggestedCollectionIDs []string `json:"suggested_collection_ids,omitempty"`
}

type getSourcesByAuthorArgs struct {
	AuthorID string `json:"author_id,omitempty"`
	Language string `json:"language,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

type availableBookToolItem struct {
	AuthorID string `json:"author_id"`
	AuthorEN string `json:"author_en"`
	AuthorHE string `json:"author_he"`
	SourceID string `json:"source_id"`
	SourceEN string `json:"source_en"`
	SourceHE string `json:"source_he"`
}

type availableBooksToolResult struct {
	ReturnedCount int                     `json:"returned_count"`
	Items         []availableBookToolItem `json:"items"`
}

type availableBookCandidate struct {
	AuthorID       string
	AuthorEN       string
	AuthorHE       string
	SourceID       string
	ParentUID      string
	GrandparentUID string
	SourceEN       string
	SourceHE       string
	ParentEN       string
	ParentHE       string
}

type getSourcesBySourceArgs struct {
	SourceID string `json:"source_id,omitempty"`
	Language string `json:"language,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

type getCollectionsArgs struct {
	CollectionID string `json:"collection_id,omitempty"`
	ContentType  string `json:"content_type,omitempty"`
	Language     string `json:"language,omitempty"`
	Limit        int    `json:"limit,omitempty"`
}

type getContentUnitsByCollectionArgs struct {
	CollectionID  string `json:"collection_id,omitempty"`
	ContentUnitID string `json:"content_unit_id,omitempty"`
	Language      string `json:"language,omitempty"`
	Limit         int    `json:"limit,omitempty"`
}

type authorToolResult struct {
	MDBID    int64  `json:"mdb_id"`
	Code     string `json:"code"`
	Name     string `json:"name,omitempty"`
	FullName string `json:"full_name,omitempty"`
}

type sourceToolResult struct {
	MDBID       int64  `json:"mdb_id"`
	UID         string `json:"uid"`
	ParentUID   string `json:"parent_uid,omitempty"`
	Type        string `json:"type,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Year        string `json:"year,omitempty"`
	Number      string `json:"number,omitempty"`
	Position    int    `json:"position"`
}

type sourcesByAuthorToolResult struct {
	Author        *authorToolResult  `json:"author,omitempty"`
	ReturnedCount int                `json:"returned_count"`
	Items         []sourceToolResult `json:"items"`
}

type sourceNodeToolResult struct {
	MDBID       int64  `json:"mdb_id"`
	UID         string `json:"uid"`
	ParentUID   string `json:"parent_uid,omitempty"`
	Type        string `json:"type,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Year        string `json:"year,omitempty"`
	Number      string `json:"number,omitempty"`
	Position    int    `json:"position"`
	HasChildren bool   `json:"has_children"`
}

type sourcesBySourceToolResult struct {
	Source        *sourceNodeToolResult  `json:"source,omitempty"`
	ReturnedCount int                    `json:"returned_count"`
	Items         []sourceNodeToolResult `json:"items"`
}

type collectionToolResult struct {
	MDBID             int64  `json:"mdb_id"`
	UID               string `json:"uid"`
	ContentType       string `json:"content_type,omitempty"`
	Name              string `json:"name,omitempty"`
	Description       string `json:"description,omitempty"`
	FilmDate          string `json:"film_date,omitempty"`
	StartDate         string `json:"start_date,omitempty"`
	EndDate           string `json:"end_date,omitempty"`
	SourceID          string `json:"source_id,omitempty"`
	Number            string `json:"number,omitempty"`
	ContentUnitsCount int64  `json:"content_units_count"`
}

type collectionsToolResult struct {
	ReturnedCount int                    `json:"returned_count"`
	Items         []collectionToolResult `json:"items"`
}

type contentUnitToolResult struct {
	MDBID            int64   `json:"mdb_id"`
	UID              string  `json:"uid"`
	ContentType      string  `json:"content_type,omitempty"`
	Name             string  `json:"name,omitempty"`
	Description      string  `json:"description,omitempty"`
	NameInCollection string  `json:"name_in_collection,omitempty"`
	Position         int     `json:"position"`
	FilmDate         string  `json:"film_date,omitempty"`
	OriginalLanguage string  `json:"original_language,omitempty"`
	Duration         float64 `json:"duration,omitempty"`
}

type contentUnitsByCollectionToolResult struct {
	Collection    *collectionToolResult   `json:"collection,omitempty"`
	ReturnedCount int                     `json:"returned_count"`
	Items         []contentUnitToolResult `json:"items"`
}

func NewGetSourcesByAuthorTool(db *sql.DB, cacheTTL time.Duration) *GetSourcesByAuthorTool {
	return &GetSourcesByAuthorTool{db: db, cache: newPostgreSQLToolCache(cacheTTL)}
}

func NewGetAvailableBooksTool(db *sql.DB, cacheTTL time.Duration) *GetAvailableBooksTool {
	return &GetAvailableBooksTool{db: db, cache: newPostgreSQLToolCache(cacheTTL)}
}

func NewGetSourcesBySourceTool(db *sql.DB, cacheTTL time.Duration) *GetSourcesBySourceTool {
	return &GetSourcesBySourceTool{db: db, cache: newPostgreSQLToolCache(cacheTTL)}
}

func NewGetCollectionsTool(db *sql.DB, cacheTTL time.Duration) *GetCollectionsTool {
	return &GetCollectionsTool{db: db, cache: newPostgreSQLToolCache(cacheTTL)}
}

func NewGetContentUnitsByCollectionTool(db *sql.DB, cacheTTL time.Duration) *GetContentUnitsByCollectionTool {
	return &GetContentUnitsByCollectionTool{db: db, cache: newPostgreSQLToolCache(cacheTTL)}
}

func (t *GetSourcesByAuthorTool) Definition() llm.ReasoningToolDefinition {
	return llm.ReasoningToolDefinition{
		Name:        "get_sources_by_author",
		Description: "Return sources linked to an author from PostgreSQL by author_id.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"author_id": map[string]interface{}{
					"type":        "string",
					"description": "Author identifier (code or numeric MDB id).",
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "Preferred UI language for names and descriptions.",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of sources to return.",
				},
			},
			"required":             []string{"author_id"},
			"additionalProperties": false,
		},
	}
}

func (t *GetAvailableBooksTool) Definition() llm.ReasoningToolDefinition {
	return llm.ReasoningToolDefinition{
		Name:        "get_available_books",
		Description: "Return top-level books under authors from PostgreSQL. This tool does not accept arguments.",
		Parameters: map[string]interface{}{
			"type":                 "object",
			"properties":           map[string]interface{}{},
			"additionalProperties": false,
		},
	}
}

func (t *GetSourcesByAuthorTool) UsageExplanation() string {
	return `Tool: get_sources_by_author
This tool allows you to retrieve structured metadata about sources (library items) linked to a specific author from PostgreSQL.
The available author codes are:
"vk" -	"various"
"bs" -	"Baal HaSulam"
"rb" -	"Rabash"
"ml" -	"Michael Laitman"
"bb" -	"Bnei Baruch"
"mr" -	"Moshe Rabbenu"
"rh" -	"Rashbi"
"ar" -	"Ari"
"rl" -	"Ramchal"
"ag" -	"Agra"
Arguments:
- author_id: required. Author code (like "bs") or numeric MDB id.
- language: optional language for localized names and descriptions (e.g., "en", "he").
- limit: optional maximum number of rows.
Behavior:
- Returns JSON with the resolved author and a list of public published sources in the library linked to that author.
- Each returned item uid can be used directly as a source filter value in elasticsearch_search or as source_id input to get_sources_by_source.`
}

func (t *GetAvailableBooksTool) UsageExplanation() string {
	return `Tool: get_available_books
		Use this tool when you need top-level author->books discovery before building a source filter for elasticsearch_search.
		This tool is backed by PostgreSQL and returns top-level books under authors, not a cached static list.
		If the author is already known, prefer get_sources_by_author first. Use get_available_books mainly when the relevant book/article root under an author is still unknown or ambiguous.
		Use this tool to find the relevant source id first, and then use get_sources_by_source to drill deeper and get the concrete child source ids.
	Arguments:
	- none. Do not pass arguments.
Behavior:
	- Returns structured JSON items with author_id, author_en, author_he, source_id, source_en, and source_he.
	- source_id values can be used as source_id input to get_sources_by_source or as source filter values in elasticsearch_search.`
}

func (t *GetSourcesBySourceTool) Definition() llm.ReasoningToolDefinition {
	return llm.ReasoningToolDefinition{
		Name:        "get_sources_by_source",
		Description: "Return direct child sources of a source from PostgreSQL by source_id.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"source_id": map[string]interface{}{
					"type":        "string",
					"description": "Source identifier (UID or numeric MDB id).",
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "Preferred UI language for names and descriptions.",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of child sources to return.",
				},
			},
			"required":             []string{"source_id"},
			"additionalProperties": false,
		},
	}
}

func (t *GetSourcesBySourceTool) UsageExplanation() string {
	return `Tool: get_sources_by_source
This tool allows you to retrieve direct child sources (library items) of a specific source from PostgreSQL.
Use it after you already know a book/source id and need to drill down to more concrete child sources for source filtering.
Arguments:
- source_id: required. Source UID or numeric MDB id.
- language: optional language for localized names and descriptions (e.g., "en", "he").
- limit: optional maximum number of rows.
Behavior:
- Returns JSON with the resolved source and a list of direct child sources.
- Child items include has_children so you can decide whether to drill deeper again or use the uid directly in elasticsearch_search filters.`
}

func (t *GetAvailableBooksTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	trimmed := strings.TrimSpace(string(arguments))
	if trimmed != "" && trimmed != "null" && trimmed != "{}" {
		return "", fmt.Errorf("get_available_books: this tool does not accept arguments")
	}
	if t.db == nil {
		return "", fmt.Errorf("get_available_books: db is nil")
	}
	if cached, ok := t.cache.get("get_available_books"); ok {
		llm.LogIfDeb(ctx, "get_available_books: cache hit")
		llm.LogIfDeb(ctx, "get_available_books: output=%s", cached)
		return cached, nil
	}

	rows, err := t.db.Query(availableBooksQuery, consts.SEC_PUBLIC)
	if err != nil {
		return "", fmt.Errorf("get_available_books: query failed: %w", err)
	}
	defer rows.Close()

	candidates := make([]availableBookCandidate, 0)
	linkedUIDsByAuthor := map[string]map[string]struct{}{}
	for rows.Next() {
		candidate := availableBookCandidate{}
		if err := rows.Scan(
			&candidate.AuthorID,
			&candidate.AuthorEN,
			&candidate.AuthorHE,
			&candidate.SourceID,
			&candidate.ParentUID,
			&candidate.GrandparentUID,
			&candidate.SourceEN,
			&candidate.SourceHE,
			&candidate.ParentEN,
			&candidate.ParentHE,
		); err != nil {
			return "", fmt.Errorf("get_available_books: rows.Scan failed: %w", err)
		}
		candidates = append(candidates, candidate)
		if linkedUIDsByAuthor[candidate.AuthorID] == nil {
			linkedUIDsByAuthor[candidate.AuthorID] = map[string]struct{}{}
		}
		linkedUIDsByAuthor[candidate.AuthorID][candidate.SourceID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("get_available_books: rows iteration failed: %w", err)
	}

	items := make([]availableBookToolItem, 0)
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		if !shouldIncludeAvailableBook(candidate, linkedUIDsByAuthor[candidate.AuthorID]) {
			continue
		}
		key := candidate.AuthorID + "|" + candidate.SourceID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, availableBookToolItem{
			AuthorID: candidate.AuthorID,
			AuthorEN: candidate.AuthorEN,
			AuthorHE: candidate.AuthorHE,
			SourceID: candidate.SourceID,
			SourceEN: candidate.SourceEN,
			SourceHE: candidate.SourceHE,
		})
	}

	result, err := marshalToolResult(availableBooksToolResult{
		ReturnedCount: len(items),
		Items:         items,
	})
	if err != nil {
		return "", err
	}
	if len(items) > 0 {
		t.cache.set("get_available_books", result)
	}
	llm.LogIfDeb(ctx, "get_available_books: returning count=%d", len(items))
	llm.LogIfDeb(ctx, "get_available_books: output=%s", result)
	return result, nil
}

func (t *GetCollectionsTool) Definition() llm.ReasoningToolDefinition {
	return llm.ReasoningToolDefinition{
		Name:        "get_collections",
		Description: "Return public collections from PostgreSQL, optionally filtered by collection_id or content_type.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"collection_id": map[string]interface{}{
					"type":        "string",
					"description": "Collection identifier (UID or numeric MDB id).",
				},
				"content_type": map[string]interface{}{
					"type":        "string",
					"description": "Optional collection content type name filter.",
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "Preferred UI language for names and descriptions.",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of collections to return.",
				},
			},
			"additionalProperties": false,
		},
	}
}

func (t *GetCollectionsTool) UsageExplanation() string {
	return `Tool: get_collections
This tool allows you to retrieve structured metadata about public collections from PostgreSQL.
Collections are groups of related content units. Each daily lesson is a collection, a TV series (program) is also a collection, and there are also collections for conventions and special events.
Use this tool when you already know the collection id or when you need collections of a specific content type.
Once you identify the relevant collection, prefer elasticsearch_search with collection filter when you still need query-based ranking, matching highlights, or the best matching concrete item inside the collection. Use get_content_units_by_collection only when you need to browse or list the collection members themselves.
Available content types filter values: ARTICLES, BOOKS, CHILDREN_LESSONS, CLIPS, CONGRESS, DAILY_LESSON, FRIENDS_GATHERINGS, HOLIDAY, LECTURE_SERIES, LESSONS_SERIES, MEALS, PICNIC, SONGS, SPECIAL_LESSON, UNITY_DAY, VIDEO_PROGRAM, VIRTUAL_LESSONS, WOMEN_LESSONS.
To retrieve all TV series (programs), use content_type = VIDEO_PROGRAM without collection_id. To retrieve a specific daily lesson, use its collection_id.
Arguments:
- collection_id: optional exact lookup by collection UID or numeric MDB id.
- content_type: optional collection content type filter.
- language: optional UI language for localized names and descriptions.
- limit: optional maximum number of rows.
Behavior:
- Returns JSON with matching public published collections and their public content unit counts.`
}

func (t *GetContentUnitsByCollectionTool) Definition() llm.ReasoningToolDefinition {
	return llm.ReasoningToolDefinition{
		Name:        "get_content_units_by_collection",
		Description: "Return public content units that belong to a collection from PostgreSQL by collection_id.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"collection_id": map[string]interface{}{
					"type":        "string",
					"description": "Collection identifier (UID or numeric MDB id).",
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "Preferred UI language for names and descriptions.",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of content units to return.",
				},
			},
			"required":             []string{"collection_id"},
			"additionalProperties": false,
		},
	}
}

func (t *GetContentUnitsByCollectionTool) UsageExplanation() string {
	return `Tool: get_content_units_by_collection
This tool allows you to retrieve structured metadata about public content units that belong to a specific collection from PostgreSQL.
Use this tool after you know the collection and need to browse or list its member content units.
Arguments:
- collection_id: required. Collection UID or numeric MDB id.
- language: optional language for localized names and descriptions.
- limit: optional maximum number of rows.
Behavior:
- Returns JSON with the resolved public collection and its public published content units.
- This tool returns collection members in collection order, not by text-query relevance.
- If you need the best matching concrete item inside a known collection for a user query, prefer elasticsearch_search with collection filter instead.`
}

func (t *GetSourcesByAuthorTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if t.db == nil {
		return "", fmt.Errorf("get_sources_by_author: db is nil")
	}

	args := getSourcesByAuthorArgs{}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return "", fmt.Errorf("get_sources_by_author: failed to parse arguments: %w", err)
	}

	authorID := strings.TrimSpace(args.AuthorID)
	if authorID == "" {
		return "", fmt.Errorf("get_sources_by_author: author_id is required")
	}

	language := normalizePostgreSQLToolLanguage(args.Language)
	limit := normalizePostgreSQLToolLimit(args.Limit)
	llm.LogIfDeb(ctx, "get_sources_by_author: start author_id=%q language=%q limit=%d", authorID, language, limit)
	cacheKey := fmt.Sprintf("get_sources_by_author|author_id=%s|language=%s|limit=%d", authorID, language, limit)
	if cached, ok := t.cache.get(cacheKey); ok {
		llm.LogIfDeb(ctx, "get_sources_by_author: cache hit author_id=%q language=%q limit=%d", authorID, language, limit)
		llm.LogIfDeb(ctx, "get_sources_by_author: output=%s", cached)
		return cached, nil
	}

	author, err := loadAuthorToolResult(t.db, authorID, language)
	if err != nil {
		return "", err
	}
	llm.LogIfDeb(ctx, "get_sources_by_author: resolved author mdb_id=%d code=%q", author.MDBID, author.Code)

	rows, err := t.db.Query(sourcesByAuthorQuery, language, author.MDBID, consts.SEC_PUBLIC, limit)
	if err != nil {
		return "", fmt.Errorf("get_sources_by_author: query failed: %w", err)
	}
	defer rows.Close()

	items := []sourceToolResult{}
	for rows.Next() {
		item := sourceToolResult{}
		if err := rows.Scan(
			&item.MDBID,
			&item.UID,
			&item.ParentUID,
			&item.Type,
			&item.Name,
			&item.Description,
			&item.Year,
			&item.Number,
			&item.Position,
		); err != nil {
			return "", fmt.Errorf("get_sources_by_author: rows.Scan failed: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("get_sources_by_author: rows iteration failed: %w", err)
	}
	llm.LogIfDeb(ctx, "get_sources_by_author: completed author_id=%q returned_count=%d", authorID, len(items))

	result, err := marshalToolResult(sourcesByAuthorToolResult{
		Author:        author,
		ReturnedCount: len(items),
		Items:         items,
	})
	if err != nil {
		return "", err
	}
	llm.LogIfDeb(ctx, "get_sources_by_author: output=%s", result)
	if len(items) > 0 {
		t.cache.set(cacheKey, result)
	}
	return result, nil
}

func (t *GetSourcesBySourceTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if t.db == nil {
		return "", fmt.Errorf("get_sources_by_source: db is nil")
	}

	args := getSourcesBySourceArgs{}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return "", fmt.Errorf("get_sources_by_source: failed to parse arguments: %w", err)
	}

	sourceID := strings.TrimSpace(args.SourceID)
	if sourceID == "" {
		return "", fmt.Errorf("get_sources_by_source: source_id is required")
	}

	language := normalizePostgreSQLToolLanguage(args.Language)
	limit := normalizePostgreSQLToolLimit(args.Limit)
	llm.LogIfDeb(ctx, "get_sources_by_source: start source_id=%q language=%q limit=%d", sourceID, language, limit)
	cacheKey := fmt.Sprintf("get_sources_by_source|source_id=%s|language=%s|limit=%d", sourceID, language, limit)
	if cached, ok := t.cache.get(cacheKey); ok {
		llm.LogIfDeb(ctx, "get_sources_by_source: cache hit source_id=%q language=%q limit=%d", sourceID, language, limit)
		return cached, nil
	}

	source, err := loadSourceNodeToolResult(t.db, sourceID, language)
	if err != nil {
		return "", err
	}
	llm.LogIfDeb(ctx, "get_sources_by_source: resolved source mdb_id=%d uid=%q", source.MDBID, source.UID)

	rows, err := t.db.Query(sourcesBySourceQuery, language, source.MDBID, consts.SEC_PUBLIC, limit)
	if err != nil {
		return "", fmt.Errorf("get_sources_by_source: query failed: %w", err)
	}
	defer rows.Close()

	items := []sourceNodeToolResult{}
	for rows.Next() {
		item, err := scanSourceNodeToolResult(rows)
		if err != nil {
			return "", fmt.Errorf("get_sources_by_source: rows.Scan failed: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("get_sources_by_source: rows iteration failed: %w", err)
	}
	llm.LogIfDeb(ctx, "get_sources_by_source: completed source_id=%q returned_count=%d", sourceID, len(items))

	result, err := marshalToolResult(sourcesBySourceToolResult{
		Source:        source,
		ReturnedCount: len(items),
		Items:         items,
	})
	if err != nil {
		return "", err
	}
	if len(items) > 0 {
		t.cache.set(cacheKey, result)
	}
	return result, nil
}

func (t *GetCollectionsTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if t.db == nil {
		return "", fmt.Errorf("get_collections: db is nil")
	}

	args := getCollectionsArgs{}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return "", fmt.Errorf("get_collections: failed to parse arguments: %w", err)
	}

	language := normalizePostgreSQLToolLanguage(args.Language)
	limit := normalizePostgreSQLToolLimit(args.Limit)
	collectionID := strings.TrimSpace(args.CollectionID)
	contentType := strings.TrimSpace(args.ContentType)
	llm.LogIfDeb(ctx, "get_collections: start collection_id=%q content_type=%q language=%q limit=%d", collectionID, contentType, language, limit)
	cacheKey := fmt.Sprintf(
		"get_collections|collection_id=%s|content_type=%s|language=%s|limit=%d",
		collectionID,
		contentType,
		language,
		limit,
	)
	if cached, ok := t.cache.get(cacheKey); ok {
		llm.LogIfDeb(ctx, "get_collections: cache hit collection_id=%q content_type=%q language=%q limit=%d", collectionID, contentType, language, limit)
		return cached, nil
	}

	queryArgs := []interface{}{language, consts.SEC_PUBLIC}
	query := strings.Builder{}
	query.WriteString(collectionsSelectQuery)
	query.WriteString("WHERE c.secure = $2\n  AND c.published IS TRUE\n")

	if collectionID != "" {
		if numericID, ok := parsePostgreSQLToolNumericID(collectionID); ok {
			query.WriteString(fmt.Sprintf("  AND c.id = %s\n", appendPostgreSQLToolQueryArg(&queryArgs, numericID)))
		} else {
			query.WriteString(fmt.Sprintf("  AND c.uid = %s\n", appendPostgreSQLToolQueryArg(&queryArgs, collectionID)))
		}
	}
	if contentType != "" {
		query.WriteString(fmt.Sprintf("  AND LOWER(ct.name) = LOWER(%s)\n", appendPostgreSQLToolQueryArg(&queryArgs, contentType)))
	}

	query.WriteString("ORDER BY COALESCE(NULLIF(c.properties->>'film_date', '')::date, c.created_at::date) DESC, c.created_at DESC\n")
	query.WriteString(fmt.Sprintf("LIMIT %s", appendPostgreSQLToolQueryArg(&queryArgs, limit)))

	rows, err := t.db.Query(query.String(), queryArgs...)
	if err != nil {
		return "", fmt.Errorf("get_collections: query failed: %w", err)
	}
	defer rows.Close()

	items := []collectionToolResult{}
	for rows.Next() {
		item, err := scanCollectionToolResult(rows)
		if err != nil {
			return "", fmt.Errorf("get_collections: rows.Scan failed: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("get_collections: rows iteration failed: %w", err)
	}

	if collectionID != "" && len(items) == 0 {
		llm.LogIfDeb(ctx, "get_collections: collection not found collection_id=%q", collectionID)
		collectionUIDs, lookupErr := loadCollectionUIDsForContentUnit(t.db, collectionID)
		if lookupErr != nil {
			llm.LogIfDeb(ctx, "get_collections: failed to check content-unit fallback for collection_id=%q err=%v", collectionID, lookupErr)
		}
		errorText := fmt.Sprintf("Collection not found for collection_id '%s'", collectionID)
		guidance := "Try another collection_id from get_collections or continue with another search query instead of repeating the same missing collection lookup."
		if len(collectionUIDs) > 0 {
			errorText = fmt.Sprintf("Collection not found for collection_id '%s'. This id looks like a content_unit_id instead.", collectionID)
			guidance = fmt.Sprintf("get_collections expects a collection_id, not a content_unit_id. Consider using one of these parent collection ids instead: %s", strings.Join(collectionUIDs, ", "))
		}
		return marshalToolResult(postgreSQLToolRecoverableResult{
			Error:                  errorText,
			RetrySuggested:         true,
			Guidance:               guidance,
			SuggestedCollectionIDs: collectionUIDs,
		})
	}
	llm.LogIfDeb(ctx, "get_collections: completed collection_id=%q returned_count=%d", collectionID, len(items))

	result, err := marshalToolResult(collectionsToolResult{
		ReturnedCount: len(items),
		Items:         items,
	})
	if err != nil {
		return "", err
	}
	if len(items) > 0 {
		t.cache.set(cacheKey, result)
	}
	return result, nil
}

func (t *GetContentUnitsByCollectionTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if t.db == nil {
		return "", fmt.Errorf("get_content_units_by_collection: db is nil")
	}

	args := getContentUnitsByCollectionArgs{}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return "", fmt.Errorf("get_content_units_by_collection: failed to parse arguments: %w", err)
	}

	collectionID := strings.TrimSpace(args.CollectionID)
	if collectionID == "" {
		contentUnitID := strings.TrimSpace(args.ContentUnitID)
		if contentUnitID != "" {
			collectionUIDs, lookupErr := loadCollectionUIDsForContentUnit(t.db, contentUnitID)
			if lookupErr != nil {
				llm.LogIfDeb(ctx, "get_content_units_by_collection: failed to check content-unit fallback for content_unit_id=%q err=%v", contentUnitID, lookupErr)
			}
			errorText := fmt.Sprintf("collection_id is required. The supplied content_unit_id '%s' cannot be used directly.", contentUnitID)
			guidance := "get_content_units_by_collection requires collection_id. Use get_collections or continue with another search query instead of repeating this content_unit_id lookup."
			if len(collectionUIDs) > 0 {
				guidance = fmt.Sprintf("get_content_units_by_collection requires collection_id, not content_unit_id. Use one of these parent collection ids instead: %s", strings.Join(collectionUIDs, ", "))
			}
			return marshalToolResult(postgreSQLToolRecoverableResult{
				Error:                  errorText,
				RetrySuggested:         len(collectionUIDs) > 0,
				Guidance:               guidance,
				SuggestedCollectionIDs: collectionUIDs,
			})
		}
		return "", fmt.Errorf("get_content_units_by_collection: collection_id is required")
	}

	language := normalizePostgreSQLToolLanguage(args.Language)
	limit := normalizePostgreSQLToolLimit(args.Limit)
	llm.LogIfDeb(ctx, "get_content_units_by_collection: start collection_id=%q language=%q limit=%d", collectionID, language, limit)
	cacheKey := fmt.Sprintf("get_content_units_by_collection|collection_id=%s|language=%s|limit=%d", collectionID, language, limit)
	if cached, ok := t.cache.get(cacheKey); ok {
		llm.LogIfDeb(ctx, "get_content_units_by_collection: cache hit collection_id=%q language=%q limit=%d", collectionID, language, limit)
		return cached, nil
	}

	collection, err := loadCollectionToolResult(t.db, collectionID, language)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			llm.LogIfDeb(ctx, "get_content_units_by_collection: collection not found collection_id=%q", collectionID)
			collectionUIDs, lookupErr := loadCollectionUIDsForContentUnit(t.db, collectionID)
			if lookupErr != nil {
				llm.LogIfDeb(ctx, "get_content_units_by_collection: failed to check content-unit fallback for collection_id=%q err=%v", collectionID, lookupErr)
			}
			errorText := fmt.Sprintf("Collection not found for collection_id '%s'", collectionID)
			guidance := "Try another collection_id from get_collections or continue with another search query instead of repeating the same missing collection lookup."
			if len(collectionUIDs) > 0 {
				errorText = fmt.Sprintf("Collection not found for collection_id '%s'. This id looks like a content_unit_id instead.", collectionID)
				guidance = fmt.Sprintf("get_content_units_by_collection requires collection_id, not content_unit_id. Use one of these parent collection ids instead: %s", strings.Join(collectionUIDs, ", "))
			}
			return marshalToolResult(postgreSQLToolRecoverableResult{
				Error:                  errorText,
				RetrySuggested:         true,
				Guidance:               guidance,
				SuggestedCollectionIDs: collectionUIDs,
			})
		}
		return "", err
	}
	llm.LogIfDeb(ctx, "get_content_units_by_collection: resolved collection mdb_id=%d uid=%q", collection.MDBID, collection.UID)

	rows, err := t.db.Query(contentUnitsByCollectionQuery, language, collection.MDBID, consts.SEC_PUBLIC, limit)
	if err != nil {
		return "", fmt.Errorf("get_content_units_by_collection: query failed: %w", err)
	}
	defer rows.Close()

	items := []contentUnitToolResult{}
	for rows.Next() {
		item := contentUnitToolResult{}
		if err := rows.Scan(
			&item.MDBID,
			&item.UID,
			&item.ContentType,
			&item.Name,
			&item.Description,
			&item.NameInCollection,
			&item.Position,
			&item.FilmDate,
			&item.OriginalLanguage,
			&item.Duration,
		); err != nil {
			return "", fmt.Errorf("get_content_units_by_collection: rows.Scan failed: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("get_content_units_by_collection: rows iteration failed: %w", err)
	}
	llm.LogIfDeb(ctx, "get_content_units_by_collection: completed collection_id=%q returned_count=%d", collectionID, len(items))

	result, err := marshalToolResult(contentUnitsByCollectionToolResult{
		Collection:    collection,
		ReturnedCount: len(items),
		Items:         items,
	})
	if err != nil {
		return "", err
	}
	if len(items) > 0 {
		t.cache.set(cacheKey, result)
	}
	return result, nil
}

func newPostgreSQLToolCache(ttl time.Duration) *postgreSQLToolCache {
	if ttl <= 0 {
		return nil
	}

	return &postgreSQLToolCache{
		ttl:   ttl,
		items: map[string]postgreSQLToolCacheItem{},
	}
}

func (c *postgreSQLToolCache) get(key string) (string, bool) {
	if c == nil {
		return "", false
	}

	now := time.Now()

	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return "", false
	}
	if now.After(item.expiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return "", false
	}
	return item.value, true
}

func (c *postgreSQLToolCache) set(key string, value string) {
	if c == nil {
		return
	}

	c.mu.Lock()
	c.items[key] = postgreSQLToolCacheItem{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

func loadSourceNodeToolResult(db *sql.DB, sourceID string, language string) (*sourceNodeToolResult, error) {
	rowQuery := sourceByUIDQuery
	queryValue := interface{}(sourceID)
	if numericID, ok := parsePostgreSQLToolNumericID(sourceID); ok {
		rowQuery = sourceByMDBIDQuery
		queryValue = numericID
	}

	row := db.QueryRow(rowQuery, language, queryValue)
	item, err := scanSourceNodeToolResult(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("get_sources_by_source: source not found for source_id '%s'", sourceID)
		}
		return nil, fmt.Errorf("get_sources_by_source: source lookup failed: %w", err)
	}

	return &item, nil
}

func loadAuthorToolResult(db *sql.DB, authorID string, language string) (*authorToolResult, error) {
	author := &authorToolResult{}
	var err error

	if numericID, ok := parsePostgreSQLToolNumericID(authorID); ok {
		err = db.QueryRow(authorMetadataByMDBIDQuery, language, numericID).Scan(
			&author.MDBID,
			&author.Code,
			&author.Name,
			&author.FullName,
		)
	} else {
		err = db.QueryRow(authorMetadataByCodeQuery, language, authorID).Scan(
			&author.MDBID,
			&author.Code,
			&author.Name,
			&author.FullName,
		)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("get_sources_by_author: author not found for author_id '%s'", authorID)
		}
		return nil, fmt.Errorf("get_sources_by_author: author lookup failed: %w", err)
	}

	return author, nil
}

func loadCollectionToolResult(db *sql.DB, collectionID string, language string) (*collectionToolResult, error) {
	rowQuery := collectionByUIDQuery
	queryValue := interface{}(collectionID)
	if numericID, ok := parsePostgreSQLToolNumericID(collectionID); ok {
		rowQuery = collectionByMDBIDQuery
		queryValue = numericID
	}

	row := db.QueryRow(rowQuery, language, consts.SEC_PUBLIC, queryValue)
	item, err := scanCollectionToolResult(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("get_content_units_by_collection: collection not found for collection_id '%s': %w", collectionID, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("get_content_units_by_collection: collection lookup failed: %w", err)
	}

	return &item, nil
}

func loadCollectionUIDsForContentUnit(db *sql.DB, contentUnitID string) ([]string, error) {
	rowQuery := collectionUIDsByContentUnitUIDQuery
	queryValue := interface{}(contentUnitID)
	if numericID, ok := parsePostgreSQLToolNumericID(contentUnitID); ok {
		rowQuery = collectionUIDsByContentUnitMDBIDQuery
		queryValue = numericID
	}

	rows, err := db.Query(rowQuery, consts.SEC_PUBLIC, queryValue)
	if err != nil {
		return nil, fmt.Errorf("content unit collection lookup failed: %w", err)
	}
	defer rows.Close()

	collectionUIDs := []string{}
	for rows.Next() {
		var collectionUID string
		if err := rows.Scan(&collectionUID); err != nil {
			return nil, fmt.Errorf("content unit collection scan failed: %w", err)
		}
		collectionUID = strings.TrimSpace(collectionUID)
		if collectionUID == "" {
			continue
		}
		collectionUIDs = append(collectionUIDs, collectionUID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("content unit collection rows failed: %w", err)
	}
	return collectionUIDs, nil
}

func scanCollectionToolResult(scanner interface {
	Scan(dest ...interface{}) error
}) (collectionToolResult, error) {
	item := collectionToolResult{}
	err := scanner.Scan(
		&item.MDBID,
		&item.UID,
		&item.ContentType,
		&item.Name,
		&item.Description,
		&item.FilmDate,
		&item.StartDate,
		&item.EndDate,
		&item.SourceID,
		&item.Number,
		&item.ContentUnitsCount,
	)
	return item, err
}

func scanSourceNodeToolResult(scanner interface {
	Scan(dest ...interface{}) error
}) (sourceNodeToolResult, error) {
	item := sourceNodeToolResult{}
	err := scanner.Scan(
		&item.MDBID,
		&item.UID,
		&item.ParentUID,
		&item.Type,
		&item.Name,
		&item.Description,
		&item.Year,
		&item.Number,
		&item.Position,
		&item.HasChildren,
	)
	return item, err
}

func shouldIncludeAvailableBook(candidate availableBookCandidate, linkedUIDs map[string]struct{}) bool {
	if candidate.SourceID == candidate.AuthorID {
		return false
	}
	if candidate.ParentUID == candidate.AuthorID {
		return true
	}
	if candidate.AuthorID == "bs" &&
		candidate.GrandparentUID == candidate.AuthorID &&
		isPrefacesAvailableBookParent(candidate.ParentEN, candidate.ParentHE) {
		return true
	}
	if candidate.ParentUID == "" {
		return true
	}
	_, parentLinked := linkedUIDs[candidate.ParentUID]
	return !parentLinked
}

func isPrefacesAvailableBookParent(parentEN string, parentHE string) bool {
	return strings.EqualFold(strings.TrimSpace(parentEN), "Prefaces") ||
		strings.TrimSpace(parentHE) == "הקדמות"
}

func marshalToolResult(v interface{}) (string, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tool result: %w", err)
	}
	return string(payload), nil
}

func normalizePostgreSQLToolLanguage(language string) string {
	trimmed := strings.ToLower(strings.TrimSpace(language))
	if trimmed == "" {
		return consts.DEFAULT_UI_LANGUAGE
	}
	return trimmed
}

func normalizePostgreSQLToolLimit(limit int) int {
	if limit <= 0 {
		return defaultPostgreSQLToolLimit
	}
	if limit > maxPostgreSQLToolLimit {
		return maxPostgreSQLToolLimit
	}
	return limit
}

func parsePostgreSQLToolNumericID(rawID string) (int64, bool) {
	value, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func appendPostgreSQLToolQueryArg(args *[]interface{}, value interface{}) string {
	*args = append(*args, value)
	return fmt.Sprintf("$%d", len(*args))
}
