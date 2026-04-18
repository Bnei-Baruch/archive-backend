package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
		handlers[name] = tool.Execute
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
