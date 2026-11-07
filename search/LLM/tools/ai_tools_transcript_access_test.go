package tools

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestTranscriptLookupQueriesRequirePublicUnitAndActiveFile(t *testing.T) {
	for _, query := range []string{
		transcriptLookupByUIDWithLanguageQuery, transcriptLookupByUIDFallbackQuery,
		transcriptLookupByMDBIDWithLanguageQuery, transcriptLookupByMDBIDFallbackQuery,
	} {
		for _, guard := range []string{"cu.secure = 0", "cu.published IS TRUE", "f.removed_at IS NULL"} {
			if !strings.Contains(query, guard) {
				t.Fatalf("transcript lookup missing access guard %q: %s", guard, query)
			}
		}
	}
}

func TestTranscriptLookupRejectsSecureUnitWithPublicFile(t *testing.T) {
	dsn := os.Getenv("REASONING_TOOLS_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set REASONING_TOOLS_TEST_POSTGRES_DSN to run PostgreSQL transcript access tests")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Keep temporary fixtures on one connection; persistent tables are never changed.
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = db.ExecContext(ctx, `
CREATE TEMP TABLE content_units (id bigint, uid text, secure smallint, published boolean, type_id bigint);
CREATE TEMP TABLE files (uid text, content_unit_id bigint, secure smallint, published boolean, removed_at timestamptz, name text, language text);
INSERT INTO content_units VALUES (1, 'unituid1', 1, TRUE, 1);
INSERT INTO files VALUES ('fileuid1', 1, 0, TRUE, NULL, 'transcript.docx', 'en');`)
	if err != nil {
		t.Fatal(err)
	}
	queries := []struct {
		name  string
		query string
		id    interface{}
	}{
		{"uid-language", transcriptLookupByUIDWithLanguageQuery, "unituid1"},
		{"uid-fallback", transcriptLookupByUIDFallbackQuery, "unituid1"},
		{"id-language", transcriptLookupByMDBIDWithLanguageQuery, int64(1)},
		{"id-fallback", transcriptLookupByMDBIDFallbackQuery, int64(1)},
	}
	for _, fixture := range []struct {
		name    string
		update  string
		allowed bool
	}{
		{"secure-unit-public-file", "UPDATE content_units SET secure = 1, published = TRUE", false},
		{"unpublished-unit-public-file", "UPDATE content_units SET secure = 0, published = FALSE", false},
		{"removed-file", "UPDATE content_units SET secure = 0, published = TRUE; UPDATE files SET removed_at = NOW()", false},
		{"public-unit-active-file", "UPDATE files SET removed_at = NULL", true},
	} {
		if _, err := db.ExecContext(ctx, fixture.update); err != nil {
			t.Fatal(err)
		}
		for _, lookup := range queries {
			t.Run(fixture.name+"/"+lookup.name, func(t *testing.T) {
				var uid string
				err := db.QueryRowContext(ctx, lookup.query, int64(99), lookup.id, "en").Scan(&uid)
				if fixture.allowed {
					if err != nil || uid != "fileuid1" {
						t.Fatalf("expected public transcript, got uid=%q err=%v", uid, err)
					}
				} else if !errors.Is(err, sql.ErrNoRows) {
					t.Fatalf("expected inaccessible transcript to be excluded, got uid=%q err=%v", uid, err)
				}
			})
		}
	}
}
