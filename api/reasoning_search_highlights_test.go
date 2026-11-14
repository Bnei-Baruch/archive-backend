package api

import (
	"database/sql"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
	"github.com/stretchr/testify/require"
	htmlparser "golang.org/x/net/html"
	"gopkg.in/gin-gonic/gin.v1"
)

func TestSanitizeReasoningSearchHighlightHTML(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{"emphasis", `<em>important</em> & text`, `<em>important</em> &amp; text`},
		{"script", `<script>alert(1)</script>`, `&lt;script&gt;alert(1)&lt;/script&gt;`},
		{"image", `<img src=x onerror="alert(1)">`, `&lt;img src=x onerror=&#34;alert(1)&#34;&gt;`},
		{"em-attributes", `<em onmouseover='alert(1)'>text</em>`, `&lt;em onmouseover=&#39;alert(1)&#39;&gt;text</em>`},
		{"uppercase", `<EM>text</EM>`, `&lt;EM&gt;text&lt;/EM&gt;`},
		{"self-closing", `<em/>text`, `&lt;em/&gt;text`},
		{"malformed", `<img src=x onerror=alert(1)`, `&lt;img src=x onerror=alert(1)`},
		{"encoded-tags", `&lt;em&gt;text&lt;/em&gt;`, `&lt;em&gt;text&lt;/em&gt;`},
		{"numeric-entities", `&#x3c;svg/onload=alert(1)&#x3e;`, `&lt;svg/onload=alert(1)&gt;`},
		{"nested-entities", `&amp;lt;script&amp;gt;`, `&amp;lt;script&amp;gt;`},
		{"existing-entities", `A &amp; B &lt; C`, `A &amp; B &lt; C`},
		{"comment", `<!-- <img src=x onerror=alert(1)> -->`, `&lt;!-- &lt;img src=x onerror=alert(1)&gt; --&gt;`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeReasoningSearchHighlightHTML(tc.input)
			require.Equal(t, tc.want, got)
			require.Equal(t, got, sanitizeReasoningSearchHighlightHTML(got), "cached highlights must not be double-escaped")
			z := htmlparser.NewTokenizer(strings.NewReader(got))
			for {
				switch z.Next() {
				case htmlparser.ErrorToken:
					require.Equal(t, io.EOF, z.Err())
					return
				case htmlparser.TextToken:
				default:
					token := z.Token()
					require.Equal(t, "em", token.Data)
					require.Empty(t, token.Attr)
				}
			}
		})
	}
}

func TestReasoningSearchHighlightsEscapeLookupEvidenceAsPlainText(t *testing.T) {
	text := `<em>literal tag</em> & <img src=x onerror=alert(1)>`
	for _, evidence := range []llm.ReasoningSearchResultEvidence{
		{SupportingSnippet: text, Content: "unused fallback"},
		{Content: text},
	} {
		result := llm.ReasoningSearchResult{LookupEvidence: []llm.ReasoningSearchResultEvidence{evidence}}
		highlights := buildReasoningSearchHighlightsFromEvidence(result, "", "")
		require.Len(t, highlights, 1)
		require.Equal(t, html.EscapeString(text), highlights[0].Text)
		visible := compactReasoningSearchHighlightsForDisplay(highlights, "", "")
		require.Len(t, visible, 1)
		require.Equal(t, html.EscapeString(text), visible[0].Text)
	}
}

func TestReasoningSearchHighlightsAreSafeAfterTruncation(t *testing.T) {
	for _, text := range []string{
		`<img src=x onerror=alert(1)> short <em>text</em>`,
		`<img src=x onerror=alert(1)> ` + strings.Repeat("ordinary text ", 100),
		strings.Repeat("background ", 100) + `<img src=x onerror=alert(1)> <em>anchor</em> ` + strings.Repeat("ending ", 100),
	} {
		visible := compactReasoningSearchHighlightsForDisplay([]llm.ReasoningSearchHighlight{{Field: "content", Text: text}}, "", "")
		require.Len(t, visible, 1)
		require.NotContains(t, visible[0].Text, "<img")
		if strings.Contains(text, "<em>") {
			require.Contains(t, visible[0].Text, "<em>")
		}
	}
}

func TestRapidReasoningSearchStatusEscapesHighlights(t *testing.T) {
	workflow := llm.NewReasoningWorkflowSessionStore(time.Minute)
	defer workflow.Close()
	progress := llm.NewReasoningProgressStore(time.Minute)
	defer progress.Close()
	id, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{})
	require.NoError(t, err)
	require.NoError(t, workflow.SetRapid(id, true))
	_, _, err = workflow.AddPartialResults(id, []llm.ReasoningSearchResult{{
		MDBUID: "uid", Highlights: []llm.ReasoningSearchHighlight{{Field: "content", Text: `<em>ES</em> <img src=x onerror=alert(1)>`}},
	}})
	require.NoError(t, err)
	_, _, err = workflow.AddPartialLookupEvidence(id, "uid", []llm.ReasoningSearchResultEvidence{{Content: "document excerpt", SupportingSnippet: `<svg onload=alert(1)> evidence`}})
	require.NoError(t, err)
	require.NoError(t, workflow.SetRapidClassifications(id, []llm.ReasoningSearchRapidClassification{{MDBUID: "uid", Relevance: "relevant"}}, nil))
	progress.Reserve(id)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("LLM_RUNTIME", &llm.Runtime{Workflow: workflow, Progress: progress})
		c.Set("MDB_DB", (*sql.DB)(nil))
	})
	router.GET("/status", ReasoningSearchStatusHandler)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/status?session_id="+id, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	var response reasoningSearchStatusResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.RapidResultsAvailable)
	require.Len(t, response.RapidResults, 1)
	require.Len(t, response.RapidResults[0].Highlights, 2)
	require.Equal(t, `&lt;svg onload=alert(1)&gt; evidence`, response.RapidResults[0].Highlights[0].Text)
	require.Equal(t, `<em>ES</em> &lt;img src=x onerror=alert(1)&gt;`, response.RapidResults[0].Highlights[1].Text)
}
