package llm

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReasoningSessionConcurrentReadsAndUpdates(t *testing.T) {
	openai := NewOpenAIReasoningSessionStore(time.Hour)
	defer openai.Close()
	openaiID, err := openai.CreateReserved("model", "")
	require.NoError(t, err)
	chat := NewChatReasoningSessionStore(time.Hour)
	defer chat.Close()
	chatID, err := chat.CreateReserved("model", "")
	require.NoError(t, err)
	workflow := NewReasoningWorkflowSessionStore(time.Hour)
	defer workflow.Close()
	workflowID, err := workflow.Create(ReasoningWorkflowStageReasoning, ReasoningWorkflowStageSession{})
	require.NoError(t, err)
	require.NoError(t, workflow.FinishDraft(workflowID, 0, &ReasoningSearchResponse{Query: "query"}))

	for _, tc := range []struct {
		name                  string
		read, refresh, update func() error
	}{
		{
			name:    "openai",
			read:    func() error { _, err := openai.Get(openaiID); return err },
			refresh: func() error { return openai.Refresh(openaiID) },
			update:  func() error { return openai.Update(openaiID, "response") },
		},
		{
			name:    "chat",
			read:    func() error { _, err := chat.Get(chatID); return err },
			refresh: func() error { return chat.Refresh(chatID) },
			update:  func() error { return chat.Update(chatID, []LLMBotMessage{{Role: "user", Content: "query"}}) },
		},
		{
			name:    "workflow",
			read:    func() error { _, err := workflow.IsFinalizedFromDraft(workflowID); return err },
			refresh: func() error { return workflow.Refresh(workflowID) },
			update:  func() error { _, err := workflow.FinalizeWithDraft(workflowID); return err },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := make(chan struct{})
			var workers sync.WaitGroup
			for _, operation := range []func() error{tc.read, tc.refresh, tc.update} {
				workers.Add(1)
				go func(operation func() error) {
					defer workers.Done()
					<-start
					for i := 0; i < 1000; i++ {
						if err := operation(); err != nil {
							t.Error(err)
							return
						}
					}
				}(operation)
			}
			close(start)
			workers.Wait()
		})
	}
}
