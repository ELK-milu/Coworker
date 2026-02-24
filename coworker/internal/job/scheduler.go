package job

import (
	"fmt"
	"log"
	"strconv"
	"strings"
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

// CalculateNextRun 根据调度类型计算下次执行时间
func (s *Scheduler) CalculateNextRun(job *Job) (int64, error) {
	now := time.Now()

	switch job.ScheduleType {
	case ScheduleOnce:
		// 单次执行：使用 RunAt 时间戳
		if job.RunAt > 0 {
			return job.RunAt, nil
		}
		return 0, fmt.Errorf("run_at is required for once schedule")

	case ScheduleDaily:
		// 每天执行：解析时间并计算下次执行
		return s.calculateDailyNext(job.Time, now)

	case ScheduleWeekly:
		// 每周执行：解析时间和星期
		return s.calculateWeeklyNext(job.Time, job.Weekdays, now)

	case ScheduleInterval:
		// 间隔执行：从上次执行时间加上间隔
		if job.IntervalMinutes <= 0 {
			return 0, fmt.Errorf("interval_minutes must be positive")
		}
		if job.LastRun > 0 {
			return job.LastRun + int64(job.IntervalMinutes)*60*1000, nil
		}
		// 首次执行，立即开始
		return now.UnixMilli(), nil

	case ScheduleCron:
		// Cron 表达式
		if job.CronExpr == "" {
			return 0, fmt.Errorf("cron_expr is required for cron schedule")
		}
		return s.NextRunTime(job.CronExpr)

	default:
		// 兼容旧数据：如果有 CronExpr 则使用
		if job.CronExpr != "" {
			return s.NextRunTime(job.CronExpr)
		}
		return 0, fmt.Errorf("unknown schedule type: %s", job.ScheduleType)
	}
}

// calculateDailyNext 计算每天执行的下次时间
func (s *Scheduler) calculateDailyNext(timeStr string, now time.Time) (int64, error) {
	hour, minute, err := parseTimeStr(timeStr)
	if err != nil {
		return 0, err
	}

	// 构建今天的执行时间
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())

	// 如果今天的时间已过，则推到明天
	if next.Before(now) || next.Equal(now) {
		next = next.AddDate(0, 0, 1)
	}

	return next.UnixMilli(), nil
}

// calculateWeeklyNext 计算每周执行的下次时间
func (s *Scheduler) calculateWeeklyNext(timeStr string, weekdays []int, now time.Time) (int64, error) {
	if len(weekdays) == 0 {
		return 0, fmt.Errorf("weekdays is required for weekly schedule")
	}

	hour, minute, err := parseTimeStr(timeStr)
	if err != nil {
		return 0, err
	}

	// 构建今天的执行时间
	todayTime := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	currentWeekday := int(now.Weekday())

	// 查找最近的执行日
	var minDays int = 8 // 最多7天
	for _, wd := range weekdays {
		days := wd - currentWeekday
		if days < 0 {
			days += 7
		}
		// 如果是今天但时间已过，则推到下周
		if days == 0 && (todayTime.Before(now) || todayTime.Equal(now)) {
			days = 7
		}
		if days < minDays {
			minDays = days
		}
	}

	next := todayTime.AddDate(0, 0, minDays)
	return next.UnixMilli(), nil
}

// parseTimeStr 解析时间字符串 "HH:MM"
func parseTimeStr(timeStr string) (int, int, error) {
	if timeStr == "" {
		return 0, 0, fmt.Errorf("time is required")
	}

	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format, expected HH:MM")
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid hour: %s", parts[0])
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid minute: %s", parts[1])
	}

	return hour, minute, nil
}
