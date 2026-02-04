package job

import (
	"log"
	"sync"
	"time"

	"github.com/gorhill/cronexpr"
)

// JobExecutor Job 执行回调
type JobExecutor func(job *Job) error

// Scheduler Job 调度器
type Scheduler struct {
	manager  *Manager
	executor JobExecutor
	ticker   *time.Ticker
	stopCh   chan struct{}
	running  bool
	mu       sync.Mutex
}

// NewScheduler 创建调度器
func NewScheduler(manager *Manager) *Scheduler {
	return &Scheduler{
		manager: manager,
		stopCh:  make(chan struct{}),
	}
}

// SetExecutor 设置执行器
func (s *Scheduler) SetExecutor(executor JobExecutor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.executor = executor
}

// Start 启动调度器
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.ticker = time.NewTicker(1 * time.Minute)
	go s.run()
	log.Println("[Scheduler] Started")
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	if s.ticker != nil {
		s.ticker.Stop()
	}
	close(s.stopCh)
	log.Println("[Scheduler] Stopped")
}

// run 调度循环
func (s *Scheduler) run() {
	for {
		select {
		case <-s.stopCh:
			return
		case <-s.ticker.C:
			s.checkAndExecute()
		}
	}
}

// checkAndExecute 检查并执行到期的 Jobs
func (s *Scheduler) checkAndExecute() {
	dueJobs := s.manager.GetDueJobs()

	for _, job := range dueJobs {
		log.Printf("[Scheduler] Job due: %s (%s)", job.Name, job.ID)

		s.mu.Lock()
		executor := s.executor
		s.mu.Unlock()

		if executor != nil {
			go s.executeJob(job, executor)
		} else {
			log.Printf("[Scheduler] No executor set, skipping job: %s", job.ID)
			// 仍然更新下次执行时间
			s.manager.MarkCompleted(job.UserID, job.ID, nil)
		}
	}
}

// executeJob 执行单个 Job
func (s *Scheduler) executeJob(job *Job, executor JobExecutor) {
	s.manager.MarkRunning(job.UserID, job.ID)

	err := executor(job)

	s.manager.MarkCompleted(job.UserID, job.ID, err)

	if err != nil {
		log.Printf("[Scheduler] Job %s failed: %v", job.ID, err)
	} else {
		log.Printf("[Scheduler] Job %s completed", job.ID)
	}
}

// NextRunTime 计算下次执行时间
func (s *Scheduler) NextRunTime(cronExprStr string) (int64, error) {
	expr, err := cronexpr.Parse(cronExprStr)
	if err != nil {
		return 0, err
	}

	next := expr.Next(time.Now())
	return next.UnixMilli(), nil
}

// ParseCron 解析 Cron 表达式
func ParseCron(cronExprStr string) (*cronexpr.Expression, error) {
	return cronexpr.Parse(cronExprStr)
}
