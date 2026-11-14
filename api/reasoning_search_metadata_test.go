package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestReasoningSearchMetadataRequiresPublicPublishedRows(t *testing.T) {
	dsn := os.Getenv("REASONING_TOOLS_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set REASONING_TOOLS_TEST_POSTGRES_DSN to run PostgreSQL metadata access tests")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	// Temporary fixtures on one connection never modify persistent tables.
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = db.ExecContext(ctx, `
CREATE TEMP TABLE content_units (id bigint, uid text, secure smallint, published boolean, type_id bigint, properties jsonb);
CREATE TEMP TABLE collections (id bigint, uid text, secure smallint, published boolean, type_id bigint, properties jsonb);
CREATE TEMP TABLE collections_content_units (collection_id bigint, content_unit_id bigint);
CREATE TEMP TABLE content_unit_i18n (content_unit_id bigint, language text, name text);
CREATE TEMP TABLE collection_i18n (collection_id bigint, language text, name text);
CREATE TEMP TABLE sources (uid text);
CREATE TEMP TABLE tags (uid text);
CREATE TEMP TABLE twitter_tweets (twitter_id text);
INSERT INTO content_units VALUES (1, 'public-u', 0, TRUE, 1, '{}'), (2, 'secure-u', 1, TRUE, 1, '{}'), (3, 'hidden-u', 0, FALSE, 1, '{}');
INSERT INTO collections VALUES (1, 'public-c', 0, TRUE, 1, '{}'), (2, 'secure-c', 1, TRUE, 1, '{}'), (3, 'hidden-c', 0, FALSE, 1, '{}');
INSERT INTO content_unit_i18n VALUES (1, 'en', 'Public unit');
INSERT INTO collection_i18n VALUES (1, 'en', 'Public collection');`)
	require.NoError(t, err)
	previousRegistry := mdb.CONTENT_TYPE_REGISTRY
	mdb.CONTENT_TYPE_REGISTRY = &mdb.ContentTypeRegistry{ByID: map[int64]*mdbmodels.ContentType{
		1: {ID: 1, Name: consts.CT_ARTICLE},
	}}
	defer func() { mdb.CONTENT_TYPE_REGISTRY = previousRegistry }()

	uids := []string{"public-u", "secure-u", "hidden-u", "public-c", "secure-c", "hidden-c", "missing"}
	types, err := loadReasoningSearchResultTypes(db, uids)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"public-u": consts.ES_RESULT_TYPE_UNITS, "public-c": consts.ES_RESULT_TYPE_COLLECTIONS}, types)
	units, err := loadReasoningSearchContentUnitMetadata(db, BaseRequest{UILanguage: "en"}, uids)
	require.NoError(t, err)
	require.Len(t, units, 1)
	require.Equal(t, "Public unit", units["public-u"].Title)
	collections, err := loadReasoningSearchCollectionMetadata(db, BaseRequest{UILanguage: "en"}, uids)
	require.NoError(t, err)
	require.Len(t, collections, 1)
	require.Equal(t, "Public collection", collections["public-c"].Title)

	results := make([]llm.ReasoningSearchResult, len(uids))
	for i, uid := range uids {
		results[i] = llm.ReasoningSearchResult{MDBUID: uid, ResultType: consts.ES_RESULT_TYPE_UNITS, Title: "Stale title"}
	}
	filtered, err := enrichReasoningSearchResults(db, "en", results)
	require.NoError(t, err)
	require.Len(t, filtered, 2)
	require.Equal(t, "public-u", filtered[0].MDBUID)
	require.Equal(t, "public-c", filtered[1].MDBUID)

	// Full mode accepts existing public UIDs without tool calls, including query UIDs.
	workflow := llm.NewReasoningWorkflowSessionStore(time.Hour)
	defer workflow.Close()
	progress := llm.NewReasoningProgressStore(time.Hour)
	defer progress.Close()
	id, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider: "stub", Model: "test", ProviderSessionID: "provider-session", MaxIterations: 1,
	})
	require.NoError(t, err)
	progress.Reserve(id)
	manager, err := llm.NewReasoningToolManager()
	require.NoError(t, err)
	runtime := &llm.Runtime{
		Tools: manager, Workflow: workflow, Progress: progress,
		Services: map[string]llm.Service{"stub": llm.NewStubLLMService([]llm.StubLLMResponseConfig{{
			Model: "test", Query: "*", Response: `{"query":"Find public-u","results":[{"mdb_uid":"public-u"},{"mdb_uid":"public-c"},{"mdb_uid":"secure-u"},{"mdb_uid":"hidden-c"},{"mdb_uid":"invented"}]}`,
		}})},
	}
	require.NoError(t, executeReasoningSearchForSession(ctx, runtime, db, ReasoningSearchRequest{Query: "Find public-u", UILanguage: "en"}, id, false))
	session, err := workflow.Get(id)
	require.NoError(t, err)
	var response llm.ReasoningSearchResponse
	require.NoError(t, json.Unmarshal(session.ResponseSnapshotJSON, &response))
	require.Len(t, response.Results, 2)
	require.Equal(t, "public-u", response.Results[0].MDBUID)
	require.Equal(t, "public-c", response.Results[1].MDBUID)
	require.Empty(t, session.PartialResults)

	for _, goodUID := range []string{"", "public-c"} {
		t.Run("rapid-fallback/good="+goodUID, func(t *testing.T) {
			rapidSession := &llm.ReasoningWorkflowSession{RapidClassifications: map[string]llm.ReasoningSearchRapidClassification{}}
			for _, uid := range []string{"secure-u", "hidden-u", "secure-c", "hidden-c", "missing-1", "missing-2", goodUID} {
				if uid == "" {
					continue
				}
				rapidSession.PartialResults = append(rapidSession.PartialResults, llm.ReasoningSearchResult{MDBUID: uid})
				rapidSession.RapidClassifications[uid] = llm.ReasoningSearchRapidClassification{Relevance: "relevant"}
			}
			rapidSession.PartialResults = append(rapidSession.PartialResults, llm.ReasoningSearchResult{MDBUID: "public-u"})
			rapidSession.RapidClassifications["public-u"] = llm.ReasoningSearchRapidClassification{Relevance: "can_be_relevant"}

			visible, err := buildRapidVisibleResults(db, "en", rapidSession)
			require.NoError(t, err)
			if goodUID == "" {
				require.Len(t, visible, 1)
			} else {
				require.Len(t, visible, 2)
				require.Equal(t, goodUID, visible[0].MDBUID)
			}
			require.Equal(t, "public-u", visible[len(visible)-1].MDBUID)
		})
	}

	// Access changes after a tool call/cache fill must also remove the result.
	_, err = db.ExecContext(ctx, "UPDATE content_units SET secure = 1; UPDATE collections SET published = FALSE")
	require.NoError(t, err)
	filtered, err = enrichReasoningSearchResults(db, "en", filtered)
	require.NoError(t, err)
	require.Empty(t, filtered)
}
