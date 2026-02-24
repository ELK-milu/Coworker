# Design: ClaudeCLI 完整功能集成

## 概述

本文档描述 ClaudeCLI 模块的架构设计，实现 Claude Code CLI 的核心功能。

---

## 架构总览

```
┌─────────────────────────────────────────────────────────────────┐
│                        Frontend (React)                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │
│  │ Coworker │ │ Session  │ │   File   │ │   Permission     │   │
│  │   Page   │ │ Sidebar  │ │ Explorer │ │   Dialog (NEW)   │   │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────────┬─────────┘   │
└───────┼────────────┼────────────┼────────────────┼──────────────┘
        │            │            │                │
        └────────────┴────────────┴────────────────┘
                            │ WebSocket
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                     WSHandler (api/websocket.go)                 │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                    Message Router                         │   │
│  │  chat | skill | task | mcp | permission | agent          │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Core Modules                                │
│                                                                  │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌──────────┐  │
│  │   Skills   │  │    MCP     │  │   Agents   │  │Permission│  │
│  │  (R1 NEW)  │  │  (R2 NEW)  │  │  (R3 NEW)  │  │ (R5 NEW) │  │
│  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘  └────┬─────┘  │
│        │               │               │              │         │
│        └───────────────┴───────────────┴──────────────┘         │
│                            │                                     │
│                            ▼                                     │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                   Tool Registry                           │   │
│  │  Bash | Read | Write | Edit | Glob | Grep                │   │
│  │  WebFetch | WebSearch | LSP | Notebook | AskUser (R4)    │   │
│  └──────────────────────────────────────────────────────────┘   │
│                            │                                     │
│                            ▼                                     │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              Existing Infrastructure                      │   │
│  │  Session | Context | Task | Workspace | Client           │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

---

## 模块设计

### M1: Skills 模块 (internal/skills/)

**职责**: 管理用户自定义技能，支持参数替换和来源优先级。

#### 目录结构

```
internal/skills/
├── skill.go          # Skill 结构定义
├── loader.go         # Skill 加载器
├── parser.go         # Frontmatter 解析
├── executor.go       # Skill 执行器
└── registry.go       # Skill 注册表
```

#### 核心接口

```go
// Skill 技能定义
type Skill struct {
    Name         string   `json:"name"`
    Description  string   `json:"description"`
    Content      string   `json:"content"`
    AllowedTools []string `json:"allowed_tools,omitempty"`
    Source       string   `json:"source"` // "project" | "user"
    FilePath     string   `json:"file_path"`
}

// SkillRegistry 技能注册表
type SkillRegistry struct {
    skills map[string]*Skill
    mu     sync.RWMutex
}

func (r *SkillRegistry) Register(skill *Skill) error
func (r *SkillRegistry) Get(name string) (*Skill, bool)
func (r *SkillRegistry) GetAll() []*Skill
func (r *SkillRegistry) LoadFromDir(dir string, source string) error

// SkillExecutor 技能执行器
type SkillExecutor struct {
    registry *SkillRegistry
}

func (e *SkillExecutor) Execute(name string, args []string) (string, error)
func (e *SkillExecutor) SubstituteParams(content string, args []string) string
```

#### 参数替换规则

| 语法 | 含义 | 示例 |
|------|------|------|
| `$0` | 完整参数字符串 | `/skill arg1 arg2` → `arg1 arg2` |
| `$1`, `$2`, ... | 位置参数 | `/skill foo bar` → `$1=foo, $2=bar` |
| `$ARGUMENTS[N]` | 数组索引 | `$ARGUMENTS[0]` = 第一个参数 |

#### Skill 文件格式

```markdown
---
name: my-skill
description: 技能描述
allowed_tools:
  - Read
  - Grep
---

技能内容，支持 $1 参数替换。
```

#### 来源优先级

1. 项目级: `{workspace}/.claude/skills/*.md`
2. 用户级: `{userdata}/skills/*.md`

项目级 Skill 优先于用户级同名 Skill。

---

### M2: MCP 模块 (internal/mcp/)

**职责**: 实现 Model Context Protocol，支持连接外部 MCP 服务器扩展工具能力。

#### 目录结构

```
internal/mcp/
├── transport/
│   ├── transport.go      # Transport 接口
│   ├── stdio.go          # Stdio 传输
│   ├── websocket.go      # WebSocket 传输
│   └── http.go           # HTTP/SSE 传输
├── connection.go         # 连接管理
├── manager.go            # MCP 管理器
├── tool.go               # MCP 工具包装
├── message.go            # JSON-RPC 消息
└── errors.go             # 错误定义
```

#### 核心接口

```go
// Transport 传输层接口
type Transport interface {
    Connect() error
    Disconnect() error
    Send(msg *Message) error
    Receive() (*Message, error)
    IsConnected() bool
}

// Message JSON-RPC 2.0 消息
type Message struct {
    JSONRPC string      `json:"jsonrpc"`
    ID      interface{} `json:"id,omitempty"`
    Method  string      `json:"method,omitempty"`
    Params  interface{} `json:"params,omitempty"`
    Result  interface{} `json:"result,omitempty"`
    Error   *RPCError   `json:"error,omitempty"`
}

// ServerConfig MCP 服务器配置
type ServerConfig struct {
    Name    string            `json:"name"`
    Type    string            `json:"type"` // stdio | websocket | http
    Command string            `json:"command,omitempty"`
    Args    []string          `json:"args,omitempty"`
    Env     map[string]string `json:"env,omitempty"`
    URL     string            `json:"url,omitempty"`
    Headers map[string]string `json:"headers,omitempty"`
}

// Manager MCP 连接管理器
type Manager struct {
    connections map[string]*Connection
    tools       map[string]*McpTool
    mu          sync.RWMutex
}

func (m *Manager) Connect(config *ServerConfig) error
func (m *Manager) Disconnect(name string) error
func (m *Manager) ListTools(serverName string) ([]*McpTool, error)
func (m *Manager) CallTool(serverName, toolName string, args map[string]interface{}) (*ToolResult, error)
func (m *Manager) GetAllTools() []*McpTool
```

#### 传输层实现

**Stdio Transport**
```go
type StdioTransport struct {
    cmd     *exec.Cmd
    stdin   io.WriteCloser
    stdout  io.ReadCloser
    scanner *bufio.Scanner
    mu      sync.Mutex
}
```

**WebSocket Transport**
```go
type WebSocketTransport struct {
    url       string
    conn      *websocket.Conn
    headers   http.Header
    reconnect bool
    mu        sync.Mutex
}
```

**HTTP Transport**
```go
type HTTPTransport struct {
    url     string
    client  *http.Client
    headers map[string]string
}
```

#### 重连策略

- 指数退避: 1s → 2s → 4s (最大 30s)
- 最大重试: 3 次
- 心跳间隔: 30s

---

### M3: Agents 模块 (internal/agents/)

**职责**: 实现子代理系统，支持专用代理类型和并行执行。

#### 目录结构

```
internal/agents/
├── agent.go          # Agent 结构定义
├── types.go          # 代理类型定义
├── executor.go       # 代理执行器
├── pool.go           # 代理池管理
└── task_tool.go      # TaskTool 实现
```

#### 核心接口

```go
// AgentType 代理类型定义
type AgentType struct {
    Name           string   `json:"name"`
    Description    string   `json:"description"`
    Tools          []string `json:"tools"`
    Model          string   `json:"model,omitempty"`
    PermissionMode string   `json:"permission_mode,omitempty"`
    ForkContext    bool     `json:"fork_context"`
}

// Agent 代理实例
type Agent struct {
    ID          string       `json:"id"`
    Type        string       `json:"type"`
    Description string       `json:"description"`
    Prompt      string       `json:"prompt"`
    Model       string       `json:"model"`
    Status      AgentStatus  `json:"status"`
    StartTime   time.Time    `json:"start_time"`
    EndTime     *time.Time   `json:"end_time,omitempty"`
    Output      string       `json:"output,omitempty"`
    Error       string       `json:"error,omitempty"`
}

type AgentStatus string

const (
    AgentStatusPending   AgentStatus = "pending"
    AgentStatusRunning   AgentStatus = "running"
    AgentStatusCompleted AgentStatus = "completed"
    AgentStatusFailed    AgentStatus = "failed"
    AgentStatusCancelled AgentStatus = "cancelled"
)
```

#### 内置代理类型

| 类型 | 工具 | 模型 | 用途 |
|------|------|------|------|
| `general-purpose` | 全部 | inherit | 复杂多步骤任务 |
| `Explore` | Glob, Grep, Read | haiku | 快速代码库探索 |
| `Plan` | 全部 | inherit | 架构规划设计 |

#### 代理执行器

```go
// Executor 代理执行器
type Executor struct {
    client    *client.ClaudeClient
    tools     *tools.Registry
    mcpMgr    *mcp.Manager
    agents    map[string]*Agent
    mu        sync.RWMutex
}

func (e *Executor) Start(agentType, prompt, model string) (*Agent, error)
func (e *Executor) Resume(agentID string) (*Agent, error)
func (e *Executor) Cancel(agentID string) error
func (e *Executor) GetOutput(agentID string) (string, error)
func (e *Executor) List() []*Agent
```

#### 模型别名解析

| 别名 | 实际模型 |
|------|----------|
| `sonnet` | claude-sonnet-4-20250514 |
| `opus` | claude-opus-4-5-20251101 |
| `haiku` | claude-haiku-4-5-20251001 |
| `inherit` | 继承父代理模型 |

---

### M4: 高级工具 (internal/tools/)

**职责**: 实现 WebFetch、WebSearch、LSP 等高级工具。

#### 新增工具

**WebFetchTool**
```go
type WebFetchTool struct {
    httpClient *http.Client
    timeout    time.Duration
}

func (t *WebFetchTool) Name() string { return "WebFetch" }
func (t *WebFetchTool) Execute(ctx context.Context, input json.RawMessage) (*ToolResult, error)
```

**WebSearchTool**
```go
type WebSearchTool struct {
    apiKey  string
    baseURL string
}

func (t *WebSearchTool) Name() string { return "WebSearch" }
func (t *WebSearchTool) Execute(ctx context.Context, input json.RawMessage) (*ToolResult, error)
```

**LSPTool**
```go
type LSPTool struct {
    servers map[string]*lsp.Client
}

func (t *LSPTool) Name() string { return "LSP" }
func (t *LSPTool) Execute(ctx context.Context, input json.RawMessage) (*ToolResult, error)
// 支持操作: goToDefinition, findReferences, hover, documentSymbol
```

**AskUserQuestionTool**
```go
type AskUserQuestionTool struct {
    sendFunc func(msg interface{}) error
}

func (t *AskUserQuestionTool) Name() string { return "AskUserQuestion" }
func (t *AskUserQuestionTool) Execute(ctx context.Context, input json.RawMessage) (*ToolResult, error)
```

---

### M5: 权限模块 (internal/permissions/)

**职责**: 实现完整的权限检查系统，支持四种权限模式。

#### 目录结构

```
internal/permissions/
├── mode.go           # 权限模式定义
├── checker.go        # 权限检查器
├── config.go         # 权限配置
└── middleware.go     # 权限中间件
```

#### 核心接口

```go
// PermissionMode 权限模式
type PermissionMode string

const (
    ModeDefault           PermissionMode = "default"
    ModeAcceptEdits       PermissionMode = "acceptEdits"
    ModePlan              PermissionMode = "plan"
    ModeBypassPermissions PermissionMode = "bypassPermissions"
)

// CheckResult 权限检查结果
type CheckResult struct {
    Behavior string      `json:"behavior"` // allow | deny | ask
    Message  string      `json:"message,omitempty"`
    Input    interface{} `json:"input,omitempty"`
}

// Checker 权限检查器
type Checker struct {
    mode   PermissionMode
    config *Config
    mu     sync.RWMutex
}

func (c *Checker) Check(toolName string, input interface{}) *CheckResult
func (c *Checker) SetMode(mode PermissionMode)
func (c *Checker) GetMode() PermissionMode
```

#### 权限模式行为

| 模式 | 文件读取 | 文件写入 | Bash | 网络 |
|------|----------|----------|------|------|
| `default` | allow | ask | ask | ask |
| `acceptEdits` | allow | allow | ask | ask |
| `plan` | allow | deny | deny | allow |
| `bypassPermissions` | allow | allow | allow | allow |

---

## 数据流

### 聊天消息处理流程

```
1. WebSocket 接收 chat 消息
2. 权限检查器验证当前模式
3. 构建系统提示词 (含 Skill 列表)
4. 调用 Claude API
5. 解析响应中的工具调用
6. 对每个工具调用:
   a. 权限检查
   b. 如需询问，发送 permission_request
   c. 等待用户响应或自动批准
   d. 执行工具 (本地或 MCP)
   e. 发送 tool_end 消息
7. 继续对话循环直到完成
8. 发送 done 消息
```

### Skill 调用流程

```
1. 用户发送 /skill-name args
2. WSHandler 识别 Skill 调用
3. SkillRegistry 查找 Skill
4. SkillExecutor 替换参数
5. 将展开内容作为用户消息处理
6. 继续正常聊天流程
```

### MCP 工具调用流程

```
1. Claude 返回 MCP 工具调用
2. 从工具名解析 serverName 和 toolName
3. Manager 获取对应连接
4. 发送 JSON-RPC 请求
5. 等待响应或超时
6. 返回工具结果
```

---

## WebSocket 协议扩展

### 新增消息类型

**Client → Server**

| 类型 | 用途 | Payload |
|------|------|---------|
| `skill_call` | 调用 Skill | `name`, `args` |
| `permission_response` | 权限响应 | `request_id`, `approved` |
| `set_permission_mode` | 设置权限模式 | `mode` |
| `agent_start` | 启动子代理 | `type`, `prompt`, `model` |
| `agent_cancel` | 取消子代理 | `agent_id` |
| `mcp_connect` | 连接 MCP | `config` |
| `mcp_disconnect` | 断开 MCP | `server_name` |

**Server → Client**

| 类型 | 用途 | Payload |
|------|------|---------|
| `permission_request` | 请求权限 | `request_id`, `tool`, `input`, `message` |
| `permission_mode_changed` | 模式变更 | `mode` |
| `agent_started` | 代理已启动 | `agent_id`, `type` |
| `agent_output` | 代理输出 | `agent_id`, `content` |
| `agent_done` | 代理完成 | `agent_id`, `output` |
| `mcp_connected` | MCP 已连接 | `server_name`, `tools` |
| `mcp_disconnected` | MCP 已断开 | `server_name` |
| `skill_expanded` | Skill 已展开 | `name`, `content` |

---

## 前端组件扩展

### PermissionModeSelector 组件

```jsx
// web/src/pages/Coworker/components/PermissionModeSelector.jsx
const PermissionModeSelector = ({ mode, onChange }) => {
  const modes = [
    { value: 'default', label: '标准模式' },
    { value: 'acceptEdits', label: '自动编辑' },
    { value: 'plan', label: '规划模式' },
    { value: 'bypassPermissions', label: '绕过权限' },
  ];
  return <Select value={mode} onChange={onChange} options={modes} />;
};
```

---

## 配置文件

### MCP 配置 (userdata/{user_id}/mcp.json)

```json
{
  "servers": {
    "ace-tool": {
      "type": "stdio",
      "command": "npx",
      "args": ["ace-tool-rs"],
      "env": {}
    }
  }
}
```

### Skill 配置目录

```
userdata/{user_id}/
├── skills/
│   ├── my-skill.md
│   └── another-skill.md
└── mcp.json
```

---

## 实施顺序

1. **Phase 1: 权限系统 (R5)**
   - 权限模式定义
   - 权限检查器
   - 前端权限对话框

2. **Phase 2: Skill 系统 (R1)**
   - Skill 解析器
   - Skill 注册表
   - 参数替换

3. **Phase 3: 高级工具 (R4)**
   - WebFetchTool
   - AskUserQuestionTool
   - WebSearchTool (可选)

4. **Phase 4: MCP 集成 (R2)**
   - Stdio 传输
   - 连接管理器
   - 工具动态注册

5. **Phase 5: 子代理系统 (R3)**
   - 代理类型定义
   - TaskTool
   - 并行执行

---

*Created: 2026-02-01*
*Status: Draft*
