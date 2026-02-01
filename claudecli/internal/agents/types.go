package agents

// AgentType 代理类型
type AgentType string

const (
	AgentExplore        AgentType = "Explore"
	AgentPlan           AgentType = "Plan"
	AgentGeneralPurpose AgentType = "general-purpose"
	AgentDebugger       AgentType = "debugger"
)

// AgentConfig 代理配置
type AgentConfig struct {
	Type        AgentType `json:"type"`
	Description string    `json:"description"`
	Model       string    `json:"model,omitempty"`
	MaxTurns    int       `json:"max_turns,omitempty"`
}

// AgentTask 代理任务
type AgentTask struct {
	ID          string      `json:"id"`
	Type        AgentType   `json:"type"`
	Prompt      string      `json:"prompt"`
	Description string      `json:"description"`
	Status      TaskStatus  `json:"status"`
	Result      string      `json:"result,omitempty"`
	Error       string      `json:"error,omitempty"`
}

// TaskStatus 任务状态
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
)

// DefaultAgents 默认代理配置
var DefaultAgents = map[AgentType]*AgentConfig{
	AgentExplore: {
		Type:        AgentExplore,
		Description: "Fast agent for exploring codebases",
		MaxTurns:    10,
	},
	AgentPlan: {
		Type:        AgentPlan,
		Description: "Software architect for designing plans",
		MaxTurns:    15,
	},
	AgentGeneralPurpose: {
		Type:        AgentGeneralPurpose,
		Description: "General-purpose agent for complex tasks",
		MaxTurns:    20,
	},
	AgentDebugger: {
		Type:        AgentDebugger,
		Description: "Debugging specialist for errors",
		MaxTurns:    15,
	},
}
