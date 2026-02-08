# Claude Agent SDK for Go

**Pure Go Agent SDK with pluggable team topologies** — no subprocess, no runtime dependency.
Build single agents or orchestrate multi-agent teams with composable topology primitives and native goroutine concurrency.

[![Go Reference](https://pkg.go.dev/badge/github.com/armatrix/claude-agent-sdk-go.svg)](https://pkg.go.dev/github.com/armatrix/claude-agent-sdk-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/armatrix/claude-agent-sdk-go)](https://goreportcard.com/report/github.com/armatrix/claude-agent-sdk-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Why This SDK?

|  | Official TS/Python SDK | This SDK (Go) |
|--|----------------------|---------------|
| Runtime | Spawns Claude Code subprocess | Pure Go binary, zero dependency |
| Agent Teams | Leader-only topology | **Pluggable topologies** — 6 built-in, compose your own |
| Concurrency | Process isolation, JSON stdio | goroutine + channel, sub-microsecond messaging |
| Embedding | Requires Node.js/Python runtime | Single binary, embed in any Go project |
| Long sessions | Bound to subprocess lifetime | In-process, unlimited duration |

## Install

```bash
go get github.com/armatrix/claude-agent-sdk-go@latest
```

Requires Go 1.23+ and an `ANTHROPIC_API_KEY` environment variable.

## Quick Start

### Single-shot Agent

```go
package main

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/internal/builtin"
)

func main() {
	a := agent.NewAgent(
		agent.WithModel(anthropic.ModelClaudeSonnet4_5_20250929),
		agent.WithMaxTurns(10),
	)

	// Register built-in tools (Read, Write, Edit, Bash, Glob, Grep)
	builtin.RegisterAll(a.Tools())

	stream := a.Run(context.Background(), "Read go.mod and tell me the module name.")

	for stream.Next() {
		switch e := stream.Current().(type) {
		case *agent.StreamEvent:
			fmt.Print(e.Delta)
		case *agent.ResultEvent:
			fmt.Printf("\n[%s] %d turns, %d input tokens, %d output tokens\n",
				e.Subtype, e.NumTurns, e.Usage.InputTokens, e.Usage.OutputTokens)
		}
	}
	if err := stream.Err(); err != nil {
		panic(err)
	}
}
```

### Multi-turn Client

```go
client := agent.NewClient(
	agent.WithModel(anthropic.ModelClaudeSonnet4_5_20250929),
	agent.WithMaxTurns(5),
)
defer client.Close()

ctx := context.Background()

// First turn
stream1 := client.Query(ctx, "Remember: the secret number is 42.")
for stream1.Next() { /* drain */ }

// Second turn — session history preserved
stream2 := client.Query(ctx, "What was the secret number?")
for stream2.Next() {
	if e, ok := stream2.Current().(*agent.StreamEvent); ok {
		fmt.Print(e.Delta) // prints "42"
	}
}
```

## Team Topologies

The SDK ships with 6 built-in topology templates. Each is a composable building block — mix them, chain them, or build your own.

```
Leader          Pipeline        Peer Ring       Supervisor Tree   Blackboard        MapReduce

   L            A → B → C       A ↔ B              S             A → ┌─────┐ ← B   Dispatcher
  /|\                           ↕   ↕             / \            C → │Board│ ← D    / | \
 T T T                          D ↔ C           S     W              └─────┘       W  W  W
                                               / \                                  \ | /
                                              W   W                                 Merger
```

| Topology | Pattern | Best For |
|----------|---------|----------|
| **Leader** | Star (hub-spoke) | General dev tasks, 3-6 agents |
| **Pipeline** | A → B → C → D | Review chains, data processing, content production |
| **Peer Ring** | Full-mesh | Multi-perspective review, debate, ensemble voting |
| **Supervisor Tree** | Hierarchical | Large teams (10+), fault isolation, modular projects |
| **Blackboard** | Shared-state | Expert collaboration, incident analysis, loose coupling |
| **MapReduce** | Fan-out → Fan-in | Batch migration, parallel testing, multi-repo operations |

### Build Your Own

Topologies are composable primitives, not a closed set. Combine a Pipeline with MapReduce for parallel review stages, or nest a Peer Ring inside a Supervisor Tree node.

```go
// Example: Pipeline + MapReduce hybrid
// Stage 1: Dispatcher splits tasks
// Stage 2: N workers process in parallel (MapReduce)
// Stage 3: Reviewer validates merged output (Pipeline continues)
```

> **Status**: Team topologies are under active development (Phase 4). The single-agent core and built-in tools are production-ready. See [Roadmap](#roadmap) for details.

## Custom Tools

Define a tool by implementing `Tool[T]` with a typed input struct. JSON Schema is auto-generated from struct tags.

```go
type WeatherInput struct {
	City string `json:"city" jsonschema:"required,description=City name"`
}

type WeatherTool struct{}

func (t *WeatherTool) Name() string        { return "get_weather" }
func (t *WeatherTool) Description() string { return "Get current weather for a city" }

func (t *WeatherTool) Execute(ctx context.Context, input WeatherInput) (*agent.ToolResult, error) {
	// Your logic here
	return agent.TextResult(fmt.Sprintf("72°F in %s", input.City)), nil
}

// Register
agent.RegisterTool(a.Tools(), &WeatherTool{})
```

## Configuration

All options use the functional options pattern:

```go
a := agent.NewAgent(
	agent.WithModel(anthropic.ModelClaudeOpus4_6), // Default: claude-opus-4-6
	agent.WithMaxOutputTokens(128_000),            // Default: 16,384
	agent.WithMaxTurns(20),                        // Default: 0 (unlimited)
	agent.WithContextWindow(agent.ContextWindow1M), // Default: 200,000
	agent.WithCompactTrigger(100_000),             // Default: 150,000
	agent.WithCompactDisabled(),                   // Disable auto-compaction
	agent.WithBudget(decimal.NewFromFloat(1.00)),  // Max $1.00 per run
)
```

| Option | Default | Description |
|--------|---------|-------------|
| `WithModel` | `anthropic.ModelClaudeOpus4_6` | Claude model (use SDK constants) |
| `WithMaxOutputTokens` | `16,384` | Max tokens per response |
| `WithMaxTurns` | `0` (unlimited) | Max agent loop iterations |
| `WithContextWindow` | `200,000` | Context window size |
| `WithCompactTrigger` | `150,000` | Token count to trigger compaction |
| `WithCompactDisabled` | enabled | Disable context compaction |
| `WithBudget` | `0` (unlimited) | Max budget in USD (decimal) |

## Event Types

The `AgentStream` emits typed events via an iterator pattern:

| Event | When | Key Fields |
|-------|------|------------|
| `SystemEvent` | Run start | `SessionID`, `Model` |
| `StreamEvent` | Text delta arrives | `Delta` |
| `AssistantEvent` | Complete LLM response | `Message` (full `anthropic.Message`) |
| `CompactEvent` | Context compacted | `Strategy`, `TokensBefore`, `TokensAfter` |
| `ResultEvent` | Run end | `Subtype`, `Usage`, `NumTurns`, `DurationMs`, `Errors` |

```go
stream := a.Run(ctx, "Hello")
for stream.Next() {
	switch e := stream.Current().(type) {
	case *agent.SystemEvent:
		fmt.Println("Session:", e.SessionID)
	case *agent.StreamEvent:
		fmt.Print(e.Delta)
	case *agent.ResultEvent:
		if e.IsError {
			fmt.Println("Error:", e.Errors)
		}
	}
}
```

## Built-in Tools

Register with `builtin.RegisterAll(a.Tools())`:

| Tool | Description |
|------|-------------|
| `Read` | Read files with line numbers, offset/limit support |
| `Write` | Write files, auto-creates parent directories |
| `Edit` | Find-and-replace with uniqueness check |
| `Bash` | Execute shell commands with timeout and PTY |
| `Glob` | Match file patterns (doublestar syntax) |
| `Grep` | Search file contents via `rg` (ripgrep) |

## Context Compaction

Server-side compaction is enabled by default. When context approaches the trigger threshold, the API automatically compacts the conversation history.

```go
// Custom compaction config
a := agent.NewAgent(
	agent.WithCompaction(agent.CompactConfig{
		Strategy:      agent.CompactServer,
		TriggerTokens: 100_000,
		Instructions:  "Preserve all code snippets and file paths",
	}),
)
```

## Architecture

```
Agent (stateless, shareable)     Client (stateful, per-session)
  ├── ToolRegistry                 ├── Agent (reuses)
  ├── agentOptions                 ├── Session (messages + metadata)
  └── Run() → AgentStream          └── Query() → AgentStream

Teams (pluggable topologies):
  teams/topology/  → Topology interface + 6 built-in templates
  teams/message/   → MessageBus (channel fan-out)
  teams/task/      → SharedTaskList (atomic claim)

Internal:
  internal/engine/  → RunLoop (core engine) + CompactAwareStreamer
  internal/budget/  → BudgetTracker (decimal.Decimal pricing)
  internal/schema/  → JSON Schema generation from Go structs
```

**Key design decisions:**
- **Agent = stateless, Client = stateful** — Agent holds config, Client holds session
- **No LLM abstraction** — directly uses `anthropic-sdk-go` types
- **`Tool[T]` generics** — type-safe inputs with auto schema generation
- **`decimal.Decimal` for costs** — no float64 for money
- **Pluggable topologies** — `Topology` interface, compose built-in or bring your own

## Roadmap

| Phase | Status | Scope |
|-------|--------|-------|
| **Phase 1** | **Done** | Single agent, built-in tools, streaming, compaction, budget |
| **Phase 2** | In Progress | Hooks, permissions, session persistence, Client |
| **Phase 3** | Planned | Subagents, MCP integration, plugins |
| **Phase 4** | Planned | Agent Teams with pluggable topologies |

## Contributing

Contributions welcome. Please open an issue first for non-trivial changes.

## License

MIT
