# CLAUDE.md — claude-agent-sdk-go

## Project Overview

Pure Go implementation of the Claude Agent SDK. No Claude Code binary dependency — directly calls Anthropic API via `anthropic-sdk-go`.

## Module

`github.com/armatrix/claude-agent-sdk-go`

## Architecture

Flat sub-package layout. Root defines interfaces, sub-packages provide implementations.
Dependency flows one-way: sub-package → root. Full design: `docs/plans/2026-02-08-directory-restructure.md`

```
Root package (package agent):
  agent.go        → Agent (stateless execution engine)
  client.go       → Client (stateful session container)
  session.go      → Session struct + SessionStore interface
  session_ext.go  → Clone, SessionLister, SessionForker, FullSessionStore
  stream.go       → AgentStream iterator
  event.go        → Event types for streaming
  tool.go         → Tool[T] generic interface + ToolRegistry
  output.go       → OutputFormat, structured output extraction
  option.go       → Functional Options (WithXxx)
  defaults.go     → Configurable constants
  errors.go       → Sentinel errors

Public sub-packages (user-facing):
  hook/             → Hook types + 16 event constants
  permission/       → Permission types + checker + declarative rules
  session/          → SessionStore implementations (FileStore, MemoryStore)
  tools/            → Built-in tools (Read, Write, Edit, Bash, Glob, Grep, ...)
                      + Task tool (subagent), MCP resource tools, 7 Teams tools
                      + ToolSearch meta-tool
  subagent/         → Parent→child agent delegation (Definition, Runner)
  mcp/              → MCP client (Transport, Manager, Bridge, Resource, SDKServer)
  teams/            → Multi-agent collaboration with 6 topology modes
  checkpoint/       → File change tracking + rewind
  plugin/           → Plugin loading (commands, agents, MCP servers, skills)

Internal packages (hidden from external consumers):
  internal/engine/      → Agent loop, compaction, fallback retry (core execution)
  internal/budget/      → BudgetTracker with decimal.Decimal
  internal/hookrunner/  → Hook execution engine
  internal/schema/      → JSON Schema generation from Go structs
  internal/config/      → Settings, Skills, Presets, Slash Commands loading
  internal/testutil/    → Shared test helpers

  examples/ → Example programs
```

## Key Design Decisions

1. **No LLM abstraction layer** — directly use `anthropic-sdk-go` types (anthropic.MessageParam, etc.)
2. **Tool[T] generic interface** — type-safe inputs, auto schema generation, type-erased wrapper in registry
3. **Agent = stateless, Client = stateful** — Agent holds config, Client holds session
4. **decimal.Decimal for costs** — never float64 for money/budget
5. **Server-side compaction first** — use API `context_management.edits`, client-side as fallback
6. **Flat sub-packages** — public sub-packages (`session/`, `tools/`, `hook/`, `permission/`) for user-extensible features; `internal/` for implementation details. Go's module-scoped `internal/` visibility allows public sub-packages to import `internal/` within the same module.
7. **One-way dependency** — sub-packages → root (never circular). Root defines interfaces, sub-packages implement them. User wires via explicit composition (no root→sub-package imports).
8. **6 Team Topologies** — LeaderTeammate (default), Pipeline, PeerRing, SupervisorTree, Blackboard, MapReduce. All implement `teams.Topology` interface.
9. **MCP Transport abstraction** — Pluggable Transport interface (StdioTransport, HTTPTransport). Manager bridges MCP tools into agent's ToolRegistry via `mcp.RegisterBridgedTools()`.
10. **Subagent RunFunc** — Testable abstraction for child agent execution. Runner.Spawn creates goroutine-isolated child agents.

## Coding Style

- Go idiomatic: small interfaces, table-driven tests, error wrapping
- Functional Options pattern for configuration
- Iterator pattern for streaming (Next/Current/Err)
- No global state — all dependencies injected via options or constructors
- Test files colocated with implementation

## Architecture Doc

Full design: `/Users/aaron/PKM/10-Projects/Claude-Agent-Go/26-02-08-Claude-Agent-SDK-Go-架构设计.md`

## Plans

- Phase 1 (Minimal Agent): `docs/plans/2026-02-08-phase1-minimal-agent.md`
- Directory Restructure: `docs/plans/2026-02-08-directory-restructure.md`
- Phase 3+4 (Subagents, MCP, Teams): `docs/plans/2026-02-08-phase3-4-subagents-mcp-teams.md`

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
