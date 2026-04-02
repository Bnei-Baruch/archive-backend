package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

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

	sourceLookupTargetChunkSize             = 3200
	sourceLookupMaxChunkSize                = 4200
	sourceLookupSmallSourceThreshold        = 40000
	sourceLookupLargeSourceThreshold        = 120000
	sourceLookupQueryReturnedChunks         = 3
	sourceLookupDefaultPreviewSize          = 2
	sourceLookupChunkWindowNeighbors        = 1
	sourceLookupMaxChunkRetrievalsPerSource = 3
)

type sourceLookupMode string

const (
	sourceLookupModeFull    sourceLookupMode = "full"
	sourceLookupModeHybrid  sourceLookupMode = "hybrid"
	sourceLookupModeChunked sourceLookupMode = "chunked"
)

type SourceLookupTool struct {
	db            *sql.DB
	assetsService integration.AssetsService

	cacheMu sync.RWMutex
	cache   map[string]*sourceLookupCacheEntry
}

type SourceLookupToolArgs struct {
	SourceID    string `json:"source_id,omitempty"`
	Language    string `json:"language,omitempty"`
	Query       string `json:"query,omitempty"`
	ChunkNumber int    `json:"chunk_number,omitempty"`
}

type sourceLookupCacheEntry struct {
	Content string
	Chunks  []string
	Mode    sourceLookupMode
}

type sourceLookupChunkMatch struct {
	ChunkIndex int
	Score      int
}

func NewSourceLookupTool(db *sql.DB, assetsService integration.AssetsService) *SourceLookupTool {
	return &SourceLookupTool{
		db:            db,
		assetsService: assetsService,
		cache:         map[string]*sourceLookupCacheEntry{},
	}
}

func (t *SourceLookupTool) Definition() llm.ReasoningToolDefinition {
	return llm.ReasoningToolDefinition{
		Name:        "source_lookup",
		Description: "Retrieve relevant source document text chunks from CDN by source_id.",
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
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Optional precise lexical text query used to retrieve only the most relevant chunks from the cached source document. This is not semantic search, so the query should use exact words or close wording that are likely to appear in the text.",
				},
				"chunk_number": map[string]interface{}{
					"type":        "integer",
					"description": "Optional 1-based number. Without query, it selects an absolute document chunk to inspect directly. With query, it selects the Nth lexical match for that query.",
					"minimum":     1,
				},
			},
			"required":             []string{"source_id"},
			"additionalProperties": false,
		},
	}
}

func (t *SourceLookupTool) UsageExplanation() string {
	return `Tool: source_lookup
	Use this tool to retrieve source text from a source (library item) document on the CDN using its identifier. This is useful when the search highlights are not sufficient and you need direct evidence from the source document.
Arguments:
- source_id: required. Source UID or numeric MDB source id.
- language: optional preferred language code.
- query: optional. When provided, the tool searches inside the cached source text and returns only the most relevant chunks. This is lexical search, not semantic search, so the query should be precise and use words that are likely to appear in the source text itself.
- chunk_number: optional 1-based number. Without query, use it to inspect a specific absolute document chunk directly. With query, use it to select the Nth lexical match from the query results. The tool automatically adds one neighboring chunk on each side when available when inspecting a specific match or chunk.
Behavior:
	- Looks up a public source document file from CDN, converts it to plain text with Doc2Text, and caches the full text server-side.
	- If language is omitted or not found, the tool falls back by language preference.
	- Three-tier policy:
	  1. Small sources are returned in full.
	  2. Medium sources are returned in full by default, but support targeted chunk retrieval when query or chunk_number is provided.
	  3. Very large sources are always handled in chunk mode.
	- If query is provided in chunk mode without chunk_number, the tool returns the top lexical matches from the cached text.
	- If query and chunk_number are both provided in chunk mode, the tool treats chunk_number as the match number within the query results and returns that selected match plus one neighboring chunk on each side when available.
	- If only chunk_number is provided in chunk mode, the tool returns that absolute document chunk plus one neighboring chunk on each side when available.
	- If neither query nor chunk_number is provided in chunk mode, the tool returns a short preview of the beginning of the source plus chunk numbering guidance.
	- In chunk mode, the returned text includes chunk numbers so you can request a follow-up chunk if needed.
	- Chunk retrieval for the same source is limited per reasoning request, so use the chunk budget carefully.
	- If no public source document is found, it returns an empty string.
	- If a runtime lookup or text-retrieval error happens, it returns an acknowledgment message so the agent can continue with other tools.`
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
	queryText := strings.TrimSpace(args.Query)
	llm.LogIfDeb(ctx, "source_lookup: start source_id=%q language=%q query=%q chunk_number=%d", sourceID, language, queryText, args.ChunkNumber)

	cacheKey := sourceID + "|" + language
	if args.ChunkNumber < 0 {
		return "The source lookup request could not be processed: chunk_number must be positive.", nil
	}

	entry, ok := t.getFromCache(cacheKey)
	if ok {
		llm.LogIfDeb(ctx, "source_lookup: cache hit source_id=%q language=%q mode=%q chunks=%d content_len=%d", sourceID, language, entry.Mode, len(entry.Chunks), len(entry.Content))
		// Count only actual chunk navigation/search calls. A plain preview should not spend the per-source chunk budget.
		if shouldUseSourceLookupChunkRetrieval(entry, queryText, args.ChunkNumber) {
			if requestCount := llm.IncrementSourceLookupChunkRequestCount(ctx, cacheKey); requestCount > sourceLookupMaxChunkRetrievalsPerSource {
				llm.LogIfDeb(ctx, "source_lookup: chunk retrieval limit reached source_id=%q language=%q count=%d", sourceID, language, requestCount)
				return fmt.Sprintf("Further chunk retrieval for this source is limited in the current reasoning request. Maximum chunk retrieval calls per source: %d. Use the current evidence or search other sources.", sourceLookupMaxChunkRetrievalsPerSource), nil
			}
		}
		return renderSourceLookupResult(entry, queryText, args.ChunkNumber), nil
	}

	fileUID, err := t.resolveFileUID(sourceID, language)
	if err != nil {
		llm.LogIfDeb(ctx, "source_lookup: lookup fallback source_id=%q language=%q err=%v", sourceID, language, err)
		return lookupToolErrorOutput("source_lookup", err), nil
	}
	if fileUID == "" {
		llm.LogIfDeb(ctx, "source_lookup: no file uid found source_id=%q language=%q", sourceID, language)
		t.setCache(cacheKey, &sourceLookupCacheEntry{})
		return "", nil
	}
	llm.LogIfDeb(ctx, "source_lookup: resolved file uid source_id=%q language=%q file_uid=%q", sourceID, language, fileUID)

	content, err := t.assetsService.Doc2Text(fileUID)
	if err != nil {
		content = doc2TextToolOutput("source_lookup", fileUID, err)
		llm.LogIfDeb(ctx, "source_lookup: doc2text fallback source_id=%q language=%q file_uid=%q err=%v", sourceID, language, fileUID, err)
		return content, nil
	}

	entry = buildSourceLookupCacheEntry(content)
	t.setCache(cacheKey, entry)
	llm.LogIfDeb(ctx, "source_lookup: completed source_id=%q language=%q file_uid=%q content_len=%d mode=%q chunks=%d", sourceID, language, fileUID, len(content), entry.Mode, len(entry.Chunks))
	if shouldUseSourceLookupChunkRetrieval(entry, queryText, args.ChunkNumber) {
		if requestCount := llm.IncrementSourceLookupChunkRequestCount(ctx, cacheKey); requestCount > sourceLookupMaxChunkRetrievalsPerSource {
			llm.LogIfDeb(ctx, "source_lookup: chunk retrieval limit reached source_id=%q language=%q count=%d", sourceID, language, requestCount)
			return fmt.Sprintf("Further chunk retrieval for this source is limited in the current reasoning request. Maximum chunk retrieval calls per source: %d. Use the current evidence or search other sources.", sourceLookupMaxChunkRetrievalsPerSource), nil
		}
	}
	return renderSourceLookupResult(entry, queryText, args.ChunkNumber), nil
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

func (t *SourceLookupTool) getFromCache(cacheKey string) (*sourceLookupCacheEntry, bool) {
	t.cacheMu.RLock()
	defer t.cacheMu.RUnlock()
	value, ok := t.cache[cacheKey]
	return value, ok
}

func (t *SourceLookupTool) setCache(cacheKey string, value *sourceLookupCacheEntry) {
	t.cacheMu.Lock()
	defer t.cacheMu.Unlock()
	t.cache[cacheKey] = value
}

func buildSourceLookupCacheEntry(content string) *sourceLookupCacheEntry {
	content = strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	content = strings.ReplaceAll(content, "\r", "\n")
	// 3-tier policy: small sources stay as full text, medium sources support optional chunk retrieval,
	// and only very large sources default to chunk mode.
	if len(content) <= sourceLookupSmallSourceThreshold {
		return &sourceLookupCacheEntry{
			Content: content,
			Mode:    sourceLookupModeFull,
		}
	}
	chunks := splitSourceLookupChunks(content)
	if len(content) <= sourceLookupLargeSourceThreshold {
		return &sourceLookupCacheEntry{
			Content: content,
			Chunks:  chunks,
			Mode:    sourceLookupModeHybrid,
		}
	}
	return &sourceLookupCacheEntry{
		Content: content,
		Chunks:  chunks,
		Mode:    sourceLookupModeChunked,
	}
}

func splitSourceLookupChunks(content string) []string {
	if content == "" {
		return []string{}
	}

	paragraphs := make([]string, 0)
	currentLines := make([]string, 0)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(currentLines) > 0 {
				paragraphs = append(paragraphs, strings.Join(currentLines, "\n"))
				currentLines = currentLines[:0]
			}
			continue
		}
		currentLines = append(currentLines, line)
	}
	if len(currentLines) > 0 {
		paragraphs = append(paragraphs, strings.Join(currentLines, "\n"))
	}
	if len(paragraphs) == 0 {
		paragraphs = append(paragraphs, content)
	}

	parts := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		parts = append(parts, splitLongSourceLookupPart(paragraph)...)
	}

	chunks := make([]string, 0, len(parts))
	currentChunk := ""
	for _, part := range parts {
		if currentChunk == "" {
			currentChunk = part
			continue
		}
		if len(currentChunk)+2+len(part) > sourceLookupMaxChunkSize {
			chunks = append(chunks, currentChunk)
			currentChunk = part
			continue
		}
		if len(currentChunk) >= sourceLookupTargetChunkSize {
			chunks = append(chunks, currentChunk)
			currentChunk = part
			continue
		}
		currentChunk += "\n\n" + part
	}
	if currentChunk != "" {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}

func splitLongSourceLookupPart(part string) []string {
	part = strings.TrimSpace(part)
	if part == "" {
		return []string{}
	}
	if len(part) <= sourceLookupMaxChunkSize {
		return []string{part}
	}

	chunks := make([]string, 0)
	remaining := part
	for len(remaining) > sourceLookupMaxChunkSize {
		splitAt := sourceLookupMaxChunkSize
		for i := sourceLookupTargetChunkSize; i < len(remaining) && i < sourceLookupMaxChunkSize; i++ {
			if remaining[i] == ' ' || remaining[i] == '\n' || remaining[i] == '\t' {
				splitAt = i
			}
		}
		if splitAt <= 0 || splitAt > len(remaining) {
			splitAt = sourceLookupMaxChunkSize
		}
		chunks = append(chunks, strings.TrimSpace(remaining[:splitAt]))
		remaining = strings.TrimSpace(remaining[splitAt:])
	}
	if remaining != "" {
		chunks = append(chunks, remaining)
	}
	return chunks
}

func renderSourceLookupResult(entry *sourceLookupCacheEntry, query string, chunkNumber int) string {
	if entry == nil || entry.Content == "" {
		return ""
	}
	metadataHeader := renderSourceLookupMetadataHeader()
	if entry.Mode == sourceLookupModeFull || len(entry.Chunks) == 0 {
		return metadataHeader + entry.Content
	}
	if entry.Mode == sourceLookupModeHybrid && query == "" && chunkNumber == 0 {
		return metadataHeader + entry.Content
	}
	if chunkNumber > 0 {
		// With a query, chunk_number becomes "which lexical match to inspect" rather than absolute document chunk navigation.
		if query != "" {
			return metadataHeader + renderSourceLookupQueryMatchWindow(entry.Chunks, query, chunkNumber)
		}
		return metadataHeader + renderSourceLookupChunkWindow(entry.Chunks, chunkNumber)
	}
	if query != "" {
		return metadataHeader + renderSourceLookupQueryMatches(entry.Chunks, query)
	}
	return metadataHeader + renderSourceLookupPreview(entry.Chunks)
}

func shouldUseSourceLookupChunkRetrieval(entry *sourceLookupCacheEntry, query string, chunkNumber int) bool {
	if entry == nil || len(entry.Chunks) == 0 {
		return false
	}
	switch entry.Mode {
	case sourceLookupModeChunked:
		// Very large sources live in chunk mode, but the initial preview should remain free.
		return query != "" || chunkNumber > 0
	case sourceLookupModeHybrid:
		return query != "" || chunkNumber > 0
	default:
		return false
	}
}

func renderSourceLookupPreview(chunks []string) string {
	limit := sourceLookupDefaultPreviewSize
	if len(chunks) < limit {
		limit = len(chunks)
	}
	indexes := make([]int, 0, limit)
	for i := 0; i < limit; i++ {
		indexes = append(indexes, i)
	}
	return renderSourceLookupChunkSelection(
		chunks,
		indexes,
		fmt.Sprintf("Mode: preview\nTotal chunks: %d\nReturned chunks: %d\nUse query to retrieve relevant chunks, or chunk_number to inspect a specific chunk.", len(chunks), limit),
	)
}

func renderSourceLookupChunkWindow(chunks []string, chunkNumber int) string {
	if chunkNumber > len(chunks) {
		return fmt.Sprintf("Requested chunk_number %d is out of range. Total chunks: %d.", chunkNumber, len(chunks))
	}
	// Keep chunk_number behavior deterministic: requested chunk plus one chunk on each side when available.
	start := chunkNumber - 1 - sourceLookupChunkWindowNeighbors
	if start < 0 {
		start = 0
	}
	end := chunkNumber + sourceLookupChunkWindowNeighbors
	if end > len(chunks) {
		end = len(chunks)
	}
	indexes := make([]int, 0, end-start)
	for i := start; i < end; i++ {
		indexes = append(indexes, i)
	}

	return renderSourceLookupChunkSelection(
		chunks,
		indexes,
		fmt.Sprintf("Mode: chunk_window\nTotal chunks: %d\nReturned chunks: %d\nRequested chunk_number: %d\nAutomatic neighboring chunks on each side: %d", len(chunks), end-start, chunkNumber, sourceLookupChunkWindowNeighbors),
	)
}

func renderSourceLookupQueryMatches(chunks []string, query string) string {
	matches := findSourceLookupChunkMatches(chunks, query)
	if len(matches) == 0 {
		preview := renderSourceLookupPreview(chunks)
		previewCount := sourceLookupDefaultPreviewSize
		if len(chunks) < previewCount {
			previewCount = len(chunks)
		}
		return fmt.Sprintf("Mode: query\nTotal chunks: %d\nReturned chunks: %d\nQuery: %q\nNo matching chunks were found in the cached source document.\n\n%s", len(chunks), previewCount, query, preview)
	}
	if len(matches) > sourceLookupQueryReturnedChunks {
		matches = matches[:sourceLookupQueryReturnedChunks]
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].ChunkIndex < matches[j].ChunkIndex
	})

	indexes := make([]int, 0, len(matches))
	for _, match := range matches {
		indexes = append(indexes, match.ChunkIndex)
	}

	return renderSourceLookupChunkSelection(
		chunks,
		indexes,
		fmt.Sprintf("Mode: query\nTotal chunks: %d\nReturned chunks: %d\nQuery: %q\nOnly part of the document was returned. To retrieve more, either refine the query to target another lexical match or use chunk_number together with the same query to inspect a specific lexical match in more detail.", len(chunks), len(indexes), query),
	)
}

func renderSourceLookupQueryMatchWindow(chunks []string, query string, matchNumber int) string {
	matches := findSourceLookupChunkMatches(chunks, query)
	if len(matches) == 0 {
		preview := renderSourceLookupPreview(chunks)
		previewCount := sourceLookupDefaultPreviewSize
		if len(chunks) < previewCount {
			previewCount = len(chunks)
		}
		return fmt.Sprintf("Mode: query_match\nTotal chunks: %d\nReturned chunks: %d\nQuery: %q\nRequested match_number: %d\nNo matching chunks were found in the cached source document.\n\n%s", len(chunks), previewCount, query, matchNumber, preview)
	}
	if matchNumber > len(matches) {
		return fmt.Sprintf("Mode: query_match\nTotal chunks: %d\nQuery: %q\nRequested match_number: %d\nThe requested query match is out of range. Available matches: %d. Use a smaller chunk_number to inspect one of the returned lexical matches, or refine the query.", len(chunks), query, matchNumber, len(matches))
	}

	selectedChunkNumber := matches[matchNumber-1].ChunkIndex + 1
	chunkWindow := renderSourceLookupChunkWindow(chunks, selectedChunkNumber)
	return fmt.Sprintf("Mode: query_match\nQuery: %q\nRequested match_number: %d\nMatched document chunk_number: %d\nThe requested lexical match was selected and returned with one neighboring chunk on each side when available.\n\n%s", query, matchNumber, selectedChunkNumber, chunkWindow)
}

func findSourceLookupChunkMatches(chunks []string, query string) []sourceLookupChunkMatch {
	normalizedQuery := normalizeSourceLookupSearchText(query)
	if normalizedQuery == "" {
		return []sourceLookupChunkMatch{}
	}
	terms := tokenizeSourceLookupQuery(query)
	matches := make([]sourceLookupChunkMatch, 0)

	for i, chunk := range chunks {
		normalizedChunk := normalizeSourceLookupSearchText(chunk)
		score := 0
		exactCount := strings.Count(normalizedChunk, normalizedQuery)
		if exactCount > 0 {
			score += exactCount * 1000
		}
		matchedTerms := 0
		for _, term := range terms {
			termCount := strings.Count(normalizedChunk, term)
			if termCount > 0 {
				matchedTerms++
				score += termCount * 20
			}
		}
		if len(terms) > 1 && matchedTerms == len(terms) {
			score += 200
		}
		if score == 0 {
			continue
		}
		matches = append(matches, sourceLookupChunkMatch{
			ChunkIndex: i,
			Score:      score,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].ChunkIndex < matches[j].ChunkIndex
		}
		return matches[i].Score > matches[j].Score
	})

	return matches
}

func normalizeSourceLookupSearchText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func tokenizeSourceLookupQuery(query string) []string {
	parts := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	tokens := make([]string, 0, len(parts))
	seen := make(map[string]bool)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		tokens = append(tokens, part)
	}
	return tokens
}

func renderSourceLookupChunkSelection(chunks []string, indexes []int, header string) string {
	var builder strings.Builder
	builder.WriteString(header)
	for _, index := range indexes {
		if index < 0 || index >= len(chunks) {
			continue
		}
		builder.WriteString("\n\n")
		builder.WriteString(fmt.Sprintf("[chunk %d/%d]\n", index+1, len(chunks)))
		builder.WriteString(chunks[index])
	}
	return builder.String()
}

func renderSourceLookupMetadataHeader() string {
	return "result_type: " + consts.ES_RESULT_TYPE_SOURCES + "\n" +
		"content_type: " + consts.CT_SOURCE + "\n\n"
}
