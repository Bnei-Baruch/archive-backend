# Elasticsearch 9 Migration

This directory contains the ES9 migration code and data for the Bnei Baruch Archive search infrastructure.

## Directory Structure

```
es9/
├── common/              # ES9 client and utilities
│   └── client.go        # ES9Manager with health checks, connection pooling, index management
├── indexing/            # Indexing logic
│   ├── mappings.go      # Results index mapping generator (text search only, no embeddings)
│   ├── content_units.go # Content units indexer for results indices
│   └── hebrew_mapping.json  # Reference Hebrew mapping (not used, for documentation)
├── search/              # Search queries (future)
└── data/                # Generated data files
    ├── mappings/
    │   ├── results/     # Generated per-language results mappings (39 files)
    │   │   ├── results-en.json
    │   │   ├── results-he.json
    │   │   ├── results-ru.json
    │   │   ├── results-es.json
    │   │   └── ... (35 more languages)
    │   └── chunks/      # Chunks mappings for vector search (future, 39 files)
    │       ├── chunks-en.json
    │       ├── chunks-he.json
    │       └── ...
    └── synonyms/        # Synonym files (future)
```

## Key Concepts

### Two-Index Architecture

ES9 uses **two separate index types** for different search patterns:

**Index Type 1: `results_{lang}` - Classic Keyword Search**
- **Purpose**: Traditional BM25 text search with filters and facets
- **Documents**: One per content unit, collection, source, etc.
- **Size**: ~200K documents per language
- **Fields**:
  - Text: `title`, `description`, `content`, `full_content`
  - Metadata: `mdb_uid`, `result_type`, `effective_date`, `typed_uids`, `filter_values`
  - Autocomplete: `title_suggest`
  - **NO embeddings** (vectors are in separate indices)
- **Use Cases**:
  - Keyword search with hunspell/ICU analyzers
  - Filtering by date, type, source, tag, person
  - Autocomplete suggestions
  - Faceted navigation

**Index Type 2: `chunks_{lang}` - Semantic Vector Search (Future)**
- **Purpose**: Dense vector search for RAG/LLM retrieval
- **Documents**: Many per content unit (one per chunk)
- **Size**: ~4M+ documents per language (20-100 chunks per long transcript)
- **Fields**:
  - Reference: `mdb_uid` (links back to results index)
  - Content: `chunk_index`, `chunk_text`, `chunk_embedding` (768-dim vector)
  - Minimal metadata: `title`, `effective_date`, `result_type` (for filtering)
- **Use Cases**:
  - "Find passages where Rav discusses unity" (semantic search)
  - LLM context retrieval for RAG
  - Cross-lingual semantic search

**Why Two Indices?**
1. **Separation of Concerns**: Keyword search vs semantic vector search
2. **Storage Efficiency**: No metadata duplication across chunks
3. **Independent Scaling**: Chunks index grows with content length, results stays small
4. **Query Optimization**: Each index optimized for its access pattern
5. **Flexible Evolution**: Change chunking/embedding strategy without reindexing metadata
6. **Cost Control**: Vector indices are expensive; keep them separate and minimal

### Multilingual Mapping Strategy

Each language gets its own mapping file with language-specific analyzers, but the **field structure is identical** across all languages.

#### Hebrew (results-he.json)
- **Analyzers:**
  - `he_hunspell`: Morphological analysis (proven ES6 solution)
  - `he_icu`: Unicode normalization + niqqud removal (NEW)
  - `he_icu_hunspell`: Combined ICU + hunspell (experimental)
- **Fields:** Text only (no embeddings in results index)

#### English (results-en.json)
- **Analyzers:**
  - `en_stemmer`: Standard English stemming with stopwords
- **Fields:** Text only (no embeddings in results index)

#### Other Languages
- Configured in `indexing/mappings.go` with stemmer/hunspell/standard fallback
- All use the same field structure

### Search Layers

**Current (Phase 2): Keyword Search**
1. **Morphological Layer**: Hunspell (Hebrew), Stemmer (English), etc.
   - Handles word forms: סדר → מסדר, סידור, מסודר
2. **Unicode Layer**: ICU normalization (Hebrew)
   - Handles combining characters, removes niqqud

**Future (Phase 3): Hybrid Search with RRF**
Query execution will combine keyword + semantic using **Reciprocal Rank Fusion (RRF)**:

```
User Query: "לימוד קבלה"
    │
    ├─► Path 1: BM25 on results_he (keyword search)
    │   └─ title.language (hunspell) + title.icu (Unicode)
    │
    ├─► Path 2: kNN on chunks_he (semantic search)
    │   └─ chunk_embedding (AlephBERT vectors)
    │
    ↓
RRF Fusion → Ranked Results
```

## Usage

### Generate Mappings for All Languages

```bash
./archive-backend es9mappings

# Or specify output directory
./archive-backend es9mappings --output /custom/path
```

This generates 39 JSON files (one per language) in `es9/data/mappings/results/`.

### Check ES9 Cluster Health

```bash
./archive-backend es9health
```

Output:
```
============================================================
  Elasticsearch 9 Cluster Health
============================================================
Cluster Name:    docker-cluster-9
Status:          green
Version:         9.2.3
Number of Nodes: 1
Active Shards:   0
URL:             http://localhost:9201
Checked at:      2026-01-28T02:58:40Z
============================================================

✓ Cluster status: green
```

### Test ES9 Mappings and Analyzers

Test mapping creation and analyzer functionality for all languages:

```bash
# Test a single language
./archive-backend es9test --lang=en

# Test with cleanup (deletes test index after)
./archive-backend es9test --lang=he --cleanup

# Test all 39 languages at once
./archive-backend es9test --lang=all --cleanup
```

Output example:
```
================================================================================
  ES9 Multi-Language Mapping Tests
================================================================================

[1/39] Testing language: en
--------------------------------------------------------------------------------

------------------------------------------------------------
  Analyzer Tests
------------------------------------------------------------

Analyzer: standard
Input:    Morning lesson about Kabbalah wisdom and the Creator
Tokens:   morning, lesson, about, kabbalah, wisdom, and, the, creator

Analyzer: en_stemmer
Input:    Morning lesson about Kabbalah wisdom and the Creator
Tokens:   Morn, lesson, about, Kabbalah, wisdom, Creator
------------------------------------------------------------

✓ en: SUCCESS

[2/39] Testing language: he
--------------------------------------------------------------------------------

Analyzer: he_hunspell
Input:    שיעור בוקר על חכמת הקבלה ומהות הבורא
Tokens:   שיעור, בוקר, על, חכמת, הקבלה, קבלה, מהות, בורא

✓ he: SUCCESS

...

================================================================================
  Test Summary
================================================================================
Total Languages:  39
Successful:       39 (100.0%)
Failed:           0 (0.0%)
Cleaned up:       true
================================================================================
```

The test validates:
- ✓ Index creation with generated mappings
- ✓ Analyzer configuration (hunspell, ICU, stemmers)
- ✓ Document indexing (content units)
- ✓ Document retrieval
- ✓ Real tokenization via _analyze API

## Breaking Changes from ES6

### 1. Types Removed (CRITICAL)

**ES6:**
```go
index.esc.Index().
    Index(indexName).
    Type("result").     // ← REMOVED in ES7+
    BodyJson(doc)
```

**ES9:**
```go
index.esc.Index().
    Index(indexName).   // No Type() call
    BodyJson(doc)
```

**Mapping Structure:**
```json
// ES6
{
  "mappings": {
    "result": {          // ← Type name
      "properties": {...}
    }
  }
}

// ES9
{
  "mappings": {
    "properties": {...}  // ← Direct properties
  }
}
```

### 2. ICU Plugin Required

ES9 uses ICU Analysis Plugin for Hebrew Unicode normalization:

```bash
# Install ICU plugin on ES9 cluster
./bin/elasticsearch-plugin install analysis-icu
```

### 3. Embeddings in Separate Indices

**Architecture Decision**: All vector embeddings are stored in **separate indices** (`chunks_{lang}`), not in the main `results_{lang}` indices.

**Rationale**:
- Keyword search indices stay small and fast
- Vector indices expensive (HNSW graph, quantization) - keep them minimal
- Independent scaling and optimization
- Clear separation: text search vs semantic search

## Migration Strategy

### Phase 1: Results Index Infrastructure ✅ COMPLETE
- [x] ES9 client setup (`common/client.go`) with index management
- [x] Health check command (`cmd/es9health.go`)
- [x] Results mapping generator (`indexing/mappings.go`) - text fields only, no embeddings
- [x] Generate all 39 language results mappings
- [x] Mapping validation test command (`cmd/es9test.go`)
- [x] Real analyzer testing via _analyze API
- [x] All 39 languages tested successfully (100% pass rate)
- [x] Index creation before indexing (auto-create indices with mappings)
- [x] Content units indexer (`indexing/content_units.go`)

### Phase 2: Results Index Population (Current)
- [x] Index content units to results indices
- [ ] Implement full title hierarchical construction
- [ ] Implement transcript content extraction from DOCX files
- [ ] Implement batch processing for 200K+ content units
- [ ] Build indexing statistics and progress reporting
- [ ] Improve bulk indexing error handling
- [ ] Create sanity check CLI tool
- [ ] Index blog posts to results indices
- [ ] Index tweets to results indices
- [ ] Index collections to results indices
- [ ] Index sources, tags, authors to results indices

### Phase 3: Chunks Index Infrastructure (Future)
- [ ] Design chunks mapping schema (minimal fields, vector-optimized)
- [ ] Implement chunks mapping generator
- [ ] Generate all 39 language chunks mappings
- [ ] Implement transcript chunking logic (~500 tokens/chunk, 50-token overlap)
- [ ] Create chunks indexer for content units
- [ ] Set up AlephBERT embedding service (HTTP API)
- [ ] Add embedding generation to chunks indexer
- [ ] Index chunks with embeddings for long content (lessons, lectures)

### Phase 4: Search Implementation (Future)
- [ ] Basic BM25 search on results indices (hunspell + ICU)
- [ ] kNN semantic search on chunks indices (AlephBERT embeddings)
- [ ] Hybrid search with RRF (combining results + chunks)
- [ ] Grammar-based search (percolator)
- [ ] Two-phase retrieval: chunks → full documents

## Configuration

ES9 configuration in `config.toml`:

```toml
[elasticsearch9]
url="http://localhost:9201"
```

## Dependencies

- **Go 1.23+** (required by ES9 client library)
- **Elasticsearch 9.2.3+**
- **ICU Analysis Plugin** (for Hebrew Unicode normalization)
- **Hunspell dictionaries** (he_IL.aff, he_IL.dic)
- **AlephBERT** (future: for embeddings)

## Language Support

**Total Languages Supported: 39**

All languages tested and validated with 100% success rate.

### Languages with Advanced Analyzers

**Hebrew (he):**
- `he_hunspell`: Morphological analysis with hunspell dictionary
- `he_icu`: Unicode normalization + niqqud removal (ICU plugin required)
- `he_icu_hunspell`: Combined ICU + hunspell

**English (en):**
- `en_stemmer`: Porter stemming with stopword removal

**Russian (ru):**
- `ru_stemmer`: Russian snowball stemmer

**Spanish (es):**
- `es_stemmer`: Light Spanish stemmer

### All Supported Languages

English, Hebrew, Russian, Spanish, German, Italian, French, Portuguese, Turkish, Polish, Arabic, Hungarian, Finnish, Lithuanian, Dutch, Czech, Swedish, Bulgarian, Ukrainian, Romanian, Slovak, Croatian, Slovenian, Latvian, Estonian, Greek, Norwegian, Danish, Chinese, Persian, Georgian, Japanese, Amharic, Hindi, Macedonian, Indonesian, Armenian, Tagalog, Azerbaijani

Languages without specific stemmers use the standard analyzer (can be enhanced in `indexing/mappings.go`).

## Testing

### Mapping and Analyzer Testing

Use the `es9test` command to validate mappings and analyzers:

```bash
# Test single language
./archive-backend es9test --lang=en --cleanup

# Test all languages
./archive-backend es9test --lang=all --cleanup
```

**Test Results (2026-01-28):**
- ✓ 39/39 languages tested successfully (100%)
- ✓ All analyzers validated
- ✓ Document indexing and retrieval confirmed

### Production Testing Plan

Compare ES6 vs ES9 results:
1. Index same data to both ES6 and ES9
2. Run identical queries
3. Compare result relevance
4. A/B test with real users

## Next Steps

### Immediate (Phase 2: Complete Content Units Indexing)
1. **Implement full title hierarchical construction** - Build complete collection paths
2. **Implement transcript extraction** - Extract text from DOCX files
3. **Batch processing** - Handle 200K+ content units efficiently
4. **Statistics and progress reporting** - Real-time indexing progress
5. **Error handling and retry logic** - Robust bulk indexing
6. **Sanity check tool** - Validate index integrity

### Near Term (Phase 2: Other Content Types)
1. **Index blog posts** to results indices
2. **Index tweets** to results indices
3. **Index collections** to results indices
4. **Index sources, tags, authors** to results indices

### Future (Phase 3: Chunks and Embeddings)
1. **Design chunks index schema** - Minimal fields, vector-optimized
2. **Implement chunking logic** - Split long transcripts (~500 tokens/chunk)
3. **Set up AlephBERT service** - Hebrew embedding generation
4. **Create chunks indexer** - Populate chunks_{lang} indices with embeddings
5. **Index with embeddings** - Generate vectors for all chunks

### Long Term (Phase 4: Search)
1. **BM25 search on results** - Keyword search with hunspell/ICU
2. **kNN search on chunks** - Semantic vector search
3. **Hybrid search with RRF** - Combine keyword + semantic
4. **Two-phase retrieval** - Chunks → full documents
5. **Grammar-based search** - Percolator queries

## Key Files

| File | Purpose |
|------|---------|
| `common/client.go` | ES9 client with connection pooling, index management (create, delete, exists) |
| `indexing/mappings.go` | Results mapping generator (text only, no embeddings) for 39 languages |
| `indexing/content_units.go` | Content units indexer for results indices |
| `cmd/es9health.go` | Cluster health check CLI command |
| `cmd/es9mappings.go` | Mapping generation CLI command |
| `cmd/es9test.go` | Mapping validation and analyzer testing CLI command |
| `cmd/es9index.go` | Content indexing CLI command (currently: content-units) |
| `data/mappings/results/*.json` | Generated results mappings (39 files, text fields only) |

## Resources

- [Elasticsearch 9 Reference](https://www.elastic.co/docs/reference/elasticsearch/)
- [Dense Vector Field Type](https://www.elastic.co/docs/reference/elasticsearch/mapping-reference/dense-vector)
- [ICU Analysis Plugin](https://www.elastic.co/guide/en/elasticsearch/plugins/current/analysis-icu.html)
- [Hunspell Token Filter](https://www.elastic.co/guide/en/elasticsearch/reference/current/analysis-hunspell-tokenfilter.html)
- [AlephBERT Model](https://huggingface.co/imvladikon/sentence-transformers-alephbert)
