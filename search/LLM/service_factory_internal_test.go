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

func TestOpenRouterProviderPreferencesStageOverridesInheritGlobal(t *testing.T) {
	restoreViperKeys(t,
		"openrouter.provider-sort",
		"openrouter.provider-require-parameters",
		"openrouter.provider-only",
		"openrouter.provider-ignore",
		"openrouter.reasoning-search-planning-provider-sort",
		"openrouter.reasoning-search-planning-provider-only",
		"openrouter.reasoning-search-planning-provider-ignore",
	)

	viper.Set("openrouter.provider-sort", "latency")
	viper.Set("openrouter.provider-require-parameters", true)
	viper.Set("openrouter.provider-only", []string{"openai"})
	viper.Set("openrouter.provider-ignore", []string{"novita"})
	viper.Set("openrouter.reasoning-search-planning-provider-sort", "price")
	viper.Set("openrouter.reasoning-search-planning-provider-only", []string{"deepinfra"})
	viper.Set("openrouter.reasoning-search-planning-provider-ignore", []string{})

	global, err := openRouterProviderPreferencesFromConfig()
	if err != nil {
		t.Fatalf("unexpected global error: %v", err)
	}
	planning, err := openRouterProviderPreferencesForScopeFromConfig("openrouter.reasoning-search-planning", global)
	if err != nil {
		t.Fatalf("unexpected planning error: %v", err)
	}
	if planning.Sort != "price" {
		t.Fatalf("unexpected planning sort: %q", planning.Sort)
	}
	if planning.RequireParameters == nil || !*planning.RequireParameters {
		t.Fatalf("expected planning require_parameters to inherit true")
	}
	if len(planning.Only) != 1 || planning.Only[0] != "deepinfra" {
		t.Fatalf("unexpected planning only: %#v", planning.Only)
	}
	if len(planning.Ignore) != 0 {
		t.Fatalf("expected planning ignore to be cleared, got %#v", planning.Ignore)
	}
}

func restoreViperKeys(t *testing.T, keys ...string) {
	t.Helper()
	values := map[string]interface{}{}
	for _, key := range keys {
		values[key] = viper.Get(key)
	}
	t.Cleanup(func() {
		for _, key := range keys {
			viper.Set(key, values[key])
		}
	})
}
