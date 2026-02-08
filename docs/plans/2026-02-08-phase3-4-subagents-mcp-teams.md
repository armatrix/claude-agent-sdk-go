# Phase 3+4: Subagents, MCP, and Agent Teams — Design Doc

> **Scope:** Merge Phase 3 (Subagents + MCP) and Phase 4 (Agent Teams) into a single implementation phase.
> **Strategy:** PM spec → Foundation interfaces → 3 Dev agents in parallel.

---

## 1. Overview

Three new sub-packages extending the Agent SDK:

| Package | Purpose | Dependency |
|---------|---------|-----------|
| `subagent/` | Parent→child delegation, goroutine-isolated | Foundation types |
| `mcp/` | External tool ecosystem via MCP protocol | Foundation types |
| `teams/` | Multi-agent collaboration with 6 topology modes | Foundation types + subagent concepts |

All three share the existing Agent execution engine. No circular dependencies between packages.

---

## 2. Subagent Module (`subagent/`)

### 2.1 Core Concept

Parent Agent spawns child Agents via `Task` tool. Each child runs in an independent goroutine with its own session and context window. Result returns via channel.

### 2.2 Key Types

```go
// subagent/definition.go
type Definition struct {
    Name         string
    Model        anthropic.Model
    Instructions string            // system prompt override/append
    Tools        []agent.Tool      // nil = inherit parent
    MaxTurns     int
    MaxBudget    decimal.Decimal
}

// subagent/runner.go
type Runner struct {
    parent     *agent.Agent
    defs       map[string]*Definition
    active     map[string]*runHandle
    mu         sync.RWMutex
}

type runHandle struct {
    id     string                  // generateID(PrefixRun)
    ctx    context.Context
    cancel context.CancelFunc
    result chan *Result
}

type Result struct {
    Output  string
    Session *agent.Session
    Usage   agent.Usage
    Cost    decimal.Decimal
    Err     error
}
```

### 2.3 Task Tool

Built-in tool in `tools/task.go`, calls `subagent.Runner`:

```go
type TaskInput struct {
    AgentName string `json:"agent_name" jsonschema:"required"`
    Prompt    string `json:"prompt" jsonschema:"required"`
    MaxTurns  *int   `json:"max_turns,omitempty"`
}
```

### 2.4 Lifecycle

1. Parent calls Task tool → Runner looks up Definition
2. Spawn goroutine, create child Agent (inherit parent config + Definition overrides)
3. Child runs independently with own session/context window
4. On completion, Result sent via channel
5. Parent receives tool_result, continues loop

---

## 3. MCP Module (`mcp/`)

### 3.1 Core Concept

Agent connects to external MCP Servers, discovers their tools, and bridges them into the local ToolRegistry. Agent loop calls MCP tools transparently.

**Implementation:** Thin glue layer over `modelcontextprotocol/go-sdk`. No protocol reimplementation.

### 3.2 Key Types

```go
// mcp/config.go
type ServerConfig struct {
    Command   string            // stdio: startup command
    Args      []string
    Env       map[string]string
    URL       string            // SSE/HTTP: server URL
    Transport TransportType     // "stdio" | "sse" | "streamable-http"
}

type TransportType string
const (
    TransportStdio          TransportType = "stdio"
    TransportSSE            TransportType = "sse"
    TransportStreamableHTTP TransportType = "streamable-http"
)

// mcp/manager.go
type Manager struct {
    servers map[string]*serverConn
    mu      sync.RWMutex
}

type serverConn struct {
    name    string
    config  ServerConfig
    client  *mcpsdk.Client
    tools   []agent.ToolEntry
    ctx     context.Context
    cancel  context.CancelFunc
}

// mcp/bridge.go — MCP tool → agent.Tool bridge
// Naming: mcp__{server}__{tool} (aligned with Claude Code convention)
func bridgeTool(serverName string, mcpTool mcpsdk.Tool) agent.ToolEntry
```

### 3.3 Resource Support

```go
// mcp/resource.go
type Resource struct {
    URI         string
    Name        string
    Description string
    MIMEType    string
}
```

`ListMcpResources` and `ReadMcpResource` registered as built-in tools.

### 3.4 Lifecycle

1. Agent startup → Manager connects to all configured MCP Servers
2. stdio: spawn subprocess, JSON-RPC over stdin/stdout
3. SSE/HTTP: establish HTTP connection
4. `ListTools()` → bridge each tool into agent ToolRegistry
5. Agent loop calls MCP tools via bridge → JSON-RPC to remote
6. Agent shutdown → graceful close all MCP connections

### 3.5 Integration

```go
agent.NewAgent(
    agent.WithMCPServers(map[string]mcp.ServerConfig{
        "github": {Command: "npx", Args: []string{"@modelcontextprotocol/server-github"}, Transport: mcp.TransportStdio},
        "db":     {URL: "http://localhost:8080/mcp", Transport: mcp.TransportStreamableHTTP},
    }),
)
```

---

## 4. Agent Teams Module (`teams/`)

### 4.1 Core Concept

Multiple Agents form a team, communicate via MessageBus, coordinate via SharedTaskList. Supports 6 topology modes.

### 4.2 Six Topologies

| Topology | Center Node | Communication | Scale | Strength |
|----------|------------|---------------|-------|----------|
| **LeaderTeammate** | Leader | Star, bidirectional | 3-6 | Simple, controllable |
| **Pipeline** | None | Unidirectional chain | 2-8 stages | Separation of concerns |
| **PeerRing** | None | Fully connected | 2-5 | Multi-perspective, adversarial reasoning |
| **SupervisorTree** | Multi-level Supervisors | Tree, bidirectional | 10+ | Fault tolerance, scale |
| **Blackboard** | Shared state | Indirect (read/write board) | 3-8 | Loose coupling, expert collaboration |
| **MapReduce** | Dispatcher+Merger | Fan-out → Fan-in | N homogeneous | Parallel speedup |

### 4.3 Topology Interface

```go
// teams/topology.go
type Topology interface {
    Name() string
    Route(from string, msg *Message, members []string) []string
    NextTask(tasks []*Task, members []*Member) []TaskAssignment
    OnMemberJoin(name string)
    OnMemberLeave(name string)
}

type TaskAssignment struct {
    TaskID     string
    MemberName string
}
```

Implementation files:
- `topology_leader.go` — LeaderTeammate (default, fully implemented)
- `topology_pipeline.go` — Pipeline (skeleton)
- `topology_ring.go` — PeerRing (skeleton)
- `topology_supervisor.go` — SupervisorTree (skeleton)
- `topology_blackboard.go` — Blackboard (skeleton)
- `topology_mapreduce.go` — MapReduce (skeleton)

### 4.4 Key Types

```go
// teams/team.go
type Team struct {
    id       string              // generateID(PrefixTeam)
    name     string
    lead     *Member
    members  map[string]*Member
    tasks    *SharedTaskList
    bus      *MessageBus
    topology Topology
    ctx      context.Context
    cancel   context.CancelFunc
    mu       sync.RWMutex
}

func New(name string, opts ...Option) *Team
func (t *Team) Start(ctx context.Context, prompt string) *Stream
func (t *Team) SpawnMember(name string, opts ...MemberOption) error
func (t *Team) Shutdown() error

// teams/member.go
type Member struct {
    id     string               // generateID(PrefixAgent)
    name   string
    role   Role                 // Lead | Teammate
    agent  *agent.Agent
    client *agent.Client        // independent session per member
    status atomic.Int32         // Idle | Working | Shutdown
    inbox  chan *Message
    bus    *MessageBus
}

type Role int
const (
    RoleLead     Role = iota
    RoleTeammate
)

// teams/message.go
type MessageBus struct {
    subscribers map[string]chan *Message
    topology    Topology
    history     []*Message
    mu          sync.RWMutex
}

type Message struct {
    ID        string
    Type      MessageType
    From      string
    To        string            // empty = route via topology
    Content   string
    RequestID string
    Timestamp time.Time
}

type MessageType string
const (
    MessageDM               MessageType = "message"
    MessageBroadcast        MessageType = "broadcast"
    MessageShutdownRequest  MessageType = "shutdown_request"
    MessageShutdownResponse MessageType = "shutdown_response"
    MessagePlanApproval     MessageType = "plan_approval"
)

// teams/tasklist.go
type SharedTaskList struct {
    tasks map[string]*Task
    order []string
    mu    sync.RWMutex
}

type Task struct {
    ID          string
    Subject     string
    Description string
    Status      TaskStatus
    Owner       string
    BlockedBy   []string
    Blocks      []string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type TaskStatus int
const (
    TaskPending TaskStatus = iota
    TaskInProgress
    TaskCompleted
    TaskDeleted
)

func (l *SharedTaskList) Create(task *Task) error
func (l *SharedTaskList) Claim(id, owner string) error  // atomic CAS
func (l *SharedTaskList) Update(id string, u TaskUpdate) error
func (l *SharedTaskList) NextAvailable() *Task

// teams/blackboard.go — for Blackboard topology
type Blackboard struct {
    entries map[string]*Entry
    mu      sync.RWMutex
    notify  chan string
}

type Entry struct {
    Key       string
    Value     any
    Author    string
    UpdatedAt time.Time
}

// teams/stream.go — aggregated event stream
type Stream struct {
    events  chan *Event
    current *Event
    err     error
}

type Event struct {
    MemberName string
    AgentEvent agent.Event
}
```

### 4.5 Teams-specific Tools

Registered in `tools/` package, active only in Teams mode:
- `SendMessage` — DM to specific member
- `Broadcast` — broadcast to all
- `TaskCreate` / `TaskList` / `TaskUpdate` / `TaskGet` — task management
- `ShutdownRequest` — request member shutdown

### 4.6 Team Options

```go
func WithTopology(t Topology) Option
func WithLeaderTeammate(leaderName string) Option   // shortcut
func WithPipeline(stages ...string) Option          // shortcut
func WithPeerRing(members ...string) Option         // shortcut
```

---

## 5. New Package Structure

```
subagent/
  definition.go     → Subagent definition struct
  runner.go         → Lifecycle management (spawn, track, collect)
  runner_test.go

mcp/
  config.go         → ServerConfig, TransportType
  manager.go        → Multi-server connection manager
  manager_test.go
  bridge.go         → MCP tool → agent.Tool bridge
  bridge_test.go
  resource.go       → Resource types + tools

teams/
  team.go           → Team struct + lifecycle
  team_test.go
  member.go         → Member (goroutine wrapper)
  member_test.go
  message.go        → MessageBus (channel fan-out via Topology)
  message_test.go
  tasklist.go       → SharedTaskList (concurrent task management)
  tasklist_test.go
  blackboard.go     → Blackboard (shared state for Blackboard topology)
  stream.go         → Aggregated TeamStream
  topology.go       → Topology interface
  topology_leader.go      → LeaderTeammate (fully implemented)
  topology_pipeline.go    → Pipeline (skeleton)
  topology_ring.go        → PeerRing (skeleton)
  topology_supervisor.go  → SupervisorTree (skeleton)
  topology_blackboard.go  → Blackboard topology (skeleton)
  topology_mapreduce.go   → MapReduce (skeleton)

tools/ (additions)
  task.go           → Task tool (calls subagent.Runner)
  sendmessage.go    → SendMessage tool (Teams)
  broadcast.go      → Broadcast tool (Teams)
  taskcreate.go     → TaskCreate tool (Teams)
  tasklist_tool.go  → TaskList tool (Teams)
  taskupdate.go     → TaskUpdate tool (Teams)
  taskget.go        → TaskGet tool (Teams)
  shutdown.go       → ShutdownRequest tool (Teams)
  mcpresource.go    → ListMcpResources / ReadMcpResource tools
```

---

## 6. Development Plan

### Phase 0: Foundation (blocking)

**Owner:** Team Lead

1. Define public interfaces and types for all three packages
2. Define `Topology` interface + 6 struct skeletons
3. Add `WithAgents()`, `WithMCPServers()`, `WithTeam()` options to root `option.go`
4. Ensure `go build ./...` passes
5. Output: contract files (interfaces, types, exported function signatures)

**Done criteria:** `tsc` equivalent (go build) passes, all packages importable, zero implementation.

### Phase 1: Parallel Development (3 Dev agents)

| Agent | Module | Scope |
|-------|--------|-------|
| **dev-subagent** | `subagent/` + `tools/task.go` | Definition, Runner, Task tool, wire into agent loop |
| **dev-mcp** | `mcp/` | Manager, bridge, stdio/SSE transport, resource tools |
| **dev-teams** | `teams/` + teams tools | Team, Member, MessageBus, SharedTaskList, Stream, LeaderTeammate topology (full), 5 skeleton topologies, all Teams tools |

Each dev agent works in isolation. No cross-module dependencies during Phase 1.

### Phase 2: Integration & Testing

1. Wire all three modules into Agent via options
2. Cross-module integration tests
3. `go build && go test && go vet` all pass
4. Update CLAUDE.md architecture section

---

## 7. Implementation Priority

**Fully implemented this phase:**
- [x] Subagent: Definition, Runner, Task tool
- [x] MCP: Manager, Bridge, stdio transport, SSE transport
- [x] Teams: Team, Member, MessageBus, SharedTaskList, Stream
- [x] Teams: LeaderTeammate topology (full implementation)
- [x] Teams: All Teams-specific tools
- [x] 5 skeleton topologies (interface satisfied, minimal logic)

**Previously deferred, now complete:**
- [x] Pipeline topology (full: NextTask stage-priority assignment, OnMemberJoin/Leave)
- [x] PeerRing topology (full: round-robin NextTask, dynamic ring management)
- [x] SupervisorTree topology (full: leaf-only assignment, orphan re-parenting on leave)
- [x] Blackboard topology (full: idle-member assignment, Blackboard state)
- [x] MapReduce topology (full: worker-only distribution, dynamic worker pool)
- [x] MCP Server (in-process SDKServer with AddTool generic + AgentOption)
- [x] Plugin system (plugin/ package)
- [x] Sandbox configuration (WithSandbox + context propagation)
- [x] File Checkpointing (checkpoint/ package)
