# R3: 子代理系统规范

## 概述

实现子代理系统，支持专用代理类型和并行执行。

---

## 代理类型定义

```go
type AgentType struct {
    Name           string   `json:"name"`
    Description    string   `json:"description"`
    Tools          []string `json:"tools"`
    Model          string   `json:"model,omitempty"`
    PermissionMode string   `json:"permission_mode,omitempty"`
    ForkContext    bool     `json:"fork_context"`
}
```

---

## 内置代理类型

| 类型 | 工具 | 模型 | 用途 |
|------|------|------|------|
| `general-purpose` | 全部 | inherit | 复杂多步骤任务 |
| `Explore` | Glob, Grep, Read | haiku | 快速代码库探索 |
| `Plan` | 全部 | inherit | 架构规划设计 |

---

## Agent 实例

```go
type Agent struct {
    ID          string      `json:"id"`
    Type        string      `json:"type"`
    Description string      `json:"description"`
    Prompt      string      `json:"prompt"`
    Model       string      `json:"model"`
    Status      AgentStatus `json:"status"`
    StartTime   time.Time   `json:"start_time"`
    EndTime     *time.Time  `json:"end_time,omitempty"`
    Output      string      `json:"output,omitempty"`
    Error       string      `json:"error,omitempty"`
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

---

## 代理执行器

```go
type Executor struct {
    client *client.ClaudeClient
    tools  *tools.Registry
    agents map[string]*Agent
    mu     sync.RWMutex
}

func (e *Executor) Start(agentType, prompt, model string) (*Agent, error)
func (e *Executor) Resume(agentID string) (*Agent, error)
func (e *Executor) Cancel(agentID string) error
func (e *Executor) GetOutput(agentID string) (string, error)
func (e *Executor) List() []*Agent
```

---

## 模型别名

| 别名 | 实际模型 |
|------|----------|
| `sonnet` | claude-sonnet-4-20250514 |
| `opus` | claude-opus-4-5-20251101 |
| `haiku` | claude-haiku-4-5-20251001 |
| `inherit` | 继承父代理模型 |

---

## 验收标准

- [ ] 代理类型定义
- [ ] TaskTool 实现
- [ ] 模型别名解析
- [ ] 上下文隔离
- [ ] 并行执行支持
