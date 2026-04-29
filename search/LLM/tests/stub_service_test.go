package tests

import (
	"context"
	"strings"
	"testing"

	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
)

func TestStubServiceReturnsStructuredOutputByModelAndQuery(t *testing.T) {
	service := llm.NewStubLLMService([]llm.StubLLMResponseConfig{
		{
			Model:    "stub-planning",
			Query:    "אהבה",
			Response: `{"instruction_text":"use simple search","first_iteration_tools":[{"tool_name":"elasticsearch_search","params_json":"{\"query\":\"אהבה\",\"language\":\"he\"}","alternative_queries":["אהבת הזולת"]}]}`,
		},
	})

	var output llm.ReasoningSearchPlanningResponse
	debug, err := service.GetStructuredOutputWithDebugInfo(
		context.Background(),
		llm.GenerateReasoningSearchPlanningResponseJSONSchema(nil),
		"stub-planning",
		nil,
		[]llm.LLMBotMessage{{Role: "user", Content: "אהבה"}},
		nil,
		nil,
		true,
		&output,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.InstructionText != "use simple search" {
		t.Fatalf("unexpected instruction text: %s", output.InstructionText)
	}
	if len(output.FirstIterationTools) != 1 {
		t.Fatalf("unexpected tool count: %d", len(output.FirstIterationTools))
	}
	if output.FirstIterationTools[0].ToolName != "elasticsearch_search" {
		t.Fatalf("unexpected tool name: %s", output.FirstIterationTools[0].ToolName)
	}
	if debug == nil || debug.Model != "stub-planning" {
		t.Fatalf("unexpected debug info: %#v", debug)
	}
}

func TestStubServiceReturnsReasoningSessionOutput(t *testing.T) {
	service := llm.NewStubLLMService([]llm.StubLLMResponseConfig{
		{
			Model:    "stub-reasoning",
			Query:    "משה",
			Response: `{"query":"משה","summary":"stub summary","reasoning_summary":[],"results":[{"mdb_uid":"u1","reason":"stub result","highlights":[],"is_grouping_result":false}]}`,
		},
	})

	var output llm.ReasoningSearchResponse
	schema, err := llm.GenerateReasoningSearchResponseJSONSchemaForLanguage("English")
	if err != nil {
		t.Fatalf("unexpected schema error: %v", err)
	}
	sessionID, err := service.GetReasoningStructuredOutputWithToolsForSession(
		context.Background(),
		nil,
		nil,
		schema,
		"stub-reasoning",
		nil,
		[]llm.LLMBotMessage{{Role: "user", Content: "משה"}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		true,
		1,
		&output,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(sessionID) == "" {
		t.Fatalf("expected generated session id")
	}
	if output.Summary != "stub summary" {
		t.Fatalf("unexpected summary: %s", output.Summary)
	}
	if output.ReasoningIterations != 1 {
		t.Fatalf("unexpected iterations: %d", output.ReasoningIterations)
	}
	if len(output.UsedTools) != 0 {
		t.Fatalf("unexpected used tools: %#v", output.UsedTools)
	}
	if output.Debug == nil || output.Debug.MainModelUsage == nil || output.Debug.MainModelUsage.Model != "stub-reasoning" {
		t.Fatalf("unexpected debug info: %#v", output.Debug)
	}
}

func TestStubServiceExtractsQueryFromJSONUserPayload(t *testing.T) {
	service := llm.NewStubLLMService([]llm.StubLLMResponseConfig{
		{
			Model:    "stub-verification",
			Query:    "אהבה",
			Response: `{"needs_another_iteration":false,"recommendation":""}`,
		},
	})

	var output llm.ReasoningSearchVerificationResponse
	err := service.GetStructuredOutput(
		context.Background(),
		llm.GenerateReasoningSearchVerificationResponseJSONSchema(),
		"stub-verification",
		nil,
		[]llm.LLMBotMessage{{Role: "user", Content: `{"query":"אהבה","summary":"stub","results":[]}`}},
		nil,
		nil,
		&output,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.NeedsAnotherIteration {
		t.Fatalf("unexpected verification result: %#v", output)
	}
}

func TestStubServiceExtractsQueryFromAIReaderPrompt(t *testing.T) {
	service := llm.NewStubLLMService([]llm.StubLLMResponseConfig{
		{
			Model:    "stub-ai-tools",
			Query:    "אהבה",
			Response: `{"matches":[1]}`,
		},
	})

	var output struct {
		Matches []int `json:"matches"`
	}
	err := service.GetStructuredOutput(
		context.Background(),
		`{"type":"object"}`,
		"stub-ai-tools",
		nil,
		[]llm.LLMBotMessage{{Role: "user", Content: "User query:\nאהבה\n\nSelect up to 1 chunks from this batch."}},
		nil,
		nil,
		&output,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output.Matches) != 1 || output.Matches[0] != 1 {
		t.Fatalf("unexpected matches: %#v", output.Matches)
	}
}

func TestStubServiceSupportsWildcardFallback(t *testing.T) {
	service := llm.NewStubLLMService([]llm.StubLLMResponseConfig{
		{
			Model:    "stub-reasoning",
			Query:    "*",
			Response: `{"query":"fallback","summary":"fallback summary","reasoning_summary":[],"results":[]}`,
		},
	})

	var output llm.ReasoningSearchResponse
	schema, err := llm.GenerateReasoningSearchResponseJSONSchemaForLanguage("English")
	if err != nil {
		t.Fatalf("unexpected schema error: %v", err)
	}
	err = service.GetStructuredOutput(
		context.Background(),
		schema,
		"stub-reasoning",
		nil,
		[]llm.LLMBotMessage{{Role: "user", Content: "missing query"}},
		nil,
		nil,
		&output,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Summary != "fallback summary" {
		t.Fatalf("unexpected summary: %s", output.Summary)
	}
}
