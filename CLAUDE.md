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

*Last updated: 2026-02-01*
