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
	response         string
	err              error
}

func (t *fakeReasoningTool) Definition() llm.ReasoningToolDefinition {
	return t.definition
}

func (t *fakeReasoningTool) UsageExplanation() string {
	return t.usageExplanation
}

func (t *fakeReasoningTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	_ = ctx
	if t.err != nil {
		return "", t.err
	}
	if t.response != "" {
		return t.response, nil
	}
	return "ok", nil
}

func TestReasoningToolManagerRegisterDuplicate(t *testing.T) {
	manager, err := llm.NewReasoningToolManager()
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}

	err = manager.Register(&fakeReasoningTool{
		definition: llm.ReasoningToolDefinition{Name: "fake_lookup"},
	})
	if err != nil {
		t.Fatalf("unexpected error registering first tool: %v", err)
	}

	err = manager.Register(&fakeReasoningTool{
		definition: llm.ReasoningToolDefinition{Name: "fake_lookup"},
	})
	if err == nil {
		t.Fatalf("expected duplicate tool registration error")
	}
}

func TestReasoningToolManagerGeneratesToolCallsAndHandlers(t *testing.T) {
	manager, err := llm.NewReasoningToolManager(
		&fakeReasoningTool{
			definition: llm.ReasoningToolDefinition{
				Name:        "fake_lookup",
				Description: "Lookup source text",
				Parameters: map[string]interface{}{
					"type": "object",
				},
			},
			usageExplanation: "Tool: fake_lookup",
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
	handler, ok := handlers["fake_lookup"]
	if !ok {
		t.Fatalf("missing handler for fake_lookup")
	}
	result, err := handler(context.Background(), json.RawMessage(`{"source_id":"abc123"}`))
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("unexpected handler result: %s", result)
	}
}

func TestReasoningToolManagerReturnsRecoverableToolErrorAsResult(t *testing.T) {
	manager, err := llm.NewReasoningToolManager(&fakeReasoningTool{
		definition: llm.ReasoningToolDefinition{Name: "fake_lookup"},
		err:        llm.NewRecoverableToolError("fake_lookup", "missing required argument", "Retry with the required argument."),
	})
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}

	handler := manager.ToolHandlers()["fake_lookup"]
	result, err := handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("expected recoverable tool error as tool result, got %v", err)
	}
	if !strings.Contains(result, `"error":"wrong_tool_usage"`) {
		t.Fatalf("expected wrong_tool_usage result, got %s", result)
	}
	if !strings.Contains(result, `"retry_suggested":true`) {
		t.Fatalf("expected retry_suggested result, got %s", result)
	}
}

func TestReasoningToolManagerHidesElasticsearchInternalHitIDs(t *testing.T) {
	manager, err := llm.NewReasoningToolManager(
		&fakeReasoningTool{
			definition: llm.ReasoningToolDefinition{Name: "elasticsearch_search"},
			response:   `{"result":{"search_result":{"hits":{"hits":[{"_index":"units","_type":"_doc","_id":"bad-hit-id","_score":12.3,"_source":{"mdb_uid":"good-mdb-uid","title":"Good"}}]}}}}`,
		},
	)
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}

	handler := manager.ToolHandlers()["elasticsearch_search"]
	result, err := handler(context.Background(), json.RawMessage(`{"query":"test"}`))
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if strings.Contains(result, "_id") || strings.Contains(result, "_index") || strings.Contains(result, "_type") || strings.Contains(result, "bad-hit-id") {
		t.Fatalf("expected ES internal hit ids to be hidden, got %s", result)
	}
	if !strings.Contains(result, "good-mdb-uid") {
		t.Fatalf("expected mdb_uid to remain, got %s", result)
	}
	if !strings.Contains(result, "_score") {
		t.Fatalf("expected _score to remain, got %s", result)
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

	message := llm.GenerateSystemMessageForReasoningSearch(manager.Tools(), 20, 2)
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

	lowBudgetMessage := llm.GenerateSystemMessageForReasoningSearch(manager.Tools(), 4, 2)
	if !strings.Contains(lowBudgetMessage, "Current request tool rounds remaining: 4.") {
		t.Fatalf("expected remaining iterations text when tool budget is low: %s", lowBudgetMessage)
	}

	finalFollowupMessage := llm.GenerateSystemMessageForReasoningSearch(manager.Tools(), 20, 0)
	if !strings.Contains(finalFollowupMessage, "no further follow-up requests remain in this session after this response") {
		t.Fatalf("expected no-followup instruction when budget is exhausted: %s", finalFollowupMessage)
	}
}

func TestBuildFirstIterationReasoningSearchSystemMessageUsesPlannedToolDocs(t *testing.T) {
	systemMessage := strings.Join([]string{
		"Base instructions.",
		"Available tools and usage instructions:",
		"Tool: elasticsearch_search\nUse normal search.",
		"Tool: get_collections\nUse collections.",
		"YOU MUST FOLLOW THIS INSTRUCTION ON SEARCH STRATEGY FOR THE GIVEN QUERY:\nSearch inside the planned collection first.",
	}, "\n\n")
	planned := []llm.ToolCall{
		{
			Type: "function",
			Function: map[string]interface{}{
				"name":        "planned__elasticsearch_search__1",
				"description": `Search archive content. Predefined fixed arguments: {"query":"חיים חדשים","language":"he"}`,
				"parameters": map[string]interface{}{
					"type":                 "object",
					"properties":           map[string]interface{}{},
					"additionalProperties": false,
				},
			},
		},
	}

	message := llm.BuildFirstIterationReasoningSearchSystemMessage(systemMessage, planned)
	if !strings.Contains(message, "Tool: planned__elasticsearch_search__1") {
		t.Fatalf("expected planned tool name in first-iteration message: %s", message)
	}
	if !strings.Contains(message, "Predefined fixed arguments") {
		t.Fatalf("expected fixed arguments in planned tool docs: %s", message)
	}
	if strings.Contains(message, "Tool: elasticsearch_search\nUse normal search.") {
		t.Fatalf("did not expect base tool usage docs in first-iteration message: %s", message)
	}
	if strings.Contains(message, "Tool: get_collections") {
		t.Fatalf("did not expect hidden tool usage docs in first-iteration message: %s", message)
	}
	if !strings.Contains(message, "YOU MUST FOLLOW THIS INSTRUCTION ON SEARCH STRATEGY") {
		t.Fatalf("expected planning guidance to be preserved: %s", message)
	}
}

func TestResolveReasoningToolHandlerAllowsExactPlannedFallback(t *testing.T) {
	currentHandlers := map[string]llm.ToolHandler{
		"elasticsearch_search": func(ctx context.Context, arguments json.RawMessage) (string, error) {
			return "base", nil
		},
	}
	plannedHandlers := map[string]llm.ToolHandler{
		"planned__elasticsearch_search__4": func(ctx context.Context, arguments json.RawMessage) (string, error) {
			return "planned", nil
		},
	}

	handler, ok := llm.ResolveReasoningToolHandler("planned__elasticsearch_search__4", currentHandlers, plannedHandlers)
	if !ok {
		t.Fatalf("expected planned handler fallback")
	}
	result, err := handler(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if result != "planned" {
		t.Fatalf("unexpected handler result: %s", result)
	}

	if _, ok := llm.ResolveReasoningToolHandler("planned__elasticsearch_search__9", currentHandlers, plannedHandlers); ok {
		t.Fatalf("did not expect fallback for an unknown planned tool")
	}
}

func TestResolveReasoningToolHandlerSanitizesPlannedElasticsearchResult(t *testing.T) {
	plannedHandlers := map[string]llm.ToolHandler{
		"planned__elasticsearch_search__1": func(ctx context.Context, arguments json.RawMessage) (string, error) {
			return `{"hits":[{"_id":"bad-hit-id","_source":{"mdb_uid":"good-mdb-uid"}}]}`, nil
		},
	}

	handler, ok := llm.ResolveReasoningToolHandler("planned__elasticsearch_search__1", nil, plannedHandlers)
	if !ok {
		t.Fatalf("expected planned handler fallback")
	}
	result, err := handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if strings.Contains(result, "_id") || strings.Contains(result, "bad-hit-id") {
		t.Fatalf("expected planned ES result to be sanitized, got %s", result)
	}
	if !strings.Contains(result, "good-mdb-uid") {
		t.Fatalf("expected mdb_uid to remain, got %s", result)
	}
}

func TestExecuteReasoningToolExecutionsReturnsRecoverableOutputForMissingTool(t *testing.T) {
	results, err := llm.ExecuteReasoningToolExecutions(
		context.Background(),
		[]llm.ReasoningToolExecution{{Name: "get_source", Arguments: json.RawMessage(`{}`)}},
		map[string]llm.ToolHandler{},
		nil,
	)
	if err != nil {
		t.Fatalf("expected missing tool to be recoverable, got error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(results[0].Output), &payload); err != nil {
		t.Fatalf("expected JSON recoverable payload: %v", err)
	}
	if payload["error"] != "wrong_tool_usage" {
		t.Fatalf("unexpected payload: %v", payload)
	}
	if payload["tool"] != "get_source" {
		t.Fatalf("expected missing tool name in payload, got %v", payload["tool"])
	}
}

func TestGenerateReasoningSearchResponseJSONSchemaIncludesRequiredFields(t *testing.T) {
	schema, err := llm.GenerateReasoningSearchResponseJSONSchemaForLanguage("English")
	if err != nil {
		t.Fatalf("unexpected schema error: %v", err)
	}

	requiredSnippets := []string{
		`"query"`,
		`"summary"`,
		`"reasoning_summary"`,
		`"results"`,
		`"mdb_uid"`,
		`"reason"`,
		`"highlights"`,
		`"is_grouping_result"`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(schema, snippet) {
			t.Fatalf("expected schema to contain %s", snippet)
		}
	}
	if strings.Contains(schema, `"result_type"`) {
		t.Fatalf("did not expect result_type in LLM response schema: %s", schema)
	}

}

func TestGenerateReasoningSearchVerificationResponseJSONSchemaIncludesRequiredFields(t *testing.T) {
	schema := llm.GenerateReasoningSearchVerificationResponseJSONSchema()

	requiredSnippets := []string{
		`"needs_another_iteration"`,
		`"recommendation"`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(schema, snippet) {
			t.Fatalf("expected schema to contain %s", snippet)
		}
	}
}

func TestGenerateReasoningSearchPlanningResponseJSONSchemaUsesStrictToolParams(t *testing.T) {
	manager, err := llm.NewReasoningToolManager(
		llmtools.NewElasticsearchSearchTool(nil, 0),
		llmtools.NewGetCollectionsTool(nil, 0),
	)
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}

	schema := llm.GenerateReasoningSearchPlanningResponseJSONSchema(manager.Tools())
	requiredSnippets := []string{
		`"first_iteration_tools"`,
		`"tool_name"`,
		`"params_json"`,
		`"elasticsearch_search"`,
		`"get_collections"`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(schema, snippet) {
			t.Fatalf("expected planning schema to contain %s", snippet)
		}
	}
	if strings.Contains(schema, `"additionalProperties":true`) {
		t.Fatalf("planning schema should not contain open additionalProperties: %s", schema)
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

	contentUnit := llmtools.NewGetContentUnitTool(nil, 0).Definition()
	if contentUnit.Name != "get_content_unit" {
		t.Fatalf("unexpected get_content_unit tool name: %s", contentUnit.Name)
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

func TestQuerySourceAIToolDefinitionIncludesQueryArguments(t *testing.T) {
	sourceLookup := llmtools.NewQuerySourceAITool(nil, &fakeLLMService{}, &llm.AIToolsConfig{
		Provider:  "openai",
		Model:     "test-model",
		Effort:    "low",
		MaxTokens: 256,
	}).Definition()
	if sourceLookup.Name != "query_source_ai" {
		t.Fatalf("unexpected query_source_ai tool name: %s", sourceLookup.Name)
	}

	parameters, ok := sourceLookup.Parameters.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map parameters, got %T", sourceLookup.Parameters)
	}
	properties, ok := parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map properties, got %T", parameters["properties"])
	}

	for _, property := range []string{"query", "max_chunks"} {
		if _, ok := properties[property]; !ok {
			t.Fatalf("expected query_source_ai to define %q", property)
		}
	}
	for _, property := range []string{"neighbor_chunks", "chunk_number"} {
		if _, ok := properties[property]; ok {
			t.Fatalf("did not expect query_source_ai to define %q", property)
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

type fakeLLMService struct{}

func (s *fakeLLMService) GetStructuredOutput(context.Context, string, string, *int, []llm.LLMBotMessage, *string, *string, interface{}) error {
	return nil
}

func (s *fakeLLMService) GetStructuredOutputWithDebugInfo(context.Context, string, string, *int, []llm.LLMBotMessage, *string, *string, bool, interface{}) (*llm.ReasoningSearchDebugInfo, error) {
	return nil, nil
}

func (s *fakeLLMService) GetChatResponse(context.Context, string, *int, []llm.LLMBotMessage, *string, *float64, *string, *string) (*llm.LLMBotMessage, error) {
	return nil, nil
}

func (s *fakeLLMService) GetChatResponseWithDebugInfo(context.Context, string, *int, []llm.LLMBotMessage, *string, *float64, *string, *string, bool) (*llm.LLMBotMessage, *llm.ReasoningSearchDebugInfo, error) {
	return nil, nil, nil
}

func (s *fakeLLMService) GetReasoningResponseWithTools(context.Context, string, *int, []llm.LLMBotMessage, []llm.ToolCall, map[string]llm.ToolHandler, *string, *string, bool, int) (*llm.LLMBotMessage, error) {
	return nil, nil
}

func (s *fakeLLMService) GetReasoningStructuredOutputWithTools(context.Context, string, string, *int, []llm.LLMBotMessage, []llm.ToolCall, map[string]llm.ToolHandler, *string, *string, bool, int, interface{}) error {
	return nil
}

func (s *fakeLLMService) GetReasoningStructuredOutputWithToolsForSession(context.Context, *string, *string, string, string, *int, []llm.LLMBotMessage, []llm.ToolCall, map[string]llm.ToolHandler, []llm.ToolCall, map[string]llm.ToolHandler, *string, *string, bool, int, interface{}) (string, error) {
	return "", nil
}

func (s *fakeLLMService) ReserveReasoningSession(context.Context, string, *string) (string, error) {
	return "", nil
}

func (s *fakeLLMService) GetEmbeddings(context.Context, string) ([]float64, error) {
	return nil, nil
}

func TestNewAppScopedManager(t *testing.T) {
	manager, err := llmtools.NewAppScopedManager(llmtools.AppScopedManagerDeps{
		DB:             &sql.DB{},
		AssetsService:  &fakeAssetsService{},
		AIQueryService: &fakeLLMService{},
		AIQueryConfig: &llm.AIToolsConfig{
			Provider:  "openai",
			Model:     "test-model",
			Effort:    "low",
			MaxTokens: 256,
		},
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
		"query_source_ai",
		"query_transcript_ai",
		"get_available_books",
		"get_sources_by_author",
		"get_sources_by_source",
		"get_collections",
		"get_content_unit",
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
				AssetsService:  &fakeAssetsService{},
				AIQueryService: &fakeLLMService{},
				AIQueryConfig: &llm.AIToolsConfig{
					Provider:  "openai",
					Model:     "test-model",
					Effort:    "low",
					MaxTokens: 256,
				},
				NewElasticsearchSearchEngine: func() (llmtools.ElasticsearchSearchEngine, error) {
					return &fakeElasticsearchSearchEngine{}, nil
				},
			},
			want: "db is nil",
		},
		{
			name: "missing assets service",
			deps: llmtools.AppScopedManagerDeps{
				DB:             &sql.DB{},
				AIQueryService: &fakeLLMService{},
				AIQueryConfig: &llm.AIToolsConfig{
					Provider:  "openai",
					Model:     "test-model",
					Effort:    "low",
					MaxTokens: 256,
				},
				NewElasticsearchSearchEngine: func() (llmtools.ElasticsearchSearchEngine, error) {
					return &fakeElasticsearchSearchEngine{}, nil
				},
			},
			want: "assets service is nil",
		},
		{
			name: "missing engine factory",
			deps: llmtools.AppScopedManagerDeps{
				DB:             &sql.DB{},
				AssetsService:  &fakeAssetsService{},
				AIQueryService: &fakeLLMService{},
				AIQueryConfig: &llm.AIToolsConfig{
					Provider:  "openai",
					Model:     "test-model",
					Effort:    "low",
					MaxTokens: 256,
				},
			},
			want: "engine factory is nil",
		},
		{
			name: "missing ai service",
			deps: llmtools.AppScopedManagerDeps{
				DB:            &sql.DB{},
				AssetsService: &fakeAssetsService{},
				AIQueryConfig: &llm.AIToolsConfig{
					Provider:  "openai",
					Model:     "test-model",
					Effort:    "low",
					MaxTokens: 256,
				},
				NewElasticsearchSearchEngine: func() (llmtools.ElasticsearchSearchEngine, error) {
					return &fakeElasticsearchSearchEngine{}, nil
				},
			},
			want: "ai query service is nil",
		},
		{
			name: "missing ai config",
			deps: llmtools.AppScopedManagerDeps{
				DB:             &sql.DB{},
				AssetsService:  &fakeAssetsService{},
				AIQueryService: &fakeLLMService{},
				NewElasticsearchSearchEngine: func() (llmtools.ElasticsearchSearchEngine, error) {
					return &fakeElasticsearchSearchEngine{}, nil
				},
			},
			want: "ai query config is nil",
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
