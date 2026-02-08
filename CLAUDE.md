# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Coworker is a fork of [new-api](https://github.com/Calcium-Ion/new-api), an LLM Gateway and AI Asset Management System. This fork adds a ClaudeCLI module that provides Claude Code CLI functionality via WebSocket.

**Tech Stack:**
- Backend: Go 1.25.1 + Gin + GORM + Air (hot reload)
- Frontend: React 18 + Vite + Semi-UI + Bun
- Database: PostgreSQL (dev), SQLite/MySQL (supported)
- Cache: Redis
- Deployment: Docker + Docker Compose

---

## Development Commands

### Start Development Environment

```bash
# Start all services (backend, frontend build, postgres, redis)
docker-compose -f docker-compose-dev.yml up

# Start in background
docker-compose -f docker-compose-dev.yml up -d

# View logs
docker logs -f new-api-dev  # Backend

# Rebuild frontend after code changes
docker-compose -f docker-compose-dev.yml restart web-dev
```

**Access URL:**
- Application: http://localhost:3000 (Backend serves static files)
- PostgreSQL: localhost:5432
- Redis: localhost:6379

**Note:** Frontend uses build mode (not dev server). After modifying frontend code, run `docker-compose -f docker-compose-dev.yml restart web-dev` to rebuild.

### Backend Development

```bash
# Run backend locally (without Docker)
go run main.go

# Build
go build -o new-api

# Run tests
go test ./...

# Run specific test
go test -v ./model -run TestChannelCache

# Update dependencies
go mod tidy
go mod download

# Format code
go fmt ./...

# Lint (if golangci-lint installed)
golangci-lint run
```

### Frontend Development

```bash
cd web

# Install dependencies
bun install

# Start dev server
bun run dev

# Build for production
bun run build

# Preview production build
bun run preview

# Lint
bun run lint
```

---

## Architecture Overview

### Backend Architecture

**Request Flow:**
```
HTTP Request → Middleware Chain → Router → Controller → Service → Model → Database
                                                    ↓
                                              External APIs
```

**Key Layers:**

1. **Router** (`router/`)
   - `main.go` - Main router setup, integrates all sub-routers
   - `api-router.go` - Core API routes (/api/user, /api/channel, /api/token, etc.)
   - `claudecli-router.go` - ClaudeCLI WebSocket routes
   - `relay-router.go` - LLM API relay routes (/v1/*, /mj/*, /pg/*)

2. **Controller** (`controller/`)
   - HTTP request handlers
   - Input validation and response formatting
   - Calls service layer for business logic

3. **Service** (`service/`)
   - Business logic implementation
   - Channel selection strategies
   - Quota management
   - Token counting and billing

4. **Model** (`model/`)
   - GORM database models
   - Database operations (CRUD)
   - Cache management (Redis + in-memory)

5. **Middleware** (`middleware/`)
   - `auth.go` - Authentication (UserAuth, AdminAuth, RootAuth)
   - `rate-limit.go` - Rate limiting
   - `distributor.go` - Request distribution to channels
   - `logger.go` - Request logging

**Core Data Models:**
- `User` - User accounts with quota management
- `Channel` - LLM API channels (OpenAI, Claude, etc.)
- `Token` - API tokens with group/model restrictions
- `Log` - Request logs with billing info
- `Midjourney` - Midjourney task tracking
- `Task` - Async task queue

### Frontend Architecture

**Component Structure:**
```
web/src/
├── pages/              # Page components (Token, Channel, Coworker, etc.)
├── components/
│   ├── layout/        # PageLayout, SiderBar, HeaderBar
│   ├── table/         # Data tables
│   └── ...
├── hooks/             # Custom React hooks
│   └── common/
│       └── useSidebar.js  # Sidebar configuration
├── helpers/           # Utility functions
│   ├── auth.jsx       # PrivateRoute, AdminRoute
│   └── render.jsx     # Icon rendering
├── context/           # React Context (User, Status, Theme)
└── App.jsx            # Main app with routes
```

**Layout System:**
- `PageLayout.jsx` controls global layout (sidebar + header)
- Console pages (`/console/*`) automatically get sidebar
- Pages must use standard container: `<div className='mt-[60px] px-2'>`
- `cardProPages` array in PageLayout.jsx controls footer visibility

### ClaudeCLI Module

**Location:** `claudecli/` (independent module)

**Architecture:**
```
claudecli/
├── init.go                    # Module initialization
├── internal/
│   ├── api/
│   │   ├── handler.go        # REST API handlers
│   │   ├── websocket.go      # WebSocket handler
│   │   └── file_handler.go   # File upload/download handlers
│   ├── client/
│   │   └── claude.go         # Anthropic API client
│   ├── context/              # Context management (compact)
│   │   ├── context.go        # Context manager
│   │   ├── compress.go       # Message compression
│   │   ├── tokens.go         # Token estimation
│   │   └── summary.go        # Summary generation
│   ├── session/
│   │   └── manager.go        # Session management
│   ├── task/
│   │   └── task.go           # Task management (todo list)
│   ├── workspace/
│   │   └── workspace.go      # User workspace management
│   ├── sandbox/              # Microsandbox MicroVM integration
│   │   ├── microsandbox_client.go  # HTTP API client
│   │   ├── pool.go           # SandboxPool manager (task-binding)
│   │   └── sandbox.go        # Path mapping & security
│   ├── tools/                # Claude Code tools
│   │   ├── bash.go
│   │   ├── read.go
│   │   ├── write.go
│   │   ├── edit.go
│   │   ├── glob.go
│   │   └── grep.go
│   └── loop/
│       └── conversation.go   # Conversation loop
└── pkg/types/                # Type definitions
```

**Integration Points:**
- Controller: `controller/claudecli.go`
- Router: `router/claudecli-router.go`
- Frontend: `web/src/pages/Coworker/index.jsx`

**WebSocket Protocol:**
```javascript
// Client → Server (Chat)
{ "type": "chat", "payload": { "message": "...", "session_id": "...", "user_id": "...", "mode": "normal" } }
{ "type": "abort" }  // Abort current response

// Client → Server (Session Management)
{ "type": "load_history", "payload": { "session_id": "..." } }
{ "type": "list_sessions", "payload": { "user_id": "..." } }
{ "type": "delete_session", "payload": { "session_id": "..." } }

// Client → Server (File Management)
{ "type": "list_files", "payload": { "user_id": "...", "path": "..." } }
{ "type": "create_folder", "payload": { "user_id": "...", "path": "..." } }
{ "type": "delete_file", "payload": { "user_id": "...", "path": "..." } }
{ "type": "rename_file", "payload": { "user_id": "...", "path": "...", "new_name": "..." } }

// Server → Client (Chat)
{ "type": "text", "payload": { "content": "...", "session_id": "..." } }
{ "type": "tool_start", "payload": { "name": "...", "tool_id": "...", "input": {...} } }
{ "type": "tool_end", "payload": { "tool_id": "...", "result": "...", "is_error": false } }
{ "type": "status", "payload": { "model": "...", "total_tokens": 0, "context_percent": 0 } }
{ "type": "done" }
{ "type": "error", "payload": { "error": "..." } }

// Server → Client (Session/File)
{ "type": "history", "payload": { "messages": [...] } }
{ "type": "sessions_list", "payload": { "sessions": [...] } }
{ "type": "files_list", "payload": { "files": [...], "path": "..." } }
{ "type": "folder_created", "payload": { "success": true, "path": "..." } }
{ "type": "file_deleted", "payload": { "success": true, "path": "..." } }
{ "type": "file_renamed", "payload": { "success": true, "old_path": "...", "new_name": "..." } }

// Client → Server (Task Management)
{ "type": "task_create", "payload": { "user_id": "...", "list_id": "default", "subject": "...", "description": "..." } }
{ "type": "task_get", "payload": { "user_id": "...", "list_id": "default", "task_id": "..." } }
{ "type": "task_update", "payload": { "user_id": "...", "list_id": "default", "task_id": "...", "status": "..." } }
{ "type": "task_list", "payload": { "user_id": "...", "list_id": "default" } }

// Client → Server (Context Compact)
{ "type": "compact", "payload": { "session_id": "..." } }
{ "type": "context_stats", "payload": { "session_id": "..." } }

// Server → Client (Task)
{ "type": "tasks_list", "payload": { "tasks": [...] } }
{ "type": "task_created", "payload": { "success": true, "task": {...} } }
{ "type": "task_updated", "payload": { "success": true, "task": {...} } }

// Server → Client (Context)
{ "type": "compact_done", "payload": { "success": true, "stats": {...} } }
{ "type": "context_stats", "payload": { "stats": {...} } }
```

### Coworker Frontend Features

**Location:** `web/src/pages/Coworker/`

**Components:**
```
web/src/pages/Coworker/
├── index.jsx                    # Main page with WebSocket connection
├── styles.css                   # Main styles
└── components/
    ├── MessageBubble.jsx        # Chat message display
    ├── ToolCallCard.jsx         # Tool call visualization
    ├── SessionSidebar.jsx       # DeepSeek-style session sidebar
    ├── SessionSidebar.css
    ├── FileExplorer.jsx         # Google Colab-style file manager
    ├── FileExplorer.css
    ├── TaskList.jsx             # Claude Code-style task list
    └── TaskList.css
```

**File Manager Features (Google Colab Style):**
- File/folder listing with icons by file type
- Drag-and-drop file upload
- Create new folders (inline input)
- Rename files/folders (inline editing)
- Delete files/folders with confirmation
- Download files (direct download, no new window)
- Download folders as ZIP
- Breadcrumb navigation
- Right-click context menu

**Session Sidebar Features (DeepSeek Style):**
- Session list grouped by time (Today, Yesterday, Last 7 days, etc.)
- New chat button
- Session switching with history loading
- Delete sessions
- Collapsible sidebar

**Task List Features (Claude Code Style):**
- Create tasks with subject and description
- Task status: pending → in_progress → completed
- Progress bar showing completion percentage
- Task dependencies (blocks/blockedBy)
- Inline task status updates
- Delete tasks
- Persistent storage per user

**Context Compact Features:**
- Automatic context compression when usage exceeds 70%
- Token estimation for messages (English ~3.5 chars/token, Chinese ~2 chars/token)
- Code block compression (keep 60% head + 40% tail, max 50 lines)
- Tool output compression (max 2000 chars)
- Summary generation for old messages
- Manual compact trigger via WebSocket

**User Workspace Isolation:**
- Each user has isolated workspace: `./userdata/{user_id}/workspace/`
- User ID stored in localStorage: `coworker_user_id`
- Session ID stored in localStorage: `coworker_session_id`

---

## Adding New Console Pages

**Complete Checklist:**

1. **Create page component** (`web/src/pages/NewPage/index.jsx`)
   ```jsx
   const NewPage = () => {
     return (
       <div className='mt-[60px] px-2'>
         {/* Content */}
       </div>
     );
   };
   ```

2. **Add route** (`web/src/App.jsx`)
   ```jsx
   <Route path='/console/newpage' element={
     <PrivateRoute><NewPage /></PrivateRoute>
   } />
   ```

3. **Add sidebar button** (`web/src/components/layout/SiderBar.jsx`)
   - Add to `routerMap`: `newpage: '/console/newpage'`
   - Add to appropriate menu array (workspaceItems, etc.)

4. **Add to PageLayout** (`web/src/components/layout/PageLayout.jsx`)
   - Add `/console/newpage` to `cardProPages` array

5. **Add module config** (`web/src/hooks/common/useSidebar.js`)
   - Add to `DEFAULT_ADMIN_CONFIG` under appropriate section

6. **Add icon** (`web/src/helpers/render.jsx`)
   - Import icon from lucide-react
   - Add case in `getLucideIcon()` function

---

## Common Issues and Solutions

### Semi-UI Component Imports

**❌ Wrong:**
```javascript
import { Input } from '@douyinfe/semi-ui';
const { TextArea } = Input;  // TextArea will be undefined
```

**✅ Correct:**
```javascript
import { TextArea } from '@douyinfe/semi-ui';
```

**Rule:** Import Semi-UI components directly from the main package, not as sub-properties.

### Hot Reload Not Working (Docker on Windows)

**Problem:** File changes not detected in Docker container.

**Solution:** Air must use polling mode on Windows.

**File:** `.air.toml`
```toml
[build]
  poll = true
  poll_interval = 1000  # Check every second
```

**Verify:** Check logs for "building..." after file changes:
```bash
docker logs --tail 20 new-api-dev
```

### New Page Missing Sidebar

**Symptoms:** Page loads but sidebar doesn't appear.

**Checklist:**
- [ ] Page uses `<div className='mt-[60px] px-2'>` container (not custom Layout)
- [ ] Path added to `PageLayout.jsx` → `cardProPages` array
- [ ] Route configured in `App.jsx`
- [ ] Sidebar button added in `SiderBar.jsx`
- [ ] Module config added in `useSidebar.js`

### React Closure Trap in WebSocket Handlers

**Problem:** State values captured in WebSocket `onmessage` handler become stale.

**Symptoms:** After operations (create folder, delete file, rename), UI jumps to root directory instead of staying in current path.

**Cause:** `handleWebSocketMessage` function captures initial state values (e.g., `currentPath = ''`) and never updates.

**❌ Wrong:**
```javascript
const handleWebSocketMessage = (data) => {
  // currentPath is captured at function definition time
  loadFilesList(wsRef.current, currentPath);  // Always uses stale value!
};
```

**✅ Correct:**
```javascript
const currentPathRef = useRef(currentPath);

useEffect(() => {
  currentPathRef.current = currentPath;  // Sync to ref on every change
}, [currentPath]);

const handleWebSocketMessage = (data) => {
  loadFilesList(wsRef.current, currentPathRef.current);  // Always gets latest value
};
```

**Rule:** Use `useRef` to track state values that need to be accessed in closures (WebSocket handlers, event listeners, timers).

---

## Database Migrations

**Location:** `model/main.go` → `init()` function

**Auto-migration on startup:**
```go
err := db.AutoMigrate(
    &User{},
    &Channel{},
    &Token{},
    // ... all models
)
```

**Manual migration:**
```bash
# Connect to database
docker exec -it postgres-dev psql -U root -d new-api

# Run SQL
ALTER TABLE users ADD COLUMN new_field VARCHAR(255);
```

---

## Environment Variables

**Key Variables:**

```bash
# Database
SQL_DSN=postgresql://user:pass@host:5432/dbname

# Redis
REDIS_CONN_STRING=redis://host:6379

# Server
PORT=3000
GIN_MODE=release  # or debug

# Session
SESSION_SECRET=random-secret-key

# ClaudeCLI Module
ANTHROPIC_API_KEY=sk-ant-...
WORKING_DIR=/data/users
CLAUDE_MODEL=claude-sonnet-4-20250514
```

**Load order:**
1. `.env` file (if exists)
2. Environment variables
3. Default values in code

---

## Git Workflow

**Only commit when explicitly requested by user.**

**Commit message format:**
```
<type>: <subject>

<body>

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>
```

**Types:** feat, fix, docs, style, refactor, perf, test, chore

---

## Project-Specific Notes

### Channel System

Channels represent different LLM API providers (OpenAI, Claude, Gemini, etc.). The system:
- Automatically selects channels based on model, group, and priority
- Tracks usage and quotas per channel
- Supports failover and load balancing
- Caches channel data in Redis for performance

**Key files:**
- `service/channel_select.go` - Channel selection logic
- `service/channel_affinity.go` - Affinity-based routing
- `model/channel_cache.go` - Channel caching

### Token System

Tokens are API keys for accessing the gateway. Features:
- Group-based model restrictions
- Quota limits (unlimited or fixed)
- Expiration dates
- Usage tracking

**Key files:**
- `controller/token.go`
- `middleware/auth.go` - Token authentication

### Billing System

Pay-per-use billing with configurable model pricing:
- Token-based billing (input/output tokens)
- Cache billing support (prompt caching)
- Quota deduction on request completion
- Detailed logs for auditing

**Key files:**
- `service/quota.go`
- `service/token_counter.go`
- `dto/pricing.go`

---

## Recent Changes (2026-02-05)

### Microsandbox MicroVM 沙箱隔离

实现了基于 Microsandbox 的 MicroVM 级别沙箱隔离，采用任务绑定模式（Task-Binding）最大化资源利用率。

**架构设计：**
- 任务绑定模式：沙箱按需分配，执行完立即归还，不绑定用户
- 预热池：预先创建沙箱，获取延迟 < 200ms
- 硬件级隔离：MicroVM 提供比容器更强的安全隔离

**新增文件:**
- `claudecli/internal/sandbox/microsandbox_client.go` - Microsandbox HTTP API 客户端
  - `StartSandbox()` - 启动沙箱
  - `StopSandbox()` - 停止沙箱
  - `RunCommand()` - 执行命令
  - `Ping()` - 健康检查
- `claudecli/internal/sandbox/pool.go` - SandboxPool 沙箱池管理器
  - `Start()` - 启动池并预热沙箱
  - `Acquire()` - 获取空闲沙箱（阻塞等待）
  - `Release()` - 归还沙箱到池中
  - `Exec()` - 执行命令（自动获取/归还）
  - `Stats()` - 返回池统计信息

**修改文件:**
- `claudecli/internal/config/config.go` - 新增 MicrosandboxConfig 配置结构
- `claudecli/init.go` - 添加 SandboxPool 初始化和优雅关闭
- `claudecli/internal/tools/bash.go` - 新增 `executeInMicrosandbox()` 方法
- `.env.example` - 添加 Microsandbox 配置说明
- `.env` - 添加 Microsandbox 配置（默认禁用）

**执行优先级：**
```
Microsandbox (MicroVM) > Local (开发模式)
```

**环境变量配置：**
```bash
# 启用 Microsandbox
MICROSANDBOX_ENABLED=true

# 远程云服务器模式（推荐生产环境）
MSB_SERVER_URL=http://your-linux-server:5555
MSB_API_KEY=your-api-key

# 本地开发模式（--dev 无需 API Key）
MSB_SERVER_URL=http://127.0.0.1:5555
MSB_API_KEY=

# 池配置
MSB_POOL_SIZE=5          # 预热沙箱数量
MSB_MEMORY_MB=512        # 每沙箱内存
MSB_CPUS=1               # 每沙箱 CPU
MSB_EXEC_TIMEOUT=120     # 执行超时（秒）
```

**部署方式：**

1. **远程云服务器（推荐）**
   - 后端运行在 Windows，Microsandbox 部署在 Linux 云服务器
   - 安装：`curl -sSL https://get.microsandbox.dev | sh`
   - 启动：`msb server start --detach`
   - 生成 API Key：`msb server keygen --expire 3mo`

2. **本地开发模式（仅 Linux/macOS）**
   - 启动开发模式：`msb server start --dev`
   - 无需 API Key

**容量提升：**

| 指标 | gVisor 容器 | Microsandbox |
|------|------------|--------------|
| 内存开销/沙箱 | 50-100MB | 64MB |
| 4核4G 并发 | 5-6 用户 | **50 用户** |
| 启动时间 | 2-5秒 | <200ms |
| 隔离级别 | 用户态内核 | MicroVM |

---

## Microsandbox 配置

### 镜像下载（中国大陆服务器）

由于中国大陆服务器无法直接访问 Docker Hub，需要使用 SSH 反向隧道代理方式下载镜像。

**前提条件：**
- 本地 Windows 机器运行 Clash 代理（监听 127.0.0.1:7890）
- 华为云服务器已安装 microsandbox

**步骤 1：建立 SSH 反向隧道**

在 Windows PowerShell 中运行（保持窗口打开）：
```powershell
ssh -R 7890:127.0.0.1:7890 root@<华为云服务器IP>
```

**步骤 2：测试代理连通性**

在华为云服务器上：
```bash
curl -x http://127.0.0.1:7890 https://registry-1.docker.io/v2/
# 返回 {"errors":[{"code":"UNAUTHORIZED"...}]} 说明代理工作正常
```

**步骤 3：拉取镜像**

```bash
# 设置代理环境变量并拉取镜像
HTTP_PROXY=http://127.0.0.1:7890 \
HTTPS_PROXY=http://127.0.0.1:7890 \
msb pull sandboxes.io/python

# 也可以拉取其他镜像
HTTP_PROXY=http://127.0.0.1:7890 \
HTTPS_PROXY=http://127.0.0.1:7890 \
msb pull alpine
```

**步骤 4：启动 msbserver**

```bash
HTTP_PROXY=http://127.0.0.1:7890 \
HTTPS_PROXY=http://127.0.0.1:7890 \
msbserver --dev
```

**注意事项：**
- SSH 隧道断开后代理失效，需要重新建立
- 不要设置 `OCI_REGISTRY_DOMAIN` 为中国镜像源（不支持）
- `--dev` 模式用于开发，生产环境需要配置 key

---

## Recent Changes (2026-02-02)

### Tool Execution Enhancement

增强了工具执行功能，添加执行时间显示和失败日志。

**修改文件:**
- `claudecli/pkg/types/tool.go` - 扩展 ToolResult 结构，添加 ElapsedMs、TimeoutMs、TimedOut、Metadata 字段
- `claudecli/internal/tools/bash.go` - Bash 工具返回执行时间信息
- `claudecli/internal/loop/conversation.go` - 扩展 LoopEvent，添加工具失败日志
- `claudecli/internal/api/websocket.go` - tool_end 消息添加执行时间字段
- `web/src/pages/Coworker/components/ToolCallCard.jsx` - 显示执行时间和超时标签
- `web/src/pages/Coworker/index.jsx` - 处理新的 WebSocket 字段

**功能:**
- Bash 工具返回执行时间 (elapsed_ms)、超时设置 (timeout_ms)、是否超时 (timed_out)
- 前端工具卡片显示执行耗时
- 超时命令显示橙色"超时"标签
- 后端日志记录工具执行失败详情

### Structured Output Tool

新增结构化输出验证工具，支持 JSON Schema 验证。

**新增文件:**
- `claudecli/internal/tools/structured_output.go` - JSON Schema 验证工具

**修改文件:**
- `claudecli/internal/tools/registry.go` - 添加 SetStructuredOutputSchema、ClearStructuredOutputSchema 方法
- `claudecli/internal/api/websocket.go` - 添加 set_output_schema、clear_output_schema 消息处理
- `claudecli/init.go` - 注册 StructuredOutputTool
- `go.mod` - 添加 github.com/santhosh-tekuri/jsonschema/v5 依赖

**功能:**
- 动态设置 JSON Schema
- 验证 AI 输出是否符合指定格式
- WebSocket 消息支持设置/清除 schema

---

## Recent Changes (2026-02-01)

### Task Management Feature (Claude Code Style)

集成了 Claude Code CLI 的任务管理功能，支持将用户需求分解为 Todolist。

**新增文件:**
- `claudecli/internal/task/task.go` - 任务管理后端模块
- `web/src/pages/Coworker/components/TaskList.jsx` - 任务列表组件
- `web/src/pages/Coworker/components/TaskList.css` - 任务列表样式

**修改文件:**
- `claudecli/init.go` - 添加 task 包导入和 Tasks 字段
- `claudecli/internal/api/websocket.go` - 添加任务相关 WebSocket 消息处理
- `web/src/pages/Coworker/index.jsx` - 添加任务状态和 WebSocket 处理
- `web/src/pages/Coworker/components/SessionSidebar.jsx` - 添加任务标签页

### Context Compact Feature

集成了 Claude Code CLI 的上下文压缩功能，支持自动/手动压缩上下文。

**修改文件:**
- `claudecli/internal/api/websocket.go` - 添加 compact 和 context_stats 消息处理

### Microcompact Feature (New)

实现了轻量级压缩功能，清理旧的工具调用结果，保留最近 3 个工具结果。

**新增功能:**
- 工具白名单机制（Read, Bash, Grep, Glob, WebSearch, WebFetch）
- 自动清理旧工具结果，替换为占位符
- 与摘要压缩配合使用

**修改文件:**
- `claudecli/internal/context/compress.go` - 添加 Microcompact 函数
- `claudecli/internal/context/context.go` - 集成 Microcompact 到 Compact 流程

### Session Memory Feature (New)

实现了 Session Memory 功能，将对话摘要保存到结构化模板文件。

**新增文件:**
- `claudecli/internal/context/session_memory.go` - Session Memory 管理器

**功能:**
- 结构化模板保存关键信息（任务规格、文件、工作流、错误等）
- 章节 token 估算和警告
- 格式化用于系统提示

### AI Summary Generation (New)

增强了 AI 摘要生成功能，添加压缩边界标记。

**新增文件:**
- `claudecli/internal/context/summarizer.go` - 摘要生成器

**功能:**
- 生成对话摘要提示词
- 创建压缩边界标记
- 格式化摘要消息

### Inline Task Display (New)

在对话框中显示任务状态，而不仅仅在侧边栏显示。

**新增文件:**
- `web/src/pages/Coworker/components/InlineTaskCard.jsx` - 内联任务卡片
- `web/src/pages/Coworker/components/InlineTaskCard.css` - 样式

**修改文件:**
- `web/src/pages/Coworker/components/MessageBubble.jsx` - 集成任务卡片
- `web/src/pages/Coworker/index.jsx` - 传递任务到消息组件

### Async Implementation for Multi-User Support (New)

由于 ClaudeCLI 需要支持多用户同时操作以提供线上 HTTPS 服务，所有可能阻塞的操作必须使用异步实现。

**设计原则:**
- 所有文件 I/O 操作必须在 goroutine 中执行
- WebSocket 写操作使用 `sync.Mutex` 保证线程安全
- 避免在主 handler 中执行耗时操作

**异步模式示例:**

```go
// ✅ 正确：异步执行文件 I/O
func (h *WSHandler) handleTaskCreate(conn *websocket.Conn, payload json.RawMessage) {
    var req TaskCreateRequest
    if err := json.Unmarshal(payload, &req); err != nil {
        h.sendError(conn, "invalid request")
        return
    }

    // 异步执行文件 I/O 操作
    go func() {
        t, err := h.tasks.Create(req.UserID, req.ListID, req.Subject, req.Description, req.ActiveForm)
        if err != nil {
            h.sendError(conn, "failed to create task: "+err.Error())
            return
        }
        h.sendJSON(conn, map[string]interface{}{
            "type": "task_created",
            "payload": map[string]interface{}{
                "success": true,
                "task":    t,
            },
        })
    }()
}

// ❌ 错误：同步执行会阻塞其他用户
func (h *WSHandler) handleTaskCreate(conn *websocket.Conn, payload json.RawMessage) {
    // 直接执行文件 I/O，会阻塞
    t, err := h.tasks.Create(...)
    h.sendJSON(conn, ...)
}
```

**已异步化的 Handler:**
- `handleTaskCreate` - 任务创建
- `handleTaskUpdate` - 任务更新
- `handleTaskList` - 任务列表
- `handleListFiles` - 文件列表
- `handleCreateFolder` - 创建文件夹
- `handleDeleteFile` - 删除文件
- `handleRenameFile` - 重命名文件
- `handleCompact` - 上下文压缩
- `handleChat` - 聊天（已使用 goroutine）

**线程安全的 WebSocket 写入:**

```go
type WSHandler struct {
    writeMu sync.Mutex  // 保护 WebSocket 写操作
}

func (h *WSHandler) sendJSON(conn *websocket.Conn, data interface{}) {
    h.writeMu.Lock()
    defer h.writeMu.Unlock()
    conn.WriteJSON(data)
}
```

---

## Project Statistics (2026-02-01 Scan)

### File Count Summary

| Category | Count | Location |
|----------|-------|----------|
| Go Files | 488 | Backend |
| JS/JSX/TS/TSX Files | 421 | Frontend |
| Frontend Pages | 62 | `web/src/pages/` |
| Relay Channels | 148 | `relay/channel/` |
| Service Files | 41 | `service/` |
| Model Files | 29 | `model/` |
| Controller Files | 30+ | `controller/` |
| Middleware Files | 18 | `middleware/` |
| Router Files | 7 | `router/` |
| ClaudeCLI Files | 30+ | `claudecli/` |

### Module Structure

```
E:\PythonWorks\Coworker\
├── main.go                 # Application entry point
├── router/                 # Route definitions (7 files)
│   ├── main.go            # Main router setup
│   ├── api-router.go      # Core API routes
│   ├── claudecli-router.go # ClaudeCLI WebSocket routes
│   ├── relay-router.go    # LLM API relay routes
│   ├── video-router.go    # Video API routes
│   ├── dashboard.go       # Dashboard routes
│   └── web-router.go      # Static file serving
├── controller/            # HTTP handlers (30+ files)
├── service/               # Business logic (41 files)
├── model/                 # Database models (29 files)
├── middleware/            # Request middleware (18 files)
├── common/                # Shared utilities (20+ files)
├── dto/                   # Data transfer objects (20+ files)
├── relay/                 # LLM API relay system
│   └── channel/           # Provider adapters (148 files)
│       ├── ali/           # Alibaba Cloud
│       ├── aws/           # AWS Bedrock
│       ├── baidu/         # Baidu Wenxin
│       ├── claude/        # Anthropic Claude
│       ├── cloudflare/    # Cloudflare Workers AI
│       ├── codex/         # GitHub Copilot
│       └── ...            # 20+ providers
├── claudecli/             # Claude Code CLI module (30+ files)
│   ├── init.go            # Module initialization
│   └── internal/
│       ├── api/           # REST & WebSocket handlers
│       ├── client/        # Anthropic API client
│       ├── context/       # Context compression
│       ├── session/       # Session management
│       ├── task/          # Task management
│       ├── workspace/     # User workspace
│       ├── tools/         # CLI tools (bash, read, write, etc.)
│       ├── loop/          # Conversation loop
│       └── prompt/        # System prompt builder
└── web/                   # React frontend
    └── src/
        ├── pages/         # 35 page directories
        │   ├── Coworker/  # Claude Code CLI UI
        │   ├── Channel/   # Channel management
        │   ├── Token/     # Token management
        │   ├── Dashboard/ # Analytics dashboard
        │   ├── Playground/# API playground
        │   └── ...
        ├── components/    # Reusable components
        ├── hooks/         # Custom React hooks
        ├── helpers/       # Utility functions
        └── context/       # React Context providers
```

### Supported LLM Providers (relay/channel/)

| Provider | Directory | Features |
|----------|-----------|----------|
| OpenAI | `openai/` | Chat, Embeddings, Images, Audio |
| Anthropic Claude | `claude/` | Chat, Vision |
| Google Gemini | `gemini/` | Chat, Vision |
| AWS Bedrock | `aws/` | Claude, Titan |
| Azure OpenAI | `azure/` | All OpenAI models |
| Alibaba Qwen | `ali/` | Chat, Images |
| Baidu Wenxin | `baidu/`, `baidu_v2/` | Chat |
| Tencent Hunyuan | `tencent/` | Chat |
| Zhipu GLM | `zhipu/` | Chat |
| Moonshot | `moonshot/` | Chat |
| DeepSeek | `deepseek/` | Chat |
| Mistral | `mistral/` | Chat |
| Cohere | `cohere/` | Chat, Rerank |
| Cloudflare | `cloudflare/` | Workers AI |
| Ollama | `ollama/` | Local models |
| GitHub Copilot | `codex/` | Code completion |
| Midjourney | `midjourney/` | Image generation |
| Suno | `suno/` | Music generation |

### Frontend Pages (web/src/pages/)

| Page | Route | Access |
|------|-------|--------|
| Home | `/` | Public |
| Dashboard | `/console` | Private |
| Coworker | `/console/coworker` | Private |
| Channel | `/console/channel` | Admin |
| Token | `/console/token` | Private |
| User | `/console/user` | Admin |
| Log | `/console/log` | Private |
| Pricing | `/pricing` | Configurable |
| Playground | `/console/playground` | Private |
| Midjourney | `/console/midjourney` | Private |
| Task | `/console/task` | Private |
| Setting | `/console/setting` | Admin |
| Model | `/console/models` | Admin |
| Deployment | `/console/deployment` | Admin |

---

## Coverage Report

### Well-Documented Modules

| Module | Coverage | Notes |
|--------|----------|-------|
| ClaudeCLI | 95% | Fully documented with WebSocket protocol |
| Coworker Frontend | 90% | Components and features documented |
| Router | 85% | All routes documented |
| Architecture | 80% | Request flow and layers documented |

### Areas Needing Documentation

| Area | Priority | Recommendation |
|------|----------|----------------|
| Relay Channel Adapters | Medium | Add per-provider configuration docs |
| Billing/Quota System | Medium | Document pricing calculation logic |
| Model Deployment | Low | Document io.net integration |
| OAuth Providers | Low | Document each OAuth flow |

---

## Recommended Next Steps

### Immediate (High Priority)

1. **Test Coverage**: Add unit tests for `claudecli/internal/` modules
2. **Error Handling**: Improve error messages in WebSocket handlers
3. **Logging**: Add structured logging for debugging

### Short-term (Medium Priority)

1. **Documentation**: Add API documentation for relay endpoints
2. **Performance**: Profile and optimize context compression
3. **Security**: Audit workspace isolation implementation

### Long-term (Low Priority)

1. **Refactoring**: Extract common patterns in relay channel adapters
2. **Monitoring**: Add Prometheus metrics for ClaudeCLI usage
3. **Testing**: Add integration tests for WebSocket protocol

---

*Last updated: 2026-02-05*
