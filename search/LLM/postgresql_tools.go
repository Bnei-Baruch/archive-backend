package llm

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Bnei-Baruch/archive-backend/consts"
)

const (
	defaultPostgreSQLToolLimit = 25
	maxPostgreSQLToolLimit     = 100
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
  AND EXISTS (
    SELECT 1
    FROM content_units cu
    WHERE cu.uid = s.uid
      AND cu.secure = $3
      AND cu.published IS TRUE
  )
ORDER BY s.parent_id NULLS FIRST, s.position ASC, s.id ASC
LIMIT $4`

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

type GetSourcesByAuthorTool struct {
	db *sql.DB
}

type GetCollectionsTool struct {
	db *sql.DB
}

type GetContentUnitsByCollectionTool struct {
	db *sql.DB
}

type getSourcesByAuthorArgs struct {
	AuthorID string `json:"author_id,omitempty"`
	Language string `json:"language,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

type getCollectionsArgs struct {
	CollectionID string `json:"collection_id,omitempty"`
	ContentType  string `json:"content_type,omitempty"`
	Query        string `json:"query,omitempty"`
	Language     string `json:"language,omitempty"`
	Limit        int    `json:"limit,omitempty"`
}

type getContentUnitsByCollectionArgs struct {
	CollectionID string `json:"collection_id,omitempty"`
	Language     string `json:"language,omitempty"`
	Limit        int    `json:"limit,omitempty"`
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

func NewGetSourcesByAuthorTool(db *sql.DB) *GetSourcesByAuthorTool {
	return &GetSourcesByAuthorTool{db: db}
}

func NewGetCollectionsTool(db *sql.DB) *GetCollectionsTool {
	return &GetCollectionsTool{db: db}
}

func NewGetContentUnitsByCollectionTool(db *sql.DB) *GetContentUnitsByCollectionTool {
	return &GetContentUnitsByCollectionTool{db: db}
}

func (t *GetSourcesByAuthorTool) Definition() ReasoningToolDefinition {
	return ReasoningToolDefinition{
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

func (t *GetCollectionsTool) Definition() ReasoningToolDefinition {
	return ReasoningToolDefinition{
		Name:        "get_collections",
		Description: "Return public collections from PostgreSQL, optionally filtered by collection_id, content_type, or text query.",
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
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Optional text search over collection UID, name, and description.",
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

func (t *GetContentUnitsByCollectionTool) Definition() ReasoningToolDefinition {
	return ReasoningToolDefinition{
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

func (t *GetSourcesByAuthorTool) Execute(arguments json.RawMessage) (string, error) {
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

	author, err := loadAuthorToolResult(t.db, authorID, language)
	if err != nil {
		return "", err
	}

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

	return marshalPostgreSQLToolResult(sourcesByAuthorToolResult{
		Author:        author,
		ReturnedCount: len(items),
		Items:         items,
	})
}

func (t *GetCollectionsTool) Execute(arguments json.RawMessage) (string, error) {
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
	textQuery := strings.TrimSpace(args.Query)

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
	if textQuery != "" {
		pattern := "%" + textQuery + "%"
		query.WriteString(fmt.Sprintf(`  AND (
    c.uid ILIKE %s
    OR EXISTS (
      SELECT 1
      FROM collection_i18n ci
      WHERE ci.collection_id = c.id
        AND (ci.name ILIKE %s OR ci.description ILIKE %s)
    )
  )
`, appendPostgreSQLToolQueryArg(&queryArgs, pattern), appendPostgreSQLToolQueryArg(&queryArgs, pattern), appendPostgreSQLToolQueryArg(&queryArgs, pattern)))
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
		return "", fmt.Errorf("get_collections: collection not found for collection_id '%s'", collectionID)
	}

	return marshalPostgreSQLToolResult(collectionsToolResult{
		ReturnedCount: len(items),
		Items:         items,
	})
}

func (t *GetContentUnitsByCollectionTool) Execute(arguments json.RawMessage) (string, error) {
	if t.db == nil {
		return "", fmt.Errorf("get_content_units_by_collection: db is nil")
	}

	args := getContentUnitsByCollectionArgs{}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return "", fmt.Errorf("get_content_units_by_collection: failed to parse arguments: %w", err)
	}

	collectionID := strings.TrimSpace(args.CollectionID)
	if collectionID == "" {
		return "", fmt.Errorf("get_content_units_by_collection: collection_id is required")
	}

	language := normalizePostgreSQLToolLanguage(args.Language)
	limit := normalizePostgreSQLToolLimit(args.Limit)

	collection, err := loadCollectionToolResult(t.db, collectionID, language)
	if err != nil {
		return "", err
	}

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

	return marshalPostgreSQLToolResult(contentUnitsByCollectionToolResult{
		Collection:    collection,
		ReturnedCount: len(items),
		Items:         items,
	})
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
			return nil, fmt.Errorf("get_content_units_by_collection: collection not found for collection_id '%s'", collectionID)
		}
		return nil, fmt.Errorf("get_content_units_by_collection: collection lookup failed: %w", err)
	}

	return &item, nil
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

func marshalPostgreSQLToolResult(v interface{}) (string, error) {
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
