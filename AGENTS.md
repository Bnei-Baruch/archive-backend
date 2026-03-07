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

## Tooling Infrastructure
- Generic tool abstractions live in `search/LLM/reasoning_tools.go`.
- Register tools with `ReasoningToolManager`.
- Pass `manager.ToolCalls()` and `manager.ToolHandlers()` into `GetReasoningResponseWithTools`.

## Implemented Tools
- `source_lookup` in `search/LLM/source_lookup_tool.go`.
- Input: `source_id` (required), `language` (optional).
- Behavior: find public source doc/docx file UID, fetch text via `Doc2Text`, cache in memory, return text.
- `transcript_lookup` in `search/LLM/transcript_lookup_tool.go`.
- Input: `content_unit_id` (required), `language` (optional).
- Behavior: find transcript doc/docx for the content unit, fetch text via `Doc2Text`, cache in memory, return text.

## Coding Notes
- Prefer explicit errors over silent failures.
- Keep tool argument schemas strict (`additionalProperties: false` when possible).
- Reuse existing business logic patterns (security/published checks, MDB lookups) before introducing new query paths.
