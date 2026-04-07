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
- Provider selection is driven by `[llm].provider`.
- OpenAI reasoning + tools flow uses `v1/responses` with iteration via `previous_response_id`.
- OpenRouter also uses `v1/responses`, but continues sessions by replaying full message history instead of `previous_response_id`.
- Ollama uses `/api/chat` with message-history replay; it does not use `tool_choice`.
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
- The API `session_id` is a workflow session id owned by the backend, not a provider-native LLM session id.
- Response includes `session_id`, `used_tools`, token stats, and debug/cost details when `deb=true`.
- The backend currently uses one workflow stage (`reasoning`), which stores the provider-native session id internally so future stages/providers can be added without changing the API session format.
- OpenAI short-lived reasoning sessions are stored in memory only, with TTL from `openai.reasoning-session-ttl`.
- OpenRouter and Ollama sessions also live in memory, but store full replayable conversation history via `chat_reasoning_sessions.go`.
- If client sends a missing or expired `session_id`, the API returns an error; it does not silently start a new session.
- OpenAI session state stores continuation data (`last_response_id`, model, effort), not the full prompt or hidden reasoning.

## LLM Config
- Provider selection uses `[llm].provider`; default is `openai`.
- Reasoning search config is provider-specific:
  - `[openai]` for OpenAI
  - `[openrouter]` for OpenRouter
  - `[ollama]` for Ollama
- OpenRouter supports configurable provider routing and `openrouter.enforced-tool-use-iterations` (default `1`).
- Ollama supports `ollama.num-ctx`; `ollama.temperature` and `ollama.structured-output-prompt-schema` are optional.
- Pricing for cost estimation is configured per provider with `[[<provider>.pricing]]`.

## Implemented Tools
- `source_lookup`
  Path: `search/LLM/tools/source_lookup_tool.go`
  Input: `source_id` (required), `language` (optional), plus optional `query`, `chunk_number`
  Behavior: find public source doc/docx file UID, fetch text via `Doc2Text`, cache full text in memory, and use a 3-tier policy
  Small sources return full text; medium sources return full text by default but support targeted chunk retrieval; very large sources use chunk mode. Query mode returns the top matching chunks, and `chunk_number` returns the requested chunk plus one neighboring chunk on each side. Chunk retrieval for the same source is limited per reasoning request.

- `transcript_lookup`
  Path: `search/LLM/tools/transcript_lookup_tool.go`
  Input: `content_unit_id` (required), `language` (optional)
  Behavior: find transcript doc/docx for the content unit, fetch text via `Doc2Text`, cache in memory, return text.

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
