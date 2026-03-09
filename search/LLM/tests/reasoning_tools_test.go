package tests

import (
	"encoding/json"
	"testing"

	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
	llmtools "github.com/Bnei-Baruch/archive-backend/search/LLM/tools"
)

type fakeReasoningTool struct {
	definition llm.ReasoningToolDefinition
}

func (t *fakeReasoningTool) Definition() llm.ReasoningToolDefinition {
	return t.definition
}

func (t *fakeReasoningTool) Execute(arguments json.RawMessage) (string, error) {
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
	result, err := handler(json.RawMessage(`{"source_id":"abc123"}`))
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("unexpected handler result: %s", result)
	}
}

func TestPostgreSQLToolDefinitions(t *testing.T) {
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
