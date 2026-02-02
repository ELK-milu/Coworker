package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/claudecli/internal/agent"
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// TaskTool 子代理任务工具
// 参考 learn-claude-code v3 设计：分而治之，上下文隔离
type TaskTool struct {
	agentRegistry *agent.Registry
	// 子代理执行器回调（由外部注入）
	executor SubagentExecutor
}

// SubagentExecutor 子代理执行器接口
type SubagentExecutor interface {
	// Execute 执行子代理任务
	// 返回子代理的最终输出文本
	Execute(ctx context.Context, agentType *agent.AgentType, prompt string) (string, error)
}

// TaskInput Task 工具输入
type TaskInput struct {
	Description string `json:"description"` // 短任务名（3-5 词）
	Prompt      string `json:"prompt"`      // 详细指令
	AgentType   string `json:"agent_type"`  // 代理类型
}

// NewTaskTool 创建 Task 工具
func NewTaskTool() *TaskTool {
	return &TaskTool{
		agentRegistry: agent.DefaultRegistry,
	}
}

// SetExecutor 设置子代理执行器
func (t *TaskTool) SetExecutor(executor SubagentExecutor) {
	t.executor = executor
}

func (t *TaskTool) Name() string { return "Task" }

func (t *TaskTool) Description() string {
	desc := `Spawn a subagent for a focused subtask with ISOLATED context.

Subagents run in their own context - they don't see parent's history.
Use this to keep the main conversation clean and focused.

Agent types:
` + t.agentRegistry.GetDescriptions() + `
Example uses:
- Task(explore): "Find all files using the auth module"
- Task(plan): "Design a migration strategy for the database"
- Task(code): "Implement the user registration form"`

	return desc
}

func (t *TaskTool) InputSchema() map[string]interface{} {
	agentTypes := t.agentRegistry.List()
	typeNames := make([]string, 0, len(agentTypes))
	for _, at := range agentTypes {
		typeNames = append(typeNames, at.Name)
	}

	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Short task name (3-5 words) for progress display",
			},
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "Detailed instructions for the subagent",
			},
			"agent_type": map[string]interface{}{
				"type":        "string",
				"enum":        typeNames,
				"description": "Type of agent to spawn",
			},
		},
		"required": []string{"description", "prompt", "agent_type"},
	}
}

func (t *TaskTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	startTime := time.Now()

	var in TaskInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("failed to parse input: %v", err),
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 获取代理类型
	agentType := t.agentRegistry.Get(in.AgentType)
	if agentType == nil {
		return &types.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("unknown agent type: %s", in.AgentType),
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 检查是否有执行器
	if t.executor == nil {
		return &types.ToolResult{
			Success:   false,
			Error:     "subagent executor not configured",
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 执行子代理
	result, err := t.executor.Execute(ctx, agentType, in.Prompt)
	if err != nil {
		return &types.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("subagent failed: %v", err),
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	return &types.ToolResult{
		Success:   true,
		Output:    result,
		ElapsedMs: time.Since(startTime).Milliseconds(),
		Metadata: map[string]interface{}{
			"agent_type":  in.AgentType,
			"description": in.Description,
		},
	}, nil
}
