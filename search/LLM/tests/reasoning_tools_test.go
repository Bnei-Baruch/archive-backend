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
	if strings.Contains(message, "Current request tool rounds remaining: 20.") {
		t.Fatalf("did not expect remaining iterations text when tool budget is high: %s", message)
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

	lowBudgetMessage := llm.GenerateSystemMessageForReasoningSearch(manager.Tools(), 4)
	if !strings.Contains(lowBudgetMessage, "Current request tool rounds remaining: 4.") {
		t.Fatalf("expected remaining iterations text when tool budget is low: %s", lowBudgetMessage)
	}
}

func TestGenerateReasoningSearchResponseJSONSchemaIncludesRequiredFields(t *testing.T) {
	schema := llm.GenerateReasoningSearchResponseJSONSchema()

	requiredSnippets := []string{
		`"query"`,
		`"summary"`,
		`"reasoning_summary"`,
		`"results"`,
		`"mdb_uid"`,
		`"result_type"`,
		`"content_type"`,
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

	enumSnippets := []string{
		`"ARTICLES"`,
		`"DAILY_LESSON"`,
		`"ARTICLE"`,
		`"FULL_LESSON"`,
		`"SOURCE"`,
	}

	for _, snippet := range enumSnippets {
		if !strings.Contains(schema, snippet) {
			t.Fatalf("expected schema to contain content_type enum value %s", snippet)
		}
	}
}

func TestPostgreSQLToolDefinitions(t *testing.T) {
	availableBooks := llmtools.NewGetAvailableBooksTool(nil, 0).Definition()
	if availableBooks.Name != "get_available_books" {
		t.Fatalf("unexpected get_available_books tool name: %s", availableBooks.Name)
	}
	parameters, ok := availableBooks.Parameters.(map[string]interface{})
	if !ok {
		t.Fatalf("expected get_available_books parameters map, got %T", availableBooks.Parameters)
	}
	properties, ok := parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected get_available_books properties map, got %T", parameters["properties"])
	}
	if len(properties) != 0 {
		t.Fatalf("expected get_available_books to define no properties, got %v", properties)
	}

	sourcesByAuthor := llmtools.NewGetSourcesByAuthorTool(nil, 0).Definition()
	if sourcesByAuthor.Name != "get_sources_by_author" {
		t.Fatalf("unexpected get_sources_by_author tool name: %s", sourcesByAuthor.Name)
	}

	sourcesBySource := llmtools.NewGetSourcesBySourceTool(nil, 0).Definition()
	if sourcesBySource.Name != "get_sources_by_source" {
		t.Fatalf("unexpected get_sources_by_source tool name: %s", sourcesBySource.Name)
	}

	collections := llmtools.NewGetCollectionsTool(nil, 0).Definition()
	if collections.Name != "get_collections" {
		t.Fatalf("unexpected get_collections tool name: %s", collections.Name)
	}
	collectionsParameters, ok := collections.Parameters.(map[string]interface{})
	if !ok {
		t.Fatalf("expected get_collections parameters map, got %T", collections.Parameters)
	}
	collectionsProperties, ok := collectionsParameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected get_collections properties map, got %T", collectionsParameters["properties"])
	}
	if _, ok := collectionsProperties["query"]; ok {
		t.Fatalf("did not expect get_collections to define query")
	}

	contentUnitsByCollection := llmtools.NewGetContentUnitsByCollectionTool(nil, 0).Definition()
	if contentUnitsByCollection.Name != "get_content_units_by_collection" {
		t.Fatalf("unexpected get_content_units_by_collection tool name: %s", contentUnitsByCollection.Name)
	}
}

func TestGetAvailableBooksToolReturnsItems(t *testing.T) {
	tool := llmtools.NewGetAvailableBooksTool(nil, 0)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatalf("expected get_available_books to fail when db is nil")
	}
	if !strings.Contains(err.Error(), "db is nil") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetAvailableBooksToolRejectsArguments(t *testing.T) {
	tool := llmtools.NewGetAvailableBooksTool(nil, 0)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{"author_id":"bs"}`))
	if err == nil {
		t.Fatalf("expected get_available_books to reject arguments")
	}
	if !strings.Contains(err.Error(), "does not accept arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestElasticsearchSearchToolDefinition(t *testing.T) {
	elasticsearchSearch := llmtools.NewElasticsearchSearchTool(nil, 0).Definition()
	if elasticsearchSearch.Name != "elasticsearch_search" {
		t.Fatalf("unexpected elasticsearch_search tool name: %s", elasticsearchSearch.Name)
	}
}

func TestSourceLookupToolDefinitionIncludesChunkArguments(t *testing.T) {
	sourceLookup := llmtools.NewSourceLookupTool(nil, nil).Definition()
	if sourceLookup.Name != "source_lookup" {
		t.Fatalf("unexpected source_lookup tool name: %s", sourceLookup.Name)
	}

	parameters, ok := sourceLookup.Parameters.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map parameters, got %T", sourceLookup.Parameters)
	}
	properties, ok := parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map properties, got %T", parameters["properties"])
	}

	for _, property := range []string{"query", "chunk_number"} {
		if _, ok := properties[property]; !ok {
			t.Fatalf("expected source_lookup to define %q", property)
		}
	}
	for _, property := range []string{"neighbor_chunks", "max_chunks"} {
		if _, ok := properties[property]; ok {
			t.Fatalf("did not expect source_lookup to define %q", property)
		}
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

	names := make([]string, 0, len(manager.Definitions()))
	for _, definition := range manager.Definitions() {
		names = append(names, definition.Name)
	}

	expected := []string{
		"source_lookup",
		"transcript_lookup",
		"get_available_books",
		"get_sources_by_author",
		"get_sources_by_source",
		"get_collections",
		"get_content_units_by_collection",
		"elasticsearch_search",
	}
	if manager.Len() != len(expected) {
		t.Fatalf("unexpected tool count: %d", manager.Len())
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
