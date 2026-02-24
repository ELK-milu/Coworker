package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/QuantumNous/new-api/coworker/pkg/types"
)

// AgentExecutor 代理执行接口
type AgentExecutor interface {
	Execute(ctx context.Context, agentType string, prompt, desc string) (string, error)
}

// TaskAgentTool 子代理任务工具
type TaskAgentTool struct {
	executor AgentExecutor
}

// TaskAgentInput 输入参数
type TaskAgentInput struct {
	SubagentType string `json:"subagent_type"`
	Prompt       string `json:"prompt"`
	Description  string `json:"description"`
}

// NewTaskAgentTool 创建工具实例
func NewTaskAgentTool(exec AgentExecutor) *TaskAgentTool {
	return &TaskAgentTool{executor: exec}
}

func (t *TaskAgentTool) Name() string { return "Task" }

func (t *TaskAgentTool) Description() string {
	return "Launch a specialized agent to handle complex tasks."
}

func (t *TaskAgentTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"subagent_type": map[string]interface{}{
				"type": "string",
				"enum": []string{"Explore", "Plan", "general-purpose", "debugger"},
			},
			"prompt":      map[string]interface{}{"type": "string"},
			"description": map[string]interface{}{"type": "string"},
		},
		"required": []string{"subagent_type", "prompt", "description"},
	}
}

func (t *TaskAgentTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	var in TaskAgentInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	if t.executor == nil {
		return &types.ToolResult{Success: false, Error: "agent executor not available"}, nil
	}

	taskID, err := t.executor.Execute(ctx, in.SubagentType, in.Prompt, in.Description)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	output := fmt.Sprintf("Agent task started: %s", taskID)
	return &types.ToolResult{Success: true, Output: output}, nil
}
