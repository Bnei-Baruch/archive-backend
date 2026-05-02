package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

type ReasoningToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

type ReasoningTool interface {
	Definition() ReasoningToolDefinition
	UsageExplanation() string
	Execute(ctx context.Context, arguments json.RawMessage) (string, error)
}

type ReasoningToolExecution struct {
	Name      string
	Arguments json.RawMessage
}

type ReasoningToolExecutionResult struct {
	Output string
}

type ReasoningToolManager struct {
	toolsByName map[string]ReasoningTool
	order       []string
}

func NewReasoningToolManager(tools ...ReasoningTool) (*ReasoningToolManager, error) {
	manager := &ReasoningToolManager{
		toolsByName: map[string]ReasoningTool{},
		order:       []string{},
	}
	for _, tool := range tools {
		if err := manager.Register(tool); err != nil {
			return nil, err
		}
	}
	return manager, nil
}

func (m *ReasoningToolManager) Register(tool ReasoningTool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}

	definition := tool.Definition()
	if definition.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if _, exists := m.toolsByName[definition.Name]; exists {
		return fmt.Errorf("tool '%s' already registered", definition.Name)
	}

	m.toolsByName[definition.Name] = tool
	m.order = append(m.order, definition.Name)
	return nil
}

func (m *ReasoningToolManager) Definitions() []ReasoningToolDefinition {
	definitions := make([]ReasoningToolDefinition, 0, len(m.order))
	for _, name := range m.order {
		definitions = append(definitions, m.toolsByName[name].Definition())
	}
	return definitions
}

func (m *ReasoningToolManager) ToolCalls() []ToolCall {
	definitions := m.Definitions()
	ret := make([]ToolCall, 0, len(definitions))
	for _, definition := range definitions {
		function := map[string]interface{}{
			"name": definition.Name,
		}
		if definition.Description != "" {
			function["description"] = definition.Description
		}
		if definition.Parameters != nil {
			function["parameters"] = definition.Parameters
		}
		ret = append(ret, ToolCall{
			Type:     "function",
			Function: function,
		})
	}
	return ret
}

func (m *ReasoningToolManager) ToolHandlers() map[string]ToolHandler {
	handlers := map[string]ToolHandler{}
	for _, name := range m.order {
		tool := m.toolsByName[name]
		handlers[name] = wrapReasoningToolHandler(name, tool.Execute)
	}
	return handlers
}

func (m *ReasoningToolManager) Tools() []ReasoningTool {
	tools := make([]ReasoningTool, 0, len(m.order))
	for _, name := range m.order {
		tools = append(tools, m.toolsByName[name])
	}
	return tools
}

func (m *ReasoningToolManager) Len() int {
	return len(m.order)
}

const plannedReasoningToolNamePrefix = "planned__"

func MakePlannedReasoningToolName(base string, index int) string {
	return fmt.Sprintf("%s%s__%d", plannedReasoningToolNamePrefix, base, index)
}

func CanonicalReasoningToolName(name string) string {
	if !strings.HasPrefix(name, plannedReasoningToolNamePrefix) {
		return name
	}
	rest := strings.TrimPrefix(name, plannedReasoningToolNamePrefix)
	if idx := strings.Index(rest, "__"); idx > 0 {
		return rest[:idx]
	}
	return name
}

func ResolveReasoningToolHandler(name string, currentHandlers map[string]ToolHandler, plannedHandlers map[string]ToolHandler) (ToolHandler, bool) {
	if handler, ok := currentHandlers[name]; ok {
		return wrapReasoningToolHandler(name, handler), true
	}
	if !strings.HasPrefix(name, plannedReasoningToolNamePrefix) {
		return nil, false
	}
	handler, ok := plannedHandlers[name] // In case the model is not on his first iteration but still asks for the tool definded by the 'planning' stage, resolve it from the planned handlers.
	if !ok {
		return nil, false
	}
	return wrapReasoningToolHandler(name, handler), true
}

func wrapReasoningToolHandler(name string, handler ToolHandler) ToolHandler {
	return func(ctx context.Context, arguments json.RawMessage) (string, error) {
		result, err := handler(ctx, arguments)
		if err != nil {
			var recoverable *RecoverableToolError
			if errors.As(err, &recoverable) {
				payload := map[string]interface{}{
					"error":           "wrong_tool_usage",
					"retry_suggested": true,
					"message":         recoverable.Message,
				}
				if recoverable.Tool != "" {
					payload["tool"] = recoverable.Tool
				}
				if recoverable.Guidance != "" {
					payload["guidance"] = recoverable.Guidance
				}
				recoverableResult, marshalErr := json.Marshal(payload)
				if marshalErr != nil {
					return result, err
				}
				return string(recoverableResult), nil
			}
			return result, err
		}
		return sanitizeReasoningToolResult(name, result), nil
	}
}

const parallelReasoningToolConcurrency = 4

// Execute tool calls in parallel when the model emits them as one batch.
// Results are written back by input index so provider messages keep the
// same order as the original tool calls.
func ExecuteReasoningToolExecutions(ctx context.Context, calls []ReasoningToolExecution, currentHandlers map[string]ToolHandler, plannedHandlers map[string]ToolHandler) ([]ReasoningToolExecutionResult, error) {
	results := make([]ReasoningToolExecutionResult, len(calls))
	if len(calls) == 0 {
		return results, nil
	}
	if len(calls) <= 1 {
		output, err := executeReasoningToolExecution(ctx, calls[0], currentHandlers, plannedHandlers)
		if err != nil {
			return nil, err
		}
		results[0] = ReasoningToolExecutionResult{Output: output}
		return results, nil
	}

	sem := make(chan struct{}, parallelReasoningToolConcurrency)
	var wg sync.WaitGroup
	var once sync.Once
	var firstErr error
	setErr := func(err error) {
		if err != nil {
			once.Do(func() { firstErr = err })
		}
	}

	for i := range calls {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				setErr(ctx.Err())
				return
			}
			output, err := executeReasoningToolExecution(ctx, calls[i], currentHandlers, plannedHandlers)
			if err != nil {
				setErr(err)
				return
			}
			results[i] = ReasoningToolExecutionResult{Output: output}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

func executeReasoningToolExecution(ctx context.Context, call ReasoningToolExecution, currentHandlers map[string]ToolHandler, plannedHandlers map[string]ToolHandler) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	handler, ok := ResolveReasoningToolHandler(call.Name, currentHandlers, plannedHandlers)
	if !ok {
		return "", fmt.Errorf("missing handler for tool '%s'", call.Name)
	}
	args := call.Arguments
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	result, err := handler(ctx, args)
	if err != nil {
		return "", fmt.Errorf("tool '%s' execution failed: %w", call.Name, err)
	}
	return result, nil
}

func sanitizeReasoningToolResult(name string, result string) string {
	if CanonicalReasoningToolName(name) != "elasticsearch_search" {
		return result
	}

	var payload interface{}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		return result
	}
	removeElasticsearchInternalKeys(payload)
	sanitized, err := json.Marshal(payload)
	if err != nil {
		return result
	}
	return string(sanitized)
}

func removeElasticsearchInternalKeys(value interface{}) {
	switch typed := value.(type) {
	case map[string]interface{}:
		delete(typed, "_id")
		delete(typed, "_index")
		delete(typed, "_type")
		for _, child := range typed {
			removeElasticsearchInternalKeys(child)
		}
	case []interface{}:
		for _, child := range typed {
			removeElasticsearchInternalKeys(child)
		}
	}
}
