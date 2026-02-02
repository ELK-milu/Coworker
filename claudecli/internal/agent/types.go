package agent

// AgentType 代理类型配置
// 参考 learn-claude-code v3 设计哲学：分而治之，上下文隔离
type AgentType struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tools       []string `json:"tools"`       // 允许的工具列表，"*" 表示所有工具
	Prompt      string   `json:"prompt"`      // 代理特定的系统提示词
	MaxTurns    int      `json:"max_turns"`   // 最大轮次限制
	ReadOnly    bool     `json:"read_only"`   // 是否只读
}

// 预定义的代理类型
var (
	// ExploreAgent 探索代理 - 只读，用于搜索和分析
	ExploreAgent = AgentType{
		Name:        "explore",
		Description: "只读探索代理，用于搜索代码、查找文件、分析结构",
		Tools:       []string{"Bash", "Read", "Glob", "Grep"},
		Prompt: `You are an exploration agent. Your job is to search and analyze the codebase.

Rules:
- NEVER modify any files
- Search thoroughly and report findings concisely
- Return a clear, structured summary of what you found`,
		MaxTurns: 20,
		ReadOnly: true,
	}

	// CodeAgent 编码代理 - 完整权限，用于实现功能
	CodeAgent = AgentType{
		Name:        "code",
		Description: "完整编码代理，用于实现功能和修复 bug",
		Tools:       []string{"*"}, // 所有工具
		Prompt: `You are a coding agent. Your job is to implement changes efficiently.

Rules:
- Make minimal, focused changes
- Test your changes when possible
- Report what you changed clearly`,
		MaxTurns: 50,
		ReadOnly: false,
	}

	// PlanAgent 规划代理 - 只读，用于设计方案
	PlanAgent = AgentType{
		Name:        "plan",
		Description: "规划代理，用于分析代码库并设计实现方案",
		Tools:       []string{"Bash", "Read", "Glob", "Grep"},
		Prompt: `You are a planning agent. Your job is to analyze and design.

Rules:
- NEVER modify any files
- Analyze the codebase structure
- Output a numbered implementation plan
- Consider edge cases and potential issues`,
		MaxTurns: 15,
		ReadOnly: true,
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
	// 注册预定义类型
	r.Register(&ExploreAgent)
	r.Register(&CodeAgent)
	r.Register(&PlanAgent)
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

// IsToolAllowed 检查工具是否被允许
func (t *AgentType) IsToolAllowed(toolName string) bool {
	if len(t.Tools) == 1 && t.Tools[0] == "*" {
		return true
	}
	for _, allowed := range t.Tools {
		if allowed == toolName {
			return true
		}
	}
	return false
}

// 全局注册表实例
var DefaultRegistry = NewRegistry()
