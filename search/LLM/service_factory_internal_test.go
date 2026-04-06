package llm

import (
	"testing"

	"github.com/spf13/viper"
)

func TestOpenRouterProviderPreferencesDefaultRequireParametersFalse(t *testing.T) {
	oldSort := viper.GetString("openrouter.provider-sort")
	oldRequire := viper.Get("openrouter.provider-require-parameters")
	defer viper.Set("openrouter.provider-sort", oldSort)
	if oldRequire == nil {
		defer viper.Set("openrouter.provider-require-parameters", nil)
	} else {
		defer viper.Set("openrouter.provider-require-parameters", oldRequire)
	}

	viper.Set("openrouter.provider-sort", "")
	viper.Set("openrouter.provider-require-parameters", nil)

	provider, err := openRouterProviderPreferencesFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider == nil || provider.RequireParameters == nil {
		t.Fatalf("expected provider preferences with require_parameters")
	}
	if *provider.RequireParameters {
		t.Fatalf("expected require_parameters to default to false")
	}
	if provider.Sort != "latency" {
		t.Fatalf("unexpected default sort: %q", provider.Sort)
	}
}

func TestOpenRouterRequiredToolIterationsDefaultsToOne(t *testing.T) {
	oldValue := viper.Get("openrouter.enforced-tool-use-iterations")
	if oldValue == nil {
		defer viper.Set("openrouter.enforced-tool-use-iterations", nil)
	} else {
		defer viper.Set("openrouter.enforced-tool-use-iterations", oldValue)
	}

	viper.Set("openrouter.enforced-tool-use-iterations", nil)

	if iterations := openRouterRequiredToolIterationsFromConfig(); iterations != 1 {
		t.Fatalf("expected default enforced-tool-use-iterations to be 1, got %d", iterations)
	}
}

func TestOpenRouterRequiredToolIterationsUsesConfiguredValue(t *testing.T) {
	oldValue := viper.Get("openrouter.enforced-tool-use-iterations")
	if oldValue == nil {
		defer viper.Set("openrouter.enforced-tool-use-iterations", nil)
	} else {
		defer viper.Set("openrouter.enforced-tool-use-iterations", oldValue)
	}

	viper.Set("openrouter.enforced-tool-use-iterations", 4)

	if iterations := openRouterRequiredToolIterationsFromConfig(); iterations != 4 {
		t.Fatalf("expected enforced-tool-use-iterations to be 4, got %d", iterations)
	}
}
