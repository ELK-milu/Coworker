# OpenCode 项目分析报告

> 分析日期: 2026-02-04
> 目标: 对比 OpenCode 与 Coworker ClaudeCLI 模块，识别改进机会

## 1. 项目概览

**OpenCode** 是一个开源的 AI 编码代理 CLI 工具，类似于 Claude Code。

| 特性 | OpenCode | Coworker ClaudeCLI |
|------|----------|-------------------|
| 语言 | TypeScript (Bun) | Go |
| 架构 | 客户端/服务器分离 | WebSocket 单体 |
| 前端 | TUI (终端 UI) | Web UI |
| 提供商 | 多提供商支持 | Anthropic 专用 |

### 核心包结构 (packages/opencode/src/)

```
├── agent/       # AI 代理系统
├── tool/        # 工具实现 (bash, read, write, etc.)
├── session/     # 会话管理
├── mcp/         # Model Context Protocol
├── lsp/         # Language Server Protocol
├── provider/    # LLM 提供商适配
├── config/      # 配置管理
├── permission/  # 权限系统
├── snapshot/    # 快照/检查点
├── worktree/    # Git Worktree 管理
├── skill/       # 技能系统
├── plugin/      # 插件系统
├── server/      # HTTP/WebSocket 服务器
├── cli/         # CLI 入口
└── ...
```

---

## 2. 模块对比分析

### 2.1 工具系统 (Tool)

**OpenCode 工具列表:**
- `bash` - 命令执行 (带 tree-sitter 解析)
- `read` - 文件读取
- `write` - 文件写入
- `edit` - 文件编辑 (支持 LSP 诊断)
- `glob` - 文件模式匹配
- `grep` - 内容搜索
- `task` - 子代理任务
- `todo` - 任务列表管理
- `webfetch` - 网页获取
- `websearch` - 网页搜索
- `codesearch` - 代码搜索
- `lsp` - LSP 操作 (实验性)
- `batch` - 批量操作 (实验性)
- `apply_patch` - 补丁应用
- `skill` - 技能调用
- `question` - 用户提问
- `plan` - 计划模式

**亮点功能:**

1. **Bash 工具使用 tree-sitter 解析命令**
   - 解析命令结构，识别外部目录访问
   - 自动检测 `cd`, `rm`, `cp`, `mv` 等命令的目标路径
   - 权限请求更精确

2. **Edit 工具集成 LSP 诊断**
   - 编辑后自动触发 LSP 诊断
   - 返回错误信息给 AI 自动修复

3. **工具输出截断机制**
   - `Truncate` 模块统一处理大输出
   - 支持保存到临时文件

**对比 Coworker:**
| 功能 | OpenCode | Coworker |
|------|----------|----------|
| Bash 命令解析 | tree-sitter | 无 |
| LSP 集成 | 有 | 无 |
| 批量操作 | 有 | 无 |
| 代码搜索 | 有 | 无 |

### 2.2 会话管理 (Session)

**OpenCode 会话架构:**

```
session/
├── index.ts          # 会话 CRUD
├── compaction.ts     # 上下文压缩
├── summary.ts        # 会话摘要
├── message.ts        # 消息管理
├── message-v2.ts     # 消息 V2 (22KB)
├── processor.ts      # 消息处理器
├── prompt.ts         # 提示词构建 (64KB)
├── llm.ts            # LLM 调用
├── retry.ts          # 重试逻辑
├── revert.ts         # 回滚功能
└── status.ts         # 状态管理
```

**亮点功能:**

1. **消息处理器 (Processor)**
   - 流式处理 AI 响应
   - 工具调用状态追踪
   - 错误恢复机制

2. **会话摘要 (Summary)**
   - 自动生成会话标题
   - 计算文件变更统计 (additions/deletions)
   - 存储 diff 信息

3. **回滚功能 (Revert)**
   - 支持撤销 AI 的修改

**对比 Coworker:**
| 功能 | OpenCode | Coworker |
|------|----------|----------|
| 会话标题生成 | AI 自动生成 | 无 |
| 变更统计 | 有 | 无 |
| 回滚功能 | 有 | 无 |

### 2.3 上下文压缩 (Context Compaction)

**OpenCode 压缩策略:**

1. **自动溢出检测**
   - 监控 token 使用量
   - 超过模型上下文限制时自动触发

2. **工具输出修剪 (Prune)**
   - 保护最近 40K tokens 的工具调用
   - 清理旧工具输出
   - 保护特定工具 (如 `skill`)

3. **AI 摘要生成**
   - 使用专门的 `compaction` 代理
   - 生成继续对话所需的上下文

**关键常量:**
```typescript
PRUNE_MINIMUM = 20_000  // 最小修剪量
PRUNE_PROTECT = 40_000  // 保护最近的 token 数
```

**对比 Coworker:**
| 功能 | OpenCode | Coworker |
|------|----------|----------|
| 自动压缩 | 有 | 有 (70%阈值) |
| 工具输出修剪 | 智能保护 | 保留最近3个 |
| AI 摘要 | 专用代理 | 简单摘要 |

### 2.4 LSP 集成

**OpenCode LSP 架构:**

```
lsp/
├── index.ts      # LSP 管理器
├── client.ts     # LSP 客户端
├── server.ts     # 内置 LSP 服务器配置 (64KB)
└── language.ts   # 语言检测
```

**支持的 LSP 操作:**
- `hover` - 悬停信息
- `definition` - 跳转定义
- `references` - 查找引用
- `implementation` - 查找实现
- `documentSymbol` - 文档符号
- `workspaceSymbol` - 工作区符号
- `diagnostics` - 诊断信息
- `prepareCallHierarchy` - 调用层次
- `incomingCalls` / `outgoingCalls` - 调用关系

**亮点功能:**
1. 自动检测文件类型并启动对应 LSP
2. 支持自定义 LSP 配置
3. 编辑后自动触发诊断

**对比 Coworker:**
| 功能 | OpenCode | Coworker |
|------|----------|----------|
| LSP 支持 | 完整 | 无 |
| 诊断集成 | 有 | 无 |
| 符号搜索 | 有 | 无 |

### 2.5 MCP 协议

**OpenCode MCP 架构:**

```
mcp/
├── index.ts           # MCP 客户端管理
├── auth.ts            # 认证管理
├── oauth-callback.ts  # OAuth 回调
└── oauth-provider.ts  # OAuth 提供商
```

**支持的传输方式:**
- `stdio` - 标准输入输出
- `sse` - Server-Sent Events
- `streamable-http` - HTTP 流

**亮点功能:**
1. 动态工具注册
2. OAuth 认证支持
3. 工具列表变更通知

**对比 Coworker:**
| 功能 | OpenCode | Coworker |
|------|----------|----------|
| MCP 支持 | 完整 | 无 |
| OAuth | 有 | 无 |

### 2.6 权限系统

**OpenCode 权限架构:**

```typescript
Permission.ask({
  type: "bash",           // 权限类型
  pattern: ["git *"],     // 匹配模式
  sessionID,
  messageID,
  callID,
  message: "Run git command?",
  metadata: {}
})

// 响应类型
type Response = "once" | "always" | "reject"
```

**权限类型:**
- `bash` - 命令执行
- `edit` - 文件编辑
- `read` - 文件读取
- `external_directory` - 外部目录访问
- `task` - 子代理调用
- `question` - 用户提问
- `plan_enter/exit` - 计划模式

**亮点功能:**
1. 通配符模式匹配
2. "always" 选项记住选择
3. 插件可拦截权限请求

**对比 Coworker:**
| 功能 | OpenCode | Coworker |
|------|----------|----------|
| 权限系统 | 完整 | 无 |
| 模式匹配 | 通配符 | 无 |
| 记住选择 | 有 | 无 |

### 2.7 快照系统

**OpenCode 快照架构:**

使用独立的 Git 仓库跟踪文件变更:
```
~/.opencode/data/snapshot/{project_id}/
```

**核心功能:**
- `track()` - 创建快照 (git write-tree)
- `patch()` - 获取变更文件列表
- `restore()` - 恢复到快照
- `revert()` - 回滚特定文件
- `diff()` / `diffFull()` - 获取差异

**自动清理:**
- 每小时运行 `gc --prune=7.days`

**对比 Coworker:**
| 功能 | OpenCode | Coworker |
|------|----------|----------|
| 快照系统 | 完整 | 无 |
| 文件回滚 | 有 | 无 |
| 变更追踪 | 有 | 无 |

---

## 3. 我们可以学习的功能

### 3.1 工具系统改进

**Bash 命令解析 (tree-sitter)**
- 解析命令结构，识别危险操作
- 自动检测外部目录访问
- 更精确的权限请求

**Edit 工具 LSP 集成**
- 编辑后自动检查语法错误
- 返回诊断信息给 AI 自动修复

### 3.2 会话管理改进

**会话标题自动生成**
- 使用小模型生成简短标题
- 提升用户体验

**变更统计**
- 记录 additions/deletions
- 显示会话影响范围

### 3.3 上下文压缩改进

**智能工具输出修剪**
- 保护最近的工具调用
- 按 token 数量而非数量修剪

---

## 4. 我们缺失的功能

### 4.1 LSP 集成 (高优先级)

**功能描述:**
- 自动启动语言服务器
- 提供代码诊断、跳转定义、查找引用
- 编辑后自动检查错误

**实现建议:**
- 使用 gopls 作为 Go LSP
- 集成 TypeScript LSP
- 在 Edit 工具中调用诊断

### 4.2 快照/回滚系统 (高优先级)

**功能描述:**
- 每次 AI 操作前创建快照
- 支持一键回滚到任意快照
- 显示文件变更历史

**实现建议:**
- 使用独立 Git 仓库
- 存储在 `userdata/{user_id}/snapshots/`

### 4.3 权限系统 (中优先级)

**功能描述:**
- 危险操作需要用户确认
- 支持"总是允许"选项
- 通配符模式匹配

### 4.4 MCP 协议支持 (中优先级)

**功能描述:**
- 支持外部工具集成
- 标准化工具接口

### 4.5 多代理系统 (低优先级)

**功能描述:**
- build/plan/explore 等专用代理
- 子代理任务分发

---

## 5. 改进建议

### 5.1 短期改进 (1-2周)

| 功能 | 描述 | 复杂度 |
|------|------|--------|
| 会话标题生成 | 使用 AI 生成简短标题 | 低 |
| 变更统计 | 记录 additions/deletions | 低 |
| 工具输出截断优化 | 按 token 数量修剪 | 中 |

### 5.2 中期改进 (2-4周)

| 功能 | 描述 | 复杂度 |
|------|------|--------|
| 快照系统 | Git 仓库跟踪变更 | 中 |
| 权限系统 | 危险操作确认 | 中 |
| Bash 命令解析 | 识别危险操作 | 高 |

### 5.3 长期改进 (1-2月)

| 功能 | 描述 | 复杂度 |
|------|------|--------|
| LSP 集成 | 代码诊断和导航 | 高 |
| MCP 协议 | 外部工具集成 | 高 |
| 多代理系统 | 专用代理分工 | 高 |

---

## 6. OpenCode 其他特色功能

### 6.1 多提供商支持

OpenCode 支持 20+ LLM 提供商:
- Anthropic Claude
- OpenAI GPT
- Google Gemini
- AWS Bedrock
- Azure OpenAI
- Groq, Mistral, Cohere
- 本地模型 (Ollama)

**模型元数据服务:**
- 从 models.dev 获取模型信息
- 自动刷新模型列表
- 包含成本、限制、能力信息

### 6.2 Git Worktree 管理

支持创建隔离的工作目录:
- 自动生成友好名称
- 支持启动命令
- 支持重置和删除

### 6.3 技能系统 (Skill)

从 SKILL.md 文件加载技能:
- 支持 `.claude/skills/` 目录
- 支持 `.opencode/skills/` 目录
- 支持自定义路径

### 6.4 插件系统 (Plugin)

支持 npm 包形式的插件:
- 认证插件 (Codex, Copilot)
- 工具插件
- 事件钩子

---

## 7. 代码质量对比

| 方面 | OpenCode | Coworker |
|------|----------|----------|
| 类型安全 | Zod schema | Go struct |
| 错误处理 | NamedError | 标准 error |
| 事件系统 | Bus + BusEvent | 无 |
| 状态管理 | Instance.state | 全局变量 |
| 日志系统 | 结构化日志 | 标准日志 |

---

## 8. 总结

OpenCode 是一个功能完善的 AI 编码代理，其架构设计值得学习:

1. **模块化设计** - 清晰的职责分离
2. **类型安全** - Zod schema 验证
3. **事件驱动** - Bus 系统解耦
4. **可扩展性** - 插件和技能系统

**优先实现建议:**
1. 快照/回滚系统 (用户体验提升最大)
2. 会话标题生成 (低成本高收益)
3. LSP 集成 (代码质量保障)

---

*文档生成日期: 2026-02-04*
