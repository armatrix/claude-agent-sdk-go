# Directory Restructure — Flat Sub-Packages

> **Status:** Approved, pending implementation
> **Date:** 2026-02-08
> **Prerequisite:** Phase 1 complete (core agent loop works)

## Motivation

Phase 1 placed all implementation in `internal/` and public API in root. As the SDK grows, this creates:

1. **Naming collisions** — `internal/agent/` vs root `package agent` requires awkward `internalagent` import alias
2. **Duplicate code** — `file_store.go`/`memory_store.go` at root duplicate `internal/session/` implementations
3. **Flat internal/** — all future features (hooks, permissions, session, tools) pile into internal/ with no public extensibility

## Design Principles

1. **Go `internal/` is module-scoped** — packages within this module CAN import `internal/`. Only external consumers are blocked. So `tools/` (public) can import `internal/schema/` without making schema public.
2. **One-way dependency** — sub-packages import root (`package agent`), never the reverse. User wires implementations via functional options.
3. **Happy path imports** — 80% of users need 1-2 imports:

| Usage Level | Import Count | Packages |
|-------------|-------------|----------|
| Basic | 1 | `agent` |
| With tools | 2 | `agent`, `tools` |
| With persistence | 3 | + `session` |
| With hooks | 4 | + `hook` |
| Power user | 5 | + `permission` |

## Target Directory Structure

```
claude-agent-sdk-go/
│
│  ── Root package: `package agent` ──────────────────────
│  doc.go              → Package documentation (pkg.go.dev)
│  agent.go            → Agent (stateless execution engine)
│  client.go           → Client (stateful session container)
│  session.go          → Session struct + SessionStore interface
│  session_ext.go      → Clone, SessionLister, SessionForker, FullSessionStore
│  stream.go           → AgentStream iterator
│  event.go            → Event types for streaming
│  tool.go             → Tool[T] generic interface + ToolRegistry
│  output.go           → OutputFormat, structured output extraction
│  option.go           → Functional Options (WithXxx)
│  defaults.go         → Configurable constants
│  errors.go           → Sentinel errors (NEW)
│
│  ── Public sub-packages (user-facing) ──────────────────
├── hook/              → Hook types + event constants
│   └── hook.go           package hook
│
├── permission/        → Permission types + checker
│   └── permission.go     package permission
│
├── session/           → SessionStore implementations (PROMOTED from internal/session/)
│   ├── doc.go
│   ├── file.go           FileStore (disk persistence)
│   └── memory.go         MemoryStore (in-memory)
│
├── tools/             → Built-in tools (PROMOTED from internal/builtin/)
│   ├── doc.go
│   ├── register.go       RegisterAll()
│   ├── read.go, write.go, edit.go
│   ├── bash.go, glob.go, grep.go
│   ├── webfetch.go, websearch.go
│   ├── notebook.go, ask.go, planmode.go, todo.go
│   ├── configurable.go   Configurable tool base
│   └── *_test.go
│
│  ── Internal packages (implementation detail) ──────────
├── internal/
│   ├── engine/        → Agent loop + compaction (RENAMED from internal/agent/)
│   │   ├── loop.go
│   │   ├── compact.go
│   │   └── *_test.go
│   │
│   ├── budget/        → BudgetTracker (decimal.Decimal)
│   │   ├── tracker.go
│   │   └── pricing.go
│   │
│   ├── hookrunner/    → Hook execution engine (RENAMED from internal/hook/)
│   │   └── runner.go
│   │
│   ├── schema/        → JSON Schema generation (stays internal)
│   │   ├── schema.go
│   │   └── schema_test.go
│   │
│   ├── config/        → Settings/Skills loading (Phase 2)
│   │
│   └── testutil/      → Shared test helpers (NEW)
│       └── session.go    NewTestSession(), NewTestRegistry()
│
│  ── Future packages ────────────────────────────────────
├── teams/             → Agent Teams (Phase 4)
├── mcp/               → MCP client (Phase 3)
│
│  ── Non-Go ─────────────────────────────────────────────
├── examples/          → Example programs (NEW)
│   ├── basic/            main.go — minimal agent run
│   ├── tools/            main.go — with built-in tools
│   └── session/          main.go — with persistence
│
└── docs/
    ├── plans/
    └── architecture.hierarchy.excalidraw
```

## Architecture Layer Mapping

```
┌──────────────────────────────────────────────────────┐
│  User Code                                           │
│  agent.Run() / client.Query()                        │
│  imports: agent, tools, session                      │
├──────────────────────────────────────────────────────┤
│  Layer 3: Agent Teams       → teams/     (Phase 4)   │
├──────────────────────────────────────────────────────┤
│  Layer 2: SDK Framework                              │
│  ┌─────────────────────────────────────────────────┐ │
│  │ Public API (root)     │ Internal Implementation │ │
│  │ agent.go, client.go   │ internal/engine/        │ │
│  │ session.go, tool.go   │ internal/budget/        │ │
│  │ hook/                 │ internal/hookrunner/    │ │
│  │ permission/           │ internal/config/        │ │
│  │ session/              │ internal/schema/        │ │
│  └─────────────────────────────────────────────────┘ │
├──────────────────────────────────────────────────────┤
│  Layer 1: Built-in Tools → tools/                    │
├──────────────────────────────────────────────────────┤
│  API: anthropic-sdk-go → Anthropic API               │
└──────────────────────────────────────────────────────┘
```

## Migration Steps

### Step 1: Delete root duplicates

- Delete `file_store.go` (duplicate of `internal/session/file.go`)
- Delete `memory_store.go` (duplicate of `internal/session/memory.go`)

### Step 2: Promote internal packages to public

- Move `internal/session/` → `session/`
- Move `internal/builtin/` → `tools/` (rename package from `builtin` to `tools`)

### Step 3: Rename internal packages

- Rename `internal/agent/` → `internal/engine/` (package `engine`)
- Rename `internal/hook/` → `internal/hookrunner/` (package `hookrunner`)

### Step 4: Create new files

- `errors.go` — sentinel errors (`ErrBudgetExhausted`, `ErrMaxTurns`, `ErrContextOverflow`, etc.)
- `doc.go` — root package doc
- `session/doc.go`, `tools/doc.go` — sub-package docs
- `internal/testutil/session.go` — shared test helpers

### Step 5: Update all imports

- `internal/agent` → `internal/engine` (in `agent.go`, tests)
- `internal/builtin` → `tools` (in `agent.go`, `integration_test.go`)
- `internal/hook` → `internal/hookrunner` (in `agent.go`)
- `internal/session` → `session` (in `integration_test.go`)
- Root `NewMemoryStore`/`NewFileStore` → `session.NewMemoryStore`/`session.NewFileStore`
- Root `builtin.RegisterAll` → `tools.RegisterAll`

### Step 6: Verify

```bash
go build ./...
go test ./...
go vet ./...
```

## Files to Delete

| File | Reason |
|------|--------|
| `file_store.go` (root) | Exact duplicate of `internal/session/file.go` |
| `memory_store.go` (root) | Exact duplicate of `internal/session/memory.go` |

## Key Insight: `internal/` Visibility

Go's `internal/` restriction is **module-scoped**, not package-scoped. Any package within `github.com/armatrix/claude-agent-sdk-go` can import `internal/schema/`, `internal/engine/`, etc. Only external consumers (other Go modules) are blocked.

This means:
- `tools/` (public) can import `internal/schema/` for JSON schema generation — no need to make `schema/` public
- `session/` (public) can import root `agent` package for interfaces — standard Go pattern
- External users see clean public API; internal wiring is hidden
