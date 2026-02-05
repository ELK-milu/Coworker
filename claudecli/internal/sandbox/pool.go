package sandbox

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// SandboxPool 任务绑定模式的沙箱池
// 沙箱按需分配，执行完立即归还，不绑定用户
type SandboxPool struct {
	client      *MicrosandboxClient
	config      PoolConfig
	mu          sync.Mutex
	idlePool    chan *PooledSandbox // 空闲沙箱队列
	allSandboxes map[string]*PooledSandbox // 所有沙箱 (用于清理)
	busyCount   int32               // 当前忙碌数量
	nextID      int32               // 下一个沙箱 ID
	stopCh      chan struct{}
	wg          sync.WaitGroup
	started     bool

	// 统计
	totalExecs   int64
	totalWaitMs  int64
	totalExecMs  int64
}

// PoolConfig 池配置
type PoolConfig struct {
	PoolSize    int           // 池大小 (默认 5)
	MaxWaitTime time.Duration // 最大等待时间 (默认 30s)
	MemoryMB    int           // 每沙箱内存 (默认 512MB)
	CPUs        float64       // 每沙箱 CPU (默认 0.25)
	ExecTimeout time.Duration // 执行超时 (默认 2min)
	Image       string        // 沙箱镜像 (默认 microsandbox/python)
}

// PooledSandbox 池中的沙箱实例
type PooledSandbox struct {
	ID         string
	Name       string
	Status     string    // "idle", "busy"
	CreatedAt  time.Time
	LastUsedAt time.Time
	ExecCount  int       // 复用次数
}

// DefaultPoolConfig 返回默认配置
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		PoolSize:    5,
		MaxWaitTime: 30 * time.Second,
		MemoryMB:    512,
		CPUs:        0.25,
		ExecTimeout: 2 * time.Minute,
		Image:       "microsandbox/python",
	}
}

// NewSandboxPool 创建新的沙箱池
func NewSandboxPool(client *MicrosandboxClient, config PoolConfig) (*SandboxPool, error) {
	if config.PoolSize <= 0 {
		config.PoolSize = 5
	}
	if config.MaxWaitTime <= 0 {
		config.MaxWaitTime = 30 * time.Second
	}
	if config.MemoryMB <= 0 {
		config.MemoryMB = 512
	}
	if config.CPUs <= 0 {
		config.CPUs = 0.25
	}
	if config.ExecTimeout <= 0 {
		config.ExecTimeout = 2 * time.Minute
	}
	if config.Image == "" {
		config.Image = "microsandbox/python"
	}

	pool := &SandboxPool{
		client:       client,
		config:       config,
		idlePool:     make(chan *PooledSandbox, config.PoolSize),
		allSandboxes: make(map[string]*PooledSandbox),
		stopCh:       make(chan struct{}),
	}

	return pool, nil
}

// Start 启动池，预热沙箱
func (p *SandboxPool) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return nil
	}
	p.started = true
	p.mu.Unlock()

	log.Printf("[SandboxPool] Starting pool with size=%d", p.config.PoolSize)

	// 预热沙箱
	for i := 0; i < p.config.PoolSize; i++ {
		sb, err := p.createSandbox(ctx)
		if err != nil {
			log.Printf("[SandboxPool] Warning: failed to create sandbox %d: %v", i, err)
			continue
		}
		p.idlePool <- sb
		log.Printf("[SandboxPool] Sandbox %s ready", sb.Name)
	}

	log.Printf("[SandboxPool] Pool started with %d sandboxes", len(p.idlePool))
	return nil
}

// createSandbox 创建新沙箱
func (p *SandboxPool) createSandbox(ctx context.Context) (*PooledSandbox, error) {
	id := atomic.AddInt32(&p.nextID, 1)
	name := fmt.Sprintf("pool-sandbox-%d", id)

	err := p.client.StartSandbox(ctx, name, p.config.Image, p.config.MemoryMB, p.config.CPUs)
	if err != nil {
		return nil, fmt.Errorf("start sandbox: %w", err)
	}

	sb := &PooledSandbox{
		ID:         fmt.Sprintf("%d", id),
		Name:       name,
		Status:     "idle",
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
	}

	p.mu.Lock()
	p.allSandboxes[name] = sb
	p.mu.Unlock()

	return sb, nil
}

// Acquire 获取空闲沙箱 (阻塞等待)
func (p *SandboxPool) Acquire(ctx context.Context) (*PooledSandbox, error) {
	startTime := time.Now()

	select {
	case sb := <-p.idlePool:
		// 立即获取到空闲沙箱
		sb.Status = "busy"
		sb.LastUsedAt = time.Now()
		atomic.AddInt32(&p.busyCount, 1)
		atomic.AddInt64(&p.totalWaitMs, time.Since(startTime).Milliseconds())
		return sb, nil

	case <-time.After(p.config.MaxWaitTime):
		// 超时
		return nil, ErrPoolExhausted

	case <-ctx.Done():
		// 上下文取消
		return nil, ctx.Err()

	case <-p.stopCh:
		// 池已关闭
		return nil, ErrPoolClosed
	}
}

// Release 归还沙箱到池中
func (p *SandboxPool) Release(sb *PooledSandbox) {
	if sb == nil {
		return
	}

	sb.Status = "idle"
	sb.ExecCount++
	atomic.AddInt32(&p.busyCount, -1)

	// 尝试归还到池中
	select {
	case p.idlePool <- sb:
		// 成功归还
	default:
		// 池已满，停止沙箱
		log.Printf("[SandboxPool] Pool full, stopping sandbox %s", sb.Name)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		p.client.StopSandbox(ctx, sb.Name)

		p.mu.Lock()
		delete(p.allSandboxes, sb.Name)
		p.mu.Unlock()
	}
}

// Exec 在沙箱中执行命令 (自动获取/归还)
func (p *SandboxPool) Exec(ctx context.Context, command string, timeout time.Duration) (*CommandResult, error) {
	if timeout <= 0 {
		timeout = p.config.ExecTimeout
	}

	// 获取沙箱
	sb, err := p.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire sandbox: %w", err)
	}
	defer p.Release(sb)

	// 执行命令
	startTime := time.Now()
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := p.client.RunCommand(execCtx, sb.Name, "bash", []string{"-c", command}, int(timeout.Seconds()))

	// 统计
	atomic.AddInt64(&p.totalExecs, 1)
	atomic.AddInt64(&p.totalExecMs, time.Since(startTime).Milliseconds())

	if err != nil {
		return nil, fmt.Errorf("run command: %w", err)
	}

	return result, nil
}

// Stop 停止池和所有沙箱
func (p *SandboxPool) Stop() {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return
	}
	p.started = false
	p.mu.Unlock()

	// 通知关闭
	close(p.stopCh)

	// 清空空闲池
	close(p.idlePool)
	for sb := range p.idlePool {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		p.client.StopSandbox(ctx, sb.Name)
		cancel()
	}

	// 停止所有沙箱
	p.mu.Lock()
	for name := range p.allSandboxes {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		p.client.StopSandbox(ctx, name)
		cancel()
	}
	p.allSandboxes = make(map[string]*PooledSandbox)
	p.mu.Unlock()

	log.Printf("[SandboxPool] Pool stopped")
}

// PoolStats 池统计信息
type PoolStats struct {
	PoolSize    int     // 池大小
	IdleCount   int     // 空闲数量
	BusyCount   int     // 忙碌数量
	TotalExecs  int64   // 总执行次数
	AvgWaitMs   float64 // 平均等待时间
	AvgExecMs   float64 // 平均执行时间
}

// Stats 返回池统计信息
func (p *SandboxPool) Stats() PoolStats {
	totalExecs := atomic.LoadInt64(&p.totalExecs)
	totalWaitMs := atomic.LoadInt64(&p.totalWaitMs)
	totalExecMs := atomic.LoadInt64(&p.totalExecMs)

	var avgWaitMs, avgExecMs float64
	if totalExecs > 0 {
		avgWaitMs = float64(totalWaitMs) / float64(totalExecs)
		avgExecMs = float64(totalExecMs) / float64(totalExecs)
	}

	return PoolStats{
		PoolSize:   p.config.PoolSize,
		IdleCount:  len(p.idlePool),
		BusyCount:  int(atomic.LoadInt32(&p.busyCount)),
		TotalExecs: totalExecs,
		AvgWaitMs:  avgWaitMs,
		AvgExecMs:  avgExecMs,
	}
}

// 错误定义
var (
	ErrPoolExhausted = fmt.Errorf("sandbox pool exhausted, no available sandbox")
	ErrPoolClosed    = fmt.Errorf("sandbox pool is closed")
)
