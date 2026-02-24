package task

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Status 任务状态
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusDeleted    Status = "deleted"
)

// 约束常量 - 参考 learn-claude-code v2 设计哲学
const (
	MaxTasks = 20 // 最多 20 条任务，防止无限任务列表
)

// ErrMaxTasksReached 达到最大任务数
var ErrMaxTasksReached = fmt.Errorf("max %d tasks allowed", MaxTasks)

// ErrOnlyOneInProgress 只能有一个进行中的任务
var ErrOnlyOneInProgress = fmt.Errorf("only one task can be in_progress at a time")

// ErrActiveFormRequired activeForm 必填
var ErrActiveFormRequired = fmt.Errorf("activeForm is required for task creation")

// Task 任务
type Task struct {
	ID          string                 `json:"id"`                    // 动态计算的显示序号，基于列表位置
	InternalID  string                 `json:"internalId,omitempty"`  // 内部标识，用于文件存储
	Subject     string                 `json:"subject"`
	Description string                 `json:"description"`
	ActiveForm  string                 `json:"activeForm,omitempty"`
	Status      Status                 `json:"status"`
	Blocks      []string               `json:"blocks"`
	BlockedBy   []string               `json:"blockedBy"`
	Owner       string                 `json:"owner,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Order       int                    `json:"order"`
	CreatedAt   int64                  `json:"createdAt"`
	UpdatedAt   int64                  `json:"updatedAt"`
}

// Manager 任务管理器
type Manager struct {
	baseDir string
	tasks   map[string]map[string]*Task // userID:listID -> internalID -> Task
	mu      sync.RWMutex
}

// NewManager 创建任务管理器
func NewManager(baseDir string) *Manager {
	m := &Manager{
		baseDir: baseDir,
		tasks:   make(map[string]map[string]*Task),
	}
	return m
}

// getTaskDir 获取任务存储目录
func (m *Manager) getTaskDir(userID, listID string) string {
	return filepath.Join(m.baseDir, userID, ".coworker", "tasks", listID)
}

// Create 创建任务
func (m *Manager) Create(userID, listID string, subject, description, activeForm string) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 约束检查 1: activeForm 必填
	if activeForm == "" {
		return nil, ErrActiveFormRequired
	}

	// 加载现有任务以检查约束
	existingTasks := m.listTasksLocked(userID, listID)

	// 约束检查 2: 最多 20 条任务
	activeCount := 0
	maxOrder := 0
	for _, t := range existingTasks {
		if t.Status != StatusCompleted && t.Status != StatusDeleted {
			activeCount++
		}
		if t.Order > maxOrder {
			maxOrder = t.Order
		}
	}
	if activeCount >= MaxTasks {
		return nil, ErrMaxTasksReached
	}

	// 使用时间戳作为内部标识
	internalID := fmt.Sprintf("%d", time.Now().UnixNano())
	newOrder := maxOrder + 1

	task := &Task{
		ID:          "",  // 动态计算，创建时不设置
		InternalID:  internalID,
		Subject:     subject,
		Description: description,
		ActiveForm:  activeForm,
		Status:      StatusPending,
		Blocks:      []string{},
		BlockedBy:   []string{},
		Metadata:    make(map[string]interface{}),
		Order:       newOrder,
		CreatedAt:   time.Now().UnixMilli(),
		UpdatedAt:   time.Now().UnixMilli(),
	}

	// 保存到内存
	key := userID + ":" + listID
	if m.tasks[key] == nil {
		m.tasks[key] = make(map[string]*Task)
	}
	m.tasks[key][internalID] = task

	// 保存到文件
	if err := m.saveTask(userID, listID, task); err != nil {
		return nil, err
	}

	// 计算动态 ID 并返回
	task.ID = fmt.Sprintf("%d", len(existingTasks)+1)
	return task, nil
}

// saveTask 保存任务到文件
func (m *Manager) saveTask(userID, listID string, task *Task) error {
	taskDir := m.getTaskDir(userID, listID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return err
	}

	// 使用 InternalID 作为文件名
	taskFile := filepath.Join(taskDir, task.InternalID+".json")

	// 保存时清空动态 ID，只保存 InternalID
	taskToSave := *task
	taskToSave.ID = ""

	data, err := json.MarshalIndent(taskToSave, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(taskFile, data, 0644)
}

// Get 获取任务 - taskID 是动态 ID（列表位置）
func (m *Manager) Get(userID, listID, taskID string) *Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取排序后的任务列表
	tasks := m.listTasksLocked(userID, listID)

	// 按 Order 排序并分配动态 ID
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Order < tasks[j].Order
	})

	// 查找匹配的任务
	for i, task := range tasks {
		dynamicID := fmt.Sprintf("%d", i+1)
		if dynamicID == taskID {
			task.ID = dynamicID
			return task
		}
	}

	return nil
}

// Update 更新任务 - taskID 是动态 ID（列表位置）
func (m *Manager) Update(userID, listID, taskID string, updates map[string]interface{}) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := userID + ":" + listID

	// 获取排序后的任务列表，找到对应的任务
	tasks := m.listTasksLocked(userID, listID)
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Order < tasks[j].Order
	})

	// 查找匹配的任务
	var task *Task
	var dynamicID string
	for i, t := range tasks {
		dynamicID = fmt.Sprintf("%d", i+1)
		if dynamicID == taskID {
			task = t
			break
		}
	}

	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	// 应用更新
	if subject, ok := updates["subject"].(string); ok {
		task.Subject = subject
	}
	if description, ok := updates["description"].(string); ok {
		task.Description = description
	}
	if activeForm, ok := updates["activeForm"].(string); ok {
		task.ActiveForm = activeForm
	}
	if status, ok := updates["status"].(string); ok {
		newStatus := Status(status)
		// 约束检查: 只能有一个 in_progress 任务
		if newStatus == StatusInProgress && task.Status != StatusInProgress {
			for _, t := range tasks {
				if t.InternalID != task.InternalID && t.Status == StatusInProgress {
					return nil, ErrOnlyOneInProgress
				}
			}
		}
		task.Status = newStatus
	}
	if owner, ok := updates["owner"].(string); ok {
		task.Owner = owner
	}

	task.UpdatedAt = time.Now().UnixMilli()

	// 如果状态是 deleted，删除文件
	if task.Status == StatusDeleted {
		taskFile := filepath.Join(m.getTaskDir(userID, listID), task.InternalID+".json")
		os.Remove(taskFile)
		delete(m.tasks[key], task.InternalID)
		task.ID = dynamicID
		return task, nil
	}

	// 保存到文件
	if err := m.saveTask(userID, listID, task); err != nil {
		return nil, err
	}

	task.ID = dynamicID
	return task, nil
}

// UpdateByInternalID 通过 InternalID 更新任务（更稳定，不受其他任务删除影响）
func (m *Manager) UpdateByInternalID(userID, listID, internalID string, updates map[string]interface{}) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := userID + ":" + listID

	// 加载任务列表
	tasks := m.listTasksLocked(userID, listID)

	// 查找匹配的任务
	var task *Task
	for _, t := range tasks {
		if t.InternalID == internalID {
			task = t
			break
		}
	}

	if task == nil {
		return nil, fmt.Errorf("task not found: internalId=%s", internalID)
	}

	// 应用更新
	if subject, ok := updates["subject"].(string); ok {
		task.Subject = subject
	}
	if description, ok := updates["description"].(string); ok {
		task.Description = description
	}
	if activeForm, ok := updates["activeForm"].(string); ok {
		task.ActiveForm = activeForm
	}
	if status, ok := updates["status"].(string); ok {
		newStatus := Status(status)
		// 约束检查: 只能有一个 in_progress 任务
		if newStatus == StatusInProgress && task.Status != StatusInProgress {
			for _, t := range tasks {
				if t.InternalID != task.InternalID && t.Status == StatusInProgress {
					return nil, ErrOnlyOneInProgress
				}
			}
		}
		task.Status = newStatus
	}
	if owner, ok := updates["owner"].(string); ok {
		task.Owner = owner
	}

	task.UpdatedAt = time.Now().UnixMilli()

	// 计算动态 ID
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Order < tasks[j].Order
	})
	dynamicID := ""
	for i, t := range tasks {
		if t.InternalID == internalID {
			dynamicID = fmt.Sprintf("%d", i+1)
			break
		}
	}

	// 如果状态是 deleted，删除文件
	if task.Status == StatusDeleted {
		taskFile := filepath.Join(m.getTaskDir(userID, listID), task.InternalID+".json")
		os.Remove(taskFile)
		delete(m.tasks[key], task.InternalID)
		task.ID = dynamicID
		return task, nil
	}

	// 保存到文件
	if err := m.saveTask(userID, listID, task); err != nil {
		return nil, err
	}

	task.ID = dynamicID
	return task, nil
}

// List 列出所有任务
func (m *Manager) List(userID, listID string) []*Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	tasks := m.listTasksLocked(userID, listID)

	// 按 Order 排序后分配动态 ID
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Order < tasks[j].Order
	})

	// 分配动态 ID（基于列表位置）
	for i, task := range tasks {
		task.ID = fmt.Sprintf("%d", i+1)
	}

	return tasks
}

// listTasksLocked 内部方法，列出所有任务（调用者需持有锁）
func (m *Manager) listTasksLocked(userID, listID string) []*Task {
	key := userID + ":" + listID

	// 从文件加载所有任务
	taskDir := m.getTaskDir(userID, listID)
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		if m.tasks[key] != nil {
			tasks := make([]*Task, 0, len(m.tasks[key]))
			for _, t := range m.tasks[key] {
				if t.Status != StatusDeleted {
					tasks = append(tasks, t)
				}
			}
			return tasks
		}
		return []*Task{}
	}

	if m.tasks[key] == nil {
		m.tasks[key] = make(map[string]*Task)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		// 文件名就是 InternalID
		internalID := entry.Name()[:len(entry.Name())-5] // 去掉 .json
		if _, ok := m.tasks[key][internalID]; ok {
			continue
		}

		taskFile := filepath.Join(taskDir, entry.Name())
		data, err := os.ReadFile(taskFile)
		if err != nil {
			continue
		}

		var task Task
		if err := json.Unmarshal(data, &task); err != nil {
			continue
		}

		// 确保 InternalID 被设置
		if task.InternalID == "" {
			task.InternalID = internalID
		}

		m.tasks[key][internalID] = &task
	}

	tasks := make([]*Task, 0, len(m.tasks[key]))
	for _, t := range m.tasks[key] {
		if t.Status != StatusDeleted {
			tasks = append(tasks, t)
		}
	}

	return tasks
}

// ClearCompleted 清除已完成的任务
func (m *Manager) ClearCompleted(userID, listID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := userID + ":" + listID
	if m.tasks[key] == nil {
		return 0
	}

	count := 0
	for internalID, task := range m.tasks[key] {
		if task.Status == StatusCompleted {
			taskFile := filepath.Join(m.getTaskDir(userID, listID), internalID+".json")
			os.Remove(taskFile)
			delete(m.tasks[key], internalID)
			count++
		}
	}

	return count
}

// Render 渲染任务列表为人类可读的文本格式
// 格式参考 learn-claude-code v2 设计:
//   [x] 已完成任务
//   [>] 进行中任务 <- 正在做什么...
//   [ ] 待办任务
//   (2/3 completed)
func (m *Manager) Render(userID, listID string) string {
	tasks := m.List(userID, listID)
	if len(tasks) == 0 {
		return "No todos."
	}

	var lines []string
	completed := 0

	for _, t := range tasks {
		var line string
		switch t.Status {
		case StatusCompleted:
			line = fmt.Sprintf("[x] (id:%s) %s", t.InternalID, t.Subject)
			completed++
		case StatusInProgress:
			if t.ActiveForm != "" {
				line = fmt.Sprintf("[>] (id:%s) %s <- %s", t.InternalID, t.Subject, t.ActiveForm)
			} else {
				line = fmt.Sprintf("[>] (id:%s) %s", t.InternalID, t.Subject)
			}
		default:
			line = fmt.Sprintf("[ ] (id:%s) %s", t.InternalID, t.Subject)
		}
		lines = append(lines, line)
	}

	lines = append(lines, fmt.Sprintf("\n(%d/%d completed)", completed, len(tasks)))
	return strings.Join(lines, "\n")
}

// RenderCompact 紧凑渲染任务列表（用于系统提示词嵌入）
// 只显示 in_progress + pending + 最近3个completed
// maxItems 限制最大显示数量
func (m *Manager) RenderCompact(userID, listID string, maxItems int) string {
	tasks := m.List(userID, listID)
	if len(tasks) == 0 {
		return ""
	}

	var inProgress []*Task
	var pending []*Task
	var completed []*Task

	for _, t := range tasks {
		switch t.Status {
		case StatusInProgress:
			inProgress = append(inProgress, t)
		case StatusPending:
			pending = append(pending, t)
		case StatusCompleted:
			completed = append(completed, t)
		}
	}

	// 只保留最近3个已完成任务
	if len(completed) > 3 {
		completed = completed[len(completed)-3:]
	}

	var lines []string
	count := 0

	// 先显示进行中的任务
	for _, t := range inProgress {
		if count >= maxItems {
			break
		}
		if t.ActiveForm != "" {
			lines = append(lines, fmt.Sprintf("[>] (id:%s) %s <- %s", t.InternalID, t.Subject, t.ActiveForm))
		} else {
			lines = append(lines, fmt.Sprintf("[>] (id:%s) %s", t.InternalID, t.Subject))
		}
		count++
	}

	// 再显示待办任务
	for _, t := range pending {
		if count >= maxItems {
			break
		}
		lines = append(lines, fmt.Sprintf("[ ] (id:%s) %s", t.InternalID, t.Subject))
		count++
	}

	// 最后显示最近完成的任务
	for _, t := range completed {
		if count >= maxItems {
			break
		}
		lines = append(lines, fmt.Sprintf("[x] (id:%s) %s", t.InternalID, t.Subject))
		count++
	}

	if len(lines) == 0 {
		return ""
	}

	// 添加统计
	lines = append(lines, fmt.Sprintf("\n(%d/%d completed)", len(completed), len(tasks)))
	return strings.Join(lines, "\n")
}

// contains 检查切片是否包含元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// UpdateOrder 更新任务排序 - taskIDs 是动态 ID 列表
func (m *Manager) UpdateOrder(userID, listID string, taskIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取当前任务列表，建立动态 ID 到 InternalID 的映射
	tasks := m.listTasksLocked(userID, listID)
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Order < tasks[j].Order
	})

	// 建立动态 ID -> Task 的映射
	dynamicIDToTask := make(map[string]*Task)
	for i, task := range tasks {
		dynamicID := fmt.Sprintf("%d", i+1)
		dynamicIDToTask[dynamicID] = task
	}

	// 按照传入的顺序更新 Order 字段
	for i, dynamicID := range taskIDs {
		if task, ok := dynamicIDToTask[dynamicID]; ok {
			task.Order = i + 1
			task.UpdatedAt = time.Now().UnixMilli()
			if err := m.saveTask(userID, listID, task); err != nil {
				return err
			}
		}
	}

	return nil
}
