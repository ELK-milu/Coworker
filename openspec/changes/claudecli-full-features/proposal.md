# Proposal: ClaudeCLI 完整功能集成

## Context

### 用户需求
将 Claude Code CLI 的核心功能逆向实现为 Go 语言版本，为用户提供在线 Claude Code CLI 沙箱服务。

### 目标
- 用户专属文件空间
- 会话历史记忆
- 持久记忆的私人 Agent 助手
- 完整的 Claude Code CLI 功能体验

### 参考实现
- 逆向工程项目：`E:\PythonWorks\claude-code-open`
- 当前实现：`E:\PythonWorks\Coworker\claudecli`

---

## 当前状态

### 已实现 (16 项)
- [x] WebSocket 实时通信
- [x] 会话管理和持久化
- [x] 6 个基础工具 (Bash, Read, Write, Edit, Glob, Grep)
- [x] 用户工作空间隔离
- [x] 任务管理 (TodoList)
- [x] 上下文压缩 (Microcompact)
- [x] Session Memory
- [x] AI 摘要生成
- [x] 系统提示词构建
- [x] Claude API 集成
- [x] 流式响应处理
- [x] Thinking 块支持
- [x] 多用户异步支持
- [x] 线程安全机制
- [x] Token 估算
- [x] 前端 Coworker 页面

### 缺失功能 (待实现)
- [ ] Skill 系统
- [ ] MCP 集成 (Stdio/WebSocket/HTTP)
- [ ] 子代理系统
- [ ] 高级工具 (LSP, WebFetch, WebSearch)
- [ ] 完整权限检查系统
- [ ] NotebookEdit 工具
- [ ] AskUserQuestion 工具

---

## Requirements

### R1: Skill 系统

**描述**: 实现用户自定义技能系统，支持参数替换和技能来源管理。

**场景**:
- 用户可以定义和调用自定义 Skill
- 支持 `$0`, `$1`, `$ARGUMENTS[N]` 参数语法
- 支持项目级和用户级 Skill 来源

**验收标准**:
- [ ] SkillTool 工具实现
- [ ] Skill 定义解析 (name, description, content, allowedTools)
- [ ] 参数替换逻辑
- [ ] Skill 来源优先级 (项目 > 用户)

**参考**: `claude-code-open/src/tools/skill.ts`

### R2: MCP 集成

**描述**: 实现 Model Context Protocol 集成，支持连接外部 MCP 服务器扩展工具能力。

**场景**:
- 支持 Stdio 传输 (本地 MCP 服务器)
- 支持 WebSocket 传输 (远程 MCP 服务器)
- 支持 HTTP/SSE 传输 (REST 风格 MCP)

**验收标准**:
- [ ] MCP 连接管理器
- [ ] Stdio 传输实现
- [ ] WebSocket 传输实现
- [ ] HTTP/SSE 传输实现
- [ ] MCP 工具动态注册
- [ ] 连接池和重连机制

**参考**: `claude-code-open/src/mcp/`

### R3: 子代理系统

**描述**: 实现子代理系统，支持 Explore/Plan 等专用代理和并行执行。

**场景**:
- 支持 Explore 代理 (代码库探索)
- 支持 Plan 代理 (架构规划)
- 支持并行子代理执行
- 支持模型继承 (inherit)

**验收标准**:
- [ ] TaskTool (子代理启动)
- [ ] 代理类型定义 (Explore, Plan, general-purpose)
- [ ] 模型别名解析 (sonnet/opus/haiku/inherit)
- [ ] 子代理上下文隔离
- [ ] 并行执行支持

**参考**: `claude-code-open/src/agents/`, `claude-code-open/src/tools/agent.ts`

### R4: 高级工具

**描述**: 实现 LSP、WebFetch、WebSearch 等高级工具。

**验收标准**:
- [ ] WebFetchTool - 网页内容获取
- [ ] WebSearchTool - 网页搜索
- [ ] LSPTool - 代码智能 (goToDefinition, findReferences)
- [ ] NotebookEditTool - Jupyter 笔记本编辑
- [ ] AskUserQuestionTool - 用户交互问答

**参考**: `claude-code-open/src/tools/`

### R5: 权限检查系统

**描述**: 实现完整的权限检查系统，支持四种权限模式。

**权限模式**:
- `default` - 需要用户批准
- `acceptEdits` - 自动批准文件编辑
- `planMode` - 规划模式 (只读探索)
- `bypassPermissions` - 绕过所有权限

**验收标准**:
- [ ] 权限模式切换
- [ ] 工具级权限检查
- [ ] 前端权限交互 UI
- [ ] 权限配置持久化

---

## Constraints (约束集)

### 硬约束 (不可违反)

1. **用户隔离**: 每个用户的工作空间、会话、任务必须完全隔离
2. **线程安全**: 所有共享资源必须使用 Mutex 保护
3. **异步处理**: 耗时操作必须在 goroutine 中执行
4. **路径安全**: 所有文件操作必须限制在用户工作空间内
5. **API 兼容**: 保持与现有 WebSocket 协议的向后兼容

### 软约束 (建议遵循)

1. **代码风格**: 遵循 Go 标准代码风格 (gofmt)
2. **错误处理**: 使用 error wrapping 提供上下文
3. **日志规范**: 使用 `[模块名]` 前缀的结构化日志
4. **测试覆盖**: 核心逻辑应有单元测试

### 依赖关系

```
R1 (Skill) ──────────────────────────────┐
                                         │
R2 (MCP) ────────────────────────────────┼──► R3 (子代理)
                                         │
R4 (高级工具) ───────────────────────────┘

R5 (权限系统) ──► 所有工具
```

**实施顺序建议**:
1. R5 权限系统 (基础设施)
2. R1 Skill 系统 (独立模块)
3. R4 高级工具 (独立模块)
4. R2 MCP 集成 (复杂度高)
5. R3 子代理系统 (依赖 MCP)

---

## Risks (风险)

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| MCP 协议复杂度高 | 实现周期长 | 先实现 Stdio，再扩展其他传输 |
| 子代理资源消耗 | 服务器负载 | 限制并发子代理数量 |
| 权限系统与现有代码冲突 | 重构工作量 | 渐进式集成，保持兼容 |

---

## Success Criteria (成功标准)

1. **Skill 系统**: 用户可通过 `/skill-name` 调用自定义技能
2. **MCP 集成**: 可连接至少一个外部 MCP 服务器并调用其工具
3. **子代理**: 可启动 Explore 子代理进行代码库探索
4. **高级工具**: WebFetch 可获取网页内容
5. **权限系统**: 可在前端切换权限模式

---

*Created: 2026-02-01*
*Status: Ready for Implementation*
