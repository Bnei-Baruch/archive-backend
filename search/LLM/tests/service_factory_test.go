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
	oldAPIEndpoint := viper.GetString("openai.api-endpoint")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("openai.token", oldToken)
	defer viper.Set("openai.api-endpoint", oldAPIEndpoint)

	viper.Set("llm.provider", "")
	viper.Set("openai.token", "test-token")
	viper.Set("openai.api-endpoint", "")

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

func TestNewServiceFromConfigAllowsCustomAPIEndpointWithoutToken(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldToken := viper.GetString("openai.token")
	oldAPIEndpoint := viper.GetString("openai.api-endpoint")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("openai.token", oldToken)
	defer viper.Set("openai.api-endpoint", oldAPIEndpoint)

	viper.Set("llm.provider", "openai")
	viper.Set("openai.token", "")
	viper.Set("openai.api-endpoint", "http://localhost:8000")

	service, err := llm.NewServiceFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := service.(*llm.OpenAIService); !ok {
		t.Fatalf("unexpected service type: %T", service)
	}
}

func TestNewServiceFromConfigSupportsOpenRouter(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldToken := viper.GetString("openrouter.token")
	oldAPIEndpoint := viper.GetString("openrouter.api-endpoint")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("openrouter.token", oldToken)
	defer viper.Set("openrouter.api-endpoint", oldAPIEndpoint)

	viper.Set("llm.provider", "openrouter")
	viper.Set("openrouter.token", "test-token")
	viper.Set("openrouter.api-endpoint", "")

	service, err := llm.NewServiceFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := service.(*llm.OpenRouterService); !ok {
		t.Fatalf("unexpected service type: %T", service)
	}
}

func TestNewServiceFromConfigSupportsOllama(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldEndpoint := viper.GetString("ollama.api-endpoint")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("ollama.api-endpoint", oldEndpoint)

	viper.Set("llm.provider", "ollama")
	viper.Set("ollama.api-endpoint", "https://ollama.kab.sh")

	service, err := llm.NewServiceFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := service.(*llm.OllamaService); !ok {
		t.Fatalf("unexpected service type: %T", service)
	}
}

func TestNewServiceFromConfigSupportsZAI(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldToken := viper.GetString("zai.token")
	oldEndpoint := viper.GetString("zai.api-endpoint")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("zai.token", oldToken)
	defer viper.Set("zai.api-endpoint", oldEndpoint)

	viper.Set("llm.provider", "zai")
	viper.Set("zai.token", "test-token")
	viper.Set("zai.api-endpoint", "")

	service, err := llm.NewServiceFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := service.(*llm.ZAIService); !ok {
		t.Fatalf("unexpected service type: %T", service)
	}
}

func TestNewServiceFromConfigSupportsXAI(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldToken := viper.GetString("xai.token")
	oldEndpoint := viper.GetString("xai.api-endpoint")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("xai.token", oldToken)
	defer viper.Set("xai.api-endpoint", oldEndpoint)

	viper.Set("llm.provider", "xai")
	viper.Set("xai.token", "test-token")
	viper.Set("xai.api-endpoint", "")

	service, err := llm.NewServiceFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := service.(*llm.XAIService); !ok {
		t.Fatalf("unexpected service type: %T", service)
	}
}

func TestNewServiceFromConfigSupportsArcee(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldToken := viper.GetString("arcee.token")
	oldEndpoint := viper.GetString("arcee.api-endpoint")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("arcee.token", oldToken)
	defer viper.Set("arcee.api-endpoint", oldEndpoint)

	viper.Set("llm.provider", "arcee")
	viper.Set("arcee.token", "test-token")
	viper.Set("arcee.api-endpoint", "")

	service, err := llm.NewServiceFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := service.(*llm.ArceeService); !ok {
		t.Fatalf("unexpected service type: %T", service)
	}
}

func TestNewServiceFromConfigSupportsDeepSeek(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldToken := viper.GetString("deepseek.token")
	oldEndpoint := viper.GetString("deepseek.api-endpoint")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("deepseek.token", oldToken)
	defer viper.Set("deepseek.api-endpoint", oldEndpoint)

	viper.Set("llm.provider", "deepseek")
	viper.Set("deepseek.token", "test-token")
	viper.Set("deepseek.api-endpoint", "")

	service, err := llm.NewServiceFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := service.(*llm.DeepSeekService); !ok {
		t.Fatalf("unexpected service type: %T", service)
	}
}

func TestNewServiceFromConfigSupportsStub(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	defer viper.Set("llm.provider", oldProvider)

	viper.Set("llm.provider", "stub")

	service, err := llm.NewServiceFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := service.(*llm.StubLLMService); !ok {
		t.Fatalf("unexpected service type: %T", service)
	}
}

func TestReasoningSearchConfigFromConfigUsesOpenAISection(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldVerificationEnabled := viper.GetBool("llm.reasoning-search-verification-enabled")
	oldVerificationProvider := viper.GetString("llm.reasoning-search-verification-provider")
	oldModel := viper.GetString("openai.reasoning-search-model")
	oldEffort := viper.GetString("openai.reasoning-search-effort")
	oldMaxTokens := viper.GetInt("openai.reasoning-search-max-output-tokens")
	oldMaxIterations := viper.GetInt("openai.reasoning-search-max-iterations")
	oldRerunMaxIterations := viper.GetInt("openai.reasoning-search-rerun-max-iterations")
	oldMaxFollowups := viper.Get("llm.reasoning-search-max-followups")
	oldVerificationModel := viper.GetString("openai.reasoning-search-verification-model")
	oldVerificationEffort := viper.GetString("openai.reasoning-search-verification-effort")
	oldVerificationMaxTokens := viper.GetInt("openai.reasoning-search-verification-max-output-tokens")
	oldVerificationMaxInputTokens := viper.Get("llm.reasoning-search-verification-max-input-tokens")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("llm.reasoning-search-verification-enabled", oldVerificationEnabled)
	defer viper.Set("llm.reasoning-search-verification-provider", oldVerificationProvider)
	defer viper.Set("openai.reasoning-search-model", oldModel)
	defer viper.Set("openai.reasoning-search-effort", oldEffort)
	defer viper.Set("openai.reasoning-search-max-output-tokens", oldMaxTokens)
	defer viper.Set("openai.reasoning-search-max-iterations", oldMaxIterations)
	defer viper.Set("openai.reasoning-search-rerun-max-iterations", oldRerunMaxIterations)
	defer viper.Set("llm.reasoning-search-max-followups", oldMaxFollowups)
	defer viper.Set("openai.reasoning-search-verification-model", oldVerificationModel)
	defer viper.Set("openai.reasoning-search-verification-effort", oldVerificationEffort)
	defer viper.Set("openai.reasoning-search-verification-max-output-tokens", oldVerificationMaxTokens)
	defer viper.Set("llm.reasoning-search-verification-max-input-tokens", oldVerificationMaxInputTokens)

	viper.Set("llm.provider", "openai")
	viper.Set("openai.reasoning-search-model", "test-model")
	viper.Set("openai.reasoning-search-effort", "high")
	viper.Set("openai.reasoning-search-max-output-tokens", 1234)
	viper.Set("openai.reasoning-search-max-iterations", 6)
	viper.Set("openai.reasoning-search-rerun-max-iterations", 2)
	viper.Set("llm.reasoning-search-max-followups", 3)
	viper.Set("llm.reasoning-search-verification-enabled", true)
	viper.Set("llm.reasoning-search-verification-provider", "openai")
	viper.Set("openai.reasoning-search-verification-model", "verifier-model")
	viper.Set("openai.reasoning-search-verification-effort", "medium")
	viper.Set("openai.reasoning-search-verification-max-output-tokens", 4321)
	viper.Set("llm.reasoning-search-verification-max-input-tokens", 5555)

	cfg, err := llm.ReasoningSearchConfigFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Model != "test-model" {
		t.Fatalf("unexpected model: %s", cfg.Model)
	}
	if cfg.Provider != "openai" {
		t.Fatalf("unexpected provider: %s", cfg.Provider)
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
	if cfg.RerunMaxIterations != 2 {
		t.Fatalf("unexpected rerun max iterations: %d", cfg.RerunMaxIterations)
	}
	if cfg.MaxFollowups != 3 {
		t.Fatalf("unexpected max followups: %d", cfg.MaxFollowups)
	}
	if cfg.Verification == nil {
		t.Fatalf("expected verification config")
	}
	if cfg.Verification.Provider != "openai" {
		t.Fatalf("unexpected verification provider: %s", cfg.Verification.Provider)
	}
	if cfg.Verification.Model != "verifier-model" {
		t.Fatalf("unexpected verification model: %s", cfg.Verification.Model)
	}
	if cfg.Verification.Effort != "medium" {
		t.Fatalf("unexpected verification effort: %s", cfg.Verification.Effort)
	}
	if cfg.Verification.MaxTokens != 4321 {
		t.Fatalf("unexpected verification max tokens: %d", cfg.Verification.MaxTokens)
	}
	if cfg.Verification.MaxInputTokens != 5555 {
		t.Fatalf("unexpected verification max input tokens: %d", cfg.Verification.MaxInputTokens)
	}
}

func TestReasoningSearchConfigFromConfigSupportsCrossProviderVerification(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldVerificationEnabled := viper.GetBool("llm.reasoning-search-verification-enabled")
	oldVerificationProvider := viper.GetString("llm.reasoning-search-verification-provider")
	oldModel := viper.GetString("openai.reasoning-search-model")
	oldEffort := viper.GetString("openai.reasoning-search-effort")
	oldVerificationModel := viper.GetString("openrouter.reasoning-search-verification-model")
	oldVerificationEffort := viper.GetString("openrouter.reasoning-search-verification-effort")
	oldVerificationMaxTokens := viper.GetInt("openrouter.reasoning-search-verification-max-output-tokens")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("llm.reasoning-search-verification-enabled", oldVerificationEnabled)
	defer viper.Set("llm.reasoning-search-verification-provider", oldVerificationProvider)
	defer viper.Set("openai.reasoning-search-model", oldModel)
	defer viper.Set("openai.reasoning-search-effort", oldEffort)
	defer viper.Set("openrouter.reasoning-search-verification-model", oldVerificationModel)
	defer viper.Set("openrouter.reasoning-search-verification-effort", oldVerificationEffort)
	defer viper.Set("openrouter.reasoning-search-verification-max-output-tokens", oldVerificationMaxTokens)

	viper.Set("llm.provider", "openai")
	viper.Set("llm.reasoning-search-verification-enabled", true)
	viper.Set("llm.reasoning-search-verification-provider", "openrouter")
	viper.Set("openai.reasoning-search-model", "test-model")
	viper.Set("openai.reasoning-search-effort", "high")
	viper.Set("openrouter.reasoning-search-verification-model", "openai/gpt-oss-20b")
	viper.Set("openrouter.reasoning-search-verification-effort", "minimal")
	viper.Set("openrouter.reasoning-search-verification-max-output-tokens", 2222)

	cfg, err := llm.ReasoningSearchConfigFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Verification == nil {
		t.Fatalf("expected verification config")
	}
	if cfg.Verification.Provider != "openrouter" {
		t.Fatalf("unexpected verification provider: %s", cfg.Verification.Provider)
	}
	if cfg.Verification.Model != "openai/gpt-oss-20b" {
		t.Fatalf("unexpected verification model: %s", cfg.Verification.Model)
	}
	if cfg.Verification.Effort != "minimal" {
		t.Fatalf("unexpected verification effort: %s", cfg.Verification.Effort)
	}
	if cfg.Verification.MaxTokens != 2222 {
		t.Fatalf("unexpected verification max tokens: %d", cfg.Verification.MaxTokens)
	}
}

func TestReasoningSearchConfigFromConfigUsesOpenRouterSection(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldModel := viper.GetString("openrouter.reasoning-search-model")
	oldEffort := viper.GetString("openrouter.reasoning-search-effort")
	oldMaxTokens := viper.GetInt("openrouter.reasoning-search-max-output-tokens")
	oldMaxIterations := viper.GetInt("openrouter.reasoning-search-max-iterations")
	oldRerunMaxIterations := viper.GetInt("openrouter.reasoning-search-rerun-max-iterations")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("openrouter.reasoning-search-model", oldModel)
	defer viper.Set("openrouter.reasoning-search-effort", oldEffort)
	defer viper.Set("openrouter.reasoning-search-max-output-tokens", oldMaxTokens)
	defer viper.Set("openrouter.reasoning-search-max-iterations", oldMaxIterations)
	defer viper.Set("openrouter.reasoning-search-rerun-max-iterations", oldRerunMaxIterations)

	viper.Set("llm.provider", "openrouter")
	viper.Set("openrouter.reasoning-search-model", "openai/gpt-oss-120b")
	viper.Set("openrouter.reasoning-search-effort", "minimal")
	viper.Set("openrouter.reasoning-search-max-output-tokens", 2345)
	viper.Set("openrouter.reasoning-search-max-iterations", 7)
	viper.Set("openrouter.reasoning-search-rerun-max-iterations", 3)

	cfg, err := llm.ReasoningSearchConfigFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Model != "openai/gpt-oss-120b" {
		t.Fatalf("unexpected model: %s", cfg.Model)
	}
	if cfg.Provider != "openrouter" {
		t.Fatalf("unexpected provider: %s", cfg.Provider)
	}
	if cfg.Effort != "minimal" {
		t.Fatalf("unexpected effort: %s", cfg.Effort)
	}
	if cfg.MaxTokens != 2345 {
		t.Fatalf("unexpected max tokens: %d", cfg.MaxTokens)
	}
	if cfg.MaxIterations != 7 {
		t.Fatalf("unexpected max iterations: %d", cfg.MaxIterations)
	}
	if cfg.RerunMaxIterations != 3 {
		t.Fatalf("unexpected rerun max iterations: %d", cfg.RerunMaxIterations)
	}
}

func TestReasoningSearchConfigFromConfigUsesOllamaSection(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldModel := viper.GetString("ollama.reasoning-search-model")
	oldEffort := viper.GetString("ollama.reasoning-search-effort")
	oldMaxTokens := viper.GetInt("ollama.reasoning-search-max-output-tokens")
	oldMaxIterations := viper.GetInt("ollama.reasoning-search-max-iterations")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("ollama.reasoning-search-model", oldModel)
	defer viper.Set("ollama.reasoning-search-effort", oldEffort)
	defer viper.Set("ollama.reasoning-search-max-output-tokens", oldMaxTokens)
	defer viper.Set("ollama.reasoning-search-max-iterations", oldMaxIterations)

	viper.Set("llm.provider", "ollama")
	viper.Set("ollama.reasoning-search-model", "gemma4:31b")
	viper.Set("ollama.reasoning-search-effort", "minimal")
	viper.Set("ollama.reasoning-search-max-output-tokens", 3456)
	viper.Set("ollama.reasoning-search-max-iterations", 9)

	cfg, err := llm.ReasoningSearchConfigFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Model != "gemma4:31b" {
		t.Fatalf("unexpected model: %s", cfg.Model)
	}
	if cfg.Provider != "ollama" {
		t.Fatalf("unexpected provider: %s", cfg.Provider)
	}
	if cfg.Effort != "minimal" {
		t.Fatalf("unexpected effort: %s", cfg.Effort)
	}
	if cfg.MaxTokens != 3456 {
		t.Fatalf("unexpected max tokens: %d", cfg.MaxTokens)
	}
	if cfg.MaxIterations != 9 {
		t.Fatalf("unexpected max iterations: %d", cfg.MaxIterations)
	}
}

func TestReasoningSearchConfigFromConfigUsesZAISection(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldModel := viper.GetString("zai.reasoning-search-model")
	oldEffort := viper.GetString("zai.reasoning-search-effort")
	oldMaxTokens := viper.GetInt("zai.reasoning-search-max-output-tokens")
	oldMaxIterations := viper.GetInt("zai.reasoning-search-max-iterations")
	oldRerunMaxIterations := viper.GetInt("zai.reasoning-search-rerun-max-iterations")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("zai.reasoning-search-model", oldModel)
	defer viper.Set("zai.reasoning-search-effort", oldEffort)
	defer viper.Set("zai.reasoning-search-max-output-tokens", oldMaxTokens)
	defer viper.Set("zai.reasoning-search-max-iterations", oldMaxIterations)
	defer viper.Set("zai.reasoning-search-rerun-max-iterations", oldRerunMaxIterations)

	viper.Set("llm.provider", "zai")
	viper.Set("zai.reasoning-search-model", "glm-5.1")
	viper.Set("zai.reasoning-search-effort", "xhigh")
	viper.Set("zai.reasoning-search-max-output-tokens", 4567)
	viper.Set("zai.reasoning-search-max-iterations", 11)
	viper.Set("zai.reasoning-search-rerun-max-iterations", 4)

	cfg, err := llm.ReasoningSearchConfigFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Model != "glm-5.1" {
		t.Fatalf("unexpected model: %s", cfg.Model)
	}
	if cfg.Provider != "zai" {
		t.Fatalf("unexpected provider: %s", cfg.Provider)
	}
	if cfg.Effort != "xhigh" {
		t.Fatalf("unexpected effort: %s", cfg.Effort)
	}
	if cfg.MaxTokens != 4567 {
		t.Fatalf("unexpected max tokens: %d", cfg.MaxTokens)
	}
	if cfg.MaxIterations != 11 {
		t.Fatalf("unexpected max iterations: %d", cfg.MaxIterations)
	}
	if cfg.RerunMaxIterations != 4 {
		t.Fatalf("unexpected rerun max iterations: %d", cfg.RerunMaxIterations)
	}
}

func TestReasoningSearchConfigFromConfigUsesXAISection(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldVerificationEnabled := viper.GetBool("llm.reasoning-search-verification-enabled")
	oldModel := viper.GetString("xai.reasoning-search-model")
	oldEffort := viper.GetString("xai.reasoning-search-effort")
	oldMaxTokens := viper.GetInt("xai.reasoning-search-max-output-tokens")
	oldMaxIterations := viper.GetInt("xai.reasoning-search-max-iterations")
	oldRerunMaxIterations := viper.GetInt("xai.reasoning-search-rerun-max-iterations")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("llm.reasoning-search-verification-enabled", oldVerificationEnabled)
	defer viper.Set("xai.reasoning-search-model", oldModel)
	defer viper.Set("xai.reasoning-search-effort", oldEffort)
	defer viper.Set("xai.reasoning-search-max-output-tokens", oldMaxTokens)
	defer viper.Set("xai.reasoning-search-max-iterations", oldMaxIterations)
	defer viper.Set("xai.reasoning-search-rerun-max-iterations", oldRerunMaxIterations)

	viper.Set("llm.provider", "xai")
	viper.Set("llm.reasoning-search-verification-enabled", false)
	viper.Set("xai.reasoning-search-model", "grok-4-1-fast-reasoning")
	viper.Set("xai.reasoning-search-effort", "")
	viper.Set("xai.reasoning-search-max-output-tokens", 3456)
	viper.Set("xai.reasoning-search-max-iterations", 9)
	viper.Set("xai.reasoning-search-rerun-max-iterations", 3)

	cfg, err := llm.ReasoningSearchConfigFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider != "xai" {
		t.Fatalf("unexpected provider: %s", cfg.Provider)
	}
	if cfg.Model != "grok-4-1-fast-reasoning" {
		t.Fatalf("unexpected model: %s", cfg.Model)
	}
	if cfg.Effort != "" {
		t.Fatalf("unexpected effort: %q", cfg.Effort)
	}
	if cfg.MaxTokens != 3456 {
		t.Fatalf("unexpected max tokens: %d", cfg.MaxTokens)
	}
	if cfg.MaxIterations != 9 {
		t.Fatalf("unexpected max iterations: %d", cfg.MaxIterations)
	}
	if cfg.RerunMaxIterations != 3 {
		t.Fatalf("unexpected rerun max iterations: %d", cfg.RerunMaxIterations)
	}
}

func TestReasoningSearchConfigFromConfigUsesArceeSection(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldModel := viper.GetString("arcee.reasoning-search-model")
	oldEffort := viper.GetString("arcee.reasoning-search-effort")
	oldMaxTokens := viper.GetInt("arcee.reasoning-search-max-output-tokens")
	oldMaxIterations := viper.GetInt("arcee.reasoning-search-max-iterations")
	oldRerunMaxIterations := viper.GetInt("arcee.reasoning-search-rerun-max-iterations")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("arcee.reasoning-search-model", oldModel)
	defer viper.Set("arcee.reasoning-search-effort", oldEffort)
	defer viper.Set("arcee.reasoning-search-max-output-tokens", oldMaxTokens)
	defer viper.Set("arcee.reasoning-search-max-iterations", oldMaxIterations)
	defer viper.Set("arcee.reasoning-search-rerun-max-iterations", oldRerunMaxIterations)

	viper.Set("llm.provider", "arcee")
	viper.Set("arcee.reasoning-search-model", "trinity-mini")
	viper.Set("arcee.reasoning-search-effort", "medium")
	viper.Set("arcee.reasoning-search-max-output-tokens", 2345)
	viper.Set("arcee.reasoning-search-max-iterations", 7)
	viper.Set("arcee.reasoning-search-rerun-max-iterations", 3)

	cfg, err := llm.ReasoningSearchConfigFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider != "arcee" {
		t.Fatalf("unexpected provider: %s", cfg.Provider)
	}
	if cfg.Model != "trinity-mini" {
		t.Fatalf("unexpected model: %s", cfg.Model)
	}
	if cfg.Effort != "medium" {
		t.Fatalf("unexpected effort: %s", cfg.Effort)
	}
	if cfg.MaxTokens != 2345 {
		t.Fatalf("unexpected max tokens: %d", cfg.MaxTokens)
	}
	if cfg.MaxIterations != 7 {
		t.Fatalf("unexpected max iterations: %d", cfg.MaxIterations)
	}
	if cfg.RerunMaxIterations != 3 {
		t.Fatalf("unexpected rerun max iterations: %d", cfg.RerunMaxIterations)
	}
}

func TestReasoningSearchConfigFromConfigUsesDeepSeekSection(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldPlanningEnabled := viper.GetBool("llm.reasoning-search-planning-enabled")
	oldVerificationEnabled := viper.GetBool("llm.reasoning-search-verification-enabled")
	oldModel := viper.GetString("deepseek.reasoning-search-model")
	oldEffort := viper.GetString("deepseek.reasoning-search-effort")
	oldMaxTokens := viper.GetInt("deepseek.reasoning-search-max-output-tokens")
	oldMaxIterations := viper.GetInt("deepseek.reasoning-search-max-iterations")
	oldRerunMaxIterations := viper.GetInt("deepseek.reasoning-search-rerun-max-iterations")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("llm.reasoning-search-planning-enabled", oldPlanningEnabled)
	defer viper.Set("llm.reasoning-search-verification-enabled", oldVerificationEnabled)
	defer viper.Set("deepseek.reasoning-search-model", oldModel)
	defer viper.Set("deepseek.reasoning-search-effort", oldEffort)
	defer viper.Set("deepseek.reasoning-search-max-output-tokens", oldMaxTokens)
	defer viper.Set("deepseek.reasoning-search-max-iterations", oldMaxIterations)
	defer viper.Set("deepseek.reasoning-search-rerun-max-iterations", oldRerunMaxIterations)

	viper.Set("llm.provider", "deepseek")
	viper.Set("llm.reasoning-search-planning-enabled", false)
	viper.Set("llm.reasoning-search-verification-enabled", false)
	viper.Set("deepseek.reasoning-search-model", "deepseek-v4-pro")
	viper.Set("deepseek.reasoning-search-effort", "xhigh")
	viper.Set("deepseek.reasoning-search-max-output-tokens", 4567)
	viper.Set("deepseek.reasoning-search-max-iterations", 11)
	viper.Set("deepseek.reasoning-search-rerun-max-iterations", 4)

	cfg, err := llm.ReasoningSearchConfigFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider != "deepseek" {
		t.Fatalf("unexpected provider: %s", cfg.Provider)
	}
	if cfg.Model != "deepseek-v4-pro" {
		t.Fatalf("unexpected model: %s", cfg.Model)
	}
	if cfg.Effort != "xhigh" {
		t.Fatalf("unexpected effort: %s", cfg.Effort)
	}
	if cfg.MaxTokens != 4567 {
		t.Fatalf("unexpected max tokens: %d", cfg.MaxTokens)
	}
	if cfg.MaxIterations != 11 {
		t.Fatalf("unexpected max iterations: %d", cfg.MaxIterations)
	}
	if cfg.RerunMaxIterations != 4 {
		t.Fatalf("unexpected rerun max iterations: %d", cfg.RerunMaxIterations)
	}
}

func TestReasoningSearchConfigFromConfigUsesStubSection(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldPlanningEnabled := viper.GetBool("llm.reasoning-search-planning-enabled")
	oldPlanningProvider := viper.GetString("llm.reasoning-search-planning-provider")
	oldVerificationEnabled := viper.GetBool("llm.reasoning-search-verification-enabled")
	oldVerificationProvider := viper.GetString("llm.reasoning-search-verification-provider")
	oldModel := viper.GetString("stub.reasoning-search-model")
	oldEffort := viper.GetString("stub.reasoning-search-effort")
	oldMaxTokens := viper.GetInt("stub.reasoning-search-max-output-tokens")
	oldMaxIterations := viper.GetInt("stub.reasoning-search-max-iterations")
	oldRerunMaxIterations := viper.GetInt("stub.reasoning-search-rerun-max-iterations")
	oldPlanningModel := viper.GetString("stub.reasoning-search-planning-model")
	oldPlanningEffort := viper.GetString("stub.reasoning-search-planning-effort")
	oldPlanningMaxTokens := viper.GetInt("stub.reasoning-search-planning-max-output-tokens")
	oldVerificationModel := viper.GetString("stub.reasoning-search-verification-model")
	oldVerificationEffort := viper.GetString("stub.reasoning-search-verification-effort")
	oldVerificationMaxTokens := viper.GetInt("stub.reasoning-search-verification-max-output-tokens")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("llm.reasoning-search-planning-enabled", oldPlanningEnabled)
	defer viper.Set("llm.reasoning-search-planning-provider", oldPlanningProvider)
	defer viper.Set("llm.reasoning-search-verification-enabled", oldVerificationEnabled)
	defer viper.Set("llm.reasoning-search-verification-provider", oldVerificationProvider)
	defer viper.Set("stub.reasoning-search-model", oldModel)
	defer viper.Set("stub.reasoning-search-effort", oldEffort)
	defer viper.Set("stub.reasoning-search-max-output-tokens", oldMaxTokens)
	defer viper.Set("stub.reasoning-search-max-iterations", oldMaxIterations)
	defer viper.Set("stub.reasoning-search-rerun-max-iterations", oldRerunMaxIterations)
	defer viper.Set("stub.reasoning-search-planning-model", oldPlanningModel)
	defer viper.Set("stub.reasoning-search-planning-effort", oldPlanningEffort)
	defer viper.Set("stub.reasoning-search-planning-max-output-tokens", oldPlanningMaxTokens)
	defer viper.Set("stub.reasoning-search-verification-model", oldVerificationModel)
	defer viper.Set("stub.reasoning-search-verification-effort", oldVerificationEffort)
	defer viper.Set("stub.reasoning-search-verification-max-output-tokens", oldVerificationMaxTokens)

	viper.Set("llm.provider", "stub")
	viper.Set("llm.reasoning-search-planning-enabled", true)
	viper.Set("llm.reasoning-search-planning-provider", "stub")
	viper.Set("llm.reasoning-search-verification-enabled", true)
	viper.Set("llm.reasoning-search-verification-provider", "stub")
	viper.Set("stub.reasoning-search-model", "main-stub")
	viper.Set("stub.reasoning-search-effort", "none")
	viper.Set("stub.reasoning-search-max-output-tokens", 1000)
	viper.Set("stub.reasoning-search-max-iterations", 1)
	viper.Set("stub.reasoning-search-rerun-max-iterations", 1)
	viper.Set("stub.reasoning-search-planning-model", "planner-stub")
	viper.Set("stub.reasoning-search-planning-effort", "plan")
	viper.Set("stub.reasoning-search-planning-max-output-tokens", 2000)
	viper.Set("stub.reasoning-search-verification-model", "verify-stub")
	viper.Set("stub.reasoning-search-verification-effort", "verify")
	viper.Set("stub.reasoning-search-verification-max-output-tokens", 3000)

	cfg, err := llm.ReasoningSearchConfigFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider != "stub" {
		t.Fatalf("unexpected provider: %s", cfg.Provider)
	}
	if cfg.Model != "main-stub" {
		t.Fatalf("unexpected model: %s", cfg.Model)
	}
	if cfg.Effort != "none" {
		t.Fatalf("unexpected effort: %s", cfg.Effort)
	}
	if cfg.MaxTokens != 1000 {
		t.Fatalf("unexpected max tokens: %d", cfg.MaxTokens)
	}
	if cfg.MaxIterations != 1 {
		t.Fatalf("unexpected max iterations: %d", cfg.MaxIterations)
	}
	if cfg.RerunMaxIterations != 1 {
		t.Fatalf("unexpected rerun max iterations: %d", cfg.RerunMaxIterations)
	}
	if cfg.Planning == nil || cfg.Planning.Provider != "stub" || cfg.Planning.Model != "planner-stub" || cfg.Planning.Effort != "plan" || cfg.Planning.MaxTokens != 2000 {
		t.Fatalf("unexpected planning config: %#v", cfg.Planning)
	}
	if cfg.Verification == nil || cfg.Verification.Provider != "stub" || cfg.Verification.Model != "verify-stub" || cfg.Verification.Effort != "verify" || cfg.Verification.MaxTokens != 3000 {
		t.Fatalf("unexpected verification config: %#v", cfg.Verification)
	}
}

func TestReasoningSearchConfigFromConfigIncludesPlanningStage(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldPlanningEnabled := viper.GetBool("llm.reasoning-search-planning-enabled")
	oldPlanningProvider := viper.GetString("llm.reasoning-search-planning-provider")
	oldModel := viper.GetString("openai.reasoning-search-model")
	oldEffort := viper.GetString("openai.reasoning-search-effort")
	oldPlanningModel := viper.GetString("openai.reasoning-search-planning-model")
	oldPlanningEffort := viper.GetString("openai.reasoning-search-planning-effort")
	oldPlanningMaxTokens := viper.GetInt("openai.reasoning-search-planning-max-output-tokens")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("llm.reasoning-search-planning-enabled", oldPlanningEnabled)
	defer viper.Set("llm.reasoning-search-planning-provider", oldPlanningProvider)
	defer viper.Set("openai.reasoning-search-model", oldModel)
	defer viper.Set("openai.reasoning-search-effort", oldEffort)
	defer viper.Set("openai.reasoning-search-planning-model", oldPlanningModel)
	defer viper.Set("openai.reasoning-search-planning-effort", oldPlanningEffort)
	defer viper.Set("openai.reasoning-search-planning-max-output-tokens", oldPlanningMaxTokens)

	viper.Set("llm.provider", "openai")
	viper.Set("llm.reasoning-search-planning-enabled", true)
	viper.Set("llm.reasoning-search-planning-provider", "openai")
	viper.Set("openai.reasoning-search-model", "gpt-5.4")
	viper.Set("openai.reasoning-search-effort", "high")
	viper.Set("openai.reasoning-search-planning-model", "gpt-5.4-mini")
	viper.Set("openai.reasoning-search-planning-effort", "medium")
	viper.Set("openai.reasoning-search-planning-max-output-tokens", 1111)

	cfg, err := llm.ReasoningSearchConfigFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Planning == nil {
		t.Fatalf("expected planning config")
	}
	if cfg.Planning.Provider != "openai" {
		t.Fatalf("unexpected planning provider: %s", cfg.Planning.Provider)
	}
	if cfg.Planning.Model != "gpt-5.4-mini" {
		t.Fatalf("unexpected planning model: %s", cfg.Planning.Model)
	}
	if cfg.Planning.Effort != "medium" {
		t.Fatalf("unexpected planning effort: %s", cfg.Planning.Effort)
	}
	if cfg.Planning.MaxTokens != 1111 {
		t.Fatalf("unexpected planning max tokens: %d", cfg.Planning.MaxTokens)
	}
}

func TestReasoningSearchConfigFromConfigRejectsXAIPlanningEffort(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldPlanningEnabled := viper.GetBool("llm.reasoning-search-planning-enabled")
	oldPlanningProvider := viper.GetString("llm.reasoning-search-planning-provider")
	oldModel := viper.GetString("xai.reasoning-search-model")
	oldPlanningModel := viper.GetString("xai.reasoning-search-planning-model")
	oldPlanningEffort := viper.GetString("xai.reasoning-search-planning-effort")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("llm.reasoning-search-planning-enabled", oldPlanningEnabled)
	defer viper.Set("llm.reasoning-search-planning-provider", oldPlanningProvider)
	defer viper.Set("xai.reasoning-search-model", oldModel)
	defer viper.Set("xai.reasoning-search-planning-model", oldPlanningModel)
	defer viper.Set("xai.reasoning-search-planning-effort", oldPlanningEffort)

	viper.Set("llm.provider", "xai")
	viper.Set("llm.reasoning-search-planning-enabled", true)
	viper.Set("llm.reasoning-search-planning-provider", "xai")
	viper.Set("xai.reasoning-search-model", "grok-4-1-fast-reasoning")
	viper.Set("xai.reasoning-search-planning-model", "grok-4-1-fast-reasoning")
	viper.Set("xai.reasoning-search-planning-effort", "high")

	_, err := llm.ReasoningSearchConfigFromConfig()
	if err == nil {
		t.Fatalf("expected error for xai planning effort")
	}
	if !strings.Contains(err.Error(), "xai.reasoning-search-planning-effort is not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReasoningSearchConfigFromConfigRejectsXAIEffort(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldModel := viper.GetString("xai.reasoning-search-model")
	oldEffort := viper.GetString("xai.reasoning-search-effort")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("xai.reasoning-search-model", oldModel)
	defer viper.Set("xai.reasoning-search-effort", oldEffort)

	viper.Set("llm.provider", "xai")
	viper.Set("xai.reasoning-search-model", "grok-4-1-fast-reasoning")
	viper.Set("xai.reasoning-search-effort", "high")

	_, err := llm.ReasoningSearchConfigFromConfig()
	if err == nil {
		t.Fatalf("expected error for xai reasoning effort")
	}
	if !strings.Contains(err.Error(), "xai.reasoning-search-effort is not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReasoningSearchConfigFromConfigRejectsInvalidGPTOssEffort(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldModel := viper.GetString("openai.reasoning-search-model")
	oldEffort := viper.GetString("openai.reasoning-search-effort")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("openai.reasoning-search-model", oldModel)
	defer viper.Set("openai.reasoning-search-effort", oldEffort)

	viper.Set("llm.provider", "openai")
	viper.Set("openai.reasoning-search-model", "gpt-oss-20b")
	viper.Set("openai.reasoning-search-effort", "xhigh")

	_, err := llm.ReasoningSearchConfigFromConfig()
	if err == nil {
		t.Fatalf("expected error for invalid gpt-oss effort")
	}
	if !strings.Contains(err.Error(), "supports only low, medium, high") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReasoningSearchConfigFromConfigRejectsInvalidOpenRouterEffort(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldModel := viper.GetString("openrouter.reasoning-search-model")
	oldEffort := viper.GetString("openrouter.reasoning-search-effort")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("openrouter.reasoning-search-model", oldModel)
	defer viper.Set("openrouter.reasoning-search-effort", oldEffort)

	viper.Set("llm.provider", "openrouter")
	viper.Set("openrouter.reasoning-search-model", "google/gemma-4-31b-it")
	viper.Set("openrouter.reasoning-search-effort", "xhigh")

	_, err := llm.ReasoningSearchConfigFromConfig()
	if err == nil {
		t.Fatalf("expected error for invalid openrouter effort")
	}
	if !strings.Contains(err.Error(), "supported values are minimal, low, medium, high") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReasoningSearchConfigFromConfigRejectsInvalidOllamaEffort(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldModel := viper.GetString("ollama.reasoning-search-model")
	oldEffort := viper.GetString("ollama.reasoning-search-effort")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("ollama.reasoning-search-model", oldModel)
	defer viper.Set("ollama.reasoning-search-effort", oldEffort)

	viper.Set("llm.provider", "ollama")
	viper.Set("ollama.reasoning-search-model", "gpt-oss:120b")
	viper.Set("ollama.reasoning-search-effort", "xhigh")

	_, err := llm.ReasoningSearchConfigFromConfig()
	if err == nil {
		t.Fatalf("expected error for invalid ollama effort")
	}
	if !strings.Contains(err.Error(), "supported values are minimal, low, medium, high") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReasoningSearchConfigFromConfigRejectsInvalidZAIEffort(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldModel := viper.GetString("zai.reasoning-search-model")
	oldEffort := viper.GetString("zai.reasoning-search-effort")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("zai.reasoning-search-model", oldModel)
	defer viper.Set("zai.reasoning-search-effort", oldEffort)

	viper.Set("llm.provider", "zai")
	viper.Set("zai.reasoning-search-model", "glm-5.1")
	viper.Set("zai.reasoning-search-effort", "ultra")

	_, err := llm.ReasoningSearchConfigFromConfig()
	if err == nil {
		t.Fatalf("expected error for invalid Z.AI effort")
	}
	if !strings.Contains(err.Error(), "supported values are minimal, low, medium, high, xhigh") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReasoningSearchConfigFromConfigRejectsInvalidArceeEffort(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldModel := viper.GetString("arcee.reasoning-search-model")
	oldEffort := viper.GetString("arcee.reasoning-search-effort")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("arcee.reasoning-search-model", oldModel)
	defer viper.Set("arcee.reasoning-search-effort", oldEffort)

	viper.Set("llm.provider", "arcee")
	viper.Set("arcee.reasoning-search-model", "trinity-mini")
	viper.Set("arcee.reasoning-search-effort", "xhigh")

	_, err := llm.ReasoningSearchConfigFromConfig()
	if err == nil {
		t.Fatalf("expected error for invalid Arcee effort")
	}
	if !strings.Contains(err.Error(), "supported values are minimal, low, medium, high") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReasoningSearchConfigFromConfigRejectsInvalidDeepSeekEffort(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldModel := viper.GetString("deepseek.reasoning-search-model")
	oldEffort := viper.GetString("deepseek.reasoning-search-effort")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("deepseek.reasoning-search-model", oldModel)
	defer viper.Set("deepseek.reasoning-search-effort", oldEffort)

	viper.Set("llm.provider", "deepseek")
	viper.Set("deepseek.reasoning-search-model", "deepseek-v4-flash")
	viper.Set("deepseek.reasoning-search-effort", "tiny")

	_, err := llm.ReasoningSearchConfigFromConfig()
	if err == nil {
		t.Fatalf("expected error for invalid DeepSeek effort")
	}
	if !strings.Contains(err.Error(), "supported values are minimal, low, medium, high, xhigh, max") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAIToolsConfigFromConfigUsesExplicitProvider(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldAIToolsProvider := viper.GetString("llm.ai-tools-provider")
	oldModel := viper.GetString("openrouter.ai-tools-model")
	oldEffort := viper.GetString("openrouter.ai-tools-effort")
	oldMaxTokens := viper.GetInt("openrouter.ai-tools-max-output-tokens")
	oldBatchConcurrency := viper.Get("openrouter.ai-tools-batch-concurrency")
	oldMaxBatches := viper.Get("llm.ai-query-tool-max-batches")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("llm.ai-tools-provider", oldAIToolsProvider)
	defer viper.Set("openrouter.ai-tools-model", oldModel)
	defer viper.Set("openrouter.ai-tools-effort", oldEffort)
	defer viper.Set("openrouter.ai-tools-max-output-tokens", oldMaxTokens)
	defer viper.Set("openrouter.ai-tools-batch-concurrency", oldBatchConcurrency)
	defer viper.Set("llm.ai-query-tool-max-batches", oldMaxBatches)

	viper.Set("llm.provider", "openai")
	viper.Set("llm.ai-tools-provider", "openrouter")
	viper.Set("llm.ai-query-tool-max-batches", 7)
	viper.Set("openrouter.ai-tools-model", "openai/gpt-oss-20b")
	viper.Set("openrouter.ai-tools-effort", "minimal")
	viper.Set("openrouter.ai-tools-max-output-tokens", 1234)
	viper.Set("openrouter.ai-tools-batch-concurrency", 5)

	cfg, err := llm.AIToolsConfigFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider != "openrouter" {
		t.Fatalf("unexpected provider: %s", cfg.Provider)
	}
	if cfg.Model != "openai/gpt-oss-20b" {
		t.Fatalf("unexpected model: %s", cfg.Model)
	}
	if cfg.Effort != "minimal" {
		t.Fatalf("unexpected effort: %s", cfg.Effort)
	}
	if cfg.MaxTokens != 1234 {
		t.Fatalf("unexpected max tokens: %d", cfg.MaxTokens)
	}
	if cfg.MaxBatches != 7 {
		t.Fatalf("unexpected max batches: %d", cfg.MaxBatches)
	}
	if cfg.BatchConcurrency != 5 {
		t.Fatalf("unexpected batch concurrency: %d", cfg.BatchConcurrency)
	}
}

func TestAIToolsConfigFromConfigSupportsStub(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldAIToolsProvider := viper.GetString("llm.ai-tools-provider")
	oldModel := viper.GetString("stub.ai-tools-model")
	oldEffort := viper.GetString("stub.ai-tools-effort")
	oldMaxTokens := viper.GetInt("stub.ai-tools-max-output-tokens")
	oldBatchConcurrency := viper.Get("stub.ai-tools-batch-concurrency")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("llm.ai-tools-provider", oldAIToolsProvider)
	defer viper.Set("stub.ai-tools-model", oldModel)
	defer viper.Set("stub.ai-tools-effort", oldEffort)
	defer viper.Set("stub.ai-tools-max-output-tokens", oldMaxTokens)
	defer viper.Set("stub.ai-tools-batch-concurrency", oldBatchConcurrency)

	viper.Set("llm.provider", "openai")
	viper.Set("llm.ai-tools-provider", "stub")
	viper.Set("stub.ai-tools-model", "reader-stub")
	viper.Set("stub.ai-tools-effort", "reader")
	viper.Set("stub.ai-tools-max-output-tokens", 3333)
	viper.Set("stub.ai-tools-batch-concurrency", 4)

	cfg, err := llm.AIToolsConfigFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider != "stub" {
		t.Fatalf("unexpected provider: %s", cfg.Provider)
	}
	if cfg.Model != "reader-stub" {
		t.Fatalf("unexpected model: %s", cfg.Model)
	}
	if cfg.Effort != "reader" {
		t.Fatalf("unexpected effort: %s", cfg.Effort)
	}
	if cfg.MaxTokens != 3333 {
		t.Fatalf("unexpected max tokens: %d", cfg.MaxTokens)
	}
	if cfg.BatchConcurrency != 4 {
		t.Fatalf("unexpected batch concurrency: %d", cfg.BatchConcurrency)
	}
}

func TestNewServiceFromConfigRejectsInvalidZAITemperature(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldToken := viper.GetString("zai.token")
	oldTemperature := viper.Get("zai.temperature")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("zai.token", oldToken)
	defer viper.Set("zai.temperature", oldTemperature)

	viper.Set("llm.provider", "zai")
	viper.Set("zai.token", "test-token")
	viper.Set("zai.temperature", 1.5)

	_, err := llm.NewServiceFromConfig()
	if err == nil {
		t.Fatalf("expected error for invalid Z.AI temperature")
	}
	if !strings.Contains(err.Error(), "zai.temperature must be between 0 and 1") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewServiceFromConfigRejectsInvalidZAITopP(t *testing.T) {
	oldProvider := viper.GetString("llm.provider")
	oldToken := viper.GetString("zai.token")
	oldTopP := viper.Get("zai.top-p")
	defer viper.Set("llm.provider", oldProvider)
	defer viper.Set("zai.token", oldToken)
	defer viper.Set("zai.top-p", oldTopP)

	viper.Set("llm.provider", "zai")
	viper.Set("zai.token", "test-token")
	viper.Set("zai.top-p", 0.0)

	_, err := llm.NewServiceFromConfig()
	if err == nil {
		t.Fatalf("expected error for invalid Z.AI top-p")
	}
	if !strings.Contains(err.Error(), "zai.top-p must be > 0 and <= 1") {
		t.Fatalf("unexpected error: %v", err)
	}
}
