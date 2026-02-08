# Claude Agent SDK for Go

Pure Go implementation of the Claude Agent SDK. Calls the Anthropic API directly via [`anthropic-sdk-go`](https://github.com/anthropics/anthropic-sdk-go) — no Claude Code binary dependency.

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
Agent (stateless)          Client (stateful)
  ├── ToolRegistry           ├── Agent
  ├── agentOptions           ├── Session (messages + metadata)
  └── Run() → AgentStream    └── Query() → AgentStream

Internal:
  internal/agent/    → RunLoop (core engine) + CompactAwareStreamer
  internal/budget/   → BudgetTracker (decimal.Decimal pricing)
  internal/builtin/  → 6 built-in tools
  internal/schema/   → JSON Schema generation from Go structs
```

**Key design decisions:**
- **Agent = stateless, Client = stateful** — Agent holds config, Client holds session
- **No LLM abstraction** — directly uses `anthropic-sdk-go` types
- **`Tool[T]` generics** — type-safe inputs with auto schema generation
- **`decimal.Decimal` for costs** — no float64 for money

## Development

```bash
# Run all tests (122 tests)
go test ./...

# Run with verbose output
go test -v ./...

# Run integration tests (requires API key)
ANTHROPIC_API_KEY=sk-xxx go test -v -run Integration ./...

# Build check
go build ./...

# Vet
go vet ./...
```

## License

MIT
