# Tasks: ClaudeCLI 完整功能集成

## 实施顺序

按依赖关系排序，每个任务都是零决策可执行的。

---

## Phase 1: 权限系统 (R5) ✅

### T1.1: 创建权限模块目录结构 ✅

```bash
mkdir -p claudecli/internal/permissions
```

**输出文件:**
- `claudecli/internal/permissions/mode.go`
- `claudecli/internal/permissions/checker.go`
- `claudecli/internal/permissions/config.go`

### T1.2: 实现权限模式定义 ✅

**文件:** `claudecli/internal/permissions/mode.go`

```go
package permissions

type PermissionMode string

const (
    ModeDefault           PermissionMode = "default"
    ModeAcceptEdits       PermissionMode = "acceptEdits"
    ModePlan              PermissionMode = "plan"
    ModeBypassPermissions PermissionMode = "bypassPermissions"
)
```

### T1.3: 实现权限检查器 ✅

**文件:** `claudecli/internal/permissions/checker.go`

**接口:**
- `Check(toolName string, input interface{}) *CheckResult`
- `SetMode(mode PermissionMode)`
- `GetMode() PermissionMode`

### T1.4: 集成权限检查到 WSHandler ✅

**修改文件:** `claudecli/internal/api/websocket.go`

**变更:**
- 添加 `permChecker *permissions.Checker` 字段
- 在工具执行前调用 `permChecker.Check()`
- 处理 `permission_response` 消息

### T1.5: 前端权限对话框 ✅

**新建文件:** `web/src/pages/Coworker/components/PermissionDialog.jsx`

**功能:**
- 显示权限请求
- 允许/拒绝按钮
- 发送 `permission_response` 消息

---

## Phase 2: Skill 系统 (R1) ✅

### T2.1: 创建 Skills 模块目录 ✅

```bash
mkdir -p claudecli/internal/skills
```

### T2.2: 实现 Skill 解析器 ✅

**文件:** `claudecli/internal/skills/parser.go`

**功能:**
- 解析 YAML frontmatter
- 提取 name, description, allowed_tools
- 返回 Skill 结构体

### T2.3: 实现 Skill 注册表 ✅

**文件:** `claudecli/internal/skills/registry.go`

**接口:**
- `Register(skill *Skill) error`
- `Get(name string) (*Skill, bool)`
- `LoadFromDir(dir, source string) error`

### T2.4: 实现参数替换 ✅

**文件:** `claudecli/internal/skills/executor.go`

**规则:**
- `$0` → 完整参数
- `$1`, `$2` → 位置参数
- `$ARGUMENTS[N]` → 数组索引

### T2.5: 集成 Skill 到 WSHandler ✅

**修改文件:** `claudecli/internal/api/websocket.go`

**变更:**
- 添加 `skills *skills.SkillRegistry` 字段
- 处理 `/skill-name` 格式消息
- 展开 Skill 内容后继续对话

---

## Phase 3: 高级工具 (R4) ✅

### T3.1: 实现 WebFetchTool ✅

**文件:** `claudecli/internal/tools/webfetch.go`

**功能:**
- HTTP GET 获取网页
- HTML 转 Markdown
- 超时处理
- SSRF 防护（阻止 localhost/私有 IP）

### T3.2: 实现 AskUserQuestionTool ✅

**文件:** `claudecli/internal/tools/askuser.go`

**功能:**
- 发送问题到前端
- 等待用户响应
- 返回选择结果

### T3.3: 注册新工具 ✅

**修改文件:** `claudecli/init.go`

```go
registry.Register(tools.NewWebFetchTool())
registry.Register(tools.NewAskUserQuestionTool())
```

---

## Phase 4: MCP 集成 (R2) ✅

### T4.1: 创建 MCP 模块目录 ✅

```bash
mkdir -p claudecli/internal/mcp/transport
```

### T4.2: 实现 Transport 接口 ✅

**文件:** `claudecli/internal/mcp/transport/transport.go`

### T4.3: 实现 Stdio Transport ✅

**文件:** `claudecli/internal/mcp/transport/stdio.go`

### T4.4: 实现连接管理器 ✅

**文件:** `claudecli/internal/mcp/manager.go`

### T4.5: 集成 MCP 到 WSHandler ✅

**修改文件:** `claudecli/internal/api/websocket.go`

**变更:**
- 添加 `mcp *mcp.Manager` 字段
- 处理 `mcp_connect` / `mcp_disconnect` / `mcp_list` / `mcp_call` 消息

---

## Phase 5: 子代理系统 (R3) ✅

### T5.1: 创建 Agents 模块目录 ✅

```bash
mkdir -p claudecli/internal/agents
```

### T5.2: 实现代理类型定义 ✅

**文件:** `claudecli/internal/agents/types.go`

### T5.3: 实现代理执行器 ✅

**文件:** `claudecli/internal/agents/executor.go`

### T5.4: 实现 TaskTool ✅

**文件:** `claudecli/internal/tools/task_agent.go`

### T5.5: 集成到 WSHandler

**状态:** 基础框架完成，需要在 init.go 中注册 TaskAgentTool

---

## 验收检查清单

- [x] Phase 1: 权限系统可切换模式
- [x] Phase 2: Skill 可通过 /name 调用
- [x] Phase 3: WebFetch 可获取网页，AskUser 可交互
- [x] Phase 4: MCP Stdio 可连接
- [x] Phase 5: 子代理框架完成

---

*Created: 2026-02-01*
*Completed: 2026-02-01*
