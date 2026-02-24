package sandbox

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"
)

// SandboxPool nsjail 沙箱池（并发控制器）
// nsjail 是即用即启的进程模型，不需要预热
type SandboxPool struct {
	executor    *NsjailExecutor
	config      PoolConfig
	semaphore   chan struct{} // 并发限制信号量
	stopCh      chan struct{}
	started     bool

	// 统计
	totalExecs  int64
	totalExecMs int64
	busyCount   int32
}

// PoolConfig 池配置
type PoolConfig struct {
	MaxConcurrent int           // 最大并发数 (默认 50)
	MemoryMB      int           // 内存限制 (默认 512MB)
	ExecTimeout   time.Duration // 执行超时 (默认 2min)
	ContainerName string        // nsjail 容器名称
}

// DefaultPoolConfig 返回默认配置
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxConcurrent: 50,
		MemoryMB:      512,
		ExecTimeout:   2 * time.Minute,
		ContainerName: "nsjail-sandbox",
	}
}

// NewSandboxPool 创建新的沙箱池
func NewSandboxPool(config PoolConfig) (*SandboxPool, error) {
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 50
	}
	if config.MemoryMB <= 0 {
		config.MemoryMB = 512
	}
	if config.ExecTimeout <= 0 {
		config.ExecTimeout = 2 * time.Minute
	}
	if config.ContainerName == "" {
		config.ContainerName = "nsjail-sandbox"
	}

	executor := NewNsjailExecutor(NsjailConfig{
		ContainerName: config.ContainerName,
		MemoryMB:      config.MemoryMB,
		ExecTimeout:   config.ExecTimeout,
	})

	// 检查 nsjail 是否可用
	if err := executor.Ping(); err != nil {
		return nil, fmt.Errorf("nsjail not available: %w", err)
	}

	pool := &SandboxPool{
		executor:  executor,
		config:    config,
		semaphore: make(chan struct{}, config.MaxConcurrent),
		stopCh:    make(chan struct{}),
	}

	return pool, nil
}

// Start 启动池（nsjail 不需要预热，仅标记启动状态）
func (p *SandboxPool) Start(ctx context.Context) error {
	if p.started {
		return nil
	}
	p.started = true
	log.Printf("[SandboxPool] nsjail pool started (max_concurrent=%d)", p.config.MaxConcurrent)
	return nil
}

// Exec 在 nsjail 沙箱中执行命令
func (p *SandboxPool) Exec(ctx context.Context, workspacePath, command string, timeout time.Duration) (*CommandResult, error) {
	if !p.started {
		return nil, ErrPoolClosed
	}

	if timeout <= 0 {
		timeout = p.config.ExecTimeout
	}

	// 获取信号量（并发控制）
	select {
	case p.semaphore <- struct{}{}:
		// 获取成功
		atomic.AddInt32(&p.busyCount, 1)
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.stopCh:
		return nil, ErrPoolClosed
	}

	// 确保释放信号量
	defer func() {
		<-p.semaphore
		atomic.AddInt32(&p.busyCount, -1)
	}()

	// 执行命令
	startTime := time.Now()
	result, err := p.executor.Exec(ctx, workspacePath, command, timeout)
	execMs := time.Since(startTime).Milliseconds()

	// 统计
	atomic.AddInt64(&p.totalExecs, 1)
	atomic.AddInt64(&p.totalExecMs, execMs)

	if err != nil {
		return nil, fmt.Errorf("nsjail exec: %w", err)
	}

	return result, nil
}

// Stop 停止池
func (p *SandboxPool) Stop() {
	if !p.started {
		return
	}
	p.started = false
	close(p.stopCh)
	log.Printf("[SandboxPool] nsjail pool stopped")
}

// PoolStats 池统计信息
type PoolStats struct {
	MaxConcurrent int     // 最大并发数
	BusyCount     int     // 当前忙碌数量
	TotalExecs    int64   // 总执行次数
	AvgExecMs     float64 // 平均执行时间
}

// Stats 返回池统计信息
func (p *SandboxPool) Stats() PoolStats {
	totalExecs := atomic.LoadInt64(&p.totalExecs)
	totalExecMs := atomic.LoadInt64(&p.totalExecMs)

	var avgExecMs float64
	if totalExecs > 0 {
		avgExecMs = float64(totalExecMs) / float64(totalExecs)
	}

	return PoolStats{
		MaxConcurrent: p.config.MaxConcurrent,
		BusyCount:     int(atomic.LoadInt32(&p.busyCount)),
		TotalExecs:    totalExecs,
		AvgExecMs:     avgExecMs,
	}
}

// 错误定义
var (
	ErrPoolExhausted = fmt.Errorf("sandbox pool exhausted")
	ErrPoolClosed    = fmt.Errorf("sandbox pool is closed")
)
