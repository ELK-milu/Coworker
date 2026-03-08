<div align="center">

![new-api](/web/public/logo.png)

# New API

🍥 **下一代 LLM 网关与 AI 资产管理系统**

<p align="center">
  <strong>简体中文</strong> |
  <a href="./README.zh_TW.md">繁體中文</a> |
  <a href="./README.en.md">English</a> |
  <a href="./README.fr.md">Français</a> |
  <a href="./README.ja.md">日本語</a>
</p>

<p align="center">
  <a href="https://raw.githubusercontent.com/Calcium-Ion/new-api/main/LICENSE">
    <img src="https://img.shields.io/github/license/Calcium-Ion/new-api?color=brightgreen" alt="license">
  </a>
  <a href="https://github.com/Calcium-Ion/new-api/releases/latest">
    <img src="https://img.shields.io/github/v/release/Calcium-Ion/new-api?color=brightgreen&include_prereleases" alt="release">
  </a>
  <a href="https://hub.docker.com/r/calciumion/new-api">
    <img src="https://img.shields.io/docker/pulls/calciumion/new-api?color=blue" alt="docker pulls">
  </a>
  <a href="https://goreportcard.com/report/github.com/Calcium-Ion/new-api">
    <img src="https://goreportcard.com/badge/github.com/Calcium-Ion/new-api" alt="Go Report Card">
  </a>
</p>

</div>

---

## 📖 简介

New API 是一个强大的 AI API 网关/代理系统，聚合了 40+ 个上游 AI 服务商（OpenAI、Claude、Gemini、Azure、AWS Bedrock 等），提供统一的 API 接口、用户管理、计费系统、速率限制和管理后台。

> ⚠️ **声明**：本项目仅供个人学习使用，不保证稳定性和技术支持。使用者须遵守 OpenAI [使用条款](https://openai.com/policies/terms-of-use) 及相关法律法规。

---

## ✨ 核心特性

| 功能 | 描述 |
|------|------|
| 🔌 **多渠道聚合** | 支持 40+ AI 服务商，统一 API 格式 |
| 💰 **计费系统** | 按量计费、在线充值（EPay、Stripe） |
| 🔐 **多种认证** | JWT、WebAuthn/Passkeys、OAuth（GitHub、Discord、OIDC） |
| 📊 **数据看板** | 可视化统计分析和用量监控 |
| 🌍 **多语言** | 支持中文、英文、法语、日语、俄语、越南语 |
| ⚡ **智能路由** | 渠道加权随机、失败自动重试 |
| 🔄 **格式转换** | OpenAI ⇄ Claude、OpenAI → Gemini |

---

## 🚀 快速开始

### Docker Compose（推荐）

```bash
# 克隆项目
git clone https://github.com/QuantumNous/new-api.git
cd new-api

# 启动服务
docker-compose up -d

# 访问 http://localhost:3000
```

### Docker 命令

```bash
# 使用 SQLite（默认）
docker run -d --name new-api --restart always \
  -p 3000:3000 \
  -v ./data:/data \
  -e TZ=Asia/Shanghai \
  calciumion/new-api:latest
```

### 本地开发

```bash
# 后端
go mod download
cp .env.example .env
go run main.go

# 前端（需要先安装 bun）
cd web
bun install
bun run build
```

---

## 🛠️ 技术栈

| 层级 | 技术 |
|------|------|
| **后端** | Go 1.22+, Gin, GORM v2 |
| **前端** | React 18, Vite, Semi Design UI |
| **数据库** | SQLite / MySQL ≥5.7.8 / PostgreSQL ≥9.6 |
| **缓存** | Redis + 内存缓存 |

---

## 📡 支持的 API

- OpenAI Chat Completions / Responses / Realtime
- Claude Messages
- Google Gemini
- Embeddings / Rerank
- 图像生成 / 语音合成 / 语音识别
- Midjourney-Proxy / Suno API

---

## ⚙️ 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PORT` | 服务端口 | `3000` |
| `SQL_DSN` | 数据库连接字符串 | SQLite |
| `REDIS_CONN_STRING` | Redis 连接字符串 | - |
| `SESSION_SECRET` | 会话密钥（多机部署必填） | - |
| `STREAMING_TIMEOUT` | 流式响应超时（秒） | `300` |

完整配置请参考 [.env.example](./.env.example)

---

## 📚 文档

- 📖 [官方文档](https://docs.newapi.pro/docs)
- ⚙️ [环境变量配置](https://docs.newapi.pro/docs/installation/config-maintenance/environment-variables)
- 📡 [API 文档](https://docs.newapi.pro/docs/api)
- ❓ [常见问题](https://docs.newapi.pro/docs/support/faq)

---

## 🔗 相关项目

| 项目 | 说明 |
|------|------|
| [One API](https://github.com/songquanpeng/one-api) | 原始项目基础 |
| [neko-api-key-tool](https://github.com/Calcium-Ion/neko-api-key-tool) | Key 额度查询工具 |

---

## 📜 许可证

本项目基于 [AGPLv3](./LICENSE) 许可证开源，基于 [One API](https://github.com/songquanpeng/one-api)（MIT）开发。

如需商业授权，请联系：[support@quantumnous.com](mailto:support@quantumnous.com)

---

<div align="center">

**Built with ❤️ by QuantumNous**

⭐ 如果这个项目对你有帮助，欢迎 Star！

</div>
