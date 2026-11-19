package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"gopkg.in/gin-gonic/gin.v1"
)

func TestReasoningRoutesRequireLLMEnabled(t *testing.T) {
	previous := viper.Get("llm")
	t.Cleanup(func() { viper.Set("llm", previous) })
	endpoints := []struct{ method, path string }{
		{http.MethodPost, "/search/reasoning"},
		{http.MethodPost, "/search/reasoning/start"},
		{http.MethodPost, "/search/reasoning/cancel"},
		{http.MethodPost, "/search/reasoning/finish-now"},
		{http.MethodPost, "/search/reasoning/cache"},
		{http.MethodGet, "/search/reasoning/status"},
		{http.MethodGet, "/search/reasoning/result"},
	}
	for _, tc := range []struct {
		name    string
		config  map[string]interface{}
		enabled bool
	}{
		{name: "default", config: map[string]interface{}{}},
		{name: "disabled", config: map[string]interface{}{"enabled": false}},
		{name: "enabled", config: map[string]interface{}{"enabled": true}, enabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			viper.Set("llm", tc.config)
			router := gin.New()
			SetupRoutes(router)
			routes := map[string]bool{}
			for _, route := range router.Routes() {
				routes[route.Method+" "+route.Path] = true
			}
			require.True(t, routes[http.MethodGet+" /search"], "regular search must remain registered")
			for _, endpoint := range endpoints {
				require.Equal(t, tc.enabled, routes[endpoint.method+" "+endpoint.path], endpoint.path)
				if !tc.enabled {
					response := httptest.NewRecorder()
					router.ServeHTTP(response, httptest.NewRequest(endpoint.method, endpoint.path, nil))
					require.Equal(t, http.StatusNotFound, response.Code, endpoint.path)
				}
			}
		})
	}
}
