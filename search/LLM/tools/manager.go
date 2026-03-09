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
	NewElasticsearchSearchEngine ElasticsearchSearchEngineFactory
	TimeoutForHighlight          time.Duration
}

func NewAppScopedManager(deps AppScopedManagerDeps) (*llm.ReasoningToolManager, error) {
	if deps.DB == nil {
		return nil, fmt.Errorf("app-scoped reasoning tool manager: db is nil")
	}
	if deps.AssetsService == nil {
		return nil, fmt.Errorf("app-scoped reasoning tool manager: assets service is nil")
	}
	if deps.NewElasticsearchSearchEngine == nil {
		return nil, fmt.Errorf("app-scoped reasoning tool manager: elasticsearch engine factory is nil")
	}

	return llm.NewReasoningToolManager(
		NewSourceLookupTool(deps.DB, deps.AssetsService),
		NewTranscriptLookupTool(deps.DB, deps.AssetsService),
		NewGetSourcesByAuthorTool(deps.DB),
		NewGetCollectionsTool(deps.DB),
		NewGetContentUnitsByCollectionTool(deps.DB),
		NewElasticsearchSearchToolWithFactory(deps.NewElasticsearchSearchEngine, deps.TimeoutForHighlight),
	)
}
