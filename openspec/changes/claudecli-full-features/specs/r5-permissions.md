# R5: 权限系统规范

## 概述

实现完整的权限检查系统，支持四种权限模式。

---

## 权限模式

```go
type PermissionMode string

const (
    ModeDefault           PermissionMode = "default"
    ModeAcceptEdits       PermissionMode = "acceptEdits"
    ModePlan              PermissionMode = "plan"
    ModeBypassPermissions PermissionMode = "bypassPermissions"
)
```

---

## 模式行为矩阵

| 模式 | 读取 | 写入 | Bash | 网络 |
|------|------|------|------|------|
| default | allow | ask | ask | ask |
| acceptEdits | allow | allow | ask | ask |
| plan | allow | deny | deny | allow |
| bypassPermissions | allow | allow | allow | allow |

---

## 权限检查器

```go
type CheckResult struct {
    Behavior string      `json:"behavior"` // allow | deny | ask
    Message  string      `json:"message,omitempty"`
    Input    interface{} `json:"input,omitempty"`
}

type Checker struct {
    mode   PermissionMode
    config *Config
    mu     sync.RWMutex
}

func (c *Checker) Check(toolName string, input interface{}) *CheckResult
func (c *Checker) SetMode(mode PermissionMode)
func (c *Checker) GetMode() PermissionMode
```

---

## WebSocket 消息

**请求权限:**
```json
{
  "type": "permission_request",
  "payload": {
    "request_id": "uuid",
    "tool": "Bash",
    "input": {"command": "rm -rf"},
    "message": "确认执行此命令?"
  }
}
```

**响应权限:**
```json
{
  "type": "permission_response",
  "payload": {
    "request_id": "uuid",
    "approved": true
  }
}
```

---

## 验收标准

- [ ] 权限模式定义
- [ ] 权限检查器实现
- [ ] 前端权限对话框
- [ ] 模式切换功能
