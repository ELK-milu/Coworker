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
    │   ├── memory_extractor.go # 记忆提取触发器 (WS断开/压缩/手动)
    │   └── middleware.go       # 中间件
    ├── agent/
    │   └── types.go            # Agent 分层系统 (6个内置Agent)
    ├── client/
    │   ├── claude.go           # Anthropic API 客户端
    │   ├── handler.go          # 流式响应处理
    │   ├── stream.go           # SSE 流解析 (含连接重试)
    │   ├── retry.go            # API 重试机制 (指数退避+jitter)
    │   ├── convert.go          # 消息格式转换
    │   └── types.go            # API 类型定义
    ├── context/
    │   ├── context.go          # 上下文管理器
    │   ├── compress.go         # 消息压缩 (Microcompact)
    │   ├── prune.go            # Prune 层 (基于 token 修剪)
    │   ├── summary.go          # 摘要生成
    │   ├── summarizer.go       # AI 摘要生成器
    │   ├── session_memory.go   # Session Memory 管理
    │   ├── memory_generator.go # 记忆生成器
    │   └── tokens.go           # Token 估算
    ├── embedding/
    │   ├── client.go           # Embedding 客户端 (SiliconFlow/DashScope)
    │   ├── config.go           # Embedding 配置
    │   └── rerank.go           # Rerank 重排序
    ├── memory/
    │   ├── memory.go           # 记忆管理 (CRUD + 去重)
    │   ├── extractor.go        # AI 记忆提取器
    │   ├── retrieval.go        # 混合检索 (BM25 + Dense Vector)
    │   ├── semantic.go         # 语义搜索
    │   ├── bm25.go             # BM25 全文检索
    │   ├── milvus_amd64.go     # Milvus 向量数据库 (amd64)
    │   ├── milvus_amd64_ops.go # Milvus 操作 (amd64)
    │   ├── milvus_stub.go      # Milvus 桩 (非amd64平台)
    │   └── milvus_types.go     # Milvus 类型定义
    ├── permissions/
    │   ├── checker.go          # 权限检查器
    │   ├── ruleset.go          # Ruleset 评估引擎
    │   └── wildcard.go         # 通配符匹配
    ├── profile/
    │   ├── profile.go          # 用户画像管理
    │   └── learner.go          # 用户偏好学习
    ├── variable/
    │   ├── variable.go         # 变量系统
    │   └── builtin.go          # 内置变量
    ├── session/
    │   ├── manager.go          # 会话管理器
    │   ├── session.go          # 会话实体 (含记忆提取状态)
    │   └── persist.go          # 会话持久化
    ├── task/
    │   └── task.go             # 任务管理 (TodoList)
    ├── sandbox/
    │   ├── sandbox.go          # 沙箱隔离 (路径映射与安全验证)
    │   ├── microsandbox_client.go  # Microsandbox HTTP API 客户端
    │   └── pool.go             # SandboxPool 管理器 (任务绑定模式)
    ├── permission/
    │   └── memory.go           # 权限记忆管理
    ├── workspace/
    │   └── workspace.go        # 用户工作空间隔离
    ├── tools/
    │   ├── registry.go         # 工具注册表
    │   ├── factory.go          # 工具工厂 (自动验证+截断)
    │   ├── truncation.go       # 统一输出截断 (2000行/50KB)
    │   ├── filetime.go         # FileTime 外部修改检测
    │   ├── bash.go             # Bash 命令执行
    │   ├── read.go             # 文件读取 (含二进制检测)
    │   ├── write.go            # 文件写入
    │   ├── edit.go             # 文件编辑 (含diff摘要)
    │   ├── edit_replacer.go    # 9层 Replacer 链
    │   ├── glob.go             # 文件搜索 (含排除模式+结果限制)
    │   ├── grep.go             # 内容搜索 (含结果限制+分组)
    │   ├── memory_search.go    # 记忆搜索工具 (AI调用)
    │   ├── memory_save.go      # 记忆保存工具 (AI调用)
    │   └── memory_list.go      # 记忆列表工具 (AI调用)
    ├── loop/
    │   └── conversation.go     # 对话循环控制 (Doom Loop检测+步数限制)
    ├── prompt/
    │   ├── templates.go        # 系统提示词模板 (含记忆指南)
    │   ├── builder.go          # 提示词构建器 (含COWORKER.md嵌入)
    │   └── git.go              # Git 状态获取
    └── config/
        └── config.go           # 配置管理 (含Milvus/Embedding配置)
```

### 前端 (web/src/pages/Coworker/)

```
web/src/pages/Coworker/
├── index.jsx                   # 主页面 (WebSocket 连接)
├── styles.css                  # 主样式
├── services/
│   └── api.js                  # REST API 服务 (记忆/配置/画像)
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
    ├── InlineTaskCard.css
    ├── ConfigPanel.jsx         # 配置面板
    ├── ConfigPanel.css
    ├── MemoryPanel.jsx         # 记忆管理面板
    ├── MemoryPanel.css
    ├── PermissionDialog.jsx    # 权限确认对话框
    ├── PermissionModeSelector.jsx # 权限模式选择器
    ├── ProfileSettings.jsx     # 用户画像设置
    └── ProfileSettings.css
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

### 2026-02-08 (COWORKER.md 动态嵌入 + Bug修复)

- [x] COWORKER.md 动态嵌入系统提示词 (`websocket.go` + `builder.go`)
  - `buildUserSystemPrompt()` 调用 `workspace.LoadConfig(userID)` 加载用户自定义指令
  - 内容注入到 `PromptContext.CustomRules` 字段
  - `Build()` 方法将 CustomRules 包装为 `# User Custom Instructions (COWORKER.md)` 段落
  - 添加全分支诊断日志（成功/失败/空/workspace为nil）
- [x] stream.go 缺失 `time` 导入修复 (`client/stream.go`)
  - 上次重试机制修改遗漏的 import

### 2026-02-08 (流式回复重试bug + 标题生成503)

- [x] 流式调用重试逻辑重构 (`client/stream.go`)
  - 不再用 `retryWithBackoff` 包裹整个流消费循环
  - 仅在连接建立失败时重试，事件开始产出后不重试（避免重复内容）
  - `eventStarted` 标志位区分连接失败 vs 流中断
- [x] 标题生成 503 修复 (`client/stream.go`)
  - `CreateSimpleMessage` 直接使用主模型，移除 Haiku 硬编码
- [x] edit.go 缺失 `fmt` 导入修复 (`tools/edit.go`)

### 2026-02-08 (记忆提取去重)

- [x] Session 记忆提取状态追踪 (`session/session.go`)
  - 添加 `LastExtractedAt` / `LastExtractedMsgCount` 字段
  - `NeedsMemoryExtraction()` 检查是否有新内容需要提取
  - `MarkMemoryExtracted()` 标记已提取
- [x] 记忆去重机制 (`memory/memory.go`)
  - `ContentHash()` 内容哈希计算
  - `FindSimilar()` 查找相似记忆
  - `CreateOrMerge()` 创建或合并重复记忆
- [x] 提取逻辑优化 (`api/memory_extractor.go`)
  - 提取前检查会话状态，避免重复提取
  - 使用 `CreateOrMerge` 替代 `Create`
  - 提取后标记会话
- [x] 会话持久化增强 (`session/persist.go`)
  - 序列化/反序列化记忆提取状态字段

### 2026-02-08 (记忆系统集成 — AI工具 + 三场景自动提取)

- [x] 记忆 AI 工具 (`tools/memory_*.go`)
  - `MemorySearch` — AI 主动搜索相关记忆
  - `MemorySave` — AI 主动保存重要信息
  - `MemoryList` — AI 列出用户记忆
- [x] 记忆提取三场景 (`api/memory_extractor.go`)
  - WebSocket 断开时自动提取
  - 上下文压缩时自动提取
  - 用户手动触发 (`extract_memories` WebSocket 消息)
- [x] 系统提示词记忆集成 (`prompt/builder.go` + `templates.go`)
  - 相关记忆注入系统提示词 (`RelevantMemories` 字段)
  - 记忆工具使用指南 (`MemoryGuidelines` 模板)
- [x] WebSocket 记忆消息处理 (`api/websocket.go`)
  - 记忆搜索/保存/列表/提取消息处理

### 2026-02-08 (Milvus SDK v2 迁移 + 记忆系统基础设施)

- [x] Milvus SDK 迁移 (`memory/milvus_*.go`)
  - 从 `milvus-sdk-go/v2` 迁移到 `milvus/client/v2` 新 SDK
  - 更新 Milvus 版本到 v2.5.6
  - 平台条件编译: amd64 使用 Milvus，其他平台使用桩实现
- [x] Embedding 客户端 (`embedding/`)
  - 支持 SiliconFlow / DashScope 两种 Embedding 服务
  - Rerank 重排序支持
  - 可配置的模型和维度
- [x] Memory 包 (`memory/`)
  - 向量存储和混合搜索
  - BM25 + Dense Vector 混合检索
  - 语义搜索和全文检索
  - AI 记忆提取器
- [x] 配置扩展 (`config/config.go`)
  - Milvus 连接配置
  - Embedding 服务配置
- [x] 模块初始化集成 (`init.go`)
  - Memory/Embedding 客户端初始化
  - 记忆工具注册

### 2026-02-08 (OpenCode 对比优化 — 对话循环)

- [x] Doom Loop 检测 (`conversation.go`)
  - 记录最近 5 次工具调用的 (name, inputHash)
  - 连续 3 次相同调用 → 返回错误提示 AI 换策略
  - 参考 OpenCode `session/processor.ts:144-169`
- [x] 循环步数限制 (`conversation.go`)
  - `maxSteps` 默认 50，Agent 可配置
  - 达到限制时注入 MaxStepsReached 提示词（参考 OpenCode `prompt/max-steps.txt`）
  - 最后一次 API 调用不带工具定义，强制文本回复
- [x] Finish Reason 精确检查 (`conversation.go`)
  - `end_turn` → 结束循环
  - `tool_use` → 继续执行工具
  - `max_tokens` → 注入 system-reminder 包装的 continue 提示
  - 其他 → 结束循环
- [x] Context overflow 后检测 (`conversation.go`)
  - 工具执行后检查 `IsContextNearLimit()`，不仅在循环开始前
- [x] 会话 Busy 锁 (`websocket.go`)
  - `sync.Map` 防止同一会话并发处理
- [x] 会话标题自动生成 (`websocket.go`)
  - 使用 OpenCode 风格 TitlePrompt（≤50字符、同语言、无工具名）

### 2026-02-08 (OpenCode 对比优化 — 工具系统)

- [x] 统一工具输出截断 (`truncation.go` + `factory.go`)
  - 最大 2000 行 / 50KB
  - 截断后保存完整输出到文件
  - 7 天过期自动清理
  - 参考 OpenCode `tool/truncation.ts`
- [x] 工具工厂模式 (`factory.go`)
  - 自动输入验证 + 输出截断
  - 参考 OpenCode `tool/tool.ts` 的 `Tool.define()`
- [x] Edit 9 层 Replacer 链 (`edit_replacer.go`)
  - 补全至与 OpenCode 完全一致的 9 层
- [x] Edit diff 摘要输出 (`edit.go`)
  - 显示 +N/-N 行变更统计
  - 显示匹配方法（非精确匹配时）
- [x] FileTime 外部修改检测 (`filetime.go`)
  - Write/Edit 前检测文件是否被外部修改
- [x] Glob 结果限制 + 排序 (`glob.go`)
  - 100 结果硬限制
  - 按修改时间排序（最新优先）
  - 超限时返回截断警告
- [x] Grep 结果限制 + 分组 (`grep.go`)
  - 100 结果硬限制
  - 单行 2000 字符截断
  - 按文件分组输出格式
  - 超限时返回截断警告

### 2026-02-08 (OpenCode 对比优化 — 架构)

- [x] Agent 分层系统 (`agent/types.go`)
  - 6 个内置 Agent: build, plan, explore, general, compaction, title
  - 每个 Agent 独立的工具白名单和步数限制
- [x] 权限 Ruleset 评估引擎 (`permissions/`)
  - 规则格式: permission + pattern + action (allow/deny/ask)
  - 通配符匹配 (`permissions/wildcard.go`)
  - 层级合并: defaults → agent-specific → user-config
- [x] API 调用重试机制 (`client/retry.go`)
  - 指数退避 + jitter
  - 429/503/529/500 自动重试
  - 解析 retry-after header
- [x] 系统提示词增强 (`prompt/templates.go`)
  - MaxStepsReached、PlanModeReminder、BuildSwitchReminder
  - CompactionPrompt、TitlePrompt、SummaryPrompt

### 2026-02-05 (Microsandbox MicroVM 沙箱)

- [x] Microsandbox HTTP API 客户端 (`sandbox/microsandbox_client.go`)
  - JSON-RPC over HTTP 通信
  - StartSandbox/StopSandbox 沙箱生命周期
  - RunCommand 命令执行
  - Ping 健康检查
- [x] SandboxPool 沙箱池管理器 (`sandbox/pool.go`)
  - 任务绑定模式（非用户绑定）
  - 预热池机制，获取延迟 < 200ms
  - Acquire/Release 沙箱获取归还
  - Exec 自动获取执行归还
  - Stats 池统计信息
- [x] Bash 工具 Microsandbox 执行模式
  - `executeInMicrosandbox()` 方法
  - 优先级：Microsandbox > Local
  - `SetSandboxPool()` 注入沙箱池
- [x] Microsandbox 配置 (`config/config.go`)
  - `MICROSANDBOX_ENABLED` 启用/禁用
  - `MSB_SERVER_URL` 服务器地址
  - `MSB_API_KEY` API 密钥
  - `MSB_POOL_SIZE/MEMORY_MB/CPUS/EXEC_TIMEOUT`
- [x] 模块初始化集成 (`init.go`)
  - SandboxPool 创建和预热
  - 优雅关闭时停止沙箱池
- [x] 环境变量配置 (`.env.example`)
  - 远程云服务器模式说明
  - 本地开发模式说明

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

- [ ] LSP 诊断反馈 (Write/Edit 后返回编译错误，减少 AI 盲目修复)
- [ ] Bash 实时输出流 (长命令执行时实时更新 UI)
- [ ] 上下文压缩性能优化
- [x] 工作空间安全审计 (已通过沙箱隔离实现)
- [x] OpenCode 对比优化全量移植 (2026-02-08 完成)

### 低优先级

- [ ] Instruction 注入 (Read 文件时加载关联 CLAUDE.md 指令)
- [ ] Prometheus 监控指标
- [ ] 集成测试

---

*Last updated: 2026-02-08 (记忆系统 + OpenCode优化 + COWORKER.md嵌入修复)*
