package common

import (
	"database/sql"
	"testing"

	"github.com/Bnei-Baruch/archive-backend/integration"
	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestInitLLMRuntimeIsOptIn(t *testing.T) {
	oldRuntime, oldDB, oldAssets := LLM_RUNTIME, DB, ASSETS
	t.Cleanup(func() {
		LLM_RUNTIME, DB, ASSETS = oldRuntime, oldDB, oldAssets
		viper.Reset()
	})
	for _, tc := range []struct {
		name    string
		config  map[string]interface{}
		enabled bool
		invalid bool
	}{
		{name: "no-llm-config"},
		{name: "disabled-with-invalid-provider", config: map[string]interface{}{"enabled": false, "provider": "invalid"}},
		{name: "enabled-with-stub", config: map[string]interface{}{"enabled": true, "provider": "stub"}, enabled: true},
		{name: "invalid-mode", config: map[string]interface{}{"enabled": true, "reasoning-search-mode": "unknown"}, invalid: true},
		{name: "rapid-only-without-default-provider", enabled: true, config: map[string]interface{}{
			"enabled": true, "reasoning-search-mode": "rapid", "ai-tools-provider": "stub",
			"reasoning-search-rapid-gather-provider": "stub", "reasoning-search-rapid-classifier-provider": "stub",
			"reasoning-search-planning-enabled": true, "reasoning-search-planning-provider": "unused",
			"reasoning-search-verification-enabled": true, "reasoning-search-verification-provider": "unused",
			"reasoning-search-draft-enabled": true, "reasoning-search-draft-provider": "unused",
		}},
		{name: "non-rapid-only-without-rapid-providers", enabled: true, config: map[string]interface{}{
			"enabled": true, "provider": "stub", "reasoning-search-mode": "non-rapid",
			"reasoning-search-rapid-gather-provider": "unused", "reasoning-search-rapid-classifier-provider": "unused",
			"reasoning-search-rapid-finalizer-enabled": true, "reasoning-search-rapid-finalizer-provider": "unused",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			viper.Reset()
			LLM_RUNTIME, DB, ASSETS = nil, nil, nil
			if tc.config != nil {
				viper.Set("llm", tc.config)
			}
			if tc.enabled {
				// Tool registration needs dependencies, but must not query them.
				DB = &sql.DB{}
				ASSETS = integration.NewAssetsService("http://assets.test")
				viper.Set("stub.model", "test-model")
			}
			if tc.invalid {
				require.Panics(t, initLLMRuntime)
				require.Nil(t, LLM_RUNTIME)
				return
			}
			initLLMRuntime()
			if !tc.enabled {
				require.Nil(t, LLM_RUNTIME)
				return
			}
			require.NotNil(t, LLM_RUNTIME)
			t.Cleanup(func() {
				require.NoError(t, LLM_RUNTIME.Progress.Close())
				require.NoError(t, LLM_RUNTIME.Workflow.Close())
				if LLM_RUNTIME.ReasoningCache != nil {
					require.NoError(t, LLM_RUNTIME.ReasoningCache.Close())
				}
			})
			require.IsType(t, &llm.StubLLMService{}, LLM_RUNTIME.Services["stub"])
			require.NotNil(t, LLM_RUNTIME.Tools)
			require.NotNil(t, LLM_RUNTIME.RunGuard)
			if tc.config["reasoning-search-mode"] == "non-rapid" {
				require.Nil(t, LLM_RUNTIME.RapidConfig)
			} else {
				require.NotNil(t, LLM_RUNTIME.RapidConfig)
			}
			if tc.config["reasoning-search-mode"] == "rapid" {
				require.Nil(t, LLM_RUNTIME.DraftConfig)
				require.Len(t, LLM_RUNTIME.Services, 1, "unused default and non-rapid stage providers must not be initialized")
			}
		})
	}
}
