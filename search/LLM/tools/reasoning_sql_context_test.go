package tools

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestReasoningSQLLookupsHonorRunCancellation(t *testing.T) {
	dsn := os.Getenv("REASONING_TOOLS_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set REASONING_TOOLS_TEST_POSTGRES_DSN to test reasoning SQL cancellation")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	// Hold the only connection: canceled lookups must stop waiting for it.
	setupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	connection, err := db.Conn(setupCtx)
	require.NoError(t, err)
	defer connection.Close()
	source := &aiSourceDocumentLoader{db: db}
	transcript := &aiTranscriptDocumentLoader{db: db}
	for name, lookup := range map[string]func(context.Context) error{
		"books": func(ctx context.Context) error {
			_, err := NewGetAvailableBooksTool(db, 0).Execute(ctx, nil)
			return err
		},
		"author": func(ctx context.Context) error {
			_, err := loadAuthorToolResult(ctx, db, "bs", "en")
			return err
		},
		"source": func(ctx context.Context) error {
			_, err := loadSourceNodeToolResult(ctx, db, "source-uid", "en")
			return err
		},
		"collection": func(ctx context.Context) error {
			_, err := loadCollectionToolResult(ctx, db, "collection-uid", "en")
			return err
		},
		"unit": func(ctx context.Context) error {
			_, err := loadContentUnitToolResult(ctx, db, "unit-uid", "en")
			return err
		},
		"unit-collections": func(ctx context.Context) error {
			_, err := loadContentUnitCollections(ctx, db, 1, "en")
			return err
		},
		"unit-sources": func(ctx context.Context) error {
			_, err := loadContentUnitSources(ctx, db, 1, "en")
			return err
		},
		"unit-tags": func(ctx context.Context) error {
			_, err := loadContentUnitTags(ctx, db, 1, "en")
			return err
		},
		"parent-uids": func(ctx context.Context) error {
			_, err := loadCollectionUIDsForContentUnit(ctx, db, "unit-uid")
			return err
		},
		"source-document-uid": func(ctx context.Context) error {
			_, err := source.resolveFileUID(ctx, "source-uid", "en")
			return err
		},
		"source-document-id": func(ctx context.Context) error {
			_, err := source.resolveFileUID(ctx, "123", "en")
			return err
		},
		"source-document-fallback": func(ctx context.Context) error {
			_, err := source.resolveFileUID(ctx, "source-uid", "")
			return err
		},
		"transcript-document-uid": func(ctx context.Context) error {
			_, err := transcript.resolveFileUID(ctx, "unit-uid", "en", 1)
			return err
		},
		"transcript-document-id": func(ctx context.Context) error {
			_, err := transcript.resolveFileUID(ctx, "123", "en", 1)
			return err
		},
		"transcript-document-fallback": func(ctx context.Context) error {
			_, err := transcript.resolveFileUID(ctx, "unit-uid", "", 1)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			waits := db.Stats().WaitCount
			go func() { done <- lookup(ctx) }()
			require.Eventually(t, func() bool { return db.Stats().WaitCount > waits }, time.Second, time.Millisecond)
			cancel()
			select {
			case err := <-done:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(time.Second):
				t.Fatal("lookup ignored cancellation while waiting for a database connection")
			}
		})
	}
}
