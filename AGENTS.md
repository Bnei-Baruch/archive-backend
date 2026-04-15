# AGENTS.md

## Scope
Instructions for coding agents working in this repository.

## Project Basics
- Language: Go (`go 1.16`)
- CLI app entrypoint: `archive-backend <command>`
- Main commands: `server`, `index`, `events`, `cms`, `eval`, `version`
- Main LLM package: `search/LLM`
- Focused test command: `go test ./search/LLM/...`

## Repo Snapshot
- Purpose: archive backend API + ETL/indexing from MDB (Postgres) to Elasticsearch.
- Core modules:
  - `api/` HTTP handlers and routes
  - `search/` search/query logic
  - `es/` Elasticsearch indexing pipelines
  - `mdb/` SQLBoiler-generated MDB models
  - `events/` NATS-based event processing
- Main external services: Postgres (`[mdb]`), Elasticsearch (`[elasticsearch]`), NATS (`[nats]`), assets/doc2text (`[assets_service]` / unzip URL), LLM providers (`[openai]`, `[openrouter]`, `[ollama]`, `[xai]`, `[zai]`).
- Config: `config.toml` (see `config.sample.toml`).

## LLM Architecture
- Use the provider-agnostic `llm.Service` interface in `search/LLM/service.go`.
- Provider selection is driven by `[llm].provider`.
- Shared service layers:
  - `search/LLM/base_service.go`: provider-agnostic client/pricing/debug helpers
  - `search/LLM/openai_compatible_service.go`: shared OpenAI-compatible `/v1/responses` and chat logic
- OpenAI reasoning + tools flow uses `v1/responses` with iteration via `previous_response_id`.
- xAI also uses `v1/responses` with iteration via `previous_response_id`, but resumed requests must omit `instructions`.
- OpenRouter also uses `v1/responses`, but continues sessions by replaying full message history instead of `previous_response_id`.
- Ollama uses `/api/chat` with message-history replay; it does not use `tool_choice`.
- `common.Init()` builds one app-scoped `common.LLM_RUNTIME`; do not construct new LLM services per request.
- `common.LLM_RUNTIME` stores the shared tool manager, progress/workflow stores, and provider services keyed by provider.
- Tool implementations live under `search/LLM/tools/`.
- LLM package tests live under `search/LLM/tests/`.

## Tooling Infrastructure
- Generic tool abstractions live in `search/LLM/reasoning_tools.go`.
- Register tools with `ReasoningToolManager`.
- Pass `manager.ToolCalls()` and `manager.ToolHandlers()` into `GetReasoningResponseWithTools`.
- App-scoped manager builder lives in `search/LLM/tools/manager.go`.
- `common.Init()` builds the shared tool manager inside `common.LLM_RUNTIME`.

## Reasoning Search
- API endpoints: `POST /search/reasoning/start`, `GET /search/reasoning/status`, `POST /search/reasoning`.
- Request supports `q`, optional `deb`, optional `session_id`, optional `ui_language`.
- The API `session_id` is a workflow session id owned by the backend, not a provider-native LLM session id.
- Response includes `session_id`, `used_tools`, token stats, and debug/cost details when `deb=true`.
- The backend persists a `reasoning` workflow stage and, when enabled, a `verification` workflow stage.
- Workflow stages own their provider/model/effort settings; handlers should execute a stage from stored workflow state, not by re-reading current config for existing sessions.
- Verification is currently a one-shot structured call, so its stored stage metadata may have an empty provider-native session id.
- OpenAI short-lived reasoning sessions are stored in memory only, with TTL from `openai.reasoning-session-ttl`.
- xAI short-lived reasoning sessions are also stored in memory only, with TTL from `xai.reasoning-session-ttl`.
- OpenRouter and Ollama sessions also live in memory, but store full replayable conversation history via `chat_reasoning_sessions.go`.
- If client sends a missing or expired `session_id`, the API returns an error; it does not silently start a new session.
- OpenAI session state stores continuation data (`last_response_id`, model, effort), not the full prompt or hidden reasoning.

## LLM Config
- Provider selection uses `[llm].provider`; default is `openai`.
- Verification enable/provider selection lives under `[llm]`:
  - `llm.reasoning-search-verification-enabled`
  - `llm.reasoning-search-verification-provider`
- Reasoning search config is provider-specific:
  - `[openai]` for OpenAI
  - `[openrouter]` for OpenRouter
  - `[ollama]` for Ollama
  - `[xai]` for xAI
  - `[zai]` for Z.AI
- Verification model settings are also provider-specific:
  - `<provider>.reasoning-search-verification-model`
  - `<provider>.reasoning-search-verification-effort`
  - `<provider>.reasoning-search-verification-max-output-tokens`
- OpenRouter supports configurable provider routing and `openrouter.enforced-tool-use-iterations` (default `1`).
- xAI Grok 4 fast reasoning models do not support `reasoning_effort`; keep xAI effort config empty.
- Ollama supports `ollama.num-ctx`; `ollama.temperature` and `ollama.structured-output-prompt-schema` are optional.
- Pricing for cost estimation is configured per provider with `[[<provider>.pricing]]`.
- Shared pricing/cost helpers live in `search/LLM/llm_pricing.go`.

## Implemented Tools
- `query_source_ai`
  Path: `search/LLM/tools/ai_tools.go`
  Input: `source_id` (required), `query` (required), optional `language`, optional `max_chunks`
  Behavior: resolve a public source document file UID, fetch text via `Doc2Text`, cache it in memory, then use the configured cheaper AI reader model to return semantically relevant chunk ranges and excerpts.

- `query_transcript_ai`
  Path: `search/LLM/tools/ai_tools.go`
  Input: `content_unit_id` (required), `query` (required), optional `language`, optional `max_chunks`
  Behavior: resolve a public transcript doc/docx file UID, fetch text via `Doc2Text`, cache it in memory, then use the configured cheaper AI reader model to return semantically relevant chunk ranges and excerpts.

- PostgreSQL list tools
  Path: `search/LLM/tools/postgresql_tools.go`
  `get_available_books`: return top-level books under authors
  `get_sources_by_author`: return sources by `author_id` (author code or MDB id)
  `get_sources_by_source`: return direct child sources by `source_id` (source UID or MDB id)
  `get_collections`: return public collections, optionally filtered by `collection_id`, `content_type`, or text query
  `get_content_units_by_collection`: return public content units for a `collection_id`

- `elasticsearch_search`
  Path: `search/LLM/tools/elasticsearch_search_tool.go`
  Input: `query` (optional if filters are provided), `filters`, `language`, `sort_by`, `from`, `size`, `exact_phrase`
  Behavior: normalize filters, build `search.Query`, run `search.ESEngine.DoSearch`, return JSON.

## Coding Notes
- Prefer explicit errors over silent failures.
- Keep tool argument schemas strict (`additionalProperties: false` when possible).
- Reuse existing business logic patterns (security/published checks, MDB lookups) before introducing new query paths.
