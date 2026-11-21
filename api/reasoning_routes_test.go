package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"gopkg.in/gin-gonic/gin.v1"
)

func TestReasoningRoutesRequireLLMEnabled(t *testing.T) {
	previous := viper.AllSettings()
	viper.Reset()
	t.Cleanup(func() {
		viper.Reset()
		for key, value := range previous {
			viper.Set(key, value)
		}
	})
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
		name           string
		config         map[string]interface{}
		mode           string
		enabled        bool
		syncRegistered bool
	}{
		{name: "default", config: map[string]interface{}{}},
		{name: "disabled", config: map[string]interface{}{"enabled": false}},
		{name: "enabled-debug-default-sync-off", config: map[string]interface{}{"enabled": true}, mode: gin.DebugMode, enabled: true},
		{name: "enabled-debug-explicit-sync-off", config: map[string]interface{}{"enabled": true, "sync-endpoint-enabled": false}, mode: gin.DebugMode, enabled: true},
		{name: "enabled-debug-sync-on", config: map[string]interface{}{"enabled": true, "sync-endpoint-enabled": true}, mode: gin.DebugMode, enabled: true, syncRegistered: true},
		{name: "enabled-release-sync-on", config: map[string]interface{}{"enabled": true, "sync-endpoint-enabled": true}, mode: gin.ReleaseMode, enabled: true, syncRegistered: true},
		{name: "enabled-test-sync-on", config: map[string]interface{}{"enabled": true, "sync-endpoint-enabled": true}, mode: gin.TestMode, enabled: true, syncRegistered: true},
		{name: "disabled-debug-sync-on", config: map[string]interface{}{"enabled": false, "sync-endpoint-enabled": true}, mode: gin.DebugMode},
	} {
		t.Run(tc.name, func(t *testing.T) {
			viper.Set("llm", tc.config)
			viper.Set("server.mode", tc.mode)
			router := gin.New()
			SetupRoutes(router)
			routes := map[string]bool{}
			for _, route := range router.Routes() {
				routes[route.Method+" "+route.Path] = true
			}
			require.True(t, routes[http.MethodGet+" /search"], "regular search must remain registered")
			for _, endpoint := range endpoints {
				expected := tc.enabled
				if endpoint.path == "/search/reasoning" {
					expected = tc.syncRegistered
				}
				require.Equal(t, expected, routes[endpoint.method+" "+endpoint.path], endpoint.path)
				if !expected {
					response := httptest.NewRecorder()
					router.ServeHTTP(response, httptest.NewRequest(endpoint.method, endpoint.path, nil))
					require.Equal(t, http.StatusNotFound, response.Code, endpoint.path)
				}
			}
		})
	}
}

func TestSynchronousReasoningSearchOptInIsIndependentOfServerMode(t *testing.T) {
	oldFlag, oldMode := viper.Get("llm.sync-endpoint-enabled"), viper.Get("server.mode")
	t.Cleanup(func() {
		viper.Set("llm.sync-endpoint-enabled", oldFlag)
		viper.Set("server.mode", oldMode)
	})
	for _, tc := range []struct {
		name, mode string
		enabled    bool
		wantStatus int
	}{
		{"disabled-debug", gin.DebugMode, false, http.StatusForbidden},
		{"disabled-release", gin.ReleaseMode, false, http.StatusForbidden},
		{"disabled-test", gin.TestMode, false, http.StatusForbidden},
		{"enabled-debug", gin.DebugMode, true, http.StatusBadRequest},
		{"enabled-release", gin.ReleaseMode, true, http.StatusBadRequest},
		{"enabled-test", gin.TestMode, true, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			viper.Set("llm.sync-endpoint-enabled", tc.enabled)
			viper.Set("server.mode", tc.mode)
			router := gin.New()
			// No runtime: disabled requests stop at the guard; enabled ones reach validation.
			router.POST("/sync", ReasoningSearchHandler)
			body := `{"q":"test","deb":true}`
			if tc.enabled {
				body = `{"q":`
			}
			request := httptest.NewRequest(http.MethodPost, "/sync", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, tc.wantStatus, response.Code)
		})
	}
}
