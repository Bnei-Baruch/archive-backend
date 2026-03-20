package tests

import (
	"strings"
	"testing"

	"github.com/spf13/viper"

	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
)

func TestNewServiceFromConfigDefaultsToOpenAI(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldToken := viper.GetString("openai.token")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("openai.token", oldToken)

	viper.Set("llm.provider", "")
	viper.Set("openai.token", "test-token")

	service, err := llm.NewServiceFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := service.(*llm.OpenAIService); !ok {
		t.Fatalf("unexpected service type: %T", service)
	}
}

func TestNewServiceFromConfigRejectsUnknownProvider(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	defer viper.Set("llm.provider", oldProvider)

	viper.Set("llm.provider", "unknown")

	_, err := llm.NewServiceFromConfig()
	if err == nil {
		t.Fatalf("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unsupported llm provider") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReasoningSearchConfigFromConfigUsesOpenAISection(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldModel := viper.GetString("openai.reasoning-search-model")
	oldEffort := viper.GetString("openai.reasoning-search-effort")
	oldMaxTokens := viper.GetInt("openai.reasoning-search-max-output-tokens")
	oldMaxIterations := viper.GetInt("openai.reasoning-search-max-iterations")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("openai.reasoning-search-model", oldModel)
	defer viper.Set("openai.reasoning-search-effort", oldEffort)
	defer viper.Set("openai.reasoning-search-max-output-tokens", oldMaxTokens)
	defer viper.Set("openai.reasoning-search-max-iterations", oldMaxIterations)

	viper.Set("llm.provider", "openai")
	viper.Set("openai.reasoning-search-model", "test-model")
	viper.Set("openai.reasoning-search-effort", "high")
	viper.Set("openai.reasoning-search-max-output-tokens", 1234)
	viper.Set("openai.reasoning-search-max-iterations", 6)

	cfg, err := llm.ReasoningSearchConfigFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Model != "test-model" {
		t.Fatalf("unexpected model: %s", cfg.Model)
	}
	if cfg.Effort != "high" {
		t.Fatalf("unexpected effort: %s", cfg.Effort)
	}
	if cfg.MaxTokens != 1234 {
		t.Fatalf("unexpected max tokens: %d", cfg.MaxTokens)
	}
	if cfg.MaxIterations != 6 {
		t.Fatalf("unexpected max iterations: %d", cfg.MaxIterations)
	}
}
