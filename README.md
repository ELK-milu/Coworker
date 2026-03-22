<div align="center">

![Coworker](/web/public/logo.png)

# Coworker

**Web-based, Multi-tenant OpenClaw — 把 Claude Code 搬进浏览器**

<p align="center">
  <strong>简体中文</strong>
</p>

<p align="center">
  <a href="https://github.com/ELK-milu/Coworker/releases/latest">
    <img src="https://img.shields.io/github/v/release/ELK-milu/Coworker?color=brightgreen&include_prereleases" alt="release">
  </a>
  <a href="https://raw.githubusercontent.com/ELK-milu/Coworker/main/LICENSE">
    <img src="https://img.shields.io/github/license/ELK-milu/Coworker?color=brightgreen" alt="license">
  </a>
  <a href="https://hub.docker.com/r/elkmilu/coworker">
    <img src="https://img.shields.io/docker/pulls/elkmilu/coworker?color=blue" alt="docker pulls">
  </a>
</p>

</div>

---

## What is Coworker?

[OpenClaw](https://github.com/anthropics/openclaw) 把 AI Agent 带进了 WhatsApp 和 Slack，[Claude Code](https://docs.anthropic.com/en/docs/claude-code) 把它带进了终端。**Coworker 把它带进了浏览器 —— 支持多租户、开箱即用。**

Coworker 是一个开源的 Web 端 AI Agent 平台，提供完整的 Claude Code CLI 体验：对话循环、工具调用、沙箱隔离、持久记忆、文件管理 —— 全部通过 WebSocket 实时交付到浏览器。一次部署，团队共用。

> **与 OpenClaw 的关系：** OpenClaw 面向个人，通过消息应用交互；Coworker 面向团队，通过 Web UI 交互。如果说 OpenClaw 是你口袋里的 AI 助手，Coworker 就是团队共享的 AI 工作站。

---

## 核心特性

### AI Agent 引擎

- **完整对话循环** — 与 Claude Code 一致的 Agent Loop，支持步数限制、Doom Loop 检测、自动续写
- **6 种内置 Agent** — General、Plan、Explore、Build、Compaction、Title，各有独立工具白名单
- **9 层模糊编辑** — Edit 工具支持精确匹配到缩进归一化的 9 层 Replacer 链，覆盖 AI 编码的各种边缘情况
- **上下文压缩管线** — Microcompact → Prune → Summarize 三级压缩，支持无限长对话
- **MCP 集成** — 支持 stdio 和 Streamable HTTP 两种传输，可连接任意 MCP Server

### 多租户与安全

- **用户隔离** — 每个用户独立的工作空间（`userdata/{user_id}/`）、会话、任务、记忆
- **沙箱执行** — nsjail 进程隔离（容器级）或 Microsandbox MicroVM 隔离，防止代码逃逸
- **路径遍历防护** — 虚拟路径映射（`/workspace/` ↔ 真实路径），阻断 `../../../etc/passwd` 类攻击
- **权限系统** — Ruleset 评估引擎，支持 allow/deny/ask 规则，通配符匹配

### 持久记忆系统

- **混合检索** — BM25 全文检索 + Dense Vector 语义搜索，召回率远超纯向量方案
- **自动提取** — EventBus 驱动，在上下文压缩前、对话轮次后、会话结束时自动提取关键信息
- **AI 工具集成** — AI 可主动调用 MemorySearch/MemorySave/MemoryList 读写记忆
- **向量存储** — 可选接入 Milvus 向量数据库（Linux amd64/arm64）

### 内置 LLM 网关

基于 [New API](https://github.com/Calcium-Ion/new-api) 构建，内置完整的 LLM Gateway：

- **40+ 服务商** — OpenAI、Claude、Gemini、Azure、AWS Bedrock、Vertex AI 等
- **统一 API** — 兼容 OpenAI 格式，一键切换后端模型
- **计费系统** — 按量计费、在线充值、分组倍率、模型定价
- **智能路由** — 渠道加权随机、失败自动重试、亲和性缓存

### Web 端体验

- **实时 WebSocket 通信** — 流式输出 Thinking、Text、Tool Call 全过程
- **文件管理器** — Google Colab 风格，支持上传/下载/新建/重命名/删除
- **任务管理** — Claude Code 风格 TodoList，AI 可创建/更新，支持拖拽排序
- **定时任务** — Cron 风格调度器，支持 AI 自动执行
- **技能商店** — 从 GitHub / 魔搭社区一键导入技能，支持收藏和分类
- **配置面板** — 模型选择、MCP 配置、API 令牌、用户画像

---

## 快速开始

### 生产部署（从源码构建）

```bash
git clone https://github.com/ELK-milu/Coworker.git
cd Coworker
cp .env.example .env
# 编辑 .env，填入 ANTHROPIC_API_KEY 等必要配置

docker-compose up -d --build
# 访问 http://localhost:3000
# 默认账号：root / 123456
```

构建过程自动完成：Bun 编译前端 → Go 编译后端 → 打包到 Debian slim 运行镜像。

### 开发部署

提供两种开发模式，都支持 Go 文件热重载（Air）：

```bash
# 标准开发 — 完整功能，含 Milvus 向量搜索（需 8G+ 内存）
docker-compose -f docker-compose-dev.yml up --build

# 轻量开发 — 无 Milvus，适合 4G 内存服务器（向量搜索降级为 BM25）
docker-compose -f docker-compose-dev-fast.yml up --build
```

前端修改后需手动重新构建：

```bash
docker-compose -f docker-compose-dev.yml restart web-dev
```

### 本地开发（不用 Docker）

```bash
# 后端
cp .env.example .env
go run main.go

# 前端
cd web
bun install
bun run build   # 构建到 web/dist，后端自动提供静态文件服务
bun run dev     # 或启动 Vite 开发服务器（HMR）
```

### 三种部署模式对比

| 模式 | Compose 文件 | 用途 | 服务 | 内存需求 |
|------|-------------|------|------|---------|
| **生产** | `docker-compose.yml` | 正式部署 | Coworker + Postgres + Redis + nsjail | ~1GB |
| **标准开发** | `docker-compose-dev.yml` | 全功能开发 | Air 热重载 + Postgres + Redis + nsjail + Milvus | ~4GB |
| **轻量开发** | `docker-compose-dev-fast.yml` | 低配开发 | Air 热重载 + Postgres + Redis + nsjail | ~1GB |

---

## 架构概览

```
Browser (React 18 + Semi-UI)
  │
  ├── WebSocket ──→ ClaudeCLI Module
  │                   ├── Conversation Loop (Agent Loop + Doom Loop Detection)
  │                   ├── Tool Registry (Bash, Read, Write, Edit, Glob, Grep, Memory...)
  │                   ├── Context Manager (Microcompact → Prune → Summarize)
  │                   ├── Sandbox (nsjail / Microsandbox / Local)
  │                   ├── Memory System (BM25 + Milvus Vector)
  │                   ├── Session Manager (File-based persistence)
  │                   ├── MCP Manager (stdio + Streamable HTTP)
  │                   └── EventBus (Sync + Async handlers)
  │
  └── REST API ──→ LLM Gateway (New API)
                    ├── 40+ Provider Adapters
                    ├── Quota & Billing
                    ├── Channel Selection & Retry
                    └── User & Token Management
```

---

## 技术栈

| 层级 | 技术 |
|------|------|
| **后端** | Go 1.25+, Gin, GORM, gorilla/websocket |
| **前端** | React 18, Vite 5, Semi-UI, CodeMirror |
| **数据库** | PostgreSQL（推荐）/ SQLite / MySQL |
| **缓存** | Redis（可选，支持内存缓存回退） |
| **向量库** | Milvus 2.5（Linux amd64/arm64，其他平台自动 stub） |
| **沙箱** | nsjail（进程隔离）/ Microsandbox（MicroVM） |
| **部署** | Docker + Docker Compose（多阶段构建，多架构） |

---

## 与 OpenClaw / Claude Code 的对比

| 维度 | OpenClaw | Claude Code | **Coworker** |
|------|----------|-------------|-------------|
| **交互方式** | WhatsApp / Slack / Telegram | 终端 CLI | **浏览器 Web UI** |
| **部署模型** | 个人本地运行 | 个人本地安装 | **多租户服务器部署** |
| **用户隔离** | 单用户 | 单用户 | **多用户，工作空间隔离** |
| **沙箱** | 无 | 有限 | **nsjail / MicroVM** |
| **记忆系统** | 基础 | 基础 | **BM25 + 向量混合检索** |
| **模型支持** | 多模型 | Claude 系列 | **40+ 服务商，统一网关** |
| **计费** | 无 | 按订阅 | **按量计费 + 管理后台** |
| **文件管理** | 无 | 终端操作 | **可视化文件管理器** |
| **MCP** | 支持 | 支持 | **支持 + 技能商店** |

---

## 环境变量

### 核心配置

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PORT` | 服务端口 | `3000` |
| `SQL_DSN` | 数据库连接 | SQLite |
| `REDIS_CONN_STRING` | Redis 连接 | - |
| `SESSION_SECRET` | 会话密钥 | - |

### ClaudeCLI 配置

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `ANTHROPIC_API_KEY` | Claude API Key | - |
| `ANTHROPIC_AUTH_TOKEN` | 替代认证方式 | - |
| `CLAUDE_MODEL` | 默认模型 | `claude-sonnet-4-20250514` |
| `WORKSPACE_BASE_PATH` | 工作空间根路径 | `./userdata` |

### 沙箱配置

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `NSJAIL_ENABLED` | 启用 nsjail | `true` |
| `NSJAIL_CONTAINER_NAME` | nsjail 容器名 | `nsjail-sandbox` |
| `NSJAIL_MEMORY_MB` | 内存限制 (MB) | `512` |
| `NSJAIL_EXEC_TIMEOUT` | 执行超时 (秒) | `120` |

### 记忆系统配置

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `MILVUS_ENABLED` | 启用 Milvus 向量库 | `false` |
| `EMBEDDING_PROVIDER` | Embedding 服务商 | `siliconflow` |
| `EMBEDDING_MODEL` | Embedding 模型 | `BAAI/bge-large-zh-v1.5` |

完整配置请参考 [.env.example](./.env.example)

---

## 相关项目

| 项目 | 关系 |
|------|------|
| [OpenClaw](https://github.com/anthropics/openclaw) | 灵感来源 — 个人 AI 助手的开源标杆 |
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code) | 核心对标 — Coworker 的 Agent 引擎参考实现 |
| [New API](https://github.com/Calcium-Ion/new-api) | 上游项目 — LLM 网关基座 |
| [One API](https://github.com/songquanpeng/one-api) | 原始项目 — New API 的基础 |

---

## 许可证

本项目基于 [AGPLv3](./LICENSE) 许可证开源。

LLM 网关部分基于 [New API](https://github.com/Calcium-Ion/new-api)（AGPLv3）/ [One API](https://github.com/songquanpeng/one-api)（MIT）。
</div>
