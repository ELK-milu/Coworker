# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Coworker is a fork of [new-api](https://github.com/Calcium-Ion/new-api), an LLM Gateway and AI Asset Management System. This fork adds a **ClaudeCLI module** — a full Claude Code CLI experience delivered via WebSocket, with sandbox isolation, persistent memory, and multi-user support.

**Tech Stack:**
- Backend: Go 1.25.1 + Gin + GORM + Air (hot reload)
- Frontend: React 18 + Vite 5 + Semi-UI + Bun
- Database: PostgreSQL (primary), SQLite/MySQL (supported)
- Cache: Redis (optional, falls back to in-memory)
- Vector DB: Milvus 2.5.6 (Linux amd64/arm64 only, stub on other platforms)
- Deployment: Docker + Docker Compose (multi-stage build)

---

## Development Commands

### Docker Development (3 modes)

```bash
# Mode 1: Standard dev (frontend build + Air hot reload + Postgres + Redis + Milvus)
docker-compose -f docker-compose-dev.yml up
# Rebuild frontend after changes:
docker-compose -f docker-compose-dev.yml restart web-dev

# Mode 2: Fast dev with Vite HMR (frontend at :5173, backend at :3000)
docker-compose -f docker-compose-dev-fast.yml up

# Mode 3: Production
docker-compose up -d
```

**Access URLs:**
- Application: http://localhost:3000 (backend serves static files)
- Vite HMR (fast mode only): http://localhost:5173
- PostgreSQL: localhost:5432 (user: root, db: new-api)
- Redis: localhost:6379
- Milvus: localhost:19530

### Local Development (without Docker)

```bash
# Backend
go run main.go                           # Run directly
go build -o new-api                      # Build binary
go test ./...                            # All tests
go test -v ./model -run TestChannelCache # Single test
go fmt ./...                             # Format
go mod tidy                              # Clean deps

# Frontend
cd web
bun install          # Install deps
bun run dev          # Vite dev server
bun run build        # Production build
bun run lint         # Prettier check
```

### Hot Reload (Air)

Air uses polling mode for Windows Docker compatibility (`.air.toml`). After Go file changes, watch logs for "building..." confirmation:
```bash
docker logs --tail 20 new-api-dev
```

---

## Architecture Overview

### Request Flow

```
HTTP Request
  → Middleware (CORS, Auth, RateLimit, Distribute)
  → Router (api / relay / claudecli / video / web)
  → Controller → Service → Model → Database
                          ↓
                    External LLM APIs
```

### Backend Layers

| Layer | Location | Purpose |
|-------|----------|---------|
| Router | `router/` | Route groups: api, relay (/v1/*), claudecli (WebSocket), video, dashboard, web (static) |
| Controller | `controller/` | HTTP handlers, input validation, response formatting |
| Service | `service/` | Business logic: channel selection, quota billing, format conversion |
| Model | `model/` | GORM models, DB operations, Redis+memory cache |
| Middleware | `middleware/` | Auth (UserAuth/AdminAuth/TokenAuth), rate-limit, distributor, logger |
| Relay | `relay/` | LLM API proxying with 30+ provider adapters |
| ClaudeCLI | `claudecli/` | Claude Code CLI via WebSocket (independent module) |
| Common | `common/` | Constants, env loading, crypto, database type detection |
| DTO | `dto/` | Request/response types for relay formats |

### Frontend Architecture

**Provider hierarchy** (index.jsx):
```
StatusProvider → UserProvider → BrowserRouter → ThemeProvider → SemiLocaleWrapper → PageLayout
```

**Layout system:**
- `PageLayout.jsx` — Fixed header (64px) + collapsible sidebar + content area
- Console pages (`/console/*`) automatically get sidebar via `cardProPages` array
- All console pages must use container: `<div className='mt-[60px] px-2'>`
- `useSidebar` hook controls module visibility (merges admin + user config)

**State management:** React Context (User, Status, Theme) — no Redux.

**i18n:** i18next with 6 languages (en, zh, fr, ru, ja, vi). Chinese keys as source strings.

**Key frontend patterns:**
- Semi-UI components (ByteDance design system)
- Lucide-react + @lobehub/icons for icons
- CodeMirror for code editing (Playground)
- Lazy loading for heavy pages
- Axios for REST, native WebSocket for real-time

---

## Relay System (LLM Gateway)

The relay system proxies requests to 30+ LLM providers through a unified adapter pattern.

### Adapter Interface

Every provider implements the `Adaptor` interface (`relay/channel/adapter.go`):
```go
type Adaptor interface {
    Init(info *relaycommon.RelayInfo)
    GetRequestURL(info *relaycommon.RelayInfo) (string, error)
    SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error
    ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error)
    DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error)
    DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError)
    GetModelList() []string
    GetChannelName() string
}
```

### Adding a New Provider

1. Create `relay/channel/new_provider/` with: `adaptor.go`, `constants.go`, `dto.go`, `text.go`
2. Implement `Adaptor` interface (use `relay/channel/openai/` as reference)
3. Register in `relay/relay_adaptor.go` → `GetAdaptor()` switch
4. Add constant in `relay/constant/` if needed
5. Add routes in `router/relay-router.go` if special paths needed

### Relay Request Lifecycle

```
TokenAuth → Distribute (channel selection)
  → GenRelayInfo (build context)
  → EstimateRequestToken → ModelPriceHelper
  → PreConsumeQuota (deduct estimated quota)
  → [retry loop]
    → GetChannel → Adaptor.Init → ConvertRequest → DoRequest → DoResponse
    → On error: processChannelError, try next channel
  → PostConsumeQuota (settle actual vs estimated)
  → On failure: ReturnPreConsumedQuota
```

### Channel Selection Logic

**`service/channel_select.go`** — Smart channel selection:
- Selects by model name, user group, and priority level
- Retry count maps to priority level (retry 0 = highest priority channels)
- "auto" group iterates user's auto-groups list
- Affinity cache (`service/channel_affinity.go`) remembers preferred channels per fingerprint

**Channel features:**
- Multi-key support (newline-separated keys, modes: Random/Polling/Weighted)
- Parameter override (JSON map applied to converted request)
- Header override with placeholders: `{api_key}`, `{client_header:X-Name}`
- In-memory + Redis cache with periodic DB sync

### Streaming

SSE streaming handled by `relay/helper/stream_scanner.go`:
- Buffered scanner (64MB max)
- Ping keep-alive (`": PING\n\n"`) during long responses
- Per-provider stream parsers in each adapter

### Billing Formula

```
QuotaPerUnit = 500,000 (= $1 USD)

Text billing:
  quota = (inputTokens + outputTokens × completionRatio) × groupRatio × modelRatio

Cache billing (Claude):
  quota = (promptTokens + cachedTokens × cacheRatio + cacheCreationTokens × cacheCreationRatio) × groupRatio × modelRatio

Audio billing:
  quota = (inputText + outputText × completionRatio + inputAudio × audioRatio + outputAudio × audioRatio × audioCompletionRatio) × groupRatio × modelRatio

USD = quota / 500000
```

Key files: `service/quota.go`, `setting/ratio_setting/`, `dto/pricing.go`

---

## ClaudeCLI Module

The ClaudeCLI module (`claudecli/`) is an independent subsystem providing Claude Code CLI functionality via WebSocket.

### Module Initialization (`init.go`)

Initialization sequence: Config → Workspace → Claude Client → Session Manager → Tool Registry → Task Manager → Job Manager → Variable Manager → Memory Manager → Embedding Client → Milvus Client → Profile Manager → Sandbox Pool → EventBus → REST/WS Handlers → File Handler

### Subsystem Overview

| Subsystem | Location | Purpose |
|-----------|----------|---------|
| **WebSocket API** | `internal/api/websocket.go` | Message pump, 20+ message types |
| **REST API** | `internal/api/handler.go` | Sessions, memories, jobs, files, ratio config |
| **Anthropic Client** | `internal/client/` | SSE stream parsing, retry with exponential backoff+jitter, multi-endpoint (API key or auth token) |
| **Conversation Loop** | `internal/loop/conversation.go` | AI agent loop with Doom Loop detection, step limits (default 50), finish_reason routing |
| **Context Manager** | `internal/context/` | Microcompact (tool output pruning) → Prune (token-based trimming, 40K window) → Summarize (AI summary) |
| **Tool Registry** | `internal/tools/` | Bash, Read, Write, Edit, Glob, Grep, Memory tools, Task tools, StructuredOutput |
| **Sandbox** | `internal/sandbox/sandbox.go` | Virtual path mapping (`/workspace/` ↔ real path), traversal protection |
| **Sandbox Pool** | `internal/sandbox/pool.go` | nsjail/Microsandbox task-binding pool, Acquire/Release/Exec |
| **Session Manager** | `internal/session/` | File-based persistence under `userdata/{user_id}/sessions/` |
| **Memory System** | `internal/memory/` | Hybrid retrieval (BM25 + Dense Vector), Milvus integration, AI extraction on compact/session-end |
| **Embedding Client** | `internal/embedding/` | SiliconFlow / DashScope providers, rerank support |
| **EventBus** | `internal/eventbus/` | Sync + async handlers: BeforeCompact, TurnCompleted, SessionEnd |
| **Task Manager** | `internal/task/` | TodoList CRUD, file-backed, AI can create/update via tools |
| **Job Scheduler** | `internal/job/` | Cron-like scheduled tasks with AI execution |
| **Permissions** | `internal/permissions/` | Ruleset evaluation, wildcard matching, internal tool whitelist |
| **Prompt Builder** | `internal/prompt/` | Dynamic system prompt: agent type, COWORKER.md rules, memories, git status |
| **Agent Types** | `internal/agent/` | 6 built-in agents (build, plan, explore, general, compaction, title) with per-agent tool whitelists |
| **Skills** | `internal/skills/` | Skill definitions (YAML frontmatter + content), parser, registry, executor |
| **MCP** | `internal/mcp/` | Model Context Protocol via stdio transport |
| **Profile** | `internal/profile/` | User preference learning |
| **Variables** | `internal/variable/` | Variable system with builtins |

### Conversation Loop (`loop/conversation.go`)

```
RunLoop:
  while steps < maxSteps:
    1. Build messages + tools + system prompt
    2. Call CreateMessageStream()
    3. Check finish_reason:
       - "end_turn" → return response
       - "tool_use" → execute tools
       - "max_tokens" → inject continue prompt
    4. For tool_use:
       - Doom Loop check: last 3 calls identical (name + SHA256(input))? → error
       - Execute tools in goroutines
       - Check context near limit → auto compact
    5. Continue loop
```

### Tool System

**Factory pattern** (`tools/factory.go`): Auto input validation + output truncation (2000 lines / 50KB). Full output saved to `/tmp` with 7-day TTL.

**Edit tool** (`tools/edit.go` + `edit_replacer.go`): 9-layer replacer chain for fuzzy matching:
1. SimpleReplacer (exact)
2. LineTrimmedReplacer (trailing whitespace)
3. LeadingWhitespaceReplacer
4. BlockAnchorReplacer (Levenshtein fuzzy)
5. WhitespaceNormalizedReplacer
6. IndentNormalizedReplacer (tab↔space)
7-9. Additional normalization layers

**Sandbox execution priority:** nsjail (container) > Microsandbox (MicroVM) > Local (path-isolated)

### Context Compression Pipeline

```
Trigger: context > 70% capacity (auto) or manual /compact

1. EventBus.Emit(BeforeCompact) → Memory extraction (sync, blocks)
2. Microcompact: Clean tool results older than last 3
3. Prune: Keep recent 40K tokens, remove old tool outputs
4. Summarize: AI-generated summary of pruned content
5. EventBus.Emit(TurnCompleted) → Fact extraction (async)
```

### Memory System Architecture

```
AI Tool Call (MemorySearch/MemorySave)
     ↓
memory.Manager (CRUD + dedup by content hash)
     ↓
Hybrid Retrieval: BM25 full-text + Dense Vector (embedding)
     ↓
Storage: JSON files (disk) + Milvus vectors (Linux amd64/arm64)
     ↓
Injection: Relevant memories → system prompt
```

EventBus triggers:
- `BeforeCompact` (sync) → Extract window summary before context loss
- `TurnCompleted` (async) → Extract discrete facts
- `SessionEnd` (async) → Final summary + remaining facts

### WebSocket Protocol

**Endpoint:** `/claudecli/ws` (or `/coworker/ws`)

**Client → Server:** `chat`, `abort`, `load_history`, `list_sessions`, `delete_session`, `list_files`, `create_folder`, `delete_file`, `rename_file`, `task_create`, `task_update`, `task_list`, `compact`, `context_stats`, `extract_memories`

**Server → Client:** `text`, `thinking`, `tool_start`, `tool_end`, `status`, `done`, `error`, `history`, `sessions_list`, `files_list`, `tasks_list`, `task_created`, `task_updated`, `compact_done`, `context_stats`

All messages use format: `{ "type": "<type>", "payload": { ... } }`

### Coworker Frontend (`web/src/pages/Coworker/`)

Main page (index.jsx, ~1200 lines) with:
- **Left panel:** Chat messages (MessageBubble, ToolCallCard) + input area
- **Right panel (560px):** SessionSidebar with 6 tabs — History, Files (Colab-style), Tasks (Claude Code-style), Config, Memory, Jobs
- **Status bar:** Connection state, model, cost, tokens, context %

Key components: `MessageBubble.jsx`, `ToolCallCard.jsx`, `SessionSidebar.jsx`, `FileExplorer.jsx`, `TaskList.jsx`, `ConfigPanel.jsx`, `MemoryPanel.jsx`, `InlineTaskCard.jsx`

Token billing: Real-time from API stream (`status` events), cost from REST log query (`/api/log/self/`) after `done`.

---

## Database

### Multi-Database Support

Detection via `SQL_DSN` prefix:
- `postgresql://` → PostgreSQL (recommended for production)
- `local` or empty → SQLite (default, file: `one-api.db`)
- Otherwise → MySQL

### SQL Dialect Differences

PostgreSQL uses quoted identifiers (`"group"`, `"key"`), MySQL/SQLite uses backticks. GORM handles this automatically, but raw SQL must use the correct quoting.

### Auto-Migration

All models auto-migrate on startup (`model/main.go`). Root user (root/123456) created automatically on empty database.

### Connection Pool Defaults

```go
SetMaxIdleConns(100)      // SQL_MAX_IDLE_CONNS
SetMaxOpenConns(1000)     // SQL_MAX_OPEN_CONNS
SetConnMaxLifetime(60s)   // SQL_MAX_LIFETIME
```

---

## Environment Variables

### Core

```bash
SQL_DSN=postgresql://user:pass@host:5432/dbname  # Database connection
LOG_SQL_DSN=                                      # Optional separate log DB
REDIS_CONN_STRING=redis://localhost:6379           # Redis (optional)
PORT=3000                                          # Server port
GIN_MODE=release                                   # release or debug
SESSION_SECRET=<must-change>                       # Session encryption key
CRYPTO_SECRET=                                     # Defaults to SESSION_SECRET
TZ=Asia/Shanghai                                   # Timezone
```

### ClaudeCLI

```bash
ANTHROPIC_API_KEY=sk-ant-...            # Claude API key
ANTHROPIC_AUTH_TOKEN=                    # Alternative: web auth token
ANTHROPIC_API_BASE_URL=                 # Custom base URL
CLAUDE_MODEL=claude-sonnet-4-20250514   # Default model
WORKSPACE_BASE_PATH=./userdata          # User workspace root
```

### Sandbox Isolation

```bash
# nsjail (container-based, requires privileged Docker)
NSJAIL_ENABLED=true
NSJAIL_CONTAINER_NAME=nsjail-sandbox
NSJAIL_MAX_CONCURRENT=50
NSJAIL_MEMORY_MB=512
NSJAIL_EXEC_TIMEOUT=120

# Microsandbox (MicroVM, Linux only)
MICROSANDBOX_ENABLED=false
MSB_SERVER_URL=http://linux-server:5555
MSB_API_KEY=
MSB_POOL_SIZE=5
MSB_MEMORY_MB=512
MSB_CPUS=1
MSB_EXEC_TIMEOUT=120
```

### Vector Memory

```bash
MILVUS_ENABLED=false
MILVUS_HOST=milvus
MILVUS_PORT=19530
MILVUS_COLLECTION=claude_memories
EMBEDDING_PROVIDER=siliconflow           # or dashscope
EMBEDDING_MODEL=BAAI/bge-large-zh-v1.5
EMBEDDING_DIMENSION=1024
SILICONFLOW_API_KEY=
DASHSCOPE_API_KEY=
```

### Relay & Performance

```bash
RELAY_TIMEOUT=0                # 0 = unlimited
STREAMING_TIMEOUT=300          # Stream response timeout (seconds)
SYNC_FREQUENCY=60              # Channel sync interval (seconds)
MEMORY_CACHE_ENABLED=true      # In-process cache fallback
BATCH_UPDATE_ENABLED=true      # Batch DB updates
BATCH_UPDATE_INTERVAL=5
DEBUG=false                    # Debug logging
ENABLE_PPROF=false             # CPU profiling
```

### Load Order

1. Command-line flags (`--port`, `--log-dir`)
2. `.env` file (via godotenv)
3. OS environment variables
4. Code defaults (`common/constants.go`)

---

## Adding New Console Pages

1. Create `web/src/pages/NewPage/index.jsx` with `<div className='mt-[60px] px-2'>` container
2. Add route in `web/src/App.jsx` (wrap with `<PrivateRoute>` or `<AdminRoute>`)
3. Add sidebar entry in `web/src/components/layout/SiderBar.jsx` (`routerMap` + menu array)
4. Add path to `cardProPages` array in `web/src/components/layout/PageLayout.jsx`
5. Add module config in `web/src/hooks/common/useSidebar.js` (`DEFAULT_ADMIN_CONFIG`)
6. Add icon mapping in `web/src/helpers/render.jsx` (`getLucideIcon()`)

---

## Common Issues and Solutions

### Semi-UI Component Imports

```javascript
// WRONG: TextArea will be undefined
import { Input } from '@douyinfe/semi-ui';
const { TextArea } = Input;

// CORRECT: Import directly
import { TextArea } from '@douyinfe/semi-ui';
```

### React Closure Trap in WebSocket Handlers

State values captured in WebSocket `onmessage` become stale. Always use `useRef` to track values accessed in closures:

```javascript
const currentPathRef = useRef(currentPath);
useEffect(() => { currentPathRef.current = currentPath; }, [currentPath]);

const handleMessage = (data) => {
  loadFiles(currentPathRef.current);  // Always latest value
};
```

### ClaudeCLI Async Requirements

All blocking operations in WebSocket handlers must run in goroutines. WebSocket writes must use `sync.Mutex`:

```go
// Async handler pattern
func (h *WSHandler) handleTaskCreate(conn *websocket.Conn, payload json.RawMessage) {
    go func() {
        t, err := h.tasks.Create(...)
        h.sendJSON(conn, result)  // sendJSON uses h.connMu.Lock()
    }()
}
```

### Sandbox Path Resolution

All ClaudeCLI tools must resolve paths through the sandbox to enforce workspace isolation:

```go
sb, _ := ctx.Value(types.SandboxKey).(*sandbox.Sandbox)
realPath, err := sb.ToReal(virtualPath)  // /workspace/src → /app/userdata/uid/workspace/src
// ../../../etc/passwd → ErrPathTraversal
```

### Milvus Build on Windows

Milvus SDK only compiles on Linux amd64/arm64. Other platforms use `milvus_stub.go` (build tags: `!linux || (!amd64 && !arm64)`).

### Hot Reload Not Working (Docker on Windows)

Air must use polling mode. Verify `.air.toml` has `poll = true` and `poll_interval = 1000`.

---

## Deployment

### Docker Multi-Stage Build

The `Dockerfile` uses 3 stages:
1. **Bun builder** — Installs deps, builds React frontend
2. **Go builder** — Compiles static Go binary (CGO_ENABLED=0, multi-arch)
3. **Runtime** — debian:bookworm-slim with just the binary + CA certs

### CI/CD

GitHub Actions (`.github/workflows/docker-image-alpha.yml`):
- Matrix build: amd64 (ubuntu-latest) + arm64 (ubuntu-24.04-arm)
- Multi-arch manifest pushed to Docker Hub + GHCR
- Version format: `alpha-YYYYMMDD-{short_sha}`

### nsjail Sandbox Image

`docker/nsjail/Dockerfile` — Debian-based with pre-installed Python packages (numpy, pandas, Pillow, openpyxl, etc.) and nsjail binary for process isolation.

---

## Git Workflow

**Commit message format:** `<type>: <subject>` (types: feat, fix, docs, style, refactor, perf, test, chore)

Only commit when explicitly requested by user.

---

## Key Conventions

### User Roles

```go
RoleGuestUser  = 0
RoleCommonUser = 1
RoleAdminUser  = 10
RoleRootUser   = 100
```

### Quota System

`QuotaPerUnit = 500,000` (= $1 USD). Model and group ratios stored in `setting/ratio_setting/`. Exposed via `/api/ratio_config` (if enabled) and `/coworker/ratio_config`.

### Error Pattern

```go
types.NewAPIError{Err, Code, HttpCode, SkipRetry}
```

Relay errors are normalized from provider-specific formats to OpenAI error format. Retry decisions based on HTTP status code and `SkipRetry` flag.

### Rate Limiting

Redis sorted set pattern: `rateLimit:{mark}:{clientIP}` with timestamp-based sliding window.

### Multi-Tenancy

User isolation via workspace directories (`userdata/{user_id}/`), session files, task lists, and memory stores — all keyed by user ID.

### Config Hot Reload

System options stored in DB `options` table, periodically synced via `model.SyncOptions()`. Channel cache refreshed via `model.SyncChannelCache()`.
