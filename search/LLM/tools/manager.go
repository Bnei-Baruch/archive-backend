package tools

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Bnei-Baruch/archive-backend/integration"
	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
)

type AppScopedManagerDeps struct {
	DB                           *sql.DB
	AssetsService                integration.AssetsService
	// AIQueryService is the shared provider client instance used by the AI query tools.
	AIQueryService               llm.Service
	// AIQueryConfig is the per-call reader-model config passed to that service
	// (model, effort, max tokens) for query_source_ai/query_transcript_ai.
	AIQueryConfig                *llm.AIToolsConfig
	NewElasticsearchSearchEngine ElasticsearchSearchEngineFactory
	TimeoutForHighlight          time.Duration
	PostgreSQLToolCacheTTL       *time.Duration
}

func NewAppScopedManager(deps AppScopedManagerDeps) (*llm.ReasoningToolManager, error) {
	if deps.DB == nil {
		return nil, fmt.Errorf("app-scoped reasoning tool manager: db is nil")
	}
	if deps.AssetsService == nil {
		return nil, fmt.Errorf("app-scoped reasoning tool manager: assets service is nil")
	}
	if deps.AIQueryService == nil {
		return nil, fmt.Errorf("app-scoped reasoning tool manager: ai query service is nil")
	}
	if deps.AIQueryConfig == nil {
		return nil, fmt.Errorf("app-scoped reasoning tool manager: ai query config is nil")
	}
	if deps.NewElasticsearchSearchEngine == nil {
		return nil, fmt.Errorf("app-scoped reasoning tool manager: elasticsearch engine factory is nil")
	}

	postgreSQLToolCacheTTL := defaultPostgreSQLToolCacheTTL
	if deps.PostgreSQLToolCacheTTL != nil {
		postgreSQLToolCacheTTL = *deps.PostgreSQLToolCacheTTL
	}

	sourceLookup := newAISourceDocumentLoader(deps.DB, deps.AssetsService)
	transcriptLookup := newAITranscriptDocumentLoader(deps.DB, deps.AssetsService)

	return llm.NewReasoningToolManager(
		NewQuerySourceAITool(sourceLookup, deps.AIQueryService, deps.AIQueryConfig),
		NewQueryTranscriptAITool(transcriptLookup, deps.AIQueryService, deps.AIQueryConfig),
		NewGetAvailableBooksTool(deps.DB, postgreSQLToolCacheTTL),
		NewGetSourcesByAuthorTool(deps.DB, postgreSQLToolCacheTTL),
		NewGetSourcesBySourceTool(deps.DB, postgreSQLToolCacheTTL),
		NewGetCollectionsTool(deps.DB, postgreSQLToolCacheTTL),
		NewGetContentUnitsByCollectionTool(deps.DB, postgreSQLToolCacheTTL),
		NewElasticsearchSearchToolWithFactory(deps.NewElasticsearchSearchEngine, deps.TimeoutForHighlight),
	)
}
