package llm

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReasoningProgressIgnoresStaleRunTermination(t *testing.T) {
	for _, canceled := range []bool{true, false} {
		t.Run(fmt.Sprintf("canceled=%t", canceled), func(t *testing.T) {
			store := NewReasoningProgressStore(time.Minute)
			defer store.Close()
			store.Reserve("session")
			old, err := store.Get("session")
			require.NoError(t, err)
			store.Reserve("session")
			current, err := store.Get("session")
			require.NoError(t, err)
			if canceled {
				store.Cancel("session", 3, old.RunRevision)
			} else {
				store.FailRun("session", old.RunRevision, 3, errors.New("private failure"))
			}
			after, err := store.Get("session")
			require.NoError(t, err)
			require.Equal(t, current, after)
			if canceled {
				store.Cancel("session", 3, current.RunRevision)
			} else {
				store.FailRun("session", current.RunRevision, 3, errors.New("private failure"))
			}
			after, err = store.Get("session")
			require.NoError(t, err)
			require.True(t, after.Done)
			if canceled {
				require.Equal(t, ReasoningProgressStateCanceled, after.State)
			} else {
				require.Equal(t, ReasoningProgressStateFailed, after.State)
				require.Equal(t, ReasoningProgressFailureMessage, after.Error)
			}
		})
	}
}

func TestReasoningWorkflowIgnoresStaleRunWrites(t *testing.T) {
	store := NewReasoningWorkflowSessionStore(time.Minute)
	defer store.Close()
	id, err := store.Create(ReasoningWorkflowStageReasoning, ReasoningWorkflowStageSession{})
	require.NoError(t, err)
	old, err := store.Get(id)
	require.NoError(t, err)
	require.NoError(t, store.SetResponseSnapshot(id, nil))
	_, _, err = store.AddPartialResults(id, []ReasoningSearchResult{{MDBUID: "new-run"}})
	require.NoError(t, err)
	require.NoError(t, store.SetResponseSnapshot(id, &ReasoningSearchResponse{Query: "new query"}))
	current, err := store.Get(id)
	require.NoError(t, err)
	labels := []ReasoningSearchRapidClassification{{MDBUID: "new-run", Relevance: "relevant"}}
	runs := []ReasoningSearchUsageBreakdown{{TotalTokens: 1}}
	require.NoError(t, store.SetRapidClassifications(id, labels, runs, current.RunRevision))
	current, err = store.Get(id)
	require.NoError(t, err)
	require.NoError(t, store.SetResponseSnapshot(id, nil, old.RunRevision))
	require.NoError(t, store.SetRapidClassifications(id, []ReasoningSearchRapidClassification{{MDBUID: "old-run", Relevance: "relevant"}}, runs, old.RunRevision))
	after, err := store.Get(id)
	require.NoError(t, err)
	require.Equal(t, current, after)
	require.NoError(t, store.SetResponseSnapshot(id, nil, current.RunRevision))
	after, err = store.Get(id)
	require.NoError(t, err)
	require.Equal(t, current.RunRevision+1, after.RunRevision)
	require.Empty(t, after.ResponseSnapshotJSON)
	require.Empty(t, after.PartialResults)
	require.Empty(t, after.RapidClassifications)
}
