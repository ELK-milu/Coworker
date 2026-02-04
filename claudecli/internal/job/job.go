package job

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Status Job 状态
type Status string

const (
	StatusIdle    Status = "idle"
	StatusRunning Status = "running"
	StatusFailed  Status = "failed"
)

// Job 定时事项
type Job struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"user_id"`
	Name        string                 `json:"name"`
	CronExpr    string                 `json:"cron_expr"`   // Cron 表达式
	Command     string                 `json:"command"`     // 执行的命令/提示词
	Enabled     bool                   `json:"enabled"`
	LastRun     int64                  `json:"last_run"`    // 上次执行时间戳
	NextRun     int64                  `json:"next_run"`    // 下次执行时间戳
	Status      Status                 `json:"status"`
	LastError   string                 `json:"last_error,omitempty"`
	Order       int                    `json:"order"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   int64                  `json:"created_at"`
	UpdatedAt   int64                  `json:"updated_at"`
}

// Manager Job 管理器
type Manager struct {
	baseDir   string
	jobs      map[string]map[string]*Job // userID -> jobID -> Job
	scheduler *Scheduler
	mu        sync.RWMutex
}

// NewManager 创建 Job 管理器
func NewManager(baseDir string) *Manager {
	m := &Manager{
		baseDir: baseDir,
		jobs:    make(map[string]map[string]*Job),
	}
	m.scheduler = NewScheduler(m)
	return m
}

// Start 启动调度器
func (m *Manager) Start() {
	m.scheduler.Start()
}

// Stop 停止调度器
func (m *Manager) Stop() {
	m.scheduler.Stop()
}

// getJobDir 获取 Job 存储目录
func (m *Manager) getJobDir(userID string) string {
	return filepath.Join(m.baseDir, userID, ".claude", "jobs")
}

// Create 创建 Job
func (m *Manager) Create(userID, name, cronExpr, command string) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证 Cron 表达式
	nextRun, err := m.scheduler.NextRunTime(cronExpr)
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %v", err)
	}

	// 获取最大 Order
	existingJobs := m.listJobsLocked(userID)
	maxOrder := 0
	for _, j := range existingJobs {
		if j.Order > maxOrder {
			maxOrder = j.Order
		}
	}

	jobID := fmt.Sprintf("%d", time.Now().UnixNano())
	job := &Job{
		ID:        jobID,
		UserID:    userID,
		Name:      name,
		CronExpr:  cronExpr,
		Command:   command,
		Enabled:   true,
		NextRun:   nextRun,
		Status:    StatusIdle,
		Order:     maxOrder + 1,
		Metadata:  make(map[string]interface{}),
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	}

	// 保存到内存
	if m.jobs[userID] == nil {
		m.jobs[userID] = make(map[string]*Job)
	}
	m.jobs[userID][jobID] = job

	// 保存到文件
	if err := m.saveJob(job); err != nil {
		return nil, err
	}

	return job, nil
}

// saveJob 保存 Job 到文件
func (m *Manager) saveJob(job *Job) error {
	jobDir := m.getJobDir(job.UserID)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return err
	}

	jobFile := filepath.Join(jobDir, job.ID+".json")
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(jobFile, data, 0644)
}

// Get 获取 Job
func (m *Manager) Get(userID, jobID string) *Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.jobs[userID] == nil {
		// 尝试从文件加载
		m.mu.RUnlock()
		m.mu.Lock()
		defer m.mu.Unlock()
		m.loadUserJobs(userID)
	}

	if m.jobs[userID] == nil {
		return nil
	}
	return m.jobs[userID][jobID]
}

// Update 更新 Job
func (m *Manager) Update(userID, jobID string, updates map[string]interface{}) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.jobs[userID] == nil {
		m.loadUserJobs(userID)
	}

	job := m.jobs[userID][jobID]
	if job == nil {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	// 应用更新
	if name, ok := updates["name"].(string); ok {
		job.Name = name
	}
	if cronExpr, ok := updates["cron_expr"].(string); ok {
		nextRun, err := m.scheduler.NextRunTime(cronExpr)
		if err != nil {
			return nil, fmt.Errorf("invalid cron expression: %v", err)
		}
		job.CronExpr = cronExpr
		job.NextRun = nextRun
	}
	if command, ok := updates["command"].(string); ok {
		job.Command = command
	}
	if enabled, ok := updates["enabled"].(bool); ok {
		job.Enabled = enabled
		if enabled {
			// 重新计算下次执行时间
			nextRun, _ := m.scheduler.NextRunTime(job.CronExpr)
			job.NextRun = nextRun
		}
	}
	if status, ok := updates["status"].(string); ok {
		job.Status = Status(status)
	}
	if lastError, ok := updates["last_error"].(string); ok {
		job.LastError = lastError
	}

	job.UpdatedAt = time.Now().UnixMilli()

	// 保存到文件
	if err := m.saveJob(job); err != nil {
		return nil, err
	}

	return job, nil
}

// Delete 删除 Job
func (m *Manager) Delete(userID, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.jobs[userID] != nil {
		delete(m.jobs[userID], jobID)
	}

	jobFile := filepath.Join(m.getJobDir(userID), jobID+".json")
	if _, err := os.Stat(jobFile); err == nil {
		return os.Remove(jobFile)
	}

	return nil
}

// List 列出所有 Jobs
func (m *Manager) List(userID string) []*Job {
	m.mu.Lock()
	defer m.mu.Unlock()

	jobs := m.listJobsLocked(userID)

	// 按 Order 排序
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].Order < jobs[j].Order
	})

	return jobs
}

// listJobsLocked 内部方法，列出所有 Jobs（调用者需持有锁）
func (m *Manager) listJobsLocked(userID string) []*Job {
	m.loadUserJobs(userID)

	if m.jobs[userID] == nil {
		return []*Job{}
	}

	jobs := make([]*Job, 0, len(m.jobs[userID]))
	for _, j := range m.jobs[userID] {
		jobs = append(jobs, j)
	}

	return jobs
}

// loadUserJobs 从文件加载用户的所有 Jobs
func (m *Manager) loadUserJobs(userID string) {
	if m.jobs[userID] != nil {
		return
	}

	jobDir := m.getJobDir(userID)
	entries, err := os.ReadDir(jobDir)
	if err != nil {
		m.jobs[userID] = make(map[string]*Job)
		return
	}

	m.jobs[userID] = make(map[string]*Job)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		jobFile := filepath.Join(jobDir, entry.Name())
		data, err := os.ReadFile(jobFile)
		if err != nil {
			continue
		}

		var job Job
		if err := json.Unmarshal(data, &job); err != nil {
			continue
		}

		m.jobs[userID][job.ID] = &job
	}
}

// UpdateOrder 更新 Job 排序
func (m *Manager) UpdateOrder(userID string, jobIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.loadUserJobs(userID)

	for i, jobID := range jobIDs {
		if job := m.jobs[userID][jobID]; job != nil {
			job.Order = i + 1
			job.UpdatedAt = time.Now().UnixMilli()
			if err := m.saveJob(job); err != nil {
				return err
			}
		}
	}

	return nil
}

// MarkRunning 标记 Job 为运行中
func (m *Manager) MarkRunning(userID, jobID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if job := m.jobs[userID][jobID]; job != nil {
		job.Status = StatusRunning
		job.LastRun = time.Now().UnixMilli()
		job.UpdatedAt = time.Now().UnixMilli()
		m.saveJob(job)
	}
}

// MarkCompleted 标记 Job 完成
func (m *Manager) MarkCompleted(userID, jobID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if job := m.jobs[userID][jobID]; job != nil {
		if err != nil {
			job.Status = StatusFailed
			job.LastError = err.Error()
		} else {
			job.Status = StatusIdle
			job.LastError = ""
		}

		// 计算下次执行时间
		nextRun, _ := m.scheduler.NextRunTime(job.CronExpr)
		job.NextRun = nextRun
		job.UpdatedAt = time.Now().UnixMilli()
		m.saveJob(job)
	}
}

// GetDueJobs 获取所有到期需要执行的 Jobs
func (m *Manager) GetDueJobs() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var dueJobs []*Job
	now := time.Now().UnixMilli()

	for _, userJobs := range m.jobs {
		for _, job := range userJobs {
			if job.Enabled && job.Status == StatusIdle && job.NextRun <= now {
				dueJobs = append(dueJobs, job)
			}
		}
	}

	return dueJobs
}
