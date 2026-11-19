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
	}{
		{name: "no-llm-config"},
		{name: "disabled-with-invalid-provider", config: map[string]interface{}{"enabled": false, "provider": "invalid"}},
		{name: "enabled-with-stub", config: map[string]interface{}{"enabled": true, "provider": "stub"}, enabled: true},
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
		})
	}
}
