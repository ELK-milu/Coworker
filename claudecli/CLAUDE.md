# ClaudeCLI Module - CLAUDE.md

本文档专注于 ClaudeCLI 模块及其前端代码的开发规范和进度跟踪。

---

## 模块概述

ClaudeCLI 是 Coworker 项目的核心模块，提供 Claude Code CLI 功能的 Web 版本实现。

**技术栈：**
- 后端：Go + Gin + gorilla/websocket
- 前端：React 18 + Semi-UI
- 通信：WebSocket 实时双向通信

---

## 目录结构

### 后端 (claudecli/)

```
claudecli/
├── init.go                     # 模块初始化入口
├── pkg/types/
│   ├── message.go              # 消息类型定义
│   └── tool.go                 # 工具接口定义
└── internal/
    ├── api/
    │   ├── websocket.go        # WebSocket 消息处理 (核心)
    │   ├── handler.go          # REST API 处理器
    │   ├── file_handler.go     # 文件上传下载
    │   └── middleware.go       # 中间件
    ├── client/
    │   ├── claude.go           # Anthropic API 客户端
    │   ├── handler.go          # 流式响应处理
    │   ├── stream.go           # SSE 流解析
    │   ├── convert.go          # 消息格式转换
    │   └── types.go            # API 类型定义
    ├── context/
    │   ├── context.go          # 上下文管理器
    │   ├── compress.go         # 消息压缩 (Microcompact)
    │   ├── prune.go            # Prune 层 (基于 token 修剪)
    │   ├── summary.go          # 摘要生成
    │   ├── summarizer.go       # AI 摘要生成器
    │   ├── session_memory.go   # Session Memory 管理
    │   └── tokens.go           # Token 估算
    ├── session/
    │   ├── manager.go          # 会话管理器
    │   ├── session.go          # 会话实体
    │   └── persist.go          # 会话持久化
    ├── task/
    │   └── task.go             # 任务管理 (TodoList)
    ├── sandbox/
    │   └── sandbox.go          # 沙箱隔离 (路径映射与安全验证)
    ├── permission/
    │   └── memory.go           # 权限记忆管理
    ├── workspace/
    │   └── workspace.go        # 用户工作空间隔离
    ├── tools/
    │   ├── registry.go         # 工具注册表
    │   ├── bash.go             # Bash 命令执行
    │   ├── read.go             # 文件读取 (含二进制检测)
    │   ├── write.go            # 文件写入
    │   ├── edit.go             # 文件编辑
    │   ├── edit_replacer.go    # 多层 Replacer 链
    │   ├── glob.go             # 文件搜索 (含排除模式)
    │   └── grep.go             # 内容搜索
    ├── loop/
    │   └── conversation.go     # 对话循环控制
    ├── prompt/
    │   ├── templates.go        # 系统提示词模板
    │   ├── builder.go          # 提示词构建器
    │   └── git.go              # Git 状态获取
    └── config/
        └── config.go           # 配置管理
```

### 前端 (web/src/pages/Coworker/)

```
web/src/pages/Coworker/
├── index.jsx                   # 主页面 (WebSocket 连接)
├── styles.css                  # 主样式
└── components/
    ├── MessageBubble.jsx       # 消息气泡 (含 Thinking 折叠)
    ├── ToolCallCard.jsx        # 工具调用卡片
    ├── SessionSidebar.jsx      # 会话侧边栏 (DeepSeek 风格)
    ├── SessionSidebar.css
    ├── FileExplorer.jsx        # 文件管理器 (Colab 风格)
    ├── FileExplorer.css
    ├── TaskList.jsx            # 任务列表
    ├── TaskList.css
    ├── InlineTaskCard.jsx      # 内联任务卡片
    └── InlineTaskCard.css
```

---

## WebSocket 协议

### 连接端点

```
ws://localhost:3000/claudecli/ws
```

### 消息格式

所有消息使用 JSON 格式：
```json
{
  "type": "<消息类型>",
  "payload": { ... }
}
```

### Client → Server 消息

| 类型 | 用途 | Payload |
|------|------|---------|
| `chat` | 发送聊天消息 | `message`, `session_id`, `user_id`, `working_path` |
| `abort` | 中断当前响应 | - |
| `load_history` | 加载会话历史 | `session_id` |
| `list_sessions` | 获取会话列表 | `user_id` |
| `delete_session` | 删除会话 | `session_id` |
| `list_files` | 列出文件 | `user_id`, `path` |
| `create_folder` | 创建文件夹 | `user_id`, `path` |
| `delete_file` | 删除文件 | `user_id`, `path` |
| `rename_file` | 重命名文件 | `user_id`, `path`, `new_name` |
| `task_create` | 创建任务 | `user_id`, `list_id`, `subject`, `description` |
| `task_update` | 更新任务 | `user_id`, `list_id`, `task_id`, `status` |
| `task_list` | 获取任务列表 | `user_id`, `list_id` |
| `compact` | 压缩上下文 | `session_id` |

### Server → Client 消息

| 类型 | 用途 | Payload |
|------|------|---------|
| `text` | 文本流 | `content`, `session_id` |
| `thinking` | 思考过程 | `content`, `session_id` |
| `tool_start` | 工具开始 | `name`, `tool_id`, `input` |
| `tool_end` | 工具结束 | `tool_id`, `result`, `is_error` |
| `status` | 状态更新 | `model`, `total_tokens`, `context_percent` |
| `done` | 响应完成 | - |
| `error` | 错误 | `error` |
| `history` | 历史消息 | `messages` |
| `sessions_list` | 会话列表 | `sessions` |
| `files_list` | 文件列表 | `files`, `path` |
| `tasks_list` | 任务列表 | `tasks` |

---

## 开发规范

### 异步处理要求

所有可能阻塞的操作必须在 goroutine 中执行：

```go
// ✅ 正确：异步执行
func (h *WSHandler) handleTaskCreate(conn *websocket.Conn, payload json.RawMessage) {
    go func() {
        t, err := h.tasks.Create(...)
        h.sendJSON(conn, result)
    }()
}

// ❌ 错误：同步阻塞
func (h *WSHandler) handleTaskCreate(conn *websocket.Conn, payload json.RawMessage) {
    t, err := h.tasks.Create(...)  // 会阻塞其他用户
    h.sendJSON(conn, result)
}
```

### WebSocket 线程安全

使用 `sync.Mutex` 保护并发写入：

```go
func (h *WSHandler) sendJSON(conn *websocket.Conn, v interface{}) error {
    h.connMu.Lock()
    defer h.connMu.Unlock()
    return conn.WriteJSON(v)
}
```

### 前端 React Closure 陷阱

WebSocket handler 中的状态值会被闭包捕获，使用 `useRef` 同步：

```javascript
// ✅ 正确：使用 ref 获取最新值
const currentPathRef = useRef(currentPath);
useEffect(() => {
  currentPathRef.current = currentPath;
}, [currentPath]);

const handleMessage = (data) => {
  loadFiles(currentPathRef.current);  // 始终获取最新值
};
```

### 工具路径解析

所有工具必须使用沙箱 (`sandbox.Sandbox`) 解析路径，确保安全隔离：

```go
// 获取沙箱
sb, _ := ctx.Value(types.SandboxKey).(*sandbox.Sandbox)

// 使用沙箱解析路径
path, err := sb.ToReal(in.FilePath)
if err != nil {
    return &types.ToolResult{Success: false, Error: err.Error()}, nil
}
```

### 沙箱隔离机制 (sandbox/)

沙箱提供用户工作空间的安全隔离：

**路径映射规则：**
```
输入: /workspace/src/main.go  → 输出: /app/userdata/user_xxx/workspace/src/main.go
输入: src/main.go             → 输出: /app/userdata/user_xxx/workspace/src/main.go
输入: /etc/passwd             → 错误: ErrPathOutsideSandbox
输入: ../../../etc/passwd     → 错误: ErrPathTraversal
```

**核心方法：**
- `ToReal(virtualPath)` - 虚拟路径 → 真实路径（带安全验证）
- `ToVirtual(realPath)` - 真实路径 → 虚拟路径
- `VirtualizePaths(paths)` - 批量虚拟化路径列表
- `VirtualizeOutput(output)` - 虚拟化输出字符串中的路径

---

## 已完成功能

### 2026-02-04 (工作空间沙箱隔离)

- [x] 沙箱包 (`sandbox/sandbox.go`)
  - 路径映射：虚拟路径 `/workspace` ↔ 真实路径
  - 路径遍历攻击防护
  - 系统路径访问拦截
  - 输出路径虚拟化
- [x] 动态系统提示词
  - 移除静态系统提示词构建
  - 每个用户独立的系统提示词上下文
  - 使用虚拟路径 `/workspace` 而非真实路径
- [x] 工具沙箱集成
  - Read/Write/Edit 工具使用沙箱解析路径
  - Glob/Grep 工具输出路径虚拟化
  - Bash 工具危险路径检查和输出虚拟化
- [x] Git 状态隔离
  - 只检查用户工作空间内的 git 状态
  - 虚拟化 git 状态中的文件路径
- [x] 单元测试覆盖 (`sandbox_test.go`)

### 2026-02-04 (OpenCode 学习改进)

- [x] Edit 工具多层 Replacer 链 (`edit_replacer.go`)
  - SimpleReplacer (精确匹配)
  - LineTrimmedReplacer (行尾空格容忍)
  - LeadingWhitespaceReplacer (前导空格容忍)
  - BlockAnchorReplacer (首尾行锚定 + Levenshtein 模糊匹配)
  - WhitespaceNormalizedReplacer (空格归一化)
  - IndentNormalizedReplacer (Tab/空格互换)
- [x] Read 工具增强
  - 二进制文件检测 (拒绝读取二进制文件)
  - 模糊文件建议 (文件不存在时提供相似文件名)
- [x] 会话压缩 Prune 层 (`prune.go`)
  - 基于 token 数修剪旧工具输出
  - 保护最近 40K tokens 的消息
- [x] Glob 排除模式
  - 默认排除 node_modules、.git、vendor 等
  - 支持用户自定义排除模式
- [x] 权限记忆系统 (`permission/memory.go`)
  - 支持 "always allow/deny" 选项
  - 通配符匹配 (如 "git *")
  - 持久化到 userdata/{user_id}/permissions.json

### 2026-02-02

- [x] AI 任务工具集 (TaskCreate, TaskUpdate, TaskList, TaskGet)
- [x] 任务变更实时同步到前端 (task_changed 事件)
- [x] 任务拖拽排序功能
- [x] InlineTaskCard 交互增强 (状态切换、展开详情)
- [x] Task 模型添加 Order 字段支持排序

### 2026-02-01

- [x] 系统提示词模块 (`prompt/`)
- [x] 前端文件路径同步到后端工作目录
- [x] Task 工具执行结果实时同步前端
- [x] 修复文件创建路径错误
- [x] Thinking 信息折叠面板样式
- [x] 任务管理功能 (TodoList)
- [x] 上下文压缩 (Microcompact)
- [x] Session Memory
- [x] 多用户异步支持

### 更早

- [x] WebSocket 实时通信
- [x] 会话持久化
- [x] DeepSeek 风格会话侧边栏
- [x] Google Colab 风格文件管理器
- [x] 用户工作空间隔离
- [x] 基础工具 (Bash, Read, Write, Edit, Glob, Grep)

---

## 待办事项

### 高优先级

- [ ] 单元测试覆盖
- [ ] WebSocket 错误处理改进
- [ ] 结构化日志

### 中优先级

- [ ] 上下文压缩性能优化
- [x] 工作空间安全审计 (已通过沙箱隔离实现)
- [ ] 更多工具支持 (LSP, WebFetch 等)

### 低优先级

- [ ] Prometheus 监控指标
- [ ] 集成测试

---

*Last updated: 2026-02-04 (沙箱隔离功能)*
