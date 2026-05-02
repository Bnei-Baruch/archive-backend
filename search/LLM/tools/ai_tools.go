package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/integration"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
)

const (
	// Default and hard cap for how many semantic matches a single tool call may return.
	defaultAIQueryToolMaxChunks = 3
	maxAIQueryToolMaxChunks     = 6

	// Highlight AI tool output logs with a brown ANSI color when debug logging is enabled.
	aiQueryToolOutputLogColor = "\x1b[38;5;130m"
	aiQueryToolOutputLogReset = "\x1b[0m"

	// Reader-model batches are char-based to keep prompts bounded without tokenization.
	aiQueryToolBatchMaxChars = 20000

	// Prevent scanning arbitrarily large documents in one request.
	aiQueryToolMaxBatches = 5

	// Build larger semantic windows on top of raw lookup chunks so the reader model
	// sees more surrounding context before choosing a match.
	aiQueryToolWindowTargetChars = 9000
	aiQueryToolWindowMaxChars    = 12000

	// Per batch, ask the reader model for only a few best chunk candidates.
	aiQueryToolChunkSelectionsPerRun = 3

	// Keep tool outputs compact so the main model does not replay long raw chunks.
	aiQueryToolMaxContentRunes = 900

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

	sourceLookupTargetChunkSize      = 3200
	sourceLookupMaxChunkSize         = 4200
	sourceLookupSmallSourceThreshold = 40000
)

var aiQueryStopWords = map[string]bool{
	// Common instruction words that should not decide which document batches
	// are sent to the reader model.
	"או":       true,
	"את":       true,
	"מצא":      true,
	"קטע":      true,
	"קטעים":    true,
	"מצאו":     true,
	"למצוא":    true,
	"שמצא":     true,
	"שמוצא":    true,
	"שמגדיר":   true,
	"שמגדירים": true,
	"שמסביר":   true,
	"מסביר":    true,
	"מסבירים":  true,
	"מגדיר":    true,
	"מגדירים":  true,
	"מה":       true,
	"הוא":      true,
	"היא":      true,
	"זה":       true,
	"זו":       true,

	"a":          true,
	"an":         true,
	"and":        true,
	"is":         true,
	"or":         true,
	"the":        true,
	"what":       true,
	"of":         true,
	"in":         true,
	"on":         true,
	"to":         true,
	"for":        true,
	"that":       true,
	"which":      true,
	"find":       true,
	"show":       true,
	"chunk":      true,
	"chunks":     true,
	"passage":    true,
	"passages":   true,
	"excerpt":    true,
	"excerpts":   true,
	"quote":      true,
	"quotes":     true,
	"define":     true,
	"defines":    true,
	"defining":   true,
	"explain":    true,
	"explains":   true,
	"explaining": true,

	"и":           true,
	"или":         true,
	"найди":       true,
	"найти":       true,
	"покажи":      true,
	"который":     true,
	"которая":     true,
	"которое":     true,
	"которые":     true,
	"фрагмент":    true,
	"фрагменты":   true,
	"отрывок":     true,
	"отрывки":     true,
	"цитата":      true,
	"цитаты":      true,
	"определи":    true,
	"определить":  true,
	"определяет":  true,
	"определяют":  true,
	"объясни":     true,
	"объяснить":   true,
	"объясняет":   true,
	"объясняют":   true,
	"объясняющие": true,
	"что":         true,
	"это":         true,
	"есть":        true,
	"является":    true,

	"el":           true,
	"la":           true,
	"los":          true,
	"las":          true,
	"un":           true,
	"una":          true,
	"unos":         true,
	"unas":         true,
	"de":           true,
	"del":          true,
	"en":           true,
	"para":         true,
	"por":          true,
	"que":          true,
	"buscar":       true,
	"busca":        true,
	"encuentra":    true,
	"muestra":      true,
	"fragmento":    true,
	"fragmentos":   true,
	"pasaje":       true,
	"pasajes":      true,
	"extracto":     true,
	"extractos":    true,
	"cita":         true,
	"citas":        true,
	"definir":      true,
	"definen":      true,
	"definiendo":   true,
	"explica":      true,
	"explican":     true,
	"explicar":     true,
	"explicando":   true,
	"explicativos": true,
	"qué":          true,
	"es":           true,
	"son":          true,
	"está":         true,
	"esta":         true,
}

const aiQueryChunkSelectorPrompt = `You select the most relevant chunks from a single document for a search query.
Choose chunks by meaning, not only by exact word overlap.
Prefer chunks that directly answer, define, explain, or quote the queried concept.
Return only chunk numbers that appear in the provided batch.
If uncertain, return an empty matches array.`

type QuerySourceAITool struct {
	lookup  *aiSourceDocumentLoader
	service llm.Service
	config  *llm.AIToolsConfig
}

type QueryTranscriptAITool struct {
	lookup  *aiTranscriptDocumentLoader
	service llm.Service
	config  *llm.AIToolsConfig
}

type aiSourceDocumentLoader struct {
	db            *sql.DB
	assetsService integration.AssetsService
	cache         aiQueryDocumentCache
}

type aiTranscriptDocumentLoader struct {
	db            *sql.DB
	assetsService integration.AssetsService
	cache         aiQueryDocumentCache
}

type aiQueryDocumentCache struct {
	mu      sync.RWMutex
	entries map[string]*aiQueryDocumentCacheEntry
}

type aiQueryDocumentCacheEntry struct {
	FileUID string
	Content string
	Chunks  []string
}

type querySourceAIToolArgs struct {
	SourceID  string `json:"source_id,omitempty"`
	Language  string `json:"language,omitempty"`
	Query     string `json:"query,omitempty"`
	MaxChunks int    `json:"max_chunks,omitempty"`
}

type queryTranscriptAIToolArgs struct {
	ContentUnitID string `json:"content_unit_id,omitempty"`
	Language      string `json:"language,omitempty"`
	Query         string `json:"query,omitempty"`
	MaxChunks     int    `json:"max_chunks,omitempty"`
}

type aiQueryToolResult struct {
	DocumentID    string             `json:"document_id"`
	DocumentType  string             `json:"document_type"`
	Query         string             `json:"query"`
	ReturnedCount int                `json:"returned_count"`
	Matches       []aiQueryToolMatch `json:"matches"`
}

type aiQueryToolMatch struct {
	ChunkNumber       int    `json:"chunk_number"`
	EndChunkNumber    int    `json:"end_chunk_number,omitempty"`
	Content           string `json:"content"`
	Reason            string `json:"reason,omitempty"`
	SupportingSnippet string `json:"supporting_snippet,omitempty"`
}

type aiQueryChunkSelection struct {
	Matches []int `json:"matches"`
}

type aiQueryChunkSelectionWithReasons struct {
	Matches []aiQueryChunkSelectionReasonMatch `json:"matches"`
}

type aiQueryChunkSelectionReasonMatch struct {
	ChunkNumber       int    `json:"chunk_number"`
	Reason            string `json:"reason"`
	SupportingSnippet string `json:"supporting_snippet"`
}

type aiQuerySelectedChunk struct {
	ChunkNumber       int
	Reason            string
	SupportingSnippet string
}

type aiQueryChunk struct {
	Number           int
	StartChunkNumber int
	EndChunkNumber   int
	Content          string
}

func newAISourceDocumentLoader(db *sql.DB, assetsService integration.AssetsService) *aiSourceDocumentLoader {
	return &aiSourceDocumentLoader{
		db:            db,
		assetsService: assetsService,
		cache:         newAIQueryDocumentCache(),
	}
}

func newAITranscriptDocumentLoader(db *sql.DB, assetsService integration.AssetsService) *aiTranscriptDocumentLoader {
	return &aiTranscriptDocumentLoader{
		db:            db,
		assetsService: assetsService,
		cache:         newAIQueryDocumentCache(),
	}
}

func NewQuerySourceAITool(lookup *aiSourceDocumentLoader, service llm.Service, config *llm.AIToolsConfig) *QuerySourceAITool {
	return &QuerySourceAITool{lookup: lookup, service: service, config: config}
}

func NewQueryTranscriptAITool(lookup *aiTranscriptDocumentLoader, service llm.Service, config *llm.AIToolsConfig) *QueryTranscriptAITool {
	return &QueryTranscriptAITool{lookup: lookup, service: service, config: config}
}

func (t *QuerySourceAITool) Definition() llm.ReasoningToolDefinition {
	return llm.ReasoningToolDefinition{
		Name:        "query_source_ai",
		Description: "Use a cheaper AI reader model to semantically retrieve the most relevant chunks from a source document by source_id and query.",
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
					"description": "Natural-language query describing what to find in the source. The AI reader matches by meaning, not only exact wording.",
				},
				"max_chunks": map[string]interface{}{
					"type":        "integer",
					"description": "Optional maximum number of relevant chunks to return.",
					"minimum":     1,
					"maximum":     maxAIQueryToolMaxChunks,
				},
			},
			"required":             []string{"source_id", "query"},
			"additionalProperties": false,
		},
	}
}

func (t *QuerySourceAITool) UsageExplanation() string {
	return `Tool: query_source_ai
Use this tool when search highlights are not enough and you need semantically relevant chunks from a source document.
Arguments:
- source_id: required. Source UID or numeric MDB source id.
- language: optional preferred language code.
- query: required natural-language query describing what to find.
- max_chunks: optional maximum number of chunks to return.
Behavior:
- Internally retrieves the source document from the CDN using the existing source lookup path.
- Uses a dedicated cheaper AI reader model to select the most relevant chunks by meaning, not only by exact word overlap.
- Returns compact JSON with relevant chunk ranges and text excerpts.
- Prefer this tool over raw source browsing when you need focused evidence instead of the whole text.`
}

func (t *QueryTranscriptAITool) Definition() llm.ReasoningToolDefinition {
	return llm.ReasoningToolDefinition{
		Name:        "query_transcript_ai",
		Description: "Use a cheaper AI reader model to semantically retrieve the most relevant chunks from a transcript by content_unit_id and query.",
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
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Natural-language query describing what to find in the transcript. The AI reader matches by meaning, not only exact wording.",
				},
				"max_chunks": map[string]interface{}{
					"type":        "integer",
					"description": "Optional maximum number of relevant chunks to return.",
					"minimum":     1,
					"maximum":     maxAIQueryToolMaxChunks,
				},
			},
			"required":             []string{"content_unit_id", "query"},
			"additionalProperties": false,
		},
	}
}

func (t *QueryTranscriptAITool) UsageExplanation() string {
	return `Tool: query_transcript_ai
Use this tool when search highlights are not enough and you need semantically relevant transcript chunks.
Arguments:
- content_unit_id: required. Content unit UID or numeric MDB id.
- language: optional preferred transcript language code.
- query: required natural-language query describing what to find.
- max_chunks: optional maximum number of chunks to return.
Behavior:
- Internally retrieves the transcript from the CDN using the existing transcript lookup path.
- Uses a dedicated cheaper AI reader model to select the most relevant chunks by meaning, not only by exact word overlap.
- Returns compact JSON with relevant chunk ranges and text excerpts.
- Prefer this tool over full transcript retrieval when you need focused evidence.`
}

func (t *QuerySourceAITool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if t.lookup == nil {
		return "", fmt.Errorf("query_source_ai: source lookup tool is nil")
	}
	if t.service == nil {
		return "", fmt.Errorf("query_source_ai: ai query service is nil")
	}
	if t.config == nil {
		return "", fmt.Errorf("query_source_ai: ai query config is nil")
	}

	args := querySourceAIToolArgs{}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return "", fmt.Errorf("query_source_ai: failed to parse arguments: %w", err)
	}
	sourceID := strings.TrimSpace(args.SourceID)
	if sourceID == "" {
		return "", llm.NewRecoverableToolError("query_source_ai", "source_id is required", "Call query_source_ai with a source_id from a source result or PostgreSQL source lookup tool.")
	}
	query := strings.TrimSpace(args.Query)
	if query == "" {
		return "", llm.NewRecoverableToolError("query_source_ai", "query is required", "Provide a short natural-language query describing what to find in the source.")
	}
	language := strings.ToLower(strings.TrimSpace(args.Language))
	maxChunks := normalizeAIQueryToolMaxChunks(args.MaxChunks)
	entry, err := loadSourceDocumentEntry(t.lookup, sourceID, language)
	if err != nil {
		llm.LogIfDeb(ctx, "query_source_ai: source load failed source_id=%q language=%q err=%v", sourceID, language, err)
		return aiQueryToolErrorOutput("query_source_ai", err), nil
	}
	llm.LogIfDeb(ctx, "query_source_ai: start source_id=%q file_id=%q language=%q query=%q max_chunks=%d", sourceID, entry.FileUID, language, query, maxChunks)
	result, err := executeAIQuery(ctx, t.service, t.config, "source", sourceID, query, entry, maxChunks)
	if err != nil {
		llm.LogIfDeb(ctx, "query_source_ai: semantic selection failed source_id=%q file_id=%q language=%q err=%v", sourceID, entry.FileUID, language, err)
		return aiQueryToolErrorOutput("query_source_ai", err), nil
	}
	output, err := marshalToolResult(result)
	if err != nil {
		return "", err
	}
	llm.LogIfDeb(ctx, "query_source_ai: completed source_id=%q file_id=%q returned_count=%d", sourceID, entry.FileUID, len(result.Matches))
	llm.LogIfDeb(ctx, "%squery_source_ai: output=%s%s", aiQueryToolOutputLogColor, output, aiQueryToolOutputLogReset)
	return output, nil
}

func (t *QueryTranscriptAITool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if t.lookup == nil {
		return "", fmt.Errorf("query_transcript_ai: transcript lookup tool is nil")
	}
	if t.service == nil {
		return "", fmt.Errorf("query_transcript_ai: ai query service is nil")
	}
	if t.config == nil {
		return "", fmt.Errorf("query_transcript_ai: ai query config is nil")
	}

	args := queryTranscriptAIToolArgs{}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return "", fmt.Errorf("query_transcript_ai: failed to parse arguments: %w", err)
	}
	contentUnitID := strings.TrimSpace(args.ContentUnitID)
	if contentUnitID == "" {
		return "", llm.NewRecoverableToolError("query_transcript_ai", "content_unit_id is required", "Call query_transcript_ai with a content_unit_id from a concrete content unit result.")
	}
	query := strings.TrimSpace(args.Query)
	if query == "" {
		return "", llm.NewRecoverableToolError("query_transcript_ai", "query is required", "Provide a short natural-language query describing what to find in the transcript.")
	}
	language := strings.ToLower(strings.TrimSpace(args.Language))
	maxChunks := normalizeAIQueryToolMaxChunks(args.MaxChunks)
	llm.LogIfDeb(ctx, "query_transcript_ai: start content_unit_id=%q language=%q query=%q max_chunks=%d", contentUnitID, language, query, maxChunks)

	entry, err := loadTranscriptDocumentEntry(t.lookup, contentUnitID, language)
	if err != nil {
		llm.LogIfDeb(ctx, "query_transcript_ai: transcript load failed content_unit_id=%q language=%q err=%v", contentUnitID, language, err)
		return aiQueryToolErrorOutput("query_transcript_ai", err), nil
	}
	result, err := executeAIQuery(ctx, t.service, t.config, "transcript", contentUnitID, query, entry, maxChunks)
	if err != nil {
		llm.LogIfDeb(ctx, "query_transcript_ai: semantic selection failed content_unit_id=%q language=%q err=%v", contentUnitID, language, err)
		return aiQueryToolErrorOutput("query_transcript_ai", err), nil
	}
	output, err := marshalToolResult(result)
	if err != nil {
		return "", err
	}
	llm.LogIfDeb(ctx, "query_transcript_ai: completed content_unit_id=%q returned_count=%d", contentUnitID, len(result.Matches))
	llm.LogIfDeb(ctx, "%squery_transcript_ai: output=%s%s", aiQueryToolOutputLogColor, output, aiQueryToolOutputLogReset)
	return output, nil
}

func executeAIQuery(ctx context.Context, service llm.Service, config *llm.AIToolsConfig, documentType string, documentID string, query string, entry *aiQueryDocumentCacheEntry, maxChunks int) (*aiQueryToolResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := &aiQueryToolResult{
		DocumentID:    documentID,
		DocumentType:  documentType,
		Query:         query,
		ReturnedCount: 0,
		Matches:       []aiQueryToolMatch{},
	}
	if entry == nil || strings.TrimSpace(entry.Content) == "" {
		return result, nil
	}

	chunks := aiQueryChunksFromEntry(entry)
	if len(chunks) == 0 {
		return result, nil
	}
	batches := buildAIQueryBatches(chunks, aiQueryToolBatchMaxChars)
	maxBatches := normalizeAIQueryToolMaxBatches(config)
	if len(batches) > maxBatches {
		selectedBatches := selectAIQueryBatches(query, batches, maxBatches)
		llm.LogIfDeb(ctx, "%s_ai: selected %d candidate batches out of %d", documentType, len(selectedBatches), len(batches))
		batches = selectedBatches
	}

	selected := map[int]aiQuerySelectedChunk{}
	for i, batch := range batches {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		selectedChunks, debug, err := selectAIQueryBatch(ctx, service, config, query, batch, maxChunks)
		if err != nil {
			return nil, err
		}
		if debug != nil {
			llm.LogIfDeb(ctx, "%s_ai: batch=%d total_tokens=%d estimated_cost_usd=%.6f", documentType, i+1, debug.TotalTokens, debug.EstimatedCostUSD)
		}
		for _, selectedChunk := range selectedChunks {
			chunkNumber := selectedChunk.ChunkNumber
			if chunkNumber <= 0 {
				continue
			}
			chunk := findAIQueryChunk(batch, chunkNumber)
			if chunk == nil {
				continue
			}
			reason := strings.TrimSpace(selectedChunk.Reason)
			snippet := strings.TrimSpace(selectedChunk.SupportingSnippet)
			current, ok := selected[chunkNumber]
			if !ok || (current.Reason == "" && reason != "") || (current.SupportingSnippet == "" && snippet != "") {
				selected[chunkNumber] = aiQuerySelectedChunk{
					ChunkNumber:       chunkNumber,
					Reason:            reason,
					SupportingSnippet: snippet,
				}
			}
		}
	}

	finalMatches := make([]aiQueryToolMatch, 0, len(selected))
	for chunkNumber, selectedChunk := range selected {
		chunk := chunks[chunkNumber-1]
		content := chunk.Content
		match := aiQueryToolMatch{
			ChunkNumber:       chunk.StartChunkNumber,
			Content:           extractAIQueryExcerpt(content, query, selectedChunk.Reason, selectedChunk.SupportingSnippet, aiQueryToolMaxContentRunes),
			Reason:            selectedChunk.Reason,
			SupportingSnippet: selectedChunk.SupportingSnippet,
		}
		if chunk.EndChunkNumber > chunk.StartChunkNumber {
			match.EndChunkNumber = chunk.EndChunkNumber
		}
		finalMatches = append(finalMatches, match)
	}
	sort.Slice(finalMatches, func(i, j int) bool {
		return finalMatches[i].ChunkNumber < finalMatches[j].ChunkNumber
	})
	if len(finalMatches) > maxChunks {
		finalMatches = finalMatches[:maxChunks]
	}
	result.Matches = finalMatches
	result.ReturnedCount = len(finalMatches)
	return result, nil
}

func selectAIQueryBatch(ctx context.Context, service llm.Service, config *llm.AIToolsConfig, query string, batch []aiQueryChunk, maxChunks int) ([]aiQuerySelectedChunk, *llm.ReasoningSearchDebugInfo, error) {
	debugMode := llm.DebFromContext(ctx)
	limit := minAIQueryInt(maxChunks, aiQueryToolChunkSelectionsPerRun)
	schema := aiQueryChunkSelectionSchema()
	instructions := "The matches array must contain only chunk numbers from this batch. Do not output prose, markdown, code fences, scores, or explanations."
	if debugMode {
		schema = aiQueryChunkSelectionWithReasonsSchema()
		instructions = "The matches array must contain only chunk numbers from this batch. Include a short reason and a short supporting snippet copied from the selected chunk for each selected chunk. Do not output prose, markdown, code fences, or scores."
	}
	prompt := fmt.Sprintf(
		"User query:\n%s\n\nSelect up to %d chunks from this batch.\n\nReturn exactly one JSON object that matches the schema below. %s\n\nJSON Schema:\n%s\n\nChunks:\n%s",
		query,
		limit,
		instructions,
		schema,
		renderAIQueryBatch(batch),
	)
	messages := []llm.LLMBotMessage{
		{Role: "system", Content: aiQueryChunkSelectorPrompt},
		{Role: "user", Content: prompt},
	}
	if debugMode {
		run := func() ([]aiQuerySelectedChunk, *llm.ReasoningSearchDebugInfo, error) {
			response := &aiQueryChunkSelectionWithReasons{}
			debug, err := service.GetStructuredOutputWithDebugInfo(ctx, schema, config.Model, &config.MaxTokens, messages, nil, &config.Effort, true, response)
			return aiQuerySelectedChunksFromReasons(response), debug, err
		}
		selected, debug, err := run()
		if aiQueryShouldRetrySelection(err) {
			llm.LogIfDeb(ctx, "ai_query_selector: retrying once after empty assistant output model=%q debug=%t", config.Model, debugMode)
			selected, debug, err = run()
		}
		llm.AddToolDebugInfo(ctx, debug)
		return selected, debug, err
	}
	run := func() ([]aiQuerySelectedChunk, *llm.ReasoningSearchDebugInfo, error) {
		response := &aiQueryChunkSelection{}
		debug, err := service.GetStructuredOutputWithDebugInfo(ctx, schema, config.Model, &config.MaxTokens, messages, nil, &config.Effort, false, response)
		return aiQuerySelectedChunksFromPlain(response), debug, err
	}
	selected, debug, err := run()
	if aiQueryShouldRetrySelection(err) {
		llm.LogIfDeb(ctx, "ai_query_selector: retrying once after empty assistant output model=%q debug=%t", config.Model, debugMode)
		selected, debug, err = run()
	}
	if err != nil {
		return nil, nil, err
	}
	llm.AddToolDebugInfo(ctx, debug)
	return selected, debug, nil
}

func loadSourceDocumentEntry(tool *aiSourceDocumentLoader, sourceID string, language string) (*aiQueryDocumentCacheEntry, error) {
	return loadAIQueryDocumentEntry(
		&tool.cache,
		sourceID+"|"+language,
		func() (string, error) {
			return tool.resolveFileUID(sourceID, language)
		},
		func(fileUID string) (string, error) {
			content, err := tool.assetsService.Doc2Text(fileUID)
			if err != nil {
				return "", fmt.Errorf("source text could not be retrieved: %w", err)
			}
			return content, nil
		},
	)
}

func loadTranscriptDocumentEntry(tool *aiTranscriptDocumentLoader, contentUnitID string, language string) (*aiQueryDocumentCacheEntry, error) {
	kiteiMakorType, ok := mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_KITEI_MAKOR]
	if !ok {
		return nil, fmt.Errorf("query_transcript_ai: content type '%s' is unavailable", consts.CT_KITEI_MAKOR)
	}
	return loadAIQueryDocumentEntry(
		&tool.cache,
		contentUnitID+"|"+language,
		func() (string, error) {
			return tool.resolveFileUID(contentUnitID, language, kiteiMakorType.ID)
		},
		func(fileUID string) (string, error) {
			content, err := tool.assetsService.Doc2Text(fileUID)
			if err != nil {
				return "", fmt.Errorf("transcript text could not be retrieved: %w", err)
			}
			return content, nil
		},
	)
}

func (t *aiSourceDocumentLoader) resolveFileUID(sourceID string, language string) (string, error) {
	return resolveAIQueryIdentifier(
		sourceID,
		func(numericID int64) (string, error) {
			return t.resolveByMDBID(numericID, language)
		},
		func(uid string) (string, error) {
			return t.resolveByUID(uid, language)
		},
	)
}

func (t *aiSourceDocumentLoader) resolveByUID(sourceUID string, language string) (string, error) {
	return queryAIQueryFileUIDWithLanguageFallback(
		t.db,
		"query_source_ai",
		language,
		sourceLookupByUIDWithLanguageQuery,
		[]interface{}{consts.SEC_PUBLIC, consts.SEC_PUBLIC, sourceUID, language},
		sourceLookupByUIDFallbackQuery,
		[]interface{}{consts.SEC_PUBLIC, consts.SEC_PUBLIC, sourceUID, language},
	)
}

func (t *aiSourceDocumentLoader) resolveByMDBID(sourceID int64, language string) (string, error) {
	return queryAIQueryFileUIDWithLanguageFallback(
		t.db,
		"query_source_ai",
		language,
		sourceLookupByMDBIDWithLanguageQuery,
		[]interface{}{consts.SEC_PUBLIC, consts.SEC_PUBLIC, sourceID, language},
		sourceLookupByMDBIDFallbackQuery,
		[]interface{}{consts.SEC_PUBLIC, consts.SEC_PUBLIC, sourceID, language},
	)
}

func (t *aiTranscriptDocumentLoader) resolveFileUID(contentUnitID string, language string, excludedTypeID int64) (string, error) {
	return resolveAIQueryIdentifier(
		contentUnitID,
		func(numericID int64) (string, error) {
			return t.resolveByMDBID(numericID, language, excludedTypeID)
		},
		func(uid string) (string, error) {
			return t.resolveByUID(uid, language, excludedTypeID)
		},
	)
}

func (t *aiTranscriptDocumentLoader) resolveByUID(contentUnitUID string, language string, excludedTypeID int64) (string, error) {
	return queryAIQueryFileUIDWithLanguageFallback(
		t.db,
		"query_transcript_ai",
		language,
		transcriptLookupByUIDWithLanguageQuery,
		[]interface{}{excludedTypeID, contentUnitUID, language},
		transcriptLookupByUIDFallbackQuery,
		[]interface{}{excludedTypeID, contentUnitUID, language},
	)
}

func (t *aiTranscriptDocumentLoader) resolveByMDBID(contentUnitID int64, language string, excludedTypeID int64) (string, error) {
	return queryAIQueryFileUIDWithLanguageFallback(
		t.db,
		"query_transcript_ai",
		language,
		transcriptLookupByMDBIDWithLanguageQuery,
		[]interface{}{excludedTypeID, contentUnitID, language},
		transcriptLookupByMDBIDFallbackQuery,
		[]interface{}{excludedTypeID, contentUnitID, language},
	)
}

func newAIQueryDocumentCache() aiQueryDocumentCache {
	return aiQueryDocumentCache{entries: map[string]*aiQueryDocumentCacheEntry{}}
}

func (c *aiQueryDocumentCache) get(cacheKey string) (*aiQueryDocumentCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.entries[cacheKey]
	return value, ok
}

func (c *aiQueryDocumentCache) set(cacheKey string, value *aiQueryDocumentCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[cacheKey] = value
}

func queryAIQueryFileUID(db *sql.DB, toolName string, query string, args ...interface{}) (string, error) {
	fileUID := ""
	if err := db.QueryRow(query, args...).Scan(&fileUID); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("%s: file lookup failed: %w", toolName, err)
	}
	return fileUID, nil
}

func queryAIQueryFileUIDWithLanguageFallback(db *sql.DB, toolName string, language string, withLanguageQuery string, withLanguageArgs []interface{}, fallbackQuery string, fallbackArgs []interface{}) (string, error) {
	if language != "" {
		fileUID, err := queryAIQueryFileUID(db, toolName, withLanguageQuery, withLanguageArgs...)
		if err != nil {
			return "", err
		}
		if fileUID != "" {
			return fileUID, nil
		}
	}
	return queryAIQueryFileUID(db, toolName, fallbackQuery, fallbackArgs...)
}

func resolveAIQueryIdentifier(identifier string, resolveByMDBID func(int64) (string, error), resolveByUID func(string) (string, error)) (string, error) {
	if numericID, err := strconv.ParseInt(identifier, 10, 64); err == nil {
		fileUID, err := resolveByMDBID(numericID)
		if err != nil {
			return "", err
		}
		if fileUID != "" {
			return fileUID, nil
		}
	}
	return resolveByUID(identifier)
}

func loadAIQueryDocumentEntry(cache *aiQueryDocumentCache, cacheKey string, resolveFileUID func() (string, error), fetchContent func(string) (string, error)) (*aiQueryDocumentCacheEntry, error) {
	if entry, ok := cache.get(cacheKey); ok {
		return entry, nil
	}

	fileUID, err := resolveFileUID()
	if err != nil {
		return nil, err
	}
	if fileUID == "" {
		empty := &aiQueryDocumentCacheEntry{}
		cache.set(cacheKey, empty)
		return empty, nil
	}

	content, err := fetchContent(fileUID)
	if err != nil {
		return nil, err
	}
	entry := buildAIQueryDocumentCacheEntry(content, fileUID)
	cache.set(cacheKey, entry)
	return entry, nil
}

func buildAIQueryDocumentCacheEntry(content string, fileUID string) *aiQueryDocumentCacheEntry {
	content = strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	content = strings.ReplaceAll(content, "\r", "\n")
	if len(content) <= sourceLookupSmallSourceThreshold {
		return &aiQueryDocumentCacheEntry{
			FileUID: fileUID,
			Content: content,
		}
	}
	chunks := splitAIQueryDocumentChunks(content)
	return &aiQueryDocumentCacheEntry{
		FileUID: fileUID,
		Content: content,
		Chunks:  chunks,
	}
}

func splitAIQueryDocumentChunks(content string) []string {
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
		parts = append(parts, splitLongAIQueryDocumentPart(paragraph)...)
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

func splitLongAIQueryDocumentPart(part string) []string {
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

func aiQueryChunksFromEntry(entry *aiQueryDocumentCacheEntry) []aiQueryChunk {
	if entry == nil {
		return []aiQueryChunk{}
	}
	baseChunks := entry.Chunks
	if len(baseChunks) == 0 {
		content := strings.TrimSpace(entry.Content)
		if content == "" {
			return []aiQueryChunk{}
		}
		baseChunks = splitAIQueryDocumentChunks(content)
		if len(baseChunks) == 0 {
			return []aiQueryChunk{{Number: 1, StartChunkNumber: 1, EndChunkNumber: 1, Content: content}}
		}
	}
	parts := make([]aiQueryChunk, 0, len(baseChunks))
	for i, chunk := range baseChunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		parts = append(parts, aiQueryChunk{
			Number:           i + 1,
			StartChunkNumber: i + 1,
			EndChunkNumber:   i + 1,
			Content:          chunk,
		})
	}
	return buildAIQueryWindows(parts, aiQueryToolWindowTargetChars, aiQueryToolWindowMaxChars)
}

func buildAIQueryWindows(parts []aiQueryChunk, targetChars int, maxChars int) []aiQueryChunk {
	if len(parts) == 0 {
		return []aiQueryChunk{}
	}
	if targetChars <= 0 {
		targetChars = aiQueryToolWindowTargetChars
	}
	if maxChars <= 0 {
		maxChars = aiQueryToolWindowMaxChars
	}

	windows := make([]aiQueryChunk, 0, len(parts))
	current := aiQueryChunk{}
	currentLen := 0
	for _, part := range parts {
		renderedLen := len(part.Content)
		if current.Number == 0 {
			current = aiQueryChunk{
				Number:           len(windows) + 1,
				StartChunkNumber: part.StartChunkNumber,
				EndChunkNumber:   part.EndChunkNumber,
				Content:          part.Content,
			}
			currentLen = renderedLen
			continue
		}
		if currentLen+2+renderedLen > maxChars || currentLen >= targetChars {
			windows = append(windows, current)
			current = aiQueryChunk{
				Number:           len(windows) + 1,
				StartChunkNumber: part.StartChunkNumber,
				EndChunkNumber:   part.EndChunkNumber,
				Content:          part.Content,
			}
			currentLen = renderedLen
			continue
		}
		current.Content += "\n\n" + part.Content
		current.EndChunkNumber = part.EndChunkNumber
		currentLen += 2 + renderedLen
	}
	if current.Number != 0 {
		windows = append(windows, current)
	}
	return windows
}

func buildAIQueryBatches(chunks []aiQueryChunk, maxChars int) [][]aiQueryChunk {
	if len(chunks) == 0 {
		return [][]aiQueryChunk{}
	}
	if maxChars <= 0 {
		maxChars = aiQueryToolBatchMaxChars
	}

	batches := make([][]aiQueryChunk, 0)
	current := make([]aiQueryChunk, 0)
	currentLen := 0
	for _, chunk := range chunks {
		renderedLen := len(renderAIQueryChunk(chunk))
		if len(current) > 0 && currentLen+2+renderedLen > maxChars {
			batches = append(batches, current)
			current = make([]aiQueryChunk, 0)
			currentLen = 0
		}
		current = append(current, chunk)
		if currentLen > 0 {
			currentLen += 2
		}
		currentLen += renderedLen
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

func selectAIQueryBatches(query string, batches [][]aiQueryChunk, limit int) [][]aiQueryChunk {
	if limit <= 0 || len(batches) <= limit {
		return batches
	}

	keywords := aiQueryKeywords(query)
	if len(keywords) == 0 {
		return batches[:limit]
	}

	// Score every batch cheaply before invoking the AI reader, so the limited
	// reader budget is spent near lexical matches instead of the document start.
	type scoredBatch struct {
		index int
		score int
		batch []aiQueryChunk
	}
	scored := make([]scoredBatch, 0, limit)
	for i, batch := range batches {
		score := scoreAIQueryBatch(batch, keywords)
		if score == 0 {
			continue
		}
		scored = append(scored, scoredBatch{index: i, score: score, batch: batch})
	}
	if len(scored) == 0 {
		return batches[:limit]
	}

	// Keep source order after selecting top-scoring batches. This preserves
	// document context for the reader and stable output ordering for callers.
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].index < scored[j].index
		}
		return scored[i].score > scored[j].score
	})
	selected := scored
	if len(selected) > limit {
		selected = selected[:limit]
	}
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].index < selected[j].index
	})

	result := make([][]aiQueryChunk, len(selected))
	for i, item := range selected {
		result[i] = item.batch
	}
	return result
}

func aiQueryKeywords(query string) []string {
	seen := map[string]bool{}
	keywords := []string{}
	for _, part := range strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		part = strings.TrimSpace(part)
		if utf8.RuneCountInString(part) < 2 || seen[part] || aiQueryStopWords[part] {
			continue
		}
		seen[part] = true
		keywords = append(keywords, part)
	}
	return keywords
}

func scoreAIQueryBatch(batch []aiQueryChunk, keywords []string) int {
	if len(batch) == 0 || len(keywords) == 0 {
		return 0
	}
	var builder strings.Builder
	for _, chunk := range batch {
		if builder.Len() > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(strings.ToLower(chunk.Content))
	}
	content := builder.String()
	score := 0
	for _, keyword := range keywords {
		score += strings.Count(content, keyword)
	}
	return score
}

func renderAIQueryBatch(batch []aiQueryChunk) string {
	parts := make([]string, 0, len(batch))
	for _, chunk := range batch {
		parts = append(parts, renderAIQueryChunk(chunk))
	}
	return strings.Join(parts, "\n\n")
}

func renderAIQueryChunk(chunk aiQueryChunk) string {
	if chunk.StartChunkNumber > 0 && chunk.EndChunkNumber > chunk.StartChunkNumber {
		return fmt.Sprintf("[Chunk %d | Source chunks %d-%d]\n%s", chunk.Number, chunk.StartChunkNumber, chunk.EndChunkNumber, chunk.Content)
	}
	if chunk.StartChunkNumber > 0 {
		return fmt.Sprintf("[Chunk %d | Source chunk %d]\n%s", chunk.Number, chunk.StartChunkNumber, chunk.Content)
	}
	return fmt.Sprintf("[Chunk %d]\n%s", chunk.Number, chunk.Content)
}

func findAIQueryChunk(chunks []aiQueryChunk, chunkNumber int) *aiQueryChunk {
	for i := range chunks {
		if chunks[i].Number == chunkNumber {
			return &chunks[i]
		}
	}
	return nil
}

func normalizeAIQueryToolMaxChunks(value int) int {
	if value <= 0 {
		return defaultAIQueryToolMaxChunks
	}
	if value > maxAIQueryToolMaxChunks {
		return maxAIQueryToolMaxChunks
	}
	return value
}

func normalizeAIQueryToolMaxBatches(config *llm.AIToolsConfig) int {
	if config != nil && config.MaxBatches > 0 {
		return config.MaxBatches
	}
	return aiQueryToolMaxBatches
}

func truncateAIQueryContent(content string, maxRunes int) string {
	content = strings.TrimSpace(content)
	if maxRunes <= 0 || utf8.RuneCountInString(content) <= maxRunes {
		return content
	}
	runes := []rune(content)
	return strings.TrimSpace(string(runes[:maxRunes])) + "..."
}

func extractAIQueryExcerpt(content string, query string, reason string, supportingSnippet string, maxRunes int) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	if maxRunes <= 0 || utf8.RuneCountInString(content) <= maxRunes {
		return content
	}

	if anchorByte, anchorLen := findAIQueryExcerptAnchor(content, query, reason, supportingSnippet); anchorByte >= 0 {
		startRune := utf8.RuneCountInString(content[:anchorByte])
		anchorRunes := utf8.RuneCountInString(content[anchorByte : anchorByte+anchorLen])
		return sliceAIQueryExcerpt(content, startRune, anchorRunes, maxRunes)
	}
	return truncateAIQueryContent(content, maxRunes)
}

func findAIQueryExcerptAnchor(content string, query string, reason string, supportingSnippet string) (int, int) {
	lowerContent := strings.ToLower(content)
	candidates := buildAIQueryExcerptCandidates(supportingSnippet, query, reason)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		lowerCandidate := strings.ToLower(candidate)
		if idx := strings.Index(lowerContent, lowerCandidate); idx >= 0 {
			return idx, len(candidate)
		}
	}
	return -1, 0
}

func buildAIQueryExcerptCandidates(values ...string) []string {
	seen := map[string]bool{}
	candidates := make([]string, 0, 16)
	addCandidate := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if seen[key] {
			return
		}
		seen[key] = true
		candidates = append(candidates, value)
	}
	for _, value := range values {
		for _, phrase := range splitAIQueryExcerptParts(value) {
			addCandidate(phrase)
			for _, token := range splitAIQueryExcerptParts(phrase) {
				if utf8.RuneCountInString(token) >= 4 {
					addCandidate(token)
				}
			}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return utf8.RuneCountInString(candidates[i]) > utf8.RuneCountInString(candidates[j])
	})
	return candidates
}

func splitAIQueryExcerptParts(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case ',', ';', ':', '.', '!', '?', '\n', '\r', '\t', '(', ')', '[', ']', '{', '}':
			return true
		}
		return false
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func sliceAIQueryExcerpt(content string, anchorStartRune int, anchorLenRunes int, maxRunes int) string {
	runes := []rune(content)
	if len(runes) <= maxRunes {
		return content
	}
	if anchorStartRune < 0 {
		anchorStartRune = 0
	}
	if anchorStartRune > len(runes) {
		anchorStartRune = len(runes)
	}
	if anchorLenRunes <= 0 {
		anchorLenRunes = 1
	}
	start := anchorStartRune - maxRunes/3
	if start < 0 {
		start = 0
	}
	end := start + maxRunes
	if end > len(runes) {
		end = len(runes)
		start = end - maxRunes
		if start < 0 {
			start = 0
		}
	}
	if anchorStartRune+anchorLenRunes > end {
		end = anchorStartRune + anchorLenRunes
		if end > len(runes) {
			end = len(runes)
		}
		start = end - maxRunes
		if start < 0 {
			start = 0
		}
	}
	excerpt := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		excerpt = "..." + excerpt
	}
	if end < len(runes) {
		excerpt += "..."
	}
	return excerpt
}

func aiQueryChunkSelectionSchema() string {
	return `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "matches": {
      "type": "array",
      "items": {
        "type": "integer",
        "minimum": 1
      }
    }
  },
  "required": ["matches"]
}`
}

func aiQueryChunkSelectionWithReasonsSchema() string {
	return `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "matches": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
	        "properties": {
	          "chunk_number": {
	            "type": "integer",
	            "minimum": 1
	          },
	          "reason": {
	            "type": "string",
	            "maxLength": 160
	          },
	          "supporting_snippet": {
	            "type": "string",
	            "maxLength": 240
	          }
	        },
	        "required": ["chunk_number", "reason", "supporting_snippet"]
	      }
	    }
	  },
  "required": ["matches"]
}`
}

func aiQuerySelectedChunksFromPlain(response *aiQueryChunkSelection) []aiQuerySelectedChunk {
	if response == nil || len(response.Matches) == 0 {
		return []aiQuerySelectedChunk{}
	}
	selected := make([]aiQuerySelectedChunk, 0, len(response.Matches))
	for _, chunkNumber := range response.Matches {
		selected = append(selected, aiQuerySelectedChunk{ChunkNumber: chunkNumber})
	}
	return selected
}

func aiQuerySelectedChunksFromReasons(response *aiQueryChunkSelectionWithReasons) []aiQuerySelectedChunk {
	if response == nil || len(response.Matches) == 0 {
		return []aiQuerySelectedChunk{}
	}
	selected := make([]aiQuerySelectedChunk, 0, len(response.Matches))
	for _, match := range response.Matches {
		selected = append(selected, aiQuerySelectedChunk{
			ChunkNumber:       match.ChunkNumber,
			Reason:            strings.TrimSpace(match.Reason),
			SupportingSnippet: strings.TrimSpace(match.SupportingSnippet),
		})
	}
	return selected
}

func aiQueryToolErrorOutput(toolName string, err error) string {
	return fmt.Sprintf("%s: semantic retrieval could not be completed: %v. Continue with other tools or approaches.", toolName, err)
}

func aiQueryShouldRetrySelection(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), llm.ResponsesAPIEmptyAssistantOutputError) {
		return true
	}
	var syntaxErr *json.SyntaxError
	return errors.As(err, &syntaxErr)
}

func minAIQueryInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
