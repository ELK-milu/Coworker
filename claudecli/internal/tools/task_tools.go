package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/QuantumNous/new-api/claudecli/internal/task"
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// TaskNotifier 任务变更通知接口
type TaskNotifier interface {
	NotifyTaskChanged(action string, task *task.Task)
}

// ========== TaskCreate 工具 ==========

// TaskCreateTool 创建任务工具
type TaskCreateTool struct {
	tasks    *task.Manager
	notifier TaskNotifier
}

// NewTaskCreateTool 创建 TaskCreate 工具
func NewTaskCreateTool(tm *task.Manager) *TaskCreateTool {
	return &TaskCreateTool{tasks: tm}
}

// SetNotifier 设置通知器
func (t *TaskCreateTool) SetNotifier(n TaskNotifier) {
	t.notifier = n
}

func (t *TaskCreateTool) Name() string {
	return "TaskCreate"
}

func (t *TaskCreateTool) Description() string {
	return `Create a new task in the task list. Use this to track work items and progress.

IMPORTANT: Always provide activeForm when creating tasks. The subject should be imperative (e.g., "Run tests") while activeForm should be present continuous (e.g., "Running tests").`
}

func (t *TaskCreateTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"subject": map[string]interface{}{
				"type":        "string",
				"description": "A brief title for the task in imperative form (e.g., 'Fix authentication bug')",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "A detailed description of what needs to be done",
			},
			"activeForm": map[string]interface{}{
				"type":        "string",
				"description": "Present continuous form shown in spinner when in_progress (e.g., 'Fixing authentication bug')",
			},
		},
		"required": []string{"subject", "description", "activeForm"},
	}
}

// TaskCreateInput 创建任务输入
type TaskCreateInput struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
	ActiveForm  string `json:"activeForm"`
}

func (t *TaskCreateTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	var params TaskCreateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   "Invalid input: " + err.Error(),
		}, nil
	}

	// 从 context 获取 userID
	userID := getContextUserID(ctx)
	listID := "default"

	newTask, err := t.tasks.Create(userID, listID, params.Subject, params.Description, params.ActiveForm)
	if err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   "Failed to create task: " + err.Error(),
		}, nil
	}

	// 通知前端
	if t.notifier != nil {
		t.notifier.NotifyTaskChanged("created", newTask)
	}

	// 返回结果，不显示 ID（ID 顺序应通过 TaskList 获取）
	// 只显示关键信息：subject, status, activeForm, internalId
	return &types.ToolResult{
		Success: true,
		Output: fmt.Sprintf("Task created successfully:\n  internalId: %s\n  subject: %s\n  status: %s\n  activeForm: %s\n\nUse internalId for subsequent updates to avoid ID shift issues.",
			newTask.InternalID, newTask.Subject, newTask.Status, newTask.ActiveForm),
		Metadata: map[string]interface{}{
			"task_changed": true,
			"action":       "created",
			"task":         newTask,
		},
	}, nil
}

// ========== TaskUpdate 工具 ==========

// TaskUpdateTool 更新任务工具
type TaskUpdateTool struct {
	tasks    *task.Manager
	notifier TaskNotifier
}

// NewTaskUpdateTool 创建 TaskUpdate 工具
func NewTaskUpdateTool(tm *task.Manager) *TaskUpdateTool {
	return &TaskUpdateTool{tasks: tm}
}

// SetNotifier 设置通知器
func (t *TaskUpdateTool) SetNotifier(n TaskNotifier) {
	t.notifier = n
}

func (t *TaskUpdateTool) Name() string {
	return "TaskUpdate"
}

func (t *TaskUpdateTool) Description() string {
	return `Update a task in the task list.

Use this to:
- Mark tasks as in_progress when starting work
- Mark tasks as completed when finished
- Update task details or dependencies
- Delete tasks that are no longer needed

IMPORTANT:
- Only mark a task as completed when you have FULLY accomplished it.
- Prefer using internalId (from TaskCreate output) over taskId to avoid ID shift issues when tasks are deleted.`
}

func (t *TaskUpdateTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"taskId": map[string]interface{}{
				"type":        "string",
				"description": "The dynamic ID of the task (position-based, may change when tasks are deleted)",
			},
			"internalId": map[string]interface{}{
				"type":        "string",
				"description": "The stable internal ID of the task (preferred, won't change when other tasks are deleted)",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"pending", "in_progress", "completed", "deleted"},
				"description": "New status for the task",
			},
			"subject": map[string]interface{}{
				"type":        "string",
				"description": "New subject for the task",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "New description for the task",
			},
			"activeForm": map[string]interface{}{
				"type":        "string",
				"description": "Present continuous form shown when in_progress",
			},
			"addBlocks": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Task IDs that this task blocks",
			},
			"addBlockedBy": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Task IDs that block this task",
			},
		},
	}
}

// TaskUpdateInput 更新任务输入
type TaskUpdateInput struct {
	TaskID      string   `json:"taskId"`
	InternalID  string   `json:"internalId,omitempty"`
	Status      string   `json:"status,omitempty"`
	Subject     string   `json:"subject,omitempty"`
	Description string   `json:"description,omitempty"`
	ActiveForm  string   `json:"activeForm,omitempty"`
	AddBlocks   []string `json:"addBlocks,omitempty"`
	AddBlockedBy []string `json:"addBlockedBy,omitempty"`
}

func (t *TaskUpdateTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	var params TaskUpdateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   "Invalid input: " + err.Error(),
		}, nil
	}

	userID := getContextUserID(ctx)
	listID := "default"

	// 检查是否提供了 taskId 或 internalId
	if params.TaskID == "" && params.InternalID == "" {
		return &types.ToolResult{
			Success: false,
			Error:   "Either taskId or internalId is required",
		}, nil
	}

	// 构建更新 map
	updates := make(map[string]interface{})
	if params.Status != "" {
		updates["status"] = params.Status
	}
	if params.Subject != "" {
		updates["subject"] = params.Subject
	}
	if params.Description != "" {
		updates["description"] = params.Description
	}
	if params.ActiveForm != "" {
		updates["activeForm"] = params.ActiveForm
	}
	if len(params.AddBlocks) > 0 {
		updates["addBlocks"] = params.AddBlocks
	}
	if len(params.AddBlockedBy) > 0 {
		updates["addBlockedBy"] = params.AddBlockedBy
	}

	// 优先使用 InternalID（更稳定）
	var updatedTask *task.Task
	var err error
	if params.InternalID != "" {
		updatedTask, err = t.tasks.UpdateByInternalID(userID, listID, params.InternalID, updates)
	} else {
		updatedTask, err = t.tasks.Update(userID, listID, params.TaskID, updates)
	}
	if err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   "Failed to update task: " + err.Error(),
		}, nil
	}

	// 通知前端
	action := "updated"
	if params.Status == "deleted" {
		action = "deleted"
	}
	if t.notifier != nil {
		t.notifier.NotifyTaskChanged(action, updatedTask)
	}

	taskJSON, _ := json.MarshalIndent(updatedTask, "", "  ")
	return &types.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Task updated successfully:\n%s", string(taskJSON)),
		Metadata: map[string]interface{}{
			"task_changed": true,
			"action":       action,
			"task":         updatedTask,
		},
	}, nil
}

// ========== TaskList 工具 ==========

// TaskListTool 列出任务工具
type TaskListTool struct {
	tasks *task.Manager
}

// NewTaskListTool 创建 TaskList 工具
func NewTaskListTool(tm *task.Manager) *TaskListTool {
	return &TaskListTool{tasks: tm}
}

func (t *TaskListTool) Name() string {
	return "TaskList"
}

func (t *TaskListTool) Description() string {
	return `List all tasks in the task list.

Use this to:
- See what tasks are available to work on
- Check overall progress on the project
- Find tasks that are blocked and need dependencies resolved

Prefer working on tasks in ID order (lowest ID first) when multiple tasks are available.`
}

func (t *TaskListTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *TaskListTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	userID := getContextUserID(ctx)
	listID := "default"

	tasks := t.tasks.List(userID, listID)

	if len(tasks) == 0 {
		return &types.ToolResult{
			Success: true,
			Output:  "No tasks found.",
		}, nil
	}

	// 使用 Render 方法生成人类可读的输出
	rendered := t.tasks.Render(userID, listID)
	return &types.ToolResult{
		Success: true,
		Output:  rendered,
	}, nil
}

// ========== TaskGet 工具 ==========

// TaskGetTool 获取任务详情工具
type TaskGetTool struct {
	tasks *task.Manager
}

// NewTaskGetTool 创建 TaskGet 工具
func NewTaskGetTool(tm *task.Manager) *TaskGetTool {
	return &TaskGetTool{tasks: tm}
}

func (t *TaskGetTool) Name() string {
	return "TaskGet"
}

func (t *TaskGetTool) Description() string {
	return `Get full details of a specific task by ID.

Use this to:
- Get the full description and context before starting work
- Understand task dependencies
- Verify task requirements`
}

func (t *TaskGetTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"taskId": map[string]interface{}{
				"type":        "string",
				"description": "The ID of the task to retrieve",
			},
		},
		"required": []string{"taskId"},
	}
}

// TaskGetInput 获取任务输入
type TaskGetInput struct {
	TaskID string `json:"taskId"`
}

func (t *TaskGetTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	var params TaskGetInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   "Invalid input: " + err.Error(),
		}, nil
	}

	userID := getContextUserID(ctx)
	listID := "default"

	taskItem := t.tasks.Get(userID, listID, params.TaskID)
	if taskItem == nil {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Task not found: %s", params.TaskID),
		}, nil
	}

	taskJSON, _ := json.MarshalIndent(taskItem, "", "  ")
	return &types.ToolResult{
		Success: true,
		Output:  string(taskJSON),
	}, nil
}

// ========== 辅助函数 ==========

// getContextUserID 从 context 获取用户 ID
func getContextUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(types.UserIDKey).(string); ok && userID != "" {
		return userID
	}
	return "default_user"
}
