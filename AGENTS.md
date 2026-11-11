# AGENTS.md

## Scope and Project
- Instructions for coding agents in this repository. Keep changes simple and follow existing conventions.
- Go (`go 1.16`); CLI: `archive-backend <command>`. Commands: `server`, `index`, `events`, `cms`, `eval`, `version`.
- Backend API and MDB-to-Elasticsearch ETL. Modules: `api/` handlers/routes, `search/` queries, `es/` indexing, `mdb/` generated SQLBoiler models, `events/` NATS processing.
- Services: Postgres (`[mdb]`), Elasticsearch (`[elasticsearch]`), NATS (`[nats]`), assets/doc2text (`[assets_service]` / unzip URL), and the configured LLM provider.
- Config: `config.toml`; examples and option descriptions: `config.sample.toml`.

## Tests
- LLM changes: `go test ./search/LLM/...`. Tests live in `search/LLM/`, `search/LLM/tests/`, and `search/LLM/tools/`.
- API reasoning changes: also run `go test ./api`. Some suites require configured local Elasticsearch/Postgres; report unavailable dependencies rather than claiming full coverage.
- Use stubs/mocked transports for provider tests; do not make paid provider calls unless explicitly requested.

## Runtime and Tools
- Use `llm.Service` (`search/LLM/service.go`). `common.Init()` builds one app-scoped `common.LLM_RUNTIME` containing shared tools, stores, and provider services; never construct services per request.
- Shared layers: `base_service.go` for client/pricing/debug helpers; `openai_compatible_service.go` for OpenAI-compatible Responses/chat; `replay_chat_completions_service.go` for replay-based chat providers. These paths are under `search/LLM/`.
- Tool infrastructure: `search/LLM/reasoning_tools.go`; implementations: `search/LLM/tools/`; app-scoped builder: `search/LLM/tools/manager.go`.
- Register tools with `ReasoningToolManager`; pass `ToolCalls()` and `ToolHandlers()` to reasoning calls. Same-response calls execute with bounded parallelism, preserving output order.
- Argument/usage mistakes return `llm.RecoverableToolError`, converted to JSON for model correction. Infrastructure/runtime failures remain normal errors.
- Keep schemas strict (`additionalProperties: false` where possible); reuse existing security/published checks and business logic.
- Propagate the run context through tools and `AssetsService.Doc2Text` so cancellation and finish-now interrupt document fetches.
- Never expose provider bodies, database errors, or panic details to clients when `deb=false`. Log details and use `ReasoningProgressFailureMessage`; raw failure details are debug-only.

## Provider Differences
Preserve protocol-specific behavior instead of forcing every provider through identical requests.

| Provider | Required behavior |
| --- | --- |
| OpenAI | `/v1/responses`, continuation via `previous_response_id`; store continuation ID/model/effort, not full prompts or hidden reasoning. |
| xAI | Responses continuation, but omit `instructions` on resumed requests. |
| OpenRouter | Responses API with full history replay, not `previous_response_id`. Preserve configured provider routing. |
| Ollama | `/api/chat` with history replay; no `tool_choice`. Require `ollama.api-endpoint`; no default host. |
| Arcee, Inception, Cohere | Shared replay chat-completions service. Transient HTTP retries are opt-in and enabled only for Inception; do not add global retries to `callLLMAPI`. |
| DeepSeek | Official `/chat/completions` with history replay; retain `reasoning_content` on tool-call turns. |
| Claude | Anthropic `/v1/messages` with replay. Ephemeral cache breakpoints on system text and the last tool definition, never user messages/tool results; account for cache reads/writes. |
| Stub | Local constant responses, no API calls. `[[stub.responses]]` selects by model/query; `query="*"` is a model fallback. |

Replay-provider sessions use `chat_reasoning_sessions.go`.

## Reasoning API
- Implementation and request/response definitions: `api/reasoning_search.go` (including `ReasoningSearchRequest` and `is_rapid`); routes: `api/routes.go`. Read these instead of maintaining duplicate field lists.
- Under `/search/reasoning`: `POST /start` runs in background; `GET /status` reports progress/previews; `GET /result` returns the stored final snapshot; `POST /cancel` cancels. Start may include `cancel_session_id`.
- Synchronous `POST /search/reasoning` is allowed only with `server.mode="debug"`; otherwise return 403 before work starts. Request `deb` does not bypass this.
- `POST /cache` checks the mode-specific query cache; a hit creates a fresh workflow session and response snapshot.
- Non-rapid only: `POST /finish-now` copies a prepared draft to the final snapshot, marks completion, and returns readiness metadata. No draft means conflict; fetch actual results through `GET /result`.
- Terminal states: `completed`, `failed`, `canceled`. `has_draft_results` enables finish-now; rapid previews use `rapid_results_available`/`rapid_results`. Other status hints are defined in `reasoning_progress.go`.
- Successful empty results are `no_results=true`, `results=[]`, `summary=null`, not a failure.
- Query length and per-iteration tool limits use `llm.reasoning-search-max-query-characters` and `llm.reasoning-search-max-tools-per-iteration`. Skipped calls still need protocol-compatible outputs, logging, and `debug.skipped_tool_calls`.

## Workflow and Sessions
- API `session_id` identifies a backend workflow, not a provider session. Stores are in-memory with `llm.reasoning-session-ttl`; status/result/follow-up renew live sessions. Missing/expired IDs are errors, never implicit new searches.
- Stages own provider/model/effort settings: execute stored stage configuration, not current config, for existing sessions. Stages include reasoning and optional planning/verification.
- Planning runs once on the initial request, can restrict first-iteration tools, and fails softly with a warning. Verification is one-shot and may have no provider-native session ID. Debug planning usage is merged into totals as `debug.planning_model_usage`.
- Distinguish `ReasoningCache` (shared initial results) from the exact per-session response snapshot. Query-cache keys include normalized query, regular/rapid mode, and resolved output language (`ui_language` or detection/default); preserve rapid relevance, empty-result semantics, and null summary.
- `PartialResults` is the shared candidate pool from ES and concrete PostgreSQL lookups. `get_available_books` contributes only author roots `bs`, `rh`, `ar`. AI lookup tools add evidence by UID, not new candidates.
- Draft follow-ups and mode changes start fresh provider context seeded with the visible stored response. Same-mode non-rapid follow-ups preserve provider continuation/history unless following a draft.
- Old runs must not overwrite newer state. Cancellation cleanup must use its own per-run token. Rapid cleanup/classifier writes use the captured workflow revision, terminal progress updates the captured progress revision; check revisions under store locks.
- Draft generation uses collected candidates and inherits the run context. Recover goroutine panics and clear draft-in-progress. Include draft usage in draft/final totals, with `debug.draft_model_usage` and individual `debug.draft_model_runs`.

## Rapid Search
- Gather candidates with the main reasoning model; classify with a separate model in batches; optionally finalize. Batch size and sorting rules live in `api/reasoning_search.go`.
- One classifier worker per run receives candidate/evidence notifications and stores each completed batch for previews during gathering. Reclassify candidates only when their lookup evidence changes.
- Status reads stored labels only; it must not call the classifier. Stop/join the preview worker before the final classification pass.
- Without the finalizer, return classifier-sorted results and `summary=null`. Sort by relevance first, then backend tie-breakers (type, evidence, language, freshness, source/collection boosts).
- User-facing highlights come from ES and lookup evidence, not model-generated text; backend ranking/truncation controls display.
- Regular ES search excludes LIKUTIM; reasoning opts in with `Query.IncludeLikutim`. Preserve this flag in rebuilt highlight queries.
- ES tool outputs omit internal hit IDs/routing, pagination metadata, profiling, and execution timings. Keep `_score`/`max_score`, public IDs/relationships, `_source`, highlights, and nested result data; retain timeout/shard-failure warnings. This filtering is model-facing only.

## Configuration
- `[llm].provider` defaults to `openai`. Exact keys, defaults, fallback rules, and validation live in `search/LLM/service_factory.go` and `config.sample.toml`.
- Stage provider selection uses `llm.reasoning-search-<stage>-provider`: planning, verification, draft, rapid-gather, rapid-classifier, rapid-finalizer. Enable flags are under `[llm]`; draft provider falls back to `llm.ai-tools-provider`.
- Provider-stage settings follow `<provider>.reasoning-search-<stage>-{model,effort,max-output-tokens}`; main reasoning uses `<provider>.reasoning-search-*`. Unset draft model/effort/token settings fall back to AI-tools settings.
- Unset rapid classifier/finalizer effort is `low` for OpenRouter, Ollama, Inception, Z.AI; other providers omit it. Preserve provider/model-specific effort validation.
- xAI Grok 4 fast: no effort; keep reasoning/planning effort empty.
- Arcee: optional `minimal/low/medium/high`; do not send effort to `trinity-large-thinking`.
- Inception Mercury: `instant/low/medium/high`. Cohere: optional `none/high`.
- DeepSeek: `minimal` disables thinking; `low/medium/high` map to `high`, `xhigh/max` to `max`.
- OpenRouter supports per-stage routing overrides and `openrouter.enforced-tool-use-iterations` (`0` disables forced tools). Ollama supports `num-ctx`, optional `temperature` and `structured-output-prompt-schema`; see sample config.
- OpenAI GPT-5.6+ explicit caching applies to stable instruction prefixes, not query-specific planned turns. Rapid gather/classifier/finalizer overrides inherit the global flag when unset. Tested defaults: automatic globally/gather, explicit classifier, automatic finalizer until tested.
- Pricing: `[[<provider>.pricing]]`; shared accounting: `search/LLM/llm_pricing.go`.

## Tool Lookup and Caches
- Definitions, schemas, and usage descriptions are the source of truth: `ai_tools.go` (source/transcript reader), `postgresql_tools.go` (book/source/collection/unit lookups), `elasticsearch_search_tool.go` (normalizes filters and runs `search.ESEngine.DoSearch`), under `search/LLM/tools/`.
- Source/transcript tools resolve a public file, fetch doc2text, and use the cheaper reader model for relevant excerpts. Every transcript SQL path, including numeric IDs/language fallbacks, requires a public published unit and public published file with `removed_at IS NULL`.
- AI document cache: 32 MiB byte budget per loader, counting text, chunks, keys, and overhead; text TTL 1 hour, negative TTL 5 minutes. Negative hits skip SQL; positive hits must re-resolve access and match file UID. Expired entries miss; full caches evict arbitrary entries; oversized documents are not cached.
- PostgreSQL result caches: configured TTL (default 3 hours), 8 MiB per tool. Count keys/values plus overhead, evict when full, skip oversized results. Check expiry/delete under one lock so stale reads cannot delete replacements.
