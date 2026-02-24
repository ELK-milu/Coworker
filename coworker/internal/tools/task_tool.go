package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/coworker/internal/agent"
	"github.com/QuantumNous/new-api/coworker/internal/store"
	"github.com/QuantumNous/new-api/coworker/pkg/types"
)

// TaskTool 子代理任务工具 — 渐进式披露 + Resume + 反递归
// 参考 OpenCode tool/task.ts + Claude Code Task tool:
//   - Description() 动态列出 <available_agents>（仅 name + description）
//   - Execute() 按需启动独立子会话
type TaskTool struct {
	agentRegistry *agent.Registry
	store         *store.Manager
	executor      SubagentExecutor

	mu           sync.RWMutex
	userID       string
	cachedDesc   string
	cachedAgents []*agent.AgentType
}

// SubagentExecutor 子代理执行器接口
type SubagentExecutor interface {
	// Execute 执行子代理任务
	// resumeSessionID 非空时恢复已有子会话
	// 返回子代理输出文本和子会话 ID
	Execute(ctx context.Context, agentType *agent.AgentType, prompt string, resumeSessionID string) (result string, sessionID string, err error)
}

// TaskInput Task 工具输入
type TaskInput struct {
	Description  string `json:"description"`            // 短任务名（3-5 词）
	Prompt       string `json:"prompt"`                 // 详细指令
	SubagentType string `json:"subagent_type"`          // 子代理类型
	TaskID       string `json:"task_id,omitempty"`       // 可选：恢复已有子会话
}

// NewTaskTool 创建 Task 工具
func NewTaskTool(storeMgr *store.Manager) *TaskTool {
	t := &TaskTool{
		agentRegistry: agent.DefaultRegistry,
		store:         storeMgr,
	}
	t.rebuildCache("")
	return t
}

// SetExecutor 设置子代理执行器
func (t *TaskTool) SetExecutor(executor SubagentExecutor) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.executor = executor
}

// RefreshForUser 刷新当前用户的可用子代理列表（每次对话前调用）
func (t *TaskTool) RefreshForUser(userID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.userID = userID
	t.rebuildCache(userID)
}

func (t *TaskTool) Name() string { return "Task" }

// Description 动态返回包含 <available_agents> 的描述
func (t *TaskTool) Description() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.cachedDesc
}

func (t *TaskTool) InputSchema() map[string]interface{} {
	t.mu.RLock()
	agents := t.cachedAgents
	t.mu.RUnlock()

	typeNames := make([]string, 0, len(agents))
	for _, ag := range agents {
		typeNames = append(typeNames, ag.Name)
	}

	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"description": map[string]interface{}{
				"type":        "string",
				"description": "A short (3-5 word) description of the task",
			},
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "The task for the agent to perform",
			},
			"subagent_type": map[string]interface{}{
				"type":        "string",
				"enum":        typeNames,
				"description": "The type of specialized agent to use for this task",
			},
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "Optional: resume a previous subagent session by its task_id",
			},
		},
		"required": []string{"description", "prompt", "subagent_type"},
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

	// 获取代理类型（仅 subagent/all 模式）
	agentType := t.agentRegistry.Get(in.SubagentType)
	if agentType == nil {
		return &types.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("unknown subagent type: %s. Use one from the available_agents list.", in.SubagentType),
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}
	if agentType.Mode != agent.ModeSubagent && agentType.Mode != agent.ModeAll {
		return &types.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("agent %q is not available as a subagent (mode=%s)", in.SubagentType, agentType.Mode),
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	t.mu.RLock()
	executor := t.executor
	t.mu.RUnlock()

	if executor == nil {
		return &types.ToolResult{
			Success:   false,
			Error:     "subagent executor not configured",
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 执行子代理
	result, sessionID, err := executor.Execute(ctx, agentType, in.Prompt, in.TaskID)
	if err != nil {
		return &types.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("subagent failed: %v", err),
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 格式化输出（含 task_id 供 resume）
	output := fmt.Sprintf("task_id: %s\n<task_result>\n%s\n</task_result>", sessionID, result)

	return &types.ToolResult{
		Success:   true,
		Output:    output,
		ElapsedMs: time.Since(startTime).Milliseconds(),
		Metadata: map[string]interface{}{
			"subagent_type": in.SubagentType,
			"description":   in.Description,
			"task_id":       sessionID,
		},
	}, nil
}

// rebuildCache 重建缓存（调用方需持有写锁）
func (t *TaskTool) rebuildCache(userID string) {
	// 收集：内置子代理
	agents := t.agentRegistry.ListSubagents()
	t.cachedAgents = agents

	if len(agents) == 0 {
		t.cachedDesc = "Launch a subagent to handle a focused subtask with isolated context. No agents are currently available."
		return
	}

	// 构建 description（类似 SkillsTool 的渐进式披露）
	var lines []string
	lines = append(lines,
		"Launch a new agent to handle complex, multi-step tasks autonomously.",
		"",
		"Each agent runs in an independent session with its own context.",
		"Use this to keep the main conversation clean and delegate focused work.",
		"",
		"You can resume a previous subagent session by passing its task_id.",
		"",
		"<available_agents>",
	)
	for _, ag := range agents {
		lines = append(lines,
			"  <agent>",
			fmt.Sprintf("    <name>%s</name>", ag.Name),
			fmt.Sprintf("    <description>%s</description>", ag.Description),
			"  </agent>",
		)
	}
	lines = append(lines, "</available_agents>")
	t.cachedDesc = strings.Join(lines, "\n")
}
