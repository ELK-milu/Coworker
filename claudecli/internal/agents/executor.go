package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// ToolExecutor 工具执行接口
type ToolExecutor interface {
	Execute(ctx context.Context, name string, input json.RawMessage) (*types.ToolResult, error)
	GetDefinitions() []types.ToolDefinition
}

// SendFunc 发送消息回调类型
type SendFunc func(ctx context.Context, system string, messages []map[string]interface{}, tools []types.ToolDefinition) ([]interface{}, error)

// Executor 代理执行器
type Executor struct {
	sendFunc  SendFunc
	tools     ToolExecutor
	tasks     map[string]*AgentTask
	mu        sync.RWMutex
	idCounter uint64
}

// NewExecutor 创建执行器
func NewExecutor(sendFunc SendFunc, t ToolExecutor) *Executor {
	return &Executor{
		sendFunc: sendFunc,
		tools:    t,
		tasks:    make(map[string]*AgentTask),
	}
}

// Execute 执行代理任务
func (e *Executor) Execute(ctx context.Context, agentType AgentType, prompt, desc string) (*AgentTask, error) {
	cfg, ok := DefaultAgents[agentType]
	if !ok {
		return nil, fmt.Errorf("unknown agent type: %s", agentType)
	}

	id := fmt.Sprintf("agent_%d", atomic.AddUint64(&e.idCounter, 1))
	task := &AgentTask{
		ID:          id,
		Type:        agentType,
		Prompt:      prompt,
		Description: desc,
		Status:      StatusPending,
	}

	e.mu.Lock()
	e.tasks[id] = task
	e.mu.Unlock()

	go e.run(ctx, task, cfg)
	return task, nil
}

// run 运行代理任务
func (e *Executor) run(ctx context.Context, task *AgentTask, cfg *AgentConfig) {
	e.updateStatus(task.ID, StatusRunning)
	log.Printf("[Agent] Starting %s: %s", task.Type, task.ID)

	systemPrompt := e.buildSystemPrompt(task.Type)
	messages := []map[string]interface{}{
		{"role": "user", "content": task.Prompt},
	}

	var result string
	for turn := 0; turn < cfg.MaxTurns; turn++ {
		select {
		case <-ctx.Done():
			e.setError(task.ID, "cancelled")
			return
		default:
		}

		if e.sendFunc == nil {
			e.setError(task.ID, "send function not set")
			return
		}

		resp, err := e.sendFunc(ctx, systemPrompt, messages, e.tools.GetDefinitions())
		if err != nil {
			e.setError(task.ID, err.Error())
			return
		}

		hasToolUse := false
		for _, block := range resp {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockMap["type"] == "text" {
					if text, ok := blockMap["text"].(string); ok {
						result = text
					}
				}
				if blockMap["type"] == "tool_use" {
					hasToolUse = true
					toolResult := e.executeTool(ctx, blockMap)
					messages = append(messages, map[string]interface{}{
						"role":    "assistant",
						"content": resp,
					})
					messages = append(messages, map[string]interface{}{
						"role":    "user",
						"content": []interface{}{toolResult},
					})
				}
			}
		}

		if !hasToolUse {
			break
		}
	}

	e.setResult(task.ID, result)
	log.Printf("[Agent] Completed %s: %s", task.Type, task.ID)
}

// buildSystemPrompt 构建代理系统提示
func (e *Executor) buildSystemPrompt(agentType AgentType) string {
	switch agentType {
	case AgentExplore:
		return "You are an Explore agent. Search and analyze the codebase."
	case AgentPlan:
		return "You are a Plan agent. Design implementation strategies."
	case AgentDebugger:
		return "You are a Debugger agent. Diagnose and fix issues."
	default:
		return "You are a general-purpose agent."
	}
}

// executeTool 执行工具
func (e *Executor) executeTool(ctx context.Context, block map[string]interface{}) map[string]interface{} {
	name, _ := block["name"].(string)
	id, _ := block["id"].(string)
	input, _ := json.Marshal(block["input"])

	result, err := e.tools.Execute(ctx, name, input)
	content := ""
	isError := false
	if err != nil {
		content = err.Error()
		isError = true
	} else if !result.Success {
		content = result.Error
		isError = true
	} else {
		content = result.Output
	}

	return map[string]interface{}{
		"type":        "tool_result",
		"tool_use_id": id,
		"content":     content,
		"is_error":    isError,
	}
}

// updateStatus 更新任务状态
func (e *Executor) updateStatus(id string, status TaskStatus) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if task, ok := e.tasks[id]; ok {
		task.Status = status
	}
}

// setResult 设置任务结果
func (e *Executor) setResult(id, result string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if task, ok := e.tasks[id]; ok {
		task.Status = StatusCompleted
		task.Result = result
	}
}

// setError 设置任务错误
func (e *Executor) setError(id, err string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if task, ok := e.tasks[id]; ok {
		task.Status = StatusFailed
		task.Error = err
	}
}

// Get 获取任务
func (e *Executor) Get(id string) *AgentTask {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tasks[id]
}

// List 列出所有任务
func (e *Executor) List() []*AgentTask {
	e.mu.RLock()
	defer e.mu.RUnlock()
	list := make([]*AgentTask, 0, len(e.tasks))
	for _, t := range e.tasks {
		list = append(list, t)
	}
	return list
}
