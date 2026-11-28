package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"gopkg.in/gin-gonic/gin.v1"
)

func TestDisabledReasoningModesHaveNoSideEffects(t *testing.T) {
	oldMode, oldSync := viper.Get("llm.reasoning-search-mode"), viper.Get("llm.sync-endpoint-enabled")
	t.Cleanup(func() {
		viper.Set("llm.reasoning-search-mode", oldMode)
		viper.Set("llm.sync-endpoint-enabled", oldSync)
	})
	viper.Set("llm.sync-endpoint-enabled", true)
	for _, mode := range []string{"rapid", "non-rapid"} {
		t.Run(mode, func(t *testing.T) {
			viper.Set("llm.reasoning-search-mode", mode)
			workflow := llm.NewReasoningWorkflowSessionStore(time.Minute)
			defer workflow.Close()
			progress := llm.NewReasoningProgressStore(time.Minute)
			defer progress.Close()
			cache := llm.NewReasoningSearchCacheStore(time.Minute)
			defer cache.Close()
			id, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{})
			require.NoError(t, err)
			before, err := workflow.Get(id)
			require.NoError(t, err)
			canceled := false
			manager, err := llm.NewReasoningToolManager()
			require.NoError(t, err)
			runtime := &llm.Runtime{
				Tools: manager, Workflow: workflow, Progress: progress, ReasoningCache: cache,
				RunGuard: llm.NewReasoningRunGuard(), Cancellations: llm.NewReasoningCancellationStore(),
			}
			runtime.Cancellations.Set(id, func() { canceled = true })
			disabledRapid := mode == "non-rapid"
			key, ok := llm.ReasoningSearchCacheKeyForQuery("query", disabledRapid, reasoningSearchOutputLanguageName("en", "query"))
			require.True(t, ok)
			cache.Set(key, &llm.ReasoningSearchCacheEntry{Query: "query", NoResults: true})
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("LLM_RUNTIME", runtime)
				c.Set("MDB_DB", &sql.DB{})
			})
			router.POST("/start", ReasoningSearchStartHandler)
			router.POST("/cache", ReasoningSearchCacheHandler)
			router.POST("/sync", ReasoningSearchHandler)
			router.POST("/finish-now", ReasoningSearchFinishNowHandler)
			for _, path := range []string{"/start", "/cache", "/sync"} {
				for _, sessionID := range []string{"", id} {
					body := fmt.Sprintf(`{"q":"query","ui_language":"en","is_rapid":%t,"cancel_session_id":%q`, disabledRapid, id)
					if sessionID != "" {
						body += fmt.Sprintf(`,"session_id":%q`, sessionID)
					}
					request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body+"}"))
					request.Header.Set("Content-Type", "application/json")
					response := httptest.NewRecorder()
					router.ServeHTTP(response, request)
					require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
					require.Contains(t, response.Body.String(), `"code":"reasoning_search_mode_disabled"`)
				}
			}
			if mode == "rapid" {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/finish-now?session_id="+id, nil))
				require.Equal(t, http.StatusForbidden, response.Code)
			}
			require.False(t, canceled)
			after, err := workflow.Get(id)
			require.NoError(t, err)
			require.Equal(t, before, after, "disabled requests must not change workflow state")
			// Rejected requests must not consume even the per-IP attempt allowance.
			for i := 0; i < 60; i++ {
				run, _ := runtime.RunGuard.TryStart(context.Background(), "192.0.2.1")
				require.NotNil(t, run)
				run.Finish()
			}
		})
	}
}

func TestPrepareRapidSessionDoesNotRequireNonRapidConfig(t *testing.T) {
	oldProvider, oldFollowups := viper.Get("llm.provider"), viper.Get("llm.reasoning-search-max-followups")
	t.Cleanup(func() {
		viper.Set("llm.provider", oldProvider)
		viper.Set("llm.reasoning-search-max-followups", oldFollowups)
	})
	viper.Set("llm.provider", "unused-provider")
	viper.Set("llm.reasoning-search-max-followups", 2)
	workflow := llm.NewReasoningWorkflowSessionStore(time.Minute)
	defer workflow.Close()
	progress := llm.NewReasoningProgressStore(time.Minute)
	defer progress.Close()
	runtime := &llm.Runtime{
		Workflow: workflow, Progress: progress,
		RapidConfig: &llm.ReasoningSearchRapidConfig{
			Gather: llm.ReasoningSearchRapidStageConfig{Provider: "stub", Model: "gather", MaxTokens: 1000, MaxIterations: 3},
		},
		Services: map[string]llm.Service{"stub": llm.NewStubLLMService(nil)},
	}
	id, err := prepareReasoningSearchSession(context.Background(), runtime, &ReasoningSearchRequest{Query: "query", IsRapid: true})
	require.NoError(t, err)
	session, err := workflow.Get(id)
	require.NoError(t, err)
	require.True(t, session.Rapid)
	stage := session.Stages[llm.ReasoningWorkflowStageReasoning]
	require.Equal(t, "stub", stage.Provider)
	require.Equal(t, "gather", stage.Model)
	require.Equal(t, 2, stage.MaxFollowups)
	require.NotContains(t, session.Stages, llm.ReasoningWorkflowStagePlanning)
	require.NotContains(t, session.Stages, llm.ReasoningWorkflowStageVerification)
}
