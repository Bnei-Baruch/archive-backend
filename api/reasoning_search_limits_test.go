package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

func TestReasoningSearchClientIP(t *testing.T) {
	for _, tc := range []struct{ peer, forwarded, real, want string }{
		{"203.0.113.1:123", "198.51.100.1", "192.0.2.1", "203.0.113.1"},
		{"172.18.0.2:123", "198.51.100.1, 203.0.113.1", "", "203.0.113.1"},
		{"127.0.0.1:123", "203.0.113.1, 172.18.0.3", "", "203.0.113.1"},
		{"172.18.0.2:123", "", "203.0.113.1", "203.0.113.1"},
		{"172.18.0.2:123", "invalid", "203.0.113.1", "172.18.0.2"},
		{"[2001:db8::1]:123", "203.0.113.1", "", "2001:db8::1"},
		{"bad-peer", "203.0.113.1", "", "unknown"},
	} {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.RemoteAddr = tc.peer
		r.Header.Set("X-Forwarded-For", tc.forwarded)
		r.Header.Set("X-Real-IP", tc.real)
		require.Equal(t, tc.want, reasoningSearchClientIP(r))
	}
}

type limitedRunService struct {
	*llm.StubLLMService
	started chan context.Context
	exit    chan struct{}
	mode    string
}

func (s *limitedRunService) GetReasoningStructuredOutputWithToolsForSession(ctx context.Context, providerID *string, _ *string, _ string, _ string, _ *int, _ []llm.LLMBotMessage, _ []llm.ToolCall, _ map[string]llm.ToolHandler, _ []llm.ToolCall, _ map[string]llm.ToolHandler, _ *string, _ *string, _ bool, _ int, output interface{}) (string, error) {
	s.started <- ctx
	<-s.exit
	if s.mode == "panic" {
		panic("provider panic")
	}
	if s.mode == "error" {
		return "", errors.New("provider failed")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	response := output.(*llm.ReasoningSearchResponse)
	response.Query, response.NoResults = "test", true
	return *providerID, nil
}

func TestReasoningSearchRunAdmissionAndCleanup(t *testing.T) {
	for key, value := range map[string]interface{}{"llm.provider": "stub", "llm.reasoning-search-planning-enabled": false, "llm.reasoning-search-verification-enabled": false, "server.mode": "debug"} {
		previous := viper.Get(key)
		viper.Set(key, value)
		key := key
		t.Cleanup(func() { viper.Set(key, previous) })
	}
	for _, mode := range []string{"success", "error", "panic", "cancel", "prepare-error"} {
		t.Run(mode, func(t *testing.T) {
			workflow := llm.NewReasoningWorkflowSessionStore(time.Minute)
			defer workflow.Close()
			progress := llm.NewReasoningProgressStore(time.Minute)
			defer progress.Close()
			cache := llm.NewReasoningSearchCacheStore(time.Minute)
			defer cache.Close()
			manager, err := llm.NewReasoningToolManager()
			require.NoError(t, err)
			service := &limitedRunService{StubLLMService: llm.NewStubLLMService(nil), started: make(chan context.Context, 1), exit: make(chan struct{}), mode: mode}
			defer func() {
				select {
				case <-service.exit:
				default:
					close(service.exit)
				}
			}()
			runtime := &llm.Runtime{Tools: manager, Workflow: workflow, Progress: progress, Cancellations: llm.NewReasoningCancellationStore(), RunGuard: llm.NewReasoningRunGuard(), ReasoningCache: cache, Services: map[string]llm.Service{"stub": service}}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("LLM_RUNTIME", runtime)
				c.Set("MDB_DB", &sql.DB{}) // Empty stub results do not query the database.
			})
			router.POST("/start", ReasoningSearchStartHandler)
			router.POST("/cache", ReasoningSearchCacheHandler)
			router.POST("/sync", ReasoningSearchHandler)
			request := func(path, body string) *httptest.ResponseRecorder {
				r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
				r.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, r)
				return w
			}
			if mode == "prepare-error" {
				require.Equal(t, http.StatusNotFound, request("/start", `{"q":"test","session_id":"missing"}`).Code)
			} else {
				// Canceling the start HTTP request must not cancel the accepted run.
				requestCtx, cancelRequest := context.WithCancel(context.Background())
				r := httptest.NewRequest(http.MethodPost, "/start", strings.NewReader(`{"q":"test"}`)).WithContext(requestCtx)
				r.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, r)
				cancelRequest()
				require.Equal(t, http.StatusAccepted, w.Code)
				var start struct {
					SessionID string `json:"session_id"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &start))
				var runCtx context.Context
				select {
				case runCtx = <-service.started:
				case <-time.After(time.Second):
					t.Fatal("background run did not start")
				}
				require.NoError(t, runCtx.Err())
				deadline, ok := runCtx.Deadline()
				require.True(t, ok)
				require.True(t, time.Until(deadline) > 4*time.Minute && time.Until(deadline) <= 5*time.Minute)
				if mode == "cancel" {
					require.True(t, cancelReasoningSearchSession(runtime, start.SessionID))
					require.ErrorIs(t, runCtx.Err(), context.Canceled)
				}
			}

			// Fill remaining slots; rejected requests must not touch workflow state.
			var blockers []*llm.ReasoningRun
			defer func() {
				for _, run := range blockers {
					run.Finish()
				}
			}()
			for i := 0; ; i++ {
				run, _ := runtime.RunGuard.TryStart(context.Background(), fmt.Sprint(i))
				if run == nil {
					break
				}
				blockers = append(blockers, run)
			}
			if mode == "prepare-error" {
				require.Len(t, blockers, 30, "session preparation failure leaked a slot")
			} else {
				require.Len(t, blockers, 29)
			}
			key, _ := llm.ReasoningSearchCacheKeyForQuery("test", false, reasoningSearchOutputLanguageName("en", "test"))
			cache.Set(key, &llm.ReasoningSearchCacheEntry{NoResults: true})
			canceled := false
			runtime.Cancellations.Set("victim", func() { canceled = true })
			for _, path := range []string{"/start", "/cache", "/sync"} {
				w := request(path, `{"q":"test","ui_language":"en","session_id":"missing","cancel_session_id":"victim"}`)
				require.Equal(t, http.StatusTooManyRequests, w.Code)
				require.NotEmpty(t, w.Header().Get("Retry-After"))
				require.Contains(t, w.Body.String(), "reasoning_search_limit_reached")
			}
			require.False(t, canceled, "rejection must happen before cancellation side effects")
			close(service.exit)
			if mode != "prepare-error" {
				require.Eventually(t, func() bool {
					run, _ := runtime.RunGuard.TryStart(context.Background(), "probe")
					if run == nil {
						return false
					}
					run.Finish()
					return true
				}, time.Second, time.Millisecond, "finished/canceled/panicking run leaked a slot")
			}
		})
	}
}
