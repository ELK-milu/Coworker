package agent

import (
	"strings"

	"github.com/QuantumNous/new-api/coworker/internal/permissions"
)

// AgentMode Agent 模式
// 参考 OpenCode agent.ts: mode: z.enum(["subagent", "primary", "all"])
type AgentMode string

const (
	// ModePrimary 主代理 — 可由用户直接选择
	ModePrimary AgentMode = "primary"
	// ModeSubagent 子代理 — 仅能通过 Task 工具调用
	ModeSubagent AgentMode = "subagent"
	// ModeAll 通用模式 — 可作为主代理或子代理
	ModeAll AgentMode = "all"
)

// AgentType 代理类型配置
// 参考 OpenCode packages/opencode/src/agent/agent.ts Agent.Info
type AgentType struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Mode        AgentMode           `json:"mode"`                  // primary/subagent/all
	Native      bool                `json:"native,omitempty"`      // 是否内置
	Hidden      bool                `json:"hidden,omitempty"`      // 是否隐藏（不在 UI 显示）
	Tools       []string            `json:"tools"`                 // 允许的工具列表，"*" 表示所有工具
	Permission  permissions.Ruleset `json:"permission,omitempty"`  // 权限规则集
	Prompt      string              `json:"prompt"`                // 代理特定的系统提示词
	MaxTurns    int                 `json:"max_turns"`             // 最大轮次限制（对应 OpenCode steps）
	ReadOnly    bool                `json:"read_only"`             // 是否只读
	Temperature float64             `json:"temperature,omitempty"` // 温度参数
	TopP        float64             `json:"top_p,omitempty"`       // TopP 参数
	Model       string              `json:"model,omitempty"`       // 可选的模型覆盖
}

// DefaultPermission 默认权限规则集
// 参考 OpenCode agent.ts 第 55-73 行
var DefaultPermission = permissions.Ruleset{
	{Permission: "*", Pattern: "*", Action: permissions.BehaviorAllow},
	{Permission: "doom_loop", Pattern: "*", Action: permissions.BehaviorAsk},
	{Permission: "external_directory", Pattern: "*", Action: permissions.BehaviorAsk},
	{Permission: "question", Pattern: "*", Action: permissions.BehaviorDeny},
	{Permission: "plan_enter", Pattern: "*", Action: permissions.BehaviorDeny},
	{Permission: "plan_exit", Pattern: "*", Action: permissions.BehaviorDeny},
	{Permission: "read", Pattern: "*", Action: permissions.BehaviorAllow},
	{Permission: "read", Pattern: "*.env", Action: permissions.BehaviorAsk},
	{Permission: "read", Pattern: "*.env.*", Action: permissions.BehaviorAsk},
	{Permission: "read", Pattern: "*.env.example", Action: permissions.BehaviorAllow},
}

// 预定义的代理类型
// 参考 OpenCode agent.ts 第 76-202 行
var (
	// BuildAgent 默认构建代理 — 执行编码任务
	// 参考 OpenCode: build agent (primary, 全部工具 + question + plan_enter)
	BuildAgent = AgentType{
		Name:        "build",
		Description: "默认代理，执行编码任务，拥有完整工具权限",
		Mode:        ModePrimary,
		Native:      true,
		Tools:       []string{"*"},
		Permission: permissions.Merge(
			DefaultPermission,
			permissions.Ruleset{
				{Permission: "question", Pattern: "*", Action: permissions.BehaviorAllow},
				{Permission: "plan_enter", Pattern: "*", Action: permissions.BehaviorAllow},
			},
		),
		Prompt:   "",
		MaxTurns: 50,
		ReadOnly: false,
	}

	// ExploreAgent 探索代理 — 只读，用于搜索和分析
	// 参考 OpenCode: explore agent (subagent, 仅只读工具)
	ExploreAgent = AgentType{
		Name:        "explore",
		Description: "快速代码库探索代理，用于搜索文件、查找代码、分析结构",
		Mode:        ModeSubagent,
		Native:      true,
		Tools:       []string{"Bash", "Read", "Glob", "Grep", "WebFetch", "WebSearch"},
		Permission: permissions.Merge(
			DefaultPermission,
			permissions.Ruleset{
				{Permission: "*", Pattern: "*", Action: permissions.BehaviorDeny},
				{Permission: "grep", Pattern: "*", Action: permissions.BehaviorAllow},
				{Permission: "glob", Pattern: "*", Action: permissions.BehaviorAllow},
				{Permission: "bash", Pattern: "*", Action: permissions.BehaviorAllow},
				{Permission: "read", Pattern: "*", Action: permissions.BehaviorAllow},
				{Permission: "webfetch", Pattern: "*", Action: permissions.BehaviorAllow},
				{Permission: "websearch", Pattern: "*", Action: permissions.BehaviorAllow},
				{Permission: "task", Pattern: "*", Action: permissions.BehaviorDeny},
			},
		),
		Prompt: `You are a file search specialist. You excel at thoroughly navigating and exploring codebases.

Your strengths:
- Rapidly finding files using glob patterns
- Searching code and text with powerful regex patterns
- Reading and analyzing file contents

Guidelines:
- Use Glob for broad file pattern matching
- Use Grep for searching file contents with regex
- Use Read when you know the specific file path
- Use Bash for file operations like listing directory contents
- Adapt your search approach based on the thoroughness level specified
- Return file paths as absolute paths in your final response
- Do not create any files or modify the user's system state`,
		MaxTurns: 20,
		ReadOnly: true,
	}

	// PlanAgent 规划代理 — 只读 + 计划文件编辑
	// 参考 OpenCode: plan agent (primary, 禁止编辑除计划文件)
	PlanAgent = AgentType{
		Name:        "plan",
		Description: "规划模式代理，禁止编辑工具（计划文件除外）",
		Mode:        ModePrimary,
		Native:      true,
		Tools:       []string{"Bash", "Read", "Glob", "Grep", "WebFetch", "WebSearch"},
		Permission: permissions.Merge(
			DefaultPermission,
			permissions.Ruleset{
				{Permission: "question", Pattern: "*", Action: permissions.BehaviorAllow},
				{Permission: "plan_exit", Pattern: "*", Action: permissions.BehaviorAllow},
				{Permission: "edit", Pattern: "*", Action: permissions.BehaviorDeny},
				{Permission: "edit", Pattern: "*.md", Action: permissions.BehaviorAllow},
			},
		),
		Prompt: `You are a planning agent. Your job is to analyze and design.

Rules:
- NEVER modify source code files
- You may only create/edit markdown plan files
- Analyze the codebase structure thoroughly
- Output a numbered implementation plan
- Consider edge cases and potential issues`,
		MaxTurns: 15,
		ReadOnly: true,
	}

	// GeneralAgent 通用子代理 — 用于并行多步骤任务
	// 参考 OpenCode: general agent (subagent, 全部工具但无 todo)
	GeneralAgent = AgentType{
		Name:        "general",
		Description: "通用子代理，用于执行多步骤任务",
		Mode:        ModeSubagent,
		Native:      true,
		Tools:       []string{"*"},
		Permission: permissions.Merge(
			DefaultPermission,
			permissions.Ruleset{
				{Permission: "todoread", Pattern: "*", Action: permissions.BehaviorDeny},
				{Permission: "todowrite", Pattern: "*", Action: permissions.BehaviorDeny},
				{Permission: "task", Pattern: "*", Action: permissions.BehaviorDeny},
			},
		),
		Prompt:   "",
		MaxTurns: 30,
		ReadOnly: false,
	}

	// CompactionAgent 压缩代理 — 隐藏，用于上下文压缩
	// 参考 OpenCode: compaction agent (hidden, 无工具)
	CompactionAgent = AgentType{
		Name:        "compaction",
		Description: "上下文压缩代理",
		Mode:        ModePrimary,
		Native:      true,
		Hidden:      true,
		Tools:       []string{},
		Permission: permissions.Merge(
			DefaultPermission,
			permissions.Ruleset{
				{Permission: "*", Pattern: "*", Action: permissions.BehaviorDeny},
			},
		),
		Prompt:   "",
		MaxTurns: 1,
		ReadOnly: true,
	}

	// TitleAgent 标题生成代理 — 隐藏
	// 参考 OpenCode: title agent (hidden, 无工具, temperature 0.5)
	TitleAgent = AgentType{
		Name:        "title",
		Description: "会话标题生成代理",
		Mode:        ModePrimary,
		Native:      true,
		Hidden:      true,
		Tools:       []string{},
		Permission: permissions.Merge(
			DefaultPermission,
			permissions.Ruleset{
				{Permission: "*", Pattern: "*", Action: permissions.BehaviorDeny},
			},
		),
		Prompt:      "",
		MaxTurns:    1,
		Temperature: 0.5,
		ReadOnly:    true,
	}
)

// Registry 代理类型注册表
type Registry struct {
	types map[string]*AgentType
}

// NewRegistry 创建代理类型注册表
func NewRegistry() *Registry {
	r := &Registry{
		types: make(map[string]*AgentType),
	}
	// 注册预定义类型（参考 OpenCode agent.ts）
	r.Register(&BuildAgent)
	r.Register(&ExploreAgent)
	r.Register(&PlanAgent)
	r.Register(&GeneralAgent)
	r.Register(&CompactionAgent)
	r.Register(&TitleAgent)
	return r
}

// Register 注册代理类型
func (r *Registry) Register(t *AgentType) {
	r.types[t.Name] = t
}

// Get 获取代理类型
func (r *Registry) Get(name string) *AgentType {
	return r.types[name]
}

// List 列出所有代理类型
func (r *Registry) List() []*AgentType {
	result := make([]*AgentType, 0, len(r.types))
	for _, t := range r.types {
		result = append(result, t)
	}
	return result
}

// GetDescriptions 获取所有代理类型的描述（用于系统提示词）
func (r *Registry) GetDescriptions() string {
	var desc string
	for name, t := range r.types {
		desc += "- " + name + ": " + t.Description + "\n"
	}
	return desc
}

// ListSubagents 列出可用于 Task 工具的子代理（subagent/all 模式，非 hidden）
func (r *Registry) ListSubagents() []*AgentType {
	var result []*AgentType
	for _, t := range r.types {
		if (t.Mode == ModeSubagent || t.Mode == ModeAll) && !t.Hidden {
			result = append(result, t)
		}
	}
	return result
}

// Unregister 注销代理类型
func (r *Registry) Unregister(name string) {
	delete(r.types, name)
}

// UnregisterNonNative 清理所有非内置代理
func (r *Registry) UnregisterNonNative() {
	for name, t := range r.types {
		if !t.Native {
			delete(r.types, name)
		}
	}
}

// IsToolAllowed 检查工具是否被允许（工具列表 + 权限规则集双重检查）
func (t *AgentType) IsToolAllowed(toolName string) bool {
	// 1. 先检查工具列表
	toolAllowed := false
	if len(t.Tools) == 1 && t.Tools[0] == "*" {
		toolAllowed = true
	} else {
		for _, allowed := range t.Tools {
			if allowed == toolName {
				toolAllowed = true
				break
			}
		}
	}
	if !toolAllowed {
		return false
	}

	// 2. 再检查权限规则集（如果有）
	if len(t.Permission) > 0 {
		rule := permissions.Evaluate(strings.ToLower(toolName), "*", t.Permission)
		return rule.Action != permissions.BehaviorDeny
	}

	return true
}

// CheckPermission 检查特定权限+模式
// 参考 OpenCode: PermissionNext.evaluate(permission, pattern, ruleset)
func (t *AgentType) CheckPermission(permission, pattern string) permissions.CheckBehavior {
	if len(t.Permission) == 0 {
		return permissions.BehaviorAllow
	}
	rule := permissions.Evaluate(strings.ToLower(permission), pattern, t.Permission)
	return rule.Action
}

// FilterToolDefinitions 根据 Agent 权限过滤工具定义列表
// 参考 OpenCode: PermissionNext.disabled() 过滤被禁用的工具
func (t *AgentType) FilterToolDefinitions(allDefs []interface{}) []interface{} {
	if len(t.Tools) == 1 && t.Tools[0] == "*" && len(t.Permission) == 0 {
		return allDefs // 无限制
	}

	var filtered []interface{}
	for _, def := range allDefs {
		if defMap, ok := def.(map[string]interface{}); ok {
			if name, ok := defMap["name"].(string); ok {
				if t.IsToolAllowed(name) {
					filtered = append(filtered, def)
				}
			}
		}
	}
	return filtered
}

// GetDefault 获取默认代理
func (r *Registry) GetDefault() *AgentType {
	// 优先返回 build agent
	if a := r.Get("build"); a != nil {
		return a
	}
	// 回退到第一个 primary 可见代理
	for _, a := range r.types {
		if a.Mode != ModeSubagent && !a.Hidden {
			return a
		}
	}
	return nil
}

// ListVisible 列出所有可见代理（非 hidden）
func (r *Registry) ListVisible() []*AgentType {
	var result []*AgentType
	for _, t := range r.types {
		if !t.Hidden {
			result = append(result, t)
		}
	}
	return result
}

// ListPrimary 列出所有主代理（可由用户直接选择）
func (r *Registry) ListPrimary() []*AgentType {
	var result []*AgentType
	for _, t := range r.types {
		if (t.Mode == ModePrimary || t.Mode == ModeAll) && !t.Hidden {
			result = append(result, t)
		}
	}
	return result
}

// 全局注册表实例
var DefaultRegistry = NewRegistry()
