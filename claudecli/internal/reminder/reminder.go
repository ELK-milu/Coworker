package reminder

import (
	"fmt"
	"sync"
	"time"
)

// 动态提醒配置常量
// 参考 learn-claude-code v2 设计：NAG_REMINDER 机制
const (
	// DefaultNagInterval 默认提醒间隔（轮次）
	DefaultNagInterval = 10
	// DefaultTaskNagInterval 任务更新提醒间隔
	DefaultTaskNagInterval = 5
)

// ReminderType 提醒类型
type ReminderType string

const (
	ReminderTaskUpdate   ReminderType = "task_update"
	ReminderContextUsage ReminderType = "context_usage"
	ReminderFocusTask    ReminderType = "focus_task"
)

// Reminder 提醒内容
type Reminder struct {
	Type    ReminderType `json:"type"`
	Message string       `json:"message"`
}

// Manager 提醒管理器
type Manager struct {
	turnCount         int
	lastTaskUpdate    int64 // Unix timestamp
	lastTaskUpdateTurn int
	nagInterval       int
	taskNagInterval   int
	mu                sync.RWMutex
}

// NewManager 创建提醒管理器
func NewManager() *Manager {
	return &Manager{
		nagInterval:     DefaultNagInterval,
		taskNagInterval: DefaultTaskNagInterval,
		lastTaskUpdate:  time.Now().UnixMilli(),
	}
}

// IncrementTurn 增加轮次计数
func (m *Manager) IncrementTurn() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.turnCount++
}

// RecordTaskUpdate 记录任务更新
func (m *Manager) RecordTaskUpdate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastTaskUpdate = time.Now().UnixMilli()
	m.lastTaskUpdateTurn = m.turnCount
}

// GetTurnCount 获取当前轮次
func (m *Manager) GetTurnCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.turnCount
}

// Reset 重置计数器
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.turnCount = 0
	m.lastTaskUpdate = time.Now().UnixMilli()
	m.lastTaskUpdateTurn = 0
}

// GetReminders 获取当前需要显示的提醒
// 参考 learn-claude-code NAG_REMINDER 设计
func (m *Manager) GetReminders(hasActiveTasks bool, contextUsagePercent float64) []Reminder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var reminders []Reminder

	// 1. 任务更新提醒
	turnsSinceTaskUpdate := m.turnCount - m.lastTaskUpdateTurn
	if turnsSinceTaskUpdate >= m.taskNagInterval && hasActiveTasks {
		reminders = append(reminders, Reminder{
			Type: ReminderTaskUpdate,
			Message: fmt.Sprintf(
				"[Reminder] You haven't updated the task list in %d turns. "+
					"Consider marking completed tasks or adding new ones.",
				turnsSinceTaskUpdate),
		})
	}

	// 2. 上下文使用提醒
	if contextUsagePercent >= 0.7 {
		reminders = append(reminders, Reminder{
			Type: ReminderContextUsage,
			Message: fmt.Sprintf(
				"[Reminder] Context usage is at %.0f%%. Consider using /compact to free up space.",
				contextUsagePercent*100),
		})
	}

	// 3. 专注任务提醒（每 N 轮提醒一次）
	if m.turnCount > 0 && m.turnCount%m.nagInterval == 0 {
		reminders = append(reminders, Reminder{
			Type:    ReminderFocusTask,
			Message: "[Reminder] Stay focused on the current task. Avoid scope creep.",
		})
	}

	return reminders
}

// FormatReminders 格式化提醒为系统消息
func FormatReminders(reminders []Reminder) string {
	if len(reminders) == 0 {
		return ""
	}

	result := "<system-reminder>\n"
	for _, r := range reminders {
		result += r.Message + "\n"
	}
	result += "</system-reminder>"
	return result
}
