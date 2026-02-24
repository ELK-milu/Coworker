# Coworker ClaudeCLI vs OpenCode 全量架构对比与优化方案

## Context

Coworker/ClaudeCLI 存在两个核心问题：
1. **工具调用频率过高** — AI 频繁发起不必要的工具调用
2. **AI 发送无意义消息** — 对话循环缺乏有效的终止和控制机制

通过对 OpenCode（TypeScript monorepo）的全量代码探索，发现其在工具系统、对话循环、权限控制、上下文管理等方面有成熟的设计模式，可作为 ClaudeCLI 改进的参考。

---

## 一、整体架构对比

### 1.1 技术栈差异

| 维度 | Coworker/ClaudeCLI | OpenCode |
|------|-------------------|----------|
| 语言 | Go (Gin + gorilla/websocket) | TypeScript (Bun runtime) |
| AI SDK | 直接调用 Anthropic HTTP API | Vercel AI SDK (`ai` 包) |
| 工具定义 | 手动 JSON Schema | Zod schema → 自动转换 |
| 存储 | 文件系统 JSON | 自定义 Storage 抽象层 |
| 部署模式 | 多租户 WebSocket 服务 | 单用户 CLI/Desktop |
| 权限系统 | 简单的 permission/memory.go | 完整的 Ruleset 评估引擎 |

### 1.2 模块划分对比

```
Coworker/ClaudeCLI:                    OpenCode:
claudecli/                             packages/opencode/src/
├── internal/api/websocket.go          ├── session/prompt.ts    (主循环入口)
├── internal/loop/conversation.go      ├── session/processor.ts (流处理器)
├── internal/client/claude.go          ├── session/llm.ts       (LLM 调用层)
├── internal/tools/registry.go         ├── tool/registry.ts     (工具注册)
├── internal/tools/*.go                ├── tool/*.ts            (工具实现)
├── internal/context/                  ├── session/compaction.ts(上下文压缩)
├── internal/session/                  ├── session/index.ts     (会话管理)
├── internal/prompt/                   ├── session/system.ts    (系统提示词)
└── internal/sandbox/                  ├── permission/next.ts   (权限系统)
                                       └── agent/agent.ts       (Agent 定义)
```

---

## 二、对话循环（核心差异）

### 2.1 Coworker 的对话循环（简单 for 循环）

**文件**: `claudecli/internal/loop/conversation.go:132-193`

```
用户消息 → for { API调用 → 处理流 → 有工具调用? → 执行工具 → 继续循环 }
```

**问题分析**:
1. **无循环步数限制** — `for {}` 无限循环，仅靠 `stopReason != "tool_use"` 退出
2. **无 doom loop 检测** — 相同工具+相同参数反复调用无法检测
3. **无 compaction 触发** — 仅在循环开始前检查 `IsContextNearLimit()`
4. **无 Agent 分层** — 所有工具对所有请求可用，无法按角色限制
5. **无重试机制** — API 调用失败直接返回错误

### 2.2 OpenCode 的对话循环（状态机 + 多层控制）

**文件**: `packages/opencode/src/session/prompt.ts:267-653`

```
用户消息 → while(true) {
  检查 abort → 加载消息 → 检查 finish reason →
  处理 subtask/compaction → 检查 context overflow →
  创建 processor → 流式处理 → 根据结果决定:
    "stop" → break
    "compact" → 创建 compaction 任务
    "continue" → 继续循环
} → prune 旧工具输出
```

**关键控制机制**:

| 机制 | OpenCode 实现 | Coworker 现状 |
|------|-------------|-------------|
| **步数限制** | `agent.steps ?? Infinity`，可配置 | 无 |
| **Doom Loop 检测** | 最近 3 次相同工具+相同输入 → 请求权限 | 无 |
| **Finish Reason 检查** | `!["tool-calls","unknown"].includes(finish)` → break | 仅检查 `stopReason != "tool_use"` |
| **Compaction 触发** | `isOverflow()` 检查 input+cache+output tokens | 仅 `IsContextNearLimit()` 粗略估算 |
| **Prune 机制** | 循环结束后修剪旧工具输出（保留最近 40K tokens） | Microcompact 保留最近 3 个 |
| **AbortSignal** | 贯穿整个调用链，支持优雅取消 | `context.Context` 取消 |
| **重试机制** | `SessionRetry` 指数退避 + retry-after header | 无 |
| **Busy 检测** | `assertNotBusy()` 防止同一会话并发处理 | 无（多租户下可能并发） |

---

## 三、工具系统对比

### 3.1 工具定义与验证

**OpenCode** (`tool/tool.ts:48-88`):
- 使用 `Tool.define()` 工厂函数
- Zod schema 自动验证参数
- 验证失败返回友好错误信息指导 AI 修正
- **自动截断**: 工具输出超过 2000 行或 50KB 自动截断并保存完整输出到文件
- 截断提示引导 AI 使用 Task/Grep/Read 处理大输出

**Coworker** (`tools/registry.go:56-62`):
- 简单的 `Execute(ctx, name, input)` 调用
- 无参数验证层（依赖各工具自行验证）
- 无统一截断机制

### 3.2 Read 工具对比

**OpenCode** (`tool/read.ts`):
- 默认 2000 行限制，单行 2000 字符截断
- **50KB 字节限制** — 防止大文件消耗过多 token
- 二进制文件检测（扩展名 + 内容分析）
- 文件不存在时提供模糊建议
- 图片/PDF 支持（base64 附件）
- **LSP 预热** — 读取文件时触发 LSP 客户端
- **Instruction 注入** — 读取文件时自动加载关联的 CLAUDE.md 指令
- 输出格式: `<file>00001| line content</file>`

**Coworker** (`tools/read.go`):
- 有行数限制和偏移量
- 有二进制检测和模糊建议（已从 OpenCode 学习）
- 无字节限制 — 大文件可能产生巨大输出
- 无 LSP 集成
- 无 Instruction 注入

### 3.3 Write 工具对比

**OpenCode** (`tool/write.ts`):
- 写入前生成 diff 用于权限审批
- **FileTime 断言** — 检测文件是否被外部修改
- 写入后触发 LSP 诊断
- **LSP 错误反馈** — 将编译错误直接返回给 AI，引导修复
- 发布文件编辑事件（Bus 事件系统）

**Coworker** (`tools/write.go`):
- 直接写入文件
- 无 diff 生成
- 无文件修改时间检测
- 无 LSP 诊断反馈
- 无事件系统

### 3.4 Edit 工具对比

**OpenCode** (`tool/edit.ts`):
- **9 层 Replacer 链**:
  1. SimpleReplacer（精确匹配）
  2. LineTrimmedReplacer（行尾空格容忍）
  3. BlockAnchorReplacer（首尾锚定 + Levenshtein）
  4. WhitespaceNormalizedReplacer（空格归一化）
  5. IndentationFlexibleReplacer（缩进灵活匹配）
  6. EscapeNormalizedReplacer（转义字符归一化）
  7. TrimmedBoundaryReplacer（边界空白容忍）
  8. ContextAwareReplacer（上下文感知匹配）
  9. MultiOccurrenceReplacer（多次出现处理）
- **FileTime 锁** — `FileTime.withLock()` 防止并发编辑冲突
- LSP 诊断反馈
- Snapshot 跟踪（支持 undo/revert）

**Coworker** (`tools/edit.go` + `edit_replacer.go`):
- **6 层 Replacer 链**（已从 OpenCode 早期版本学习）
- 无文件锁机制
- 无 LSP 诊断
- 无 Snapshot 跟踪

### 3.5 Bash 工具对比

**OpenCode** (`tool/bash.ts`):
- **Tree-sitter 解析命令** — 提取命令名和参数用于权限检查
- 外部目录检测 — 命令涉及项目外路径时请求权限
- **命令级权限** — 每个命令独立请求权限（如 `rm *`, `git push *`）
- 实时输出流 — 通过 metadata 回调实时更新 UI
- 超时控制 — 默认 2 分钟，可配置
- 进程树清理 — `Shell.killTree()` 清理子进程

**Coworker** (`tools/bash.go`):
- 直接执行命令
- 无命令解析和权限检查
- 有超时控制
- 有 Microsandbox 沙箱执行模式
- 无实时输出流

---

## 四、权限系统对比

### 4.1 OpenCode 权限系统 (`permission/next.ts`)

**架构**: Ruleset 评估引擎
- 每个 Agent 有独立的权限规则集
- 规则格式: `{ permission, pattern, action }` (allow/deny/ask)
- 通配符匹配: `Wildcard.match(permission, rule.permission)`
- **层级合并**: `defaults → agent-specific → user-config`
- 工具调用时通过 `ctx.ask()` 请求权限
- 支持 "always allow" 记忆

**关键规则示例**:
```typescript
// explore agent: 只允许只读工具
"*": "deny",
grep: "allow", glob: "allow", read: "allow", bash: "allow"

// build agent: 允许所有工具 + 问题工具
"*": "allow", question: "allow"

// plan agent: 禁止编辑（除计划文件）
edit: { "*": "deny", "plans/*.md": "allow" }
```

### 4.2 Coworker 权限系统 (`permission/memory.go`)

- 简单的 allow/deny 记忆
- 通配符匹配
- 持久化到 JSON 文件
- **无 Agent 级别权限分离**
- **无工具级别细粒度控制**

---

## 五、Agent 分层对比

### 5.1 OpenCode Agent 系统 (`agent/agent.ts`)

| Agent | 模式 | 权限 | 用途 |
|-------|------|------|------|
| build | primary | 全部工具 | 默认 Agent，执行编码任务 |
| plan | primary | 只读 + 计划文件编辑 | 规划模式 |
| explore | subagent | 只读工具 | 代码探索 |
| general | subagent | 全部工具（无 todo） | 通用子任务 |
| compaction | hidden | 无工具 | 上下文压缩 |
| title | hidden | 无工具 | 标题生成 |

**关键设计**:
- 每个 Agent 可配置独立的 model、temperature、topP
- 每个 Agent 有独立的权限规则集
- 可通过配置文件自定义 Agent
- **steps 限制** — 每个 Agent 可配置最大步数

### 5.2 Coworker Agent 系统

- **无 Agent 分层** — 所有请求使用相同的工具集和权限
- mode 字段仅用于 UI 显示（normal/plan/acceptEdits/bypassPermissions）
- 无子 Agent 概念

---

## 六、上下文管理对比

### 6.1 OpenCode 上下文管理

**三层压缩策略**:

1. **Prune 层** (`compaction.ts:49-90`):
   - 从后向前遍历，保护最近 40K tokens 的工具输出
   - 超过 40K 的旧工具输出标记为 compacted
   - 最小修剪阈值 20K tokens
   - 保护 skill 工具输出不被修剪

2. **Compaction 层** (`compaction.ts:92-194`):
   - 使用 AI 生成对话摘要
   - 摘要替换旧消息
   - 自动触发: `isOverflow()` 检测 token 超限
   - 手动触发: 用户请求

3. **消息过滤** (`message-v2.ts:filterCompacted`):
   - 已压缩的消息在发送给 AI 前被过滤
   - 工具输出被标记为 compacted 后替换为占位符

**Token 计算**: 使用实际 API 返回的 usage 数据

### 6.2 Coworker 上下文管理

**三层压缩策略**:

1. **Microcompact** (`compress.go`):
   - 保留最近 3 个工具结果
   - 旧工具结果替换为占位符
   - 代码块压缩（保留 60% 头 + 40% 尾）

2. **Prune** (`prune.go`):
   - 基于 token 估算修剪
   - 保护最近 40K tokens

3. **AI 摘要** (`summarizer.go`):
   - 生成对话摘要

**Token 计算**: 基于字符数估算（英文 ~3.5 字符/token，中文 ~2 字符/token）

**差异**: Coworker 的 token 估算不够精确，可能导致压缩时机不准确。

---

## 七、工具输出截断对比

### 7.1 OpenCode 截断系统 (`tool/truncation.ts`)

- **统一截断层**: 所有工具输出自动经过 `Truncate.output()`
- 限制: 2000 行 或 50KB
- 截断后保存完整输出到文件
- 提示 AI 使用 Task/Grep/Read 处理截断内容
- 定期清理过期截断文件（7 天）

### 7.2 Coworker 截断

- **无统一截断层**
- 各工具自行处理输出长度
- 大输出直接发送给 AI，消耗大量 token

---

## 八、多租户适配性分析

### 8.1 OpenCode 的单用户设计

- 单进程单用户
- 全局状态（Instance.state）
- 文件系统直接操作
- 无用户隔离需求

### 8.2 Coworker 的多租户设计

**优势**:
- WebSocket 多连接支持
- 用户工作空间隔离（sandbox）
- Microsandbox MicroVM 沙箱
- 异步 goroutine 处理

**劣势**:
- 无会话级别的 busy 检测（同一用户可能并发发送消息）
- 无 Agent 级别的资源隔离
- 工具执行无并发限制

### 8.3 多租户改进建议

| 改进项 | 优先级 | 说明 |
|--------|--------|------|
| 会话 Busy 锁 | 高 | 防止同一会话并发处理 |
| 工具输出统一截断 | 高 | 减少 token 消耗 |
| Doom Loop 检测 | 高 | 防止无限工具调用 |
| 步数限制 | 高 | 防止无限循环 |
| API 重试机制 | 中 | 提高可靠性 |
| Agent 分层 | 中 | 按角色限制工具 |
| LSP 诊断反馈 | 低 | 减少编辑错误导致的重复调用 |

---

## 九、导致工具调用过于频繁的根因分析

基于代码对比，Coworker 工具调用频繁的根因：

### 9.1 无输出截断 → token 浪费 → 更多调用

OpenCode 的 `Truncate.output()` 将所有工具输出限制在 50KB/2000行。
Coworker 无此限制，大文件读取或长命令输出直接进入上下文，导致：
- 上下文快速膨胀
- AI 需要更多调用来"理解"大量输出
- 压缩触发更频繁

### 9.2 无 Doom Loop 检测 → 重复调用

OpenCode 在 `processor.ts:144-169` 检测最近 3 次相同工具+相同输入，触发权限请求。
Coworker 无此机制，AI 可能陷入：
- 反复读取同一文件
- 反复执行同一命令
- 反复尝试相同的编辑

### 9.3 无步数限制 → 无限循环

OpenCode 的 `agent.steps` 可限制每个 Agent 的最大步数。
Coworker 的 `for {}` 无限循环，仅靠 AI 自行决定停止。

### 9.4 Edit 工具匹配失败 → 重复尝试

OpenCode 有 9 层 Replacer，Coworker 有 6 层。
缺少的 3 层（EscapeNormalized、TrimmedBoundary、ContextAware）可能导致：
- 编辑匹配失败
- AI 反复尝试不同的 oldString
- 每次失败都消耗一次工具调用

### 9.5 无 LSP 诊断反馈 → 盲目修复

OpenCode 的 Write/Edit 工具返回 LSP 编译错误。
Coworker 无此反馈，AI 写入错误代码后：
- 需要额外的 Read 调用查看结果
- 需要额外的 Bash 调用运行编译/测试
- 发现错误后需要更多 Edit 调用修复

### 9.6 AI 发送无意义消息的原因

- **无 finish reason 精确检查** — Coworker 仅检查 `stopReason != "tool_use"`
- **无 queued message 处理** — OpenCode 在循环中处理用户中途发送的消息
- **系统提示词缺乏工具使用指导** — OpenCode 的提示词明确指导何时使用工具、何时停止

---

## 十、推荐改进方案（按优先级排序）

### P0: 立即修复（解决核心问题）

#### 10.1 添加统一工具输出截断

**修改文件**: 新增 `claudecli/internal/tools/truncation.go`
**修改文件**: `claudecli/internal/tools/registry.go` — Execute 方法包装截断

参考 OpenCode `tool/truncation.ts`:
- 最大 2000 行 / 50KB
- 截断后保存完整输出到文件
- 返回截断提示

#### 10.2 添加 Doom Loop 检测

**修改文件**: `claudecli/internal/loop/conversation.go`

在 `executeTools()` 中添加：
- 记录最近 3 次工具调用的 (name, input) 哈希
- 连续 3 次相同 → 返回错误提示 AI 换策略

#### 10.3 添加循环步数限制

**修改文件**: `claudecli/internal/loop/conversation.go`

在 `runLoop()` 中添加：
- `maxSteps` 配置（默认 50）
- 达到限制时注入 "max steps reached" 提示
- 参考 OpenCode `prompt/max-steps.txt`

#### 10.4 添加会话 Busy 锁

**修改文件**: `claudecli/internal/api/websocket.go`

防止同一会话并发处理：
- 使用 `sync.Map` 记录正在处理的会话
- 新请求到达时检查是否 busy
- busy 时返回错误或排队

### P1: 短期改进（减少不必要调用）

#### 10.5 补全 Edit Replacer 链

**修改文件**: `claudecli/internal/tools/edit_replacer.go`

添加缺失的 3 层：
- EscapeNormalizedReplacer
- TrimmedBoundaryReplacer
- ContextAwareReplacer

#### 10.6 Read 工具添加字节限制

**修改文件**: `claudecli/internal/tools/read.go`

添加 50KB 字节限制，防止大文件消耗过多 token。

#### 10.7 API 调用重试机制

**修改文件**: `claudecli/internal/client/claude.go`

添加指数退避重试：
- 解析 retry-after / retry-after-ms header
- 最大重试 3 次
- 429/503/overloaded 自动重试

### P2: 中期改进（架构优化）

#### 10.8 Agent 分层系统

新增 `claudecli/internal/agent/agent.go`:
- 定义 Agent 接口（name, tools, permissions, maxSteps）
- 内置 Agent: build, explore, plan
- explore Agent 只允许只读工具
- plan Agent 禁止编辑

#### 10.9 Finish Reason 精确检查

**修改文件**: `claudecli/internal/loop/conversation.go`

参考 OpenCode 的检查逻辑：
- `end_turn` → 结束循环
- `tool_use` → 继续执行工具
- `max_tokens` → 注入 "continue" 提示
- 其他 → 结束循环

#### 10.10 系统提示词优化

**修改文件**: `claudecli/internal/prompt/templates.go`

参考 OpenCode 的提示词，添加：
- 工具使用指导（何时使用、何时停止）
- 输出截断处理指导
- 并行工具调用指导

---

## 十一、验证方案

### 测试场景

1. **Doom Loop 测试**: 让 AI 反复读取不存在的文件，验证 3 次后停止
2. **步数限制测试**: 给 AI 一个需要大量步骤的任务，验证达到限制后停止
3. **截断测试**: 读取大文件，验证输出被截断且提示正确
4. **Busy 锁测试**: 同一会话并发发送消息，验证第二个被拒绝
5. **重试测试**: 模拟 429 错误，验证自动重试

### 指标监控

- 每次对话的平均工具调用次数（目标: 减少 30%+）
- 每次对话的平均 token 消耗（目标: 减少 40%+）
- Doom Loop 触发次数
- 截断触发次数

---

*基于 OpenCode `packages/opencode/src/` 和 Coworker `claudecli/internal/` 的全量代码对比*
