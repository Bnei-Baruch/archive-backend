# ES6 → ES9 Search Migration Design Plan

## Session Summary (2026-04-01)

### What was done this session

1. **Disk space crisis resolved** — ES9 indexing was failing with 503 errors because the host disk was at 93% (ES high watermark is 90%). Freed ~20 GB by pruning Docker build cache (7.5 GB), removing old `bneibaruch/subtitles_frontend` image tags (:4/:5/:6/:8/:9/:27/:48), and deleting `/tmp/dump` (5.2 GB of ES9 dump files). Disk is now at 80%.

2. **Full ES9 indexing completed** — All 6 content types (content-units, collections, sources, tags, blog-posts, tweets) successfully indexed into ES9 across all 39 languages. Indices follow the naming convention `results_{date}_{version}_{lang}` (e.g., `results_20260331_1_en`) with aliases pointing to the latest.

3. **Incremental indexing** — Not yet implemented for ES9. Marked as a future task (Task #8).

4. **Search migration scoped** — Full audit of ES6 search codebase completed. Migration plan designed (see below).

---

## Repository Context

- **Project:** Bnei Baruch Archive (Kabbalahmedia)
- **Backend:** `archive-backend/` — Go, Gin, Elasticsearch
- **ES6 client:** `gopkg.in/olivere/elastic.v6` (port 9200)
- **ES9 client:** `github.com/elastic/go-elasticsearch/v9` (port 9201)
- **ES9 indexing:** Complete (`es9/indexing/`)
- **ES9 search:** Not yet implemented

---

## Current ES6 Search Architecture

### Entry points

| Endpoint | Handler | File |
|----------|---------|------|
| `GET /search` | `SearchHandler` | `api/handlers.go:709` |
| `GET /mobile/search` | `MobileSearchHandler` | `api/handlers.go` |
| `GET /autocomplete` | `AutocompleteHandler` | `api/handlers.go` |
| `GET /stats/search_class` | `SearchStatsHandler` | `api/handlers.go` |

### Request flow

```
HTTP /search
  → SearchHandler (api/handlers.go:709)
      parse query + filters (language, content_type, source, tag, media_language)
      detect input language
      NewESEngine + ESManager.GetClient()
      DoSearch() [parallel]
        ├─ SearchGrammars   (grammar_v2_engine.go)
        ├─ Native Search    (engine.go → query.go → ES6 HTTP)
        ├─ SearchTweets     (tweets_engine.go)
        └─ SearchLessonSeries (lessons_series_engine.go)
      merge + deduplicate results
      format JSON response
```

### Key files

| File | Purpose |
|------|---------|
| `search/engine.go` | `ESEngine` struct, `DoSearch()`, result merging (53 KB) |
| `search/query.go` | All ES6 query DSL builders using olivere fluent API (25 KB) |
| `search/elastic_search_manager.go` | ES6 client init/lifecycle |
| `search/models.go` | `Engine` interface, `Intent`, `QueryResult` — uses olivere types |
| `search/common.go` | `SearchRequestOptions`, `QueryResult` — uses olivere types |
| `search/facet.go` | Facet aggregation query builders — uses olivere types |
| `search/grammar.go` | `Grammars` struct, passes `*elastic.Client` |
| `search/variable.go` | Variable loading, passes `*elastic.Client` |
| `search/grammar_v2_engine.go` | Grammar-based intent search (47 KB) |
| `search/intent_engine.go` | Intent detection, second-round search |
| `search/typo_suggest_engine.go` | Hunspell + ES suggest |
| `search/lessons_series_engine.go` | Lesson/series aggregation search |
| `search/tweets_engine.go` | Tweet-specific search |

### ES6 index naming

- **Format:** `{namespace}_{base}_{lang}` (alias) → `{namespace}_{base}_{lang}_{date}` (physical)
- **Example alias:** `prod_results_en`

### ES9 index naming

- **Format:** `{base}_{date}_{version}_{lang}` (physical)
- **Example alias:** `results_en` → `results_20260331_1_en`

---

## Migration Design

### Core principle

Add ES9 engine files **directly into the `search/` package** alongside existing ES6 files. This gives free sharing of all ES-agnostic code with zero refactoring cost.

### File classification

#### Truly ES-free — shared automatically, zero changes needed

| File | Contents |
|------|---------|
| `search/token.go` | Tokenization logic |
| `search/grammar_v2.go` | Grammar V2 definitions |
| `search/grammar_variables_matcher.go` | Variable matching |
| `search/variable_v2.go` | Variable V2 logic |
| `search/year.go` | Year parsing |

#### Lightly coupled — small interface abstraction needed

| File | Problem | Fix |
|------|---------|-----|
| `search/grammar.go` | `Grammars` struct and `MakeGrammars()` accept `*elastic.Client` | Replace param with a `Tokenizer` interface (just the `Search()` method) |
| `search/variable.go` | Same pattern — passes client to token functions | Same fix |

#### Need porting — use olivere return types / query builders

| File | Problem | Fix |
|------|---------|-----|
| `search/models.go` | `Engine` interface returns `*elastic.SearchResult` etc. | Define local `SearchHit`, `SearchResult`, `SearchSuggest` structs |
| `search/common.go` | `QueryResult`, `SearchRequestOptions` embed olivere types | Update to use local structs |
| `search/facet.go` | Returns `*elastic.SearchRequest`, uses `elastic.Query` | Add `es9_facet.go` counterpart emitting JSON |

#### New files to create (all in `search/` package)

| New File | Ports From |
|----------|-----------|
| `search/es9_engine.go` | `search/engine.go` |
| `search/es9_query.go` | `search/query.go` |
| `search/es9_typo_suggest_engine.go` | `search/typo_suggest_engine.go` |
| `search/es9_intent_engine.go` | `search/intent_engine.go` |
| `search/es9_lessons_series_engine.go` | `search/lessons_series_engine.go` |
| `search/es9_tweets_engine.go` | `search/tweets_engine.go` |
| `search/es9_facet.go` | `search/facet.go` |

### ES9 client differences

| Aspect | ES6 (olivere) | ES9 (go-elasticsearch) |
|--------|--------------|----------------------|
| Query building | Fluent: `elastic.NewBoolQuery().Must(...)` | Raw JSON: `map[string]interface{}{"bool": ...}` |
| Executing search | `esc.Search().Index().Query().Do(ctx)` | `es.Search(es.Search.WithIndex(...), es.Search.WithBody(...))` |
| Response type | `*elastic.SearchResult` (typed) | `*esapi.Response` → `json.Unmarshal` |
| Document types | `Type("result")` in query | Not used (removed in ES8+) |
| Suggest | `elastic.NewSuggestService()` | JSON body with `suggest` key |
| Aliases | `AliasService()` | `es.Indices.PutAlias()` |

### Shared local result types (to define in models.go)

```go
type SearchHit struct {
    Index     string
    ID        string
    Score     *float64
    Source    json.RawMessage
    Highlight map[string][]string
}

type SearchResult struct {
    TotalHits int64
    MaxScore  *float64
    Hits      []*SearchHit
}

type SuggestOption struct {
    Text  string
    Score float64
}

type SearchSuggest map[string][]SuggestOption
```

### Config toggle

Add to `config.toml` / `config.sample.toml`:
```toml
[elasticsearch]
use_es9 = false   # set to true to route search through ES9
```

`SearchHandler` (and other handlers) check this flag and instantiate either `ESEngine` or `ES9Engine`. Both implement the `Engine` interface.

---

## Task List

| # | Task | Blocked By | Status |
|---|------|-----------|--------|
| 1 | Define shared result types in `search/models.go` | — | pending |
| 2 | Abstract ES client in `grammar.go` and `variable.go` | #1 | pending |
| 3 | Port `common.go` and `facet.go` to use shared result types | #1 | pending |
| 4 | Write `es9_query.go` — JSON query builders | #1, #3 | pending |
| 5 | Write `es9_engine.go` — main ES9 search engine | #2, #4 | pending |
| 6 | Port sub-engines to ES9 | #5 | pending |
| 7 | Wire ES9 engine into API handlers with config toggle | #5, #6 | pending |
| 8 | Implement incremental indexing for ES9 | — | pending |
| 9 | Validate ES9 search parity with compare command | #7 | pending |

### Dependency graph

```
#1 (models)
 ├─→ #2 (grammar/variable abstraction)
 │     └─→ #5 (es9_engine)
 └─→ #3 (common/facet)
       └─→ #4 (es9_query)
             └─→ #5 (es9_engine)
                   └─→ #6 (sub-engines)
                         └─→ #7 (API wiring)
                               └─→ #9 (parity validation)

#8 (incremental indexing) — independent
```

---

## Key Risks

1. **`query.go` conversion** — 700 lines of olivere fluent DSL to JSON. Most complex single piece.
2. **`grammar_v2_engine.go`** — 47 KB, heavily uses `*elastic.SearchResult` throughout. Port after models.go defines shared types.
3. **Alias name alignment** — ES9 indexer creates aliases as `results_{lang}` (e.g., `results_en`). ES6 uses `prod_results_en`. Search must query the correct alias. Confirm with: `curl localhost:9201/_cat/aliases?v`.
4. **Highlight field parsing** — ES6 highlights come typed via olivere. ES9 returns raw JSON, must parse `hit.highlight` manually.
5. **Completion/suggest field** — ES9 mapping must have `title_suggest` completion field identical to ES6 mapping. Verify this exists in current ES9 index mappings.
