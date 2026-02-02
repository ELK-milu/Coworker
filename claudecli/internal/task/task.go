package task

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	ID          string                 `json:"id"`
	Subject     string                 `json:"subject"`
	Description string                 `json:"description"`
	ActiveForm  string                 `json:"activeForm,omitempty"`
	Status      Status                 `json:"status"`
	Blocks      []string               `json:"blocks"`
	BlockedBy   []string               `json:"blockedBy"`
	Owner       string                 `json:"owner,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   int64                  `json:"createdAt"`
	UpdatedAt   int64                  `json:"updatedAt"`
}

// Manager 任务管理器
type Manager struct {
	baseDir       string
	tasks         map[string]map[string]*Task // listId -> taskId -> Task
	highWaterMark map[string]int              // listId -> highest ID
	mu            sync.RWMutex
}

// NewManager 创建任务管理器
func NewManager(baseDir string) *Manager {
	m := &Manager{
		baseDir:       baseDir,
		tasks:         make(map[string]map[string]*Task),
		highWaterMark: make(map[string]int),
	}
	return m
}

// getTaskDir 获取任务存储目录
func (m *Manager) getTaskDir(userID, listID string) string {
	return filepath.Join(m.baseDir, userID, ".claude", "tasks", listID)
}

// loadHighWaterMark 加载高水位线
func (m *Manager) loadHighWaterMark(userID, listID string) int {
	key := userID + ":" + listID
	if hwm, ok := m.highWaterMark[key]; ok {
		return hwm
	}

	hwmFile := filepath.Join(m.getTaskDir(userID, listID), ".high-water-mark")
	data, err := os.ReadFile(hwmFile)
	if err != nil {
		return 0
	}

	var hwm int
	if _, err := fmt.Sscanf(string(data), "%d", &hwm); err != nil {
		return 0
	}

	m.highWaterMark[key] = hwm
	return hwm
}

// saveHighWaterMark 保存高水位线
func (m *Manager) saveHighWaterMark(userID, listID string, hwm int) error {
	key := userID + ":" + listID
	m.highWaterMark[key] = hwm

	taskDir := m.getTaskDir(userID, listID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return err
	}

	hwmFile := filepath.Join(taskDir, ".high-water-mark")
	return os.WriteFile(hwmFile, []byte(fmt.Sprintf("%d", hwm)), 0644)
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
	for _, t := range existingTasks {
		if t.Status != StatusCompleted && t.Status != StatusDeleted {
			activeCount++
		}
	}
	if activeCount >= MaxTasks {
		return nil, ErrMaxTasksReached
	}

	// 获取下一个 ID
	hwm := m.loadHighWaterMark(userID, listID)
	newID := hwm + 1

	task := &Task{
		ID:          fmt.Sprintf("%d", newID),
		Subject:     subject,
		Description: description,
		ActiveForm:  activeForm,
		Status:      StatusPending,
		Blocks:      []string{},
		BlockedBy:   []string{},
		Metadata:    make(map[string]interface{}),
		CreatedAt:   time.Now().UnixMilli(),
		UpdatedAt:   time.Now().UnixMilli(),
	}

	// 保存到内存
	key := userID + ":" + listID
	if m.tasks[key] == nil {
		m.tasks[key] = make(map[string]*Task)
	}
	m.tasks[key][task.ID] = task

	// 保存到文件
	if err := m.saveTask(userID, listID, task); err != nil {
		return nil, err
	}

	// 更新高水位线
	if err := m.saveHighWaterMark(userID, listID, newID); err != nil {
		return nil, err
	}

	return task, nil
}

// saveTask 保存任务到文件
func (m *Manager) saveTask(userID, listID string, task *Task) error {
	taskDir := m.getTaskDir(userID, listID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return err
	}

	taskFile := filepath.Join(taskDir, task.ID+".json")
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(taskFile, data, 0644)
}

// Get 获取任务
func (m *Manager) Get(userID, listID, taskID string) *Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := userID + ":" + listID
	if tasks, ok := m.tasks[key]; ok {
		if task, ok := tasks[taskID]; ok {
			return task
		}
	}

	// 尝试从文件加载
	taskFile := filepath.Join(m.getTaskDir(userID, listID), taskID+".json")
	data, err := os.ReadFile(taskFile)
	if err != nil {
		return nil
	}

	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil
	}

	return &task
}

// Update 更新任务
func (m *Manager) Update(userID, listID, taskID string, updates map[string]interface{}) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := userID + ":" + listID
	if m.tasks[key] == nil {
		m.tasks[key] = make(map[string]*Task)
	}

	task := m.tasks[key][taskID]
	if task == nil {
		// 尝试从文件加载
		taskFile := filepath.Join(m.getTaskDir(userID, listID), taskID+".json")
		data, err := os.ReadFile(taskFile)
		if err != nil {
			return nil, fmt.Errorf("task not found: %s", taskID)
		}

		task = &Task{}
		if err := json.Unmarshal(data, task); err != nil {
			return nil, err
		}
		m.tasks[key][taskID] = task
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
			// 检查是否已有其他 in_progress 任务
			for _, t := range m.tasks[key] {
				if t.ID != taskID && t.Status == StatusInProgress {
					return nil, ErrOnlyOneInProgress
				}
			}
		}
		task.Status = newStatus
	}
	if owner, ok := updates["owner"].(string); ok {
		task.Owner = owner
	}

	// 处理依赖关系
	if addBlocks, ok := updates["addBlocks"].([]string); ok {
		for _, blockID := range addBlocks {
			if !contains(task.Blocks, blockID) {
				task.Blocks = append(task.Blocks, blockID)
			}
			// 更新被阻塞任务的 blockedBy
			if blockedTask := m.tasks[key][blockID]; blockedTask != nil {
				if !contains(blockedTask.BlockedBy, taskID) {
					blockedTask.BlockedBy = append(blockedTask.BlockedBy, taskID)
					m.saveTask(userID, listID, blockedTask)
				}
			}
		}
	}
	if addBlockedBy, ok := updates["addBlockedBy"].([]string); ok {
		for _, blockerID := range addBlockedBy {
			if !contains(task.BlockedBy, blockerID) {
				task.BlockedBy = append(task.BlockedBy, blockerID)
			}
			// 更新阻塞任务的 blocks
			if blockerTask := m.tasks[key][blockerID]; blockerTask != nil {
				if !contains(blockerTask.Blocks, taskID) {
					blockerTask.Blocks = append(blockerTask.Blocks, taskID)
					m.saveTask(userID, listID, blockerTask)
				}
			}
		}
	}

	task.UpdatedAt = time.Now().UnixMilli()

	// 如果状态是 deleted，删除文件
	if task.Status == StatusDeleted {
		taskFile := filepath.Join(m.getTaskDir(userID, listID), taskID+".json")
		os.Remove(taskFile)
		delete(m.tasks[key], taskID)
		return task, nil
	}

	// 保存到文件
	if err := m.saveTask(userID, listID, task); err != nil {
		return nil, err
	}

	return task, nil
}

// List 列出所有任务
func (m *Manager) List(userID, listID string) []*Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.listTasksLocked(userID, listID)
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

		taskID := entry.Name()[:len(entry.Name())-5] // 去掉 .json
		if _, ok := m.tasks[key][taskID]; ok {
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

		m.tasks[key][taskID] = &task
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
	for taskID, task := range m.tasks[key] {
		if task.Status == StatusCompleted {
			taskFile := filepath.Join(m.getTaskDir(userID, listID), taskID+".json")
			os.Remove(taskFile)
			delete(m.tasks[key], taskID)
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
			line = fmt.Sprintf("[x] %s", t.Subject)
			completed++
		case StatusInProgress:
			if t.ActiveForm != "" {
				line = fmt.Sprintf("[>] %s <- %s", t.Subject, t.ActiveForm)
			} else {
				line = fmt.Sprintf("[>] %s", t.Subject)
			}
		default:
			line = fmt.Sprintf("[ ] %s", t.Subject)
		}
		lines = append(lines, line)
	}

	lines = append(lines, fmt.Sprintf("\n(%d/%d completed)", completed, len(tasks)))
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
