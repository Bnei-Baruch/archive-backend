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
- Main external services: Postgres (`[mdb]`), Elasticsearch (`[elasticsearch]`), NATS (`[nats]`), assets/doc2text (`[assets_service]` / unzip URL), OpenAI (`[openai]`).
- Config: `config.toml` (see `config.sample.toml`).

## LLM Architecture
- Use the provider-agnostic `llm.Service` interface in `search/LLM/service.go`.
- `OpenAIService` is the current implementation.
- Reasoning + tools flow uses OpenAI `v1/responses` with iteration via `previous_response_id`.
- `common.Init()` builds an app-scoped `common.LLM_SERVICE`; do not construct a new LLM service per request.
- Tool implementations live under `search/LLM/tools/`.
- LLM package tests live under `search/LLM/tests/`.

## Tooling Infrastructure
- Generic tool abstractions live in `search/LLM/reasoning_tools.go`.
- Register tools with `ReasoningToolManager`.
- Pass `manager.ToolCalls()` and `manager.ToolHandlers()` into `GetReasoningResponseWithTools`.
- App-scoped manager builder lives in `search/LLM/tools/manager.go`.
- `common.Init()` builds the shared tool manager and exposes it as `common.LLM_TOOLS`.

## Reasoning Search
- API endpoint: `POST /search/reasoning`.
- Request supports `q`, optional `deb`, optional `session_id`.
- Response includes `session_id`, `used_tools`, token stats, and debug/cost details when `deb=true`.
- OpenAI short-lived reasoning sessions are stored in memory only, with TTL from `openai.reasoning-session-ttl`.
- If client sends a missing or expired `session_id`, the API returns an error; it does not silently start a new session.
- Session state stores OpenAI continuation data (`last_response_id`, model, effort), not the full prompt or hidden reasoning.

## OpenAI Config
- Provider selection uses `[llm].provider`; default is `openai`.
- Reasoning search config is read from `[openai]` only when provider is `openai`.
- Pricing for cost estimation is configured with `[[openai.pricing]]`.

## Implemented Tools
- `source_lookup` in `search/LLM/tools/source_lookup_tool.go`.
- Input: `source_id` (required), `language` (optional).
- Behavior: find public source doc/docx file UID, fetch text via `Doc2Text`, cache in memory, return text.
- `transcript_lookup` in `search/LLM/tools/transcript_lookup_tool.go`.
- Input: `content_unit_id` (required), `language` (optional).
- Behavior: find transcript doc/docx for the content unit, fetch text via `Doc2Text`, cache in memory, return text.
- PostgreSQL list tools in `search/LLM/tools/postgresql_tools.go`.
- `get_sources_by_author`: return sources by `author_id` (author code or MDB id).
- `get_collections`: return public collections, optionally filtered by `collection_id`, `content_type`, or text query.
- `get_content_units_by_collection`: return public content units for a `collection_id`.
- Cached tools in `search/LLM/tools/cached_tools.go`.
- `get_source_filter_values`: takes no arguments and returns the cached list of all valid `source` filter values.
- `elasticsearch_search` in `search/LLM/tools/elasticsearch_search_tool.go`.
- Input: `query` (optional if filters are provided), `filters`, `language`, `sort_by`, `from`, `size`, `exact_phrase`.
- Behavior: normalize filters, build `search.Query`, run `search.ESEngine.DoSearch`, return JSON.

## Coding Notes
- Prefer explicit errors over silent failures.
- Keep tool argument schemas strict (`additionalProperties: false` when possible).
- Reuse existing business logic patterns (security/published checks, MDB lookups) before introducing new query paths.
