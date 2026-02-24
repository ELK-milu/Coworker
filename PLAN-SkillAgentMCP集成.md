# OpenCode Skill/Agent/MCP 机制分析与集成计划

## OpenCode 机制总结

### 1. Skill 系统
- **格式**: SKILL.md 文件，YAML frontmatter (name, description) + Markdown 指令内容
- **发现**: 扫描 `.claude/skills/`, `.opencode/skill/`, 配置路径, 远程 URL
- **运行时**: `SkillTool` 工具 — AI 识别任务匹配某 skill 后调用，注入 `<skill_content>` XML 块
- **权限**: 按 agent 的 permission ruleset 过滤可用 skills

### 2. Agent 系统
- **定义**: name, description, mode(primary/subagent), prompt, model, temperature, steps, permission
- **内置**: build(默认), plan, explore, general, compaction, title, summary
- **运行时**: `TaskTool` 工具 — 创建子会话，用指定 agent 的 prompt+model+permission 执行
- **配置**: 可通过 config 覆盖/新增 agent

### 3. MCP 系统
- **配置**: type(remote/local), url/command, headers, timeout, oauth
- **连接**: Remote(StreamableHTTP/SSE) + Local(Stdio subprocess)
- **运行时**: 连接后 listTools()，转换为 AI SDK Tool 格式，名称前缀 `serverName_toolName`

## 集成方案（映射到 Coworker 技能商店）

### 核心思路
用户在技能商店安装 skill/agent/MCP → 持久化 item_ids → 对话时根据用户 installed items 动态注入：
- **Skill**: 注册 SkillTool，系统提示词列出可用 skills，AI 按需加载内容
- **Agent**: 系统提示词注入 agent 的 prompt 作为额外指令
- **MCP**: 暂不实现（需 MCP SDK，复杂度高），预留接口

### 修改文件

| 文件 | 改动 |
|------|------|
| `claudecli/internal/tools/skill.go` | 新文件：SkillTool 实现 |
| `claudecli/internal/api/websocket.go` | buildUserSystemPrompt 注入已安装 skills/agents |
| `claudecli/init.go` | 注册 SkillTool |
| `claudecli/internal/prompt/builder.go` | PromptContext 新增 InstalledSkills/InstalledAgents 字段 |

### 验证
1. `go build ./...` 编译通过
2. 技能商店安装 skill → 对话中 AI 可调用 skill tool 加载内容
3. 技能商店安装 agent → 对话中系统提示词包含 agent 指令
