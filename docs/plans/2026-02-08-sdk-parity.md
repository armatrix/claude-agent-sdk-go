# SDK Parity Plan: Align Go SDK with Official TS/Python SDKs

> **Goal:** Close feature gaps between `claude-agent-sdk-go` and the official
> `claude-agent-sdk-typescript` / `claude-agent-sdk-python` SDKs, while
> preserving Go-idiomatic design and the Go SDK's unique advantages (pure Go,
> no CLI dependency, 6 team topologies).
>
> **Baseline:** Go SDK at commit `735e24b` (2026-02-08), with `WithOnInit` /
> `WithDefinitions` / `WithServers` auto-wiring just merged.
>
> **Reference repos:**
> - TS: `github.com/anthropics/claude-agent-sdk-typescript`
> - Python: `github.com/anthropics/claude-agent-sdk-python`
> - Docs: `platform.claude.com/docs/en/agent-sdk/`

---

## Architecture Difference

| | Official TS/Python | Go SDK |
|---|---|---|
| **架构** | Claude Code CLI wrapper (spawn 子进程 JSON 通信) | 纯 Go 实现，直接调 Anthropic API |
| **优势** | 自动继承 CLI 新功能 | 零外部依赖，可嵌入任意 Go 程序 |
| **劣势** | 需安装 CLI 二进制，Python 3.10+/Node 18+ | 需自行实现每个功能 |

因此以下部分功能不适用于 Go SDK（标注 N/A），部分功能需要从零实现。

---

## Gap Summary

### Status Legend

- 🔴 **Missing** — 完全缺失，需实现
- 🟡 **Partial** — 有基础但不完整
- ✅ **Done** — 已对齐或超越官方
- ⬜ **N/A** — 架构差异导致不适用

### Feature Matrix

| # | Feature | TS | Py | Go | Priority | Phase |
|---|---------|----|----|--------|----------|-------|
| 1 | Extended Thinking (thinking tokens) | ✅ | ✅ | ✅ | **P0** | A |
| 2 | Beta Features (`betas` 选项) | ✅ | ✅ | ✅ | **P0** | A |
| 3 | SDK MCP Server (进程内自定义工具) | ✅ | ✅ | ✅ | **P0** | A |
| 4 | Tool Search (动态工具发现) | ✅ | ✅ | ✅ | **P1** | B |
| 5 | Multi-Provider Auth (Bedrock/Vertex/Azure) | ✅ | ✅ | ✅ | **P1** | B |
| 6 | Sandbox Mode | ✅ | ✅ | ✅ | **P1** | B |
| 7 | `cwd` / `env` 选项 | ✅ | ✅ | ✅ | **P1** | A |
| 8 | Fallback Model | ✅ | ✅ | ✅ | **P2** | C |
| 9 | File Checkpointing (rewind) | ✅ | ✅ | ✅ | **P2** | C |
| 10 | Plugin System | ✅ | ✅ | ✅ | **P2** | C |
| 11 | Slash Commands | ✅ | ✅ | ✅ | **P2** | C |
| 12 | System Prompt Presets | ✅ | ✅ | ✅ | **P2** | A |
| 13 | Permission Rules (细粒度) | ✅ | ✅ | ✅ | **P1** | B |
| 14 | 缺失 Hook 事件 (4个) | ✅ | 🟡 | ✅ | **P2** | B |
| 15 | Runtime Model Switch (Client) | ✅ | ✅ | ✅ | **P2** | A |
| 16 | Interrupt (mid-turn) | ✅ | ✅ | ✅ | **P1** | A |
| 17 | `continue` 最近会话 | ✅ | ✅ | ✅ | **P2** | C |
| — | Agent loop + tool use | ✅ | ✅ | ✅ | — | — |
| — | Streaming events | ✅ | ✅ | ✅ | — | — |
| — | 21 built-in tools | ✅ | ✅ | ✅ | — | — |
| — | Session (persist/clone/fork) | ✅ | ✅ | ✅ | — | — |
| — | 12 hook events | ✅ | 🟡 | ✅ | — | — |
| — | Permission (4 modes) | ✅ | ✅ | ✅ | — | — |
| — | Budget/cost (decimal) | ✅ | ✅ | ✅ | — | — |
| — | Compaction (server + client) | ✅ | ✅ | ✅ | — | — |
| — | Structured output | ✅ | ✅ | ✅ | — | — |
| — | Subagent + Task tool | ✅ | ✅ | ✅ | — | — |
| — | MCP (stdio + HTTP + bridge) | ✅ | ✅ | ✅ | — | — |
| — | Settings / Skills loading | ✅ | ✅ | ✅ | — | — |
| — | WithDefinitions / WithServers | ✅ | ✅ | ✅ | — | — |
| — | Teams (6 topologies) | ❌ | ❌ | ✅ | — | — |

---

## Phase A: Core API Parity (解锁核心能力)

> **依赖:** 无。可立即开始。
> **目标:** 补齐直接影响 API 调用能力的缺失选项。

### A1. Extended Thinking (`WithMaxThinkingTokens`)

**Gap:** 官方 SDK 支持 `maxThinkingTokens` 控制推理深度，Python `ClaudeSDKClient`
支持运行时调整 `setMaxThinkingTokens()`。Go SDK 完全缺失。

**Design:**

```go
// option.go
func WithMaxThinkingTokens(n int64) AgentOption {
    return func(o *agentOptions) { o.maxThinkingTokens = n }
}

// agentOptions 新增字段
maxThinkingTokens int64  // 0 = disabled

// client.go — 运行时调整
func (c *Client) SetMaxThinkingTokens(n int64) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.agent.opts.maxThinkingTokens = n
}
```

**Engine 变更 (internal/engine/loop.go):**

```go
// LoopConfig 新增
MaxThinkingTokens int64

// RunLoop — build API params
if cfg.MaxThinkingTokens > 0 {
    params.Thinking = &anthropic.ThinkingConfigParamUnion{
        OfThinkingConfigEnabled: &anthropic.ThinkingConfigEnabledParam{
            BudgetTokens: cfg.MaxThinkingTokens,
        },
    }
    // Thinking 模式下 MaxTokens 需 >= budget + output
    if params.MaxTokens < cfg.MaxThinkingTokens + 1024 {
        params.MaxTokens = cfg.MaxThinkingTokens + 16384
    }
}
```

**Event 变更 (event.go):**
- `AssistantEvent` 已包含 `anthropic.Message`，thinking blocks 自动透传。
- 可选：添加 `ThinkingEvent` 用于 streaming thinking deltas。

**Files:**
| File | Change |
|------|--------|
| `option.go` | 新增 `maxThinkingTokens` 字段 + `WithMaxThinkingTokens()` |
| `agent.go` | 传递到 `LoopConfig` |
| `client.go` | 新增 `SetMaxThinkingTokens()` |
| `internal/engine/loop.go` | `LoopConfig` 新增字段，构建 params 时注入 |
| `event.go` | 可选：新增 `ThinkingEvent` |
| `defaults.go` | 新增 `DefaultMaxThinkingTokens` |

**Tests:** `option_test.go`, `agent_test.go` (验证 LoopConfig 传递), `engine/loop_test.go` (验证 params 构建)

---

### A2. Beta Features (`WithBetas`)

**Gap:** 官方 SDK 支持 `betas: ["context-1m-2025-08-07"]` 等 beta 特性标记。Go SDK
compaction 模块已硬编码 `compact-2026-01-12` beta，但用户无法传入自定义 betas。

**Design:**

```go
// option.go
func WithBetas(betas ...string) AgentOption {
    return func(o *agentOptions) { o.betas = betas }
}

// agentOptions 新增
betas []string
```

**Engine 变更:**
- `LoopConfig` 新增 `Betas []string`
- 当 `len(betas) > 0` 时，使用 `BetaMessageNewParams` 替代 `MessageNewParams`
- 与现有 compaction beta 合并（compaction 也加入 betas 列表）

**Files:**
| File | Change |
|------|--------|
| `option.go` | 新增 `betas` 字段 + `WithBetas()` |
| `agent.go` | 传递到 `LoopConfig` |
| `internal/engine/loop.go` | betas 非空时切换 Beta API |
| `internal/engine/compact.go` | 合并用户 betas + compact beta |

---

### A3. SDK MCP Server (进程内自定义工具)

**Gap:** 官方 Python 有 `@tool` 装饰器 + `create_sdk_mcp_server()`，TS 有
`createSDKMcpServer()`。允许用户在同进程内定义工具，无需启动子进程。Go SDK 仅支持
外部 MCP (stdio/HTTP)。

**Design:** 不需要真实 MCP 协议。核心是一个轻量级的进程内工具注册 API，语义对齐
SDK MCP Server 但实现更简单：

```go
// mcp/sdk_server.go
package mcp

// SDKServer is an in-process MCP server that wraps Go functions as tools.
// Unlike external MCP servers, it runs in the same process — no subprocess,
// no JSON-RPC, no transport overhead.
type SDKServer struct {
    name  string
    tools []sdkTool
}

type sdkTool struct {
    name        string
    description string
    schema      anthropic.ToolInputSchemaParam
    handler     func(ctx context.Context, input json.RawMessage) (string, error)
}

func NewSDKServer(name string) *SDKServer

// AddTool registers a Go function as an MCP tool.
// The input type T is used for auto JSON schema generation.
func AddTool[T any](s *SDKServer, name, description string, handler func(ctx context.Context, input T) (string, error))

// AgentOption returns a WithOnInit option that registers all tools.
func (s *SDKServer) AgentOption() agent.AgentOption
```

**用户代码:**

```go
server := mcp.NewSDKServer("my-tools")
mcp.AddTool(server, "calculate", "Perform math", func(ctx context.Context, input CalcInput) (string, error) {
    return fmt.Sprintf("%d", input.A + input.B), nil
})

a := agent.NewAgent(
    server.AgentOption(),
)
```

工具名遵循 `mcp__{server}__{tool}` 命名约定，与外部 MCP 一致。

**Files:**
| File | Change |
|------|--------|
| `mcp/sdk_server.go` (新建) | SDKServer struct, NewSDKServer, AddTool[T], AgentOption |
| `mcp/sdk_server_test.go` (新建) | 注册、执行、schema 验证 |

---

### A4. `cwd` / `env` 选项

**Gap:** 官方 SDK 支持 `cwd` 设置工作目录（影响 Bash/Read/Write 等工具）和 `env`
传递环境变量。Go SDK 无显式选项。

**Design:**

```go
// option.go
func WithWorkDir(dir string) AgentOption {
    return func(o *agentOptions) { o.workDir = dir }
}

func WithEnv(env map[string]string) AgentOption {
    return func(o *agentOptions) { o.env = env }
}
```

工具通过 context 获取 `workDir` 和 `env`:
- `tools/bash.go` — 执行时 `cmd.Dir = workDir`, `cmd.Env` 合并 env
- `tools/read.go` / `tools/write.go` — 相对路径基于 workDir 解析
- `tools/glob.go` / `tools/grep.go` — 搜索起点基于 workDir

**Files:**
| File | Change |
|------|--------|
| `option.go` | 新增 `workDir`, `env` 字段 + 选项函数 |
| `agent.go` | 通过 context 注入 workDir/env |
| `tools/bash.go` | 使用 workDir/env |
| `tools/read.go`, `write.go`, `glob.go`, `grep.go` | 使用 workDir |

---

### A5. System Prompt Presets

**Gap:** 官方支持 `{ type: 'preset', preset: 'claude_code' }` 复用内置 prompt。

**Design:**

```go
// option.go
type SystemPromptPreset string

const (
    PresetDefault    SystemPromptPreset = ""
    PresetClaudeCode SystemPromptPreset = "claude_code"
)

func WithSystemPromptPreset(preset SystemPromptPreset) AgentOption {
    return func(o *agentOptions) { o.systemPromptPreset = preset }
}
```

Preset prompts 存储在 `internal/config/presets.go` 中（从官方 SDK 同步）。

---

### A6. Interrupt 增强 (mid-turn)

**Gap:** Go `Client.Interrupt()` 已存在，通过 `context.Cancel()` 取消整个 run。
但官方 SDK 的 interrupt 更精细 — 可以在 stream 中间中断当前 turn，而不是取消整个
session。

**Design:** 当前 `Client.Interrupt()` 已取消 context，engine loop 在下一个 check
点退出。需增强：
1. `Interrupt()` 应该能在 streaming 中途打断（当前依赖 `ctx.Done()`，基本满足）。
2. 增加 `InterruptAndContinue()` — 中断当前 turn 但保留 session 供后续 Query 使用。

```go
// client.go
func (c *Client) InterruptAndContinue() {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.cancel != nil {
        c.cancel()
        c.cancel = nil
    }
    // Session is preserved — next Query() picks up where we left off.
}
```

当前 `Interrupt()` 和 `InterruptAndContinue()` 实际行为相同（session 已保留）。
主要差异是语义明确性和文档。

---

### A7. Runtime Model Switch 增强

**Gap:** `Client.SetModel()` 已存在。缺 `SetMaxThinkingTokens()`（A1 已覆盖）和
`SetPermissionMode()`。

**Design:**

```go
// client.go 新增
func (c *Client) SetPermissionMode(mode permission.Mode) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.agent.opts.permissionMode = mode
}
```

---

## Phase B: Enterprise & Safety (企业就绪)

> **依赖:** Phase A（部分依赖 A2 Betas 机制）。
> **目标:** 多云认证、沙箱、细粒度权限、Tool Search。

### B1. Multi-Provider Authentication

**Gap:** 官方支持 Anthropic API / AWS Bedrock / Google Vertex AI / Azure。
Go SDK 仅支持 Anthropic API key (通过 `anthropic-sdk-go` 自动读 env)。

**Design:**

```go
// option.go
type AuthProvider string

const (
    AuthAnthropic AuthProvider = "anthropic"
    AuthBedrock   AuthProvider = "bedrock"
    AuthVertex    AuthProvider = "vertex"
    AuthAzure     AuthProvider = "azure"
)

func WithAuthProvider(provider AuthProvider) AgentOption
func WithBedrockConfig(region string, opts ...func(*BedrockConfig)) AgentOption
func WithVertexConfig(project, location string) AgentOption
```

**实现:** `anthropic-sdk-go` 已支持 Bedrock/Vertex — 只需传入不同的
`option.RequestOption`。Go SDK 需要的是正确初始化 `anthropic.Client`:

```go
// agent.go — NewAgent 中
switch resolved.authProvider {
case AuthBedrock:
    client = anthropic.NewBedrockClient()
case AuthVertex:
    client = anthropic.NewVertexClient(resolved.vertexProject, resolved.vertexLocation)
default:
    client = anthropic.NewClient()
}
```

**Files:**
| File | Change |
|------|--------|
| `option.go` | Auth provider 选项 |
| `agent.go` | 根据 provider 初始化不同 client |

---

### B2. Tool Search (动态工具发现)

**Gap:** 官方 SDK 在工具数量 > context 10% 时自动启用 Tool Search：不发送全部工具
schema，而是让 LLM 先搜索可用工具再调用。

**Design:**

```go
// option.go
func WithToolSearch(enabled bool) AgentOption
func WithToolSearchThreshold(ratio float64) AgentOption // default 0.1

// internal/engine/loop.go — 构建 params 时
if cfg.ToolSearch && toolSchemaTokens > cfg.ContextWindow * cfg.ToolSearchThreshold {
    // 只发送 tool search meta-tool，不发全部 schema
    params.Tools = []anthropic.ToolUnionParam{toolSearchTool}
}
```

需要实现一个 `ToolSearch` meta-tool（名为 `ToolSearch`），执行时返回匹配的工具列表。

**Files:**
| File | Change |
|------|--------|
| `option.go` | 新增 `toolSearch`, `toolSearchThreshold` |
| `tools/toolsearch.go` (新建) | ToolSearch meta-tool |
| `internal/engine/loop.go` | 条件发送工具 schema |

---

### B3. Sandbox Mode

**Gap:** 官方 SDK 支持命令执行沙箱（限制网络、文件系统访问、排除命令列表）。

**Design:**

```go
// option.go
type SandboxConfig struct {
    Enabled         bool
    AllowNetwork    bool
    AllowedDirs     []string   // 允许读写的目录白名单
    ExcludeCommands []string   // 始终允许的命令（绕过沙箱）
}

func WithSandbox(config SandboxConfig) AgentOption
```

**实现:** 在 `tools/bash.go` 中：
- `Enabled=true` 时使用 OS 级沙箱（macOS `sandbox-exec`, Linux `seccomp`/`nsjail`）
- `AllowedDirs` 映射为 read-write 路径
- `ExcludeCommands` 绕过沙箱检查

**Files:**
| File | Change |
|------|--------|
| `option.go` | SandboxConfig + WithSandbox |
| `tools/bash.go` | 沙箱执行逻辑 |
| `tools/sandbox_darwin.go` (新建) | macOS sandbox-exec |
| `tools/sandbox_linux.go` (新建) | Linux nsjail/seccomp |

---

### B4. Permission Rules (细粒度)

**Gap:** 官方 SDK 支持 `allowedTools` / `disallowedTools` 带通配符 + deny→ask→allow
求值顺序。Go SDK 有 4 modes + custom func，但缺少声明式规则系统。

**Design:**

```go
// permission/rules.go
type Rule struct {
    Pattern  string    // glob pattern, e.g. "mcp__context7__*"
    Decision Decision  // Allow, Deny, Ask
}

// permission/checker.go 增强
type Checker struct {
    mode     Mode
    rules    []Rule         // 新增
    fn       Func
}

// 求值顺序: deny rules → ask rules → allow rules → mode default
```

**Option:**

```go
func WithPermissionRules(rules ...permission.Rule) AgentOption
func WithAllowedTools(patterns ...string) AgentOption   // 语法糖
func WithDisallowedTools(patterns ...string) AgentOption // 语法糖
```

**Files:**
| File | Change |
|------|--------|
| `permission/rules.go` (新建) | Rule 类型, 匹配逻辑 |
| `permission/checker.go` | 集成 rules 到求值链 |
| `option.go` | 新增规则选项 |

---

### B5. 缺失 Hook 事件

**Gap:** Go SDK 缺少 4 个官方 hook 事件:

| Event | 用途 |
|-------|------|
| `UserPromptSubmit` | 用户提交 prompt 前注入额外 context |
| `SubagentStart` | 子 agent 启动时追踪 |
| `SubagentStop` | 子 agent 完成时汇聚结果 |
| `PermissionRequest` | 自定义权限对话框 |

**Design:**

```go
// hook/event.go 新增
const (
    UserPromptSubmit  Event = "user_prompt_submit"
    SubagentStart     Event = "subagent_start"
    SubagentStop      Event = "subagent_stop"
    PermissionRequest Event = "permission_request"
)
```

**触发点:**
- `UserPromptSubmit` → `agent.go` RunWithSession 追加 user message 之前
- `SubagentStart/Stop` → `subagent/runner.go` Spawn/Wait 前后
- `PermissionRequest` → `internal/engine/loop.go` permission check 触发 Ask 时

**Files:**
| File | Change |
|------|--------|
| `hook/event.go` | 新增 4 个事件常量 |
| `hook/input.go` | 新增对应 Input 字段 |
| `internal/engine/loop.go` | PermissionRequest 触发 |
| `agent.go` | UserPromptSubmit 触发 |
| `subagent/runner.go` | SubagentStart/Stop 触发 |
| `internal/hookrunner/runner.go` | 新增 4 个 Run 方法 |

---

## Phase C: Completeness (完整性)

> **依赖:** Phase A/B 部分功能。
> **目标:** 补齐剩余差异，达到完整 feature parity。

### C1. Fallback Model

```go
func WithFallbackModel(model anthropic.Model) AgentOption
```

Engine loop 中：当 API 返回 overloaded/model_unavailable 时自动重试一次使用
fallback model。

### C2. File Checkpointing (Rewind)

```go
// checkpoint/checkpoint.go (新 package)
type Tracker struct {
    changes map[string]*FileChange  // path → original content
}

func (t *Tracker) RecordWrite(path string, before []byte)
func (t *Tracker) Rewind(messageID string) error
```

集成到 `tools/write.go` 和 `tools/edit.go` — 写入前记录原始内容。

### C3. Plugin System

```go
// plugin/plugin.go (新 package)
type Plugin struct {
    Name     string
    Commands map[string]*Command
    Agents   map[string]*AgentDef
    MCPServers map[string]*mcp.ServerConfig
    Skills   []string  // .md file paths
}

func LoadPlugins(dirs ...string) ([]*Plugin, error)
```

### C4. Slash Commands

```go
// internal/config/commands.go
type Command struct {
    Name     string
    Content  string  // markdown template
    FilePath string
}

func LoadCommands(dirs ...string) ([]Command, error)
```

`WithCommandDirs(dirs ...string)` 选项，加载 `.claude/commands/*.md`。

### C5. Continue 最近会话

```go
// client.go
func (c *Client) ContinueLatest(ctx context.Context) error {
    // 从 SessionStore 加载最新 session
    if lister, ok := c.store.(SessionLister); ok {
        sessions, _ := lister.List(ctx)
        // sort by UpdatedAt desc, load first
    }
}
```

### C6. Per-Model Usage 在 ResultEvent

当前 `ResultEvent.ModelUsage` 字段已定义 (`map[string]ModelUsage`) 但 engine
loop 未填充。需要在 loop 中按 model 累积 token 使用量。

---

## Implementation Order & Dependencies

```
Phase A (parallel where possible)
├── A1: Thinking Tokens ──────────────────────────── (独立)
├── A2: Betas ────────────────────────────────────── (独立，A1 可能依赖)
├── A3: SDK MCP Server ───────────────────────────── (独立)
├── A4: cwd/env ──────────────────────────────────── (独立)
├── A5: System Prompt Presets ────────────────────── (独立)
├── A6: Interrupt 增强 ───────────────────────────── (独立，小改)
└── A7: Runtime Model Switch ─────────────────────── (独立，小改)

Phase B (部分依赖 A)
├── B1: Multi-Provider Auth ──────────────────────── (独立)
├── B2: Tool Search ──────────────────────────────── (依赖 A4 cwd)
├── B3: Sandbox ──────────────────────────────────── (依赖 A4 cwd)
├── B4: Permission Rules ─────────────────────────── (独立)
└── B5: 缺失 Hook 事件 ──────────────────────────── (独立)

Phase C (依赖 A/B)
├── C1: Fallback Model ───────────────────────────── (依赖 A2 Betas)
├── C2: File Checkpointing ───────────────────────── (依赖 A4 cwd)
├── C3: Plugin System ────────────────────────────── (依赖 C4)
├── C4: Slash Commands ───────────────────────────── (独立)
├── C5: Continue Latest ──────────────────────────── (独立)
└── C6: Per-Model Usage ──────────────────────────── (独立)
```

---

## Effort Estimates

| Phase | Items | Estimated Scope |
|-------|-------|-----------------|
| **A** | 7 items | ~15 files modified/created, ~1200 lines |
| **B** | 5 items | ~12 files modified/created, ~1500 lines |
| **C** | 6 items | ~10 files modified/created, ~800 lines |
| **Total** | 18 items | ~37 files, ~3500 lines |

---

## What We DON'T Need to Implement

| Feature | Why N/A |
|---------|---------|
| CLI binary bundling | Go SDK 是纯库，不依赖 CLI |
| `CLINotFoundError` / `ProcessError` | 无子进程 |
| `cli_path` option | 无 CLI |
| `cleanupPeriodDays` | 用户管理自己的文件 |
| Node.js / Python version checks | Go binary |

---

## Go SDK Unique Advantages (保持)

| Feature | 说明 |
|---------|------|
| **6 Team Topologies** | LeaderTeammate, Pipeline, PeerRing, SupervisorTree, Blackboard, MapReduce |
| **SharedTaskList** | 团队级任务板 with blocking relationships |
| **MessageBus** | 拓扑驱动路由 |
| **4 extra hook events** | PreAPIRequest, PostAPIRequest, PostCompact, ToolResult |
| **Pure Go, zero CLI dependency** | 可嵌入任意 Go 程序 |
| **Generic Tool[T]** | 类型安全的工具定义 + 自动 schema |
| **Module-level internal/** | 子包可访问 internal/ 但外部不可 |

---

## Acceptance Criteria

Phase 完成标准:

```bash
go build ./...     # 编译通过
go vet ./...       # 无警告
go test ./...      # 全部通过
```

每个 Phase 完成后：
1. 更新 `CLAUDE.md` Architecture section
2. 更新 `docs/plans/` 标记已完成项
3. 运行 code-reviewer agent 检查质量
