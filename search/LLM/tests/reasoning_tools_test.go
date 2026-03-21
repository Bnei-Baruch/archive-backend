package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Bnei-Baruch/archive-backend/integration"
	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
	llmtools "github.com/Bnei-Baruch/archive-backend/search/LLM/tools"
)

type fakeReasoningTool struct {
	definition       llm.ReasoningToolDefinition
	usageExplanation string
}

func (t *fakeReasoningTool) Definition() llm.ReasoningToolDefinition {
	return t.definition
}

func (t *fakeReasoningTool) UsageExplanation() string {
	return t.usageExplanation
}

func (t *fakeReasoningTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	_ = ctx
	return "ok", nil
}

func TestReasoningToolManagerRegisterDuplicate(t *testing.T) {
	manager, err := llm.NewReasoningToolManager()
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}

	err = manager.Register(&fakeReasoningTool{
		definition: llm.ReasoningToolDefinition{Name: "source_lookup"},
	})
	if err != nil {
		t.Fatalf("unexpected error registering first tool: %v", err)
	}

	err = manager.Register(&fakeReasoningTool{
		definition: llm.ReasoningToolDefinition{Name: "source_lookup"},
	})
	if err == nil {
		t.Fatalf("expected duplicate tool registration error")
	}
}

func TestReasoningToolManagerGeneratesToolCallsAndHandlers(t *testing.T) {
	manager, err := llm.NewReasoningToolManager(
		&fakeReasoningTool{
			definition: llm.ReasoningToolDefinition{
				Name:        "source_lookup",
				Description: "Lookup source text",
				Parameters: map[string]interface{}{
					"type": "object",
				},
			},
			usageExplanation: "Tool: source_lookup",
		},
	)
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}

	toolCalls := manager.ToolCalls()
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}
	if toolCalls[0].Type != "function" {
		t.Fatalf("unexpected tool type: %s", toolCalls[0].Type)
	}

	handlers := manager.ToolHandlers()
	handler, ok := handlers["source_lookup"]
	if !ok {
		t.Fatalf("missing handler for source_lookup")
	}
	result, err := handler(context.Background(), json.RawMessage(`{"source_id":"abc123"}`))
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("unexpected handler result: %s", result)
	}
}

func TestGenerateSystemMessageForReasoningSearchIncludesToolUsage(t *testing.T) {
	manager, err := llm.NewReasoningToolManager(
		&fakeReasoningTool{
			definition:       llm.ReasoningToolDefinition{Name: "tool_a"},
			usageExplanation: "Tool: tool_a\nUse it first.",
		},
		&fakeReasoningTool{
			definition:       llm.ReasoningToolDefinition{Name: "tool_b"},
			usageExplanation: "Tool: tool_b\nUse it second.",
		},
	)
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}

	message := llm.GenerateSystemMessageForReasoningSearch(manager.Tools(), 20)
	if !strings.Contains(message, "Available tools and usage instructions:") {
		t.Fatalf("expected tool usage header in message: %s", message)
	}
	if !strings.Contains(message, "Current request tool rounds remaining: 20.") {
		t.Fatalf("expected remaining iterations text in message: %s", message)
	}
	if !strings.Contains(message, "Tool: tool_a\nUse it first.") {
		t.Fatalf("expected first tool usage explanation in message: %s", message)
	}
	if !strings.Contains(message, "Tool: tool_b\nUse it second.") {
		t.Fatalf("expected second tool usage explanation in message: %s", message)
	}
	if strings.Index(message, "Tool: tool_a") > strings.Index(message, "Tool: tool_b") {
		t.Fatalf("expected tool explanations to preserve manager order: %s", message)
	}
}

func TestGenerateReasoningSearchResponseJSONSchemaIncludesRequiredFields(t *testing.T) {
	schema := llm.GenerateReasoningSearchResponseJSONSchema()

	requiredSnippets := []string{
		`"query"`,
		`"summary"`,
		`"reasoning_summary"`,
		`"used_tokens"`,
		`"reasoning_iterations"`,
		`"results"`,
		`"mdb_uid"`,
		`"result_type"`,
		`"reason"`,
		`"highlights"`,
		`"language"`,
		`"date"`,
		`"is_grouping_result"`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(schema, snippet) {
			t.Fatalf("expected schema to contain %s", snippet)
		}
	}
}

func TestPostgreSQLToolDefinitions(t *testing.T) {
	sourceFilterValues := llmtools.NewGetSourceFilterValuesTool().Definition()
	if sourceFilterValues.Name != "get_source_filter_values" {
		t.Fatalf("unexpected get_source_filter_values tool name: %s", sourceFilterValues.Name)
	}

	sourcesByAuthor := llmtools.NewGetSourcesByAuthorTool(nil).Definition()
	if sourcesByAuthor.Name != "get_sources_by_author" {
		t.Fatalf("unexpected get_sources_by_author tool name: %s", sourcesByAuthor.Name)
	}

	collections := llmtools.NewGetCollectionsTool(nil).Definition()
	if collections.Name != "get_collections" {
		t.Fatalf("unexpected get_collections tool name: %s", collections.Name)
	}

	contentUnitsByCollection := llmtools.NewGetContentUnitsByCollectionTool(nil).Definition()
	if contentUnitsByCollection.Name != "get_content_units_by_collection" {
		t.Fatalf("unexpected get_content_units_by_collection tool name: %s", contentUnitsByCollection.Name)
	}
}

func TestElasticsearchSearchToolDefinition(t *testing.T) {
	elasticsearchSearch := llmtools.NewElasticsearchSearchTool(nil, 0).Definition()
	if elasticsearchSearch.Name != "elasticsearch_search" {
		t.Fatalf("unexpected elasticsearch_search tool name: %s", elasticsearchSearch.Name)
	}
}

type fakeAssetsService struct{}

func (s *fakeAssetsService) Doc2Text(uid string) (string, error) {
	return uid, nil
}

func (s *fakeAssetsService) Prepare(uids []string) (bool, map[string]int, error) {
	return false, map[string]int{}, nil
}

var _ integration.AssetsService = (*fakeAssetsService)(nil)

func TestNewAppScopedManager(t *testing.T) {
	manager, err := llmtools.NewAppScopedManager(llmtools.AppScopedManagerDeps{
		DB:            &sql.DB{},
		AssetsService: &fakeAssetsService{},
		NewElasticsearchSearchEngine: func() (llmtools.ElasticsearchSearchEngine, error) {
			return &fakeElasticsearchSearchEngine{}, nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error creating app-scoped manager: %v", err)
	}

	if manager.Len() != 7 {
		t.Fatalf("unexpected tool count: %d", manager.Len())
	}

	names := make([]string, 0, len(manager.Definitions()))
	for _, definition := range manager.Definitions() {
		names = append(names, definition.Name)
	}

	expected := []string{
		"source_lookup",
		"transcript_lookup",
		"get_source_filter_values",
		"get_sources_by_author",
		"get_collections",
		"get_content_units_by_collection",
		"elasticsearch_search",
	}
	for i, name := range expected {
		if names[i] != name {
			t.Fatalf("unexpected tool order at %d: got %s want %s", i, names[i], name)
		}
	}
}

func TestNewAppScopedManagerRejectsMissingDeps(t *testing.T) {
	testCases := []struct {
		name string
		deps llmtools.AppScopedManagerDeps
		want string
	}{
		{
			name: "missing db",
			deps: llmtools.AppScopedManagerDeps{
				AssetsService: &fakeAssetsService{},
				NewElasticsearchSearchEngine: func() (llmtools.ElasticsearchSearchEngine, error) {
					return &fakeElasticsearchSearchEngine{}, nil
				},
			},
			want: "db is nil",
		},
		{
			name: "missing assets service",
			deps: llmtools.AppScopedManagerDeps{
				DB: &sql.DB{},
				NewElasticsearchSearchEngine: func() (llmtools.ElasticsearchSearchEngine, error) {
					return &fakeElasticsearchSearchEngine{}, nil
				},
			},
			want: "assets service is nil",
		},
		{
			name: "missing engine factory",
			deps: llmtools.AppScopedManagerDeps{
				DB:            &sql.DB{},
				AssetsService: &fakeAssetsService{},
			},
			want: "engine factory is nil",
		},
	}

	for _, tc := range testCases {
		_, err := llmtools.NewAppScopedManager(tc.deps)
		if err == nil {
			t.Fatalf("%s: expected error", tc.name)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
	}
}
