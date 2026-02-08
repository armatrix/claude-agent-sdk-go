# CLAUDE.md — claude-agent-sdk-go

## Project Overview

Pure Go implementation of the Claude Agent SDK. No Claude Code binary dependency — directly calls Anthropic API via `anthropic-sdk-go`.

## Module

`github.com/armatrix/claude-agent-sdk-go`

## Architecture

```
Public API (root package):
  agent.go      → Agent (stateless execution engine)
  client.go     → Client (stateful session container)
  option.go     → Functional Options (WithXxx)
  event.go      → Event types for streaming
  stream.go     → AgentStream iterator
  tool.go       → Tool[T] generic interface + ToolRegistry
  defaults.go   → Configurable constants

Internal implementation:
  internal/agent/     → Agent loop, compaction
  internal/budget/    → BudgetTracker with decimal.Decimal
  internal/builtin/   → Built-in tools (Read, Write, Edit, Bash, Glob, Grep)
  internal/hooks/     → Hook engine (Phase 2)
  internal/session/   → Session store implementations (Phase 2)
  internal/teams/     → Agent Teams (Phase 4)
  internal/mcp/       → MCP client (Phase 3)
  internal/permission/ → Permission system (Phase 2)
  internal/config/    → Settings/Skills loading (Phase 2)
  internal/schema/    → JSON Schema generation from Go structs
```

## Key Design Decisions

1. **No LLM abstraction layer** — directly use `anthropic-sdk-go` types (anthropic.MessageParam, etc.)
2. **Tool[T] generic interface** — type-safe inputs, auto schema generation, type-erased wrapper in registry
3. **Agent = stateless, Client = stateful** — Agent holds config, Client holds session
4. **decimal.Decimal for costs** — never float64 for money/budget
5. **Server-side compaction first** — use API `context_management.edits`, client-side as fallback
6. **internal/ for implementation** — root package only exposes user-facing API

## Coding Style

- Go idiomatic: small interfaces, table-driven tests, error wrapping
- Functional Options pattern for configuration
- Iterator pattern for streaming (Next/Current/Err)
- No global state — all dependencies injected via options or constructors
- Test files colocated with implementation

## Architecture Doc

Full design: `/Users/aaron/PKM/10-Projects/Claude-Agent-Go/26-02-08-Claude-Agent-SDK-Go-架构设计.md`

## Phase 1 Plan

See: `docs/plans/2026-02-08-phase1-minimal-agent.md`

## Commands

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run integration tests (requires API key)
ANTHROPIC_API_KEY=sk-xxx go test -v -run Integration ./...

# Build check
go build ./...

# Vet
go vet ./...
```
