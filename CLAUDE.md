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
│   │   └── websocket.go      # WebSocket handler
│   ├── client/
│   │   └── claude.go         # Anthropic API client
│   ├── session/
│   │   └── manager.go        # Session management
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
// Client → Server
{
  "type": "chat",
  "payload": {
    "message": "user message",
    "user_id": "user_123"
  }
}

// Server → Client
{
  "type": "text",           // or "done", "error"
  "payload": {
    "content": "response"
  }
}
```

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

*Last updated: 2026-01-31*
