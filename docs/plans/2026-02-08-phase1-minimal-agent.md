# Phase 1: Minimal Viable Agent — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a single Agent that can receive a prompt, call tools (Read/Write/Edit/Bash/Glob/Grep), stream output, track budget, and auto-compact — using `anthropic-sdk-go` directly with no LLM abstraction layer.

**Architecture:** Agent is a stateless execution engine holding config + tool registry + hooks. Client wraps Agent with session state for multi-turn conversations. Tool interface uses Go generics with type-erased wrapper for registry storage. Server-side compaction via API `context_management.edits` parameter.

**Tech Stack:** Go 1.23+, `anthropic-sdk-go` v1.22+, `shopspring/decimal`, `invopop/jsonschema`, `bmatcuk/doublestar/v4`, `creack/pty`

**Module:** `github.com/armatrix/claude-agent-sdk-go`

**Architecture Doc:** `/Users/aaron/PKM/10-Projects/Claude-Agent-Go/26-02-08-Claude-Agent-SDK-Go-架构设计.md`

---

## Task Dependency Graph

```
Task 1: Core Types & Options
    ↓
Task 2: Tool System (generic interface + registry)
    ↓
Task 3: Agent Loop + Stream ←── Task 4: Budget Tracker
    ↓
Task 5: Built-in Tools (Read, Write, Edit)
    ↓
Task 6: Built-in Tools (Bash, Glob, Grep)
    ↓
Task 7: Server-side Compaction
    ↓
Task 8: Integration Test (full agent run)
```

---

## Task 1: Core Types & Functional Options

> Foundation: public API surface — types, options, events.

**Files:**
- Create: `agent.go` — Agent struct, NewAgent(), Run(), RunWithSession()
- Create: `client.go` — Client struct, NewClient(), Query()
- Create: `option.go` — AgentOption type + all WithXxx() functions
- Create: `event.go` — Event interface, EventType enum, ResultEvent
- Create: `stream.go` — AgentStream iterator (Next/Current/Err)
- Create: `defaults.go` — DefaultModel, DefaultContextWindow, DefaultMaxOutputTokens, etc.
- Create: `go.sum` (via go mod tidy)
- Test: `option_test.go`, `stream_test.go`

**Acceptance:**
- `NewAgent(WithModel("claude-opus-4-6"))` compiles and returns configured Agent
- `AgentStream` iterator pattern compiles with Next()/Current()/Err()
- All default constants defined with correct values from architecture doc §8

**Key types (from architecture doc §4.1, §4.2, §8):**

```go
// defaults.go
const (
    DefaultModel                = "claude-opus-4-6"
    DefaultContextWindow        = 200_000
    ContextWindow1M             = 1_000_000
    DefaultMaxOutputTokens      = 16_384
    MaxOutputTokensOpus46       = 128_000
    DefaultCompactTriggerTokens = 150_000
    MinCompactTriggerTokens     = 50_000
    DefaultMaxTurns             = 0
    DefaultStreamBufferSize     = 64
)

// option.go
type AgentOption func(*agentOptions)
type agentOptions struct {
    model           string
    contextWindow   int
    maxOutputTokens int
    maxTurns        int
    // ...all options from §8.4
}

// event.go
type EventType string
const (
    EventSystem    EventType = "system"
    EventAssistant EventType = "assistant"
    EventUser      EventType = "user"
    EventStream    EventType = "stream"
    EventResult    EventType = "result"
    EventCompact   EventType = "compact"
)

// stream.go — iterator pattern
type AgentStream struct {
    events chan Event
    current Event
    err     error
    done    bool
}
func (s *AgentStream) Next() bool
func (s *AgentStream) Current() Event
func (s *AgentStream) Err() error
```

---

## Task 2: Tool System (Generic Interface + Registry)

> The type-safe tool framework. Generic Tool[T] with type-erased wrapper for registry.

**Files:**
- Create: `tool.go` — public Tool[T] interface, ToolResult, RegisterTool()
- Create: `internal/schema/schema.go` — GenerateSchema[T]() using invopop/jsonschema
- Create: `internal/schema/schema_test.go`
- Test: `tool_test.go`

**Acceptance:**
- Define a tool with typed input struct, register it, retrieve from registry
- Schema auto-generated from Go struct tags matches expected JSON Schema
- Registry returns `[]anthropic.ToolUnionParam` for API calls

**Key design (from review discussion — generics + wrapper):**

```go
// tool.go — public interface
type Tool[T any] interface {
    Name() string
    Description() string
    Execute(ctx context.Context, input T) (*ToolResult, error)
}

type ToolResult struct {
    Content  []anthropic.ContentBlockParamUnion
    IsError  bool
    Metadata map[string]any
}

// Type-erased wrapper for registry storage
type toolEntry struct {
    name        string
    description string
    schema      anthropic.ToolInputSchemaParam
    execute     func(ctx context.Context, raw json.RawMessage) (*ToolResult, error)
}

// RegisterTool registers a generic tool into the registry
func RegisterTool[T any](r *ToolRegistry, tool Tool[T]) {
    schema := schema.Generate[T]()
    r.register(toolEntry{
        name:        tool.Name(),
        description: tool.Description(),
        schema:      schema,
        execute: func(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
            var input T
            if err := json.Unmarshal(raw, &input); err != nil {
                return &ToolResult{IsError: true}, err
            }
            return tool.Execute(ctx, input)
        },
    })
}

// ToolRegistry manages registered tools
type ToolRegistry struct {
    mu    sync.RWMutex
    tools map[string]*toolEntry
}

func (r *ToolRegistry) ListForAPI() []anthropic.ToolUnionParam
func (r *ToolRegistry) Execute(ctx context.Context, name string, input json.RawMessage) (*ToolResult, error)
```

---

## Task 3: Agent Loop + Streaming

> The core execution engine. Sends messages to API, processes tool_use, loops until done.

**Files:**
- Create: `internal/agent/loop.go` — runLoop() core logic
- Create: `internal/agent/loop_test.go` — mock API responses
- Modify: `agent.go` — wire Run()/RunWithSession() to internal loop
- Modify: `stream.go` — wire events from loop to stream

**Acceptance:**
- Agent loop sends prompt → receives response → if tool_use: execute tool → append result → loop
- stop_reason "end_turn" terminates the loop
- stop_reason "max_tokens" terminates with appropriate ResultEvent subtype
- Streaming deltas emitted as EventStream events
- All events flow through AgentStream iterator

**Core loop pseudocode (from architecture doc §6):**

```
func runLoop(ctx, apiClient, session, tools, opts) -> chan Event:
  1. emit EventSystem (init info)
  2. build messages from session
  3. LOOP:
     a. call apiClient.Messages.NewStreaming(params)
     b. accumulate response, emit EventStream for text deltas
     c. on complete response:
        - emit EventAssistant
        - if stop_reason == "end_turn": break
        - if stop_reason == "max_tokens": emit ResultEvent{Subtype: "error_max_turns"}, break
        - for each tool_use block:
            * execute via registry
            * append tool_result to messages
        - if maxTurns > 0 && turns >= maxTurns: break
        - continue LOOP
  4. emit EventResult
```

**Dependencies:** Task 1 (types), Task 2 (tool registry)

---

## Task 4: Budget Tracker

> Track token usage and costs using decimal.Decimal. Runs alongside agent loop.

**Files:**
- Create: `internal/budget/tracker.go` — BudgetTracker, ModelPricing, RecordUsage()
- Create: `internal/budget/tracker_test.go`
- Create: `internal/budget/pricing.go` — DefaultPricing map
- Modify: `agent.go` — wire budget tracking into loop via options

**Acceptance:**
- BudgetTracker.RecordUsage(model, usage) correctly computes cost with decimal
- Long context pricing kicks in when input_tokens > threshold
- Budget.Remaining() returns correct value
- Budget.Exhausted() triggers loop termination
- Compaction iterations tracked separately (RecordIterations)

**Key types (from architecture doc §8.2, §10.2):**

```go
type BudgetTracker struct {
    maxBudget  decimal.Decimal   // 0 = unlimited
    totalCost  decimal.Decimal
    pricing    map[string]ModelPricing
    mu         sync.Mutex
}

func (b *BudgetTracker) RecordUsage(model string, usage anthropic.Usage)
func (b *BudgetTracker) RecordIterations(model string, iters []UsageIteration)
func (b *BudgetTracker) TotalCost() decimal.Decimal
func (b *BudgetTracker) Remaining() decimal.Decimal  // maxBudget - totalCost
func (b *BudgetTracker) Exhausted() bool
```

**No dependency on other tasks** — can be built in parallel with Task 3.

---

## Task 5: Built-in Tools — Read, Write, Edit

> File operation tools. Pure Go, no external deps.

**Files:**
- Create: `internal/builtin/read.go` + `read_test.go`
- Create: `internal/builtin/write.go` + `write_test.go`
- Create: `internal/builtin/edit.go` + `edit_test.go`
- Create: `internal/builtin/register.go` — RegisterBuiltins() helper

**Acceptance:**
- ReadTool: reads file, supports offset/limit, returns content with line numbers
- WriteTool: writes file, creates parent dirs if needed
- EditTool: replaces exact string match, fails if not unique (unless replace_all=true)
- All tools implement Tool[T] generic interface
- All tools have tests covering happy path + error cases

**Input types:**

```go
type ReadInput struct {
    FilePath string `json:"file_path" jsonschema:"required"`
    Offset   *int   `json:"offset,omitempty"`
    Limit    *int   `json:"limit,omitempty"`
}

type WriteInput struct {
    FilePath string `json:"file_path" jsonschema:"required"`
    Content  string `json:"content" jsonschema:"required"`
}

type EditInput struct {
    FilePath   string `json:"file_path" jsonschema:"required"`
    OldString  string `json:"old_string" jsonschema:"required"`
    NewString  string `json:"new_string" jsonschema:"required"`
    ReplaceAll bool   `json:"replace_all,omitempty"`
}
```

**Dependencies:** Task 2 (tool interface)

---

## Task 6: Built-in Tools — Bash, Glob, Grep

> System interaction tools. Bash needs PTY, Glob uses doublestar, Grep calls rg.

**Files:**
- Create: `internal/builtin/bash.go` + `bash_test.go`
- Create: `internal/builtin/glob.go` + `glob_test.go`
- Create: `internal/builtin/grep.go` + `grep_test.go`
- Modify: `internal/builtin/register.go` — add new tools

**Acceptance:**
- BashTool: executes command with timeout, captures stdout/stderr, supports background mode
- GlobTool: matches file patterns using doublestar, returns sorted paths
- GrepTool: invokes `rg` binary with regex, returns matches in specified output_mode
- All implement Tool[T] generic interface
- Bash has PTY support via `creack/pty`

**Input types:**

```go
type BashInput struct {
    Command         string `json:"command" jsonschema:"required"`
    Description     string `json:"description,omitempty"`
    Timeout         *int   `json:"timeout,omitempty"`        // ms, default 120000
    RunInBackground bool   `json:"run_in_background,omitempty"`
}

type GlobInput struct {
    Pattern string `json:"pattern" jsonschema:"required"`
    Path    string `json:"path,omitempty"`
}

type GrepInput struct {
    Pattern    string `json:"pattern" jsonschema:"required"`
    Path       string `json:"path,omitempty"`
    OutputMode string `json:"output_mode,omitempty"`  // content | files_with_matches | count
    Glob       string `json:"glob,omitempty"`
    Type       string `json:"type,omitempty"`
    Context    *int   `json:"context,omitempty"`
}
```

**Dependencies:** Task 2 (tool interface). Can run in parallel with Task 5.

---

## Task 7: Server-side Compaction Integration

> Wire compaction into agent loop — pass context_management.edits to API.

**Files:**
- Create: `internal/agent/compact.go` — compaction logic
- Create: `internal/agent/compact_test.go`
- Modify: `internal/agent/loop.go` — integrate compaction into loop
- Modify: `option.go` — WithCompaction(), WithCompactTrigger(), WithCompactDisabled()

**Acceptance:**
- When CompactStrategy == CompactServer: API request includes `context_management` field
- Compaction block in response is preserved in session messages
- stop_reason "compaction" correctly handled (re-send if PauseAfterCompact)
- EventCompact emitted when compaction occurs
- Budget tracker records compaction iteration costs

**Key integration point:**

```go
// In loop.go, when building MessageNewParams:
if opts.compact.Strategy == CompactServer {
    params.ContextManagement = &anthropic.ContextManagement{
        Edits: []anthropic.ContextManagementEdit{{
            Type: "compact_20260112",
            Trigger: &anthropic.CompactTrigger{
                Type:  "input_tokens",
                Value: opts.compact.TriggerTokens,
            },
        }},
    }
    // Add beta header: "compact-2026-01-12"
}
```

**Dependencies:** Task 3 (agent loop), Task 4 (budget)

---

## Task 8: Integration Test — Full Agent Run

> End-to-end test: create Agent, run with real prompt, verify tool calls and output.

**Files:**
- Create: `integration_test.go` — requires ANTHROPIC_API_KEY env var
- Create: `testutil/mock_api.go` — mock anthropic API for unit tests

**Acceptance:**
- With real API key: `agent.Run(ctx, "Read the file go.mod and tell me the module name")` returns correct module name
- With mock API: full loop with tool_use → tool_result → end_turn works
- Budget tracking shows non-zero cost after run
- AgentStream correctly delivers all event types

**Dependencies:** All previous tasks.

---

## Go Dependencies to Install

```bash
cd ~/ai/claude-agent-sdk-go
go get github.com/anthropics/anthropic-sdk-go@latest
go get github.com/shopspring/decimal@latest
go get github.com/invopop/jsonschema@latest
go get github.com/bmatcuk/doublestar/v4@latest
go get github.com/creack/pty@latest
go get github.com/stretchr/testify@latest
go mod tidy
```
