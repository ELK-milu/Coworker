package container

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// ContainerManager 管理每个用户的沙箱容器
type ContainerManager struct {
	cli         *client.Client
	mu          sync.Mutex
	containers  map[string]*UserContainer // userID -> container
	baseDir     string                    // userdata 基础路径 (容器内路径，用于磁盘配额检查)
	hostBaseDir string                    // userdata 基础路径 (宿主机路径，用于 bind mount)
	config      Config
	stopCh      chan struct{}
	wg          sync.WaitGroup // 跟踪后台 goroutine
}

// UserContainer 用户容器信息
type UserContainer struct {
	ID         string
	UserID     string
	Status     string // "running", "stopped"
	CreatedAt  time.Time
	mu         sync.Mutex
	lastUsedAt time.Time
}

// updateLastUsed 线程安全更新最后使用时间
func (uc *UserContainer) updateLastUsed() {
	uc.mu.Lock()
	uc.lastUsedAt = time.Now()
	uc.mu.Unlock()
}

// getLastUsed 线程安全获取最后使用时间
func (uc *UserContainer) getLastUsed() time.Time {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	return uc.lastUsedAt
}

// Config 容器配置
type Config struct {
	Image        string
	Runtime      string // "runsc" for gVisor, "" for default
	MemoryMB     int64
	CPUQuota     float64
	PidLimit     int64
	DiskMB       int64
	IdleTimeout  time.Duration
	HostBasePath string // 宿主机上的 userdata 路径 (Docker-in-Docker 场景必需)
}

// validUserID 验证用户 ID 格式 (只允许字母数字和下划线横线)
var validUserID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// MaxExecTimeout 最大执行超时时间
const MaxExecTimeout = 10 * time.Minute

// NewContainerManager 创建容器管理器
func NewContainerManager(baseDir string, cfg Config) (*ContainerManager, error) {
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("resolve base dir: %w", err)
	}

	// 确定宿主机路径：如果配置了 HostBasePath 则使用，否则使用 baseDir
	hostBaseDir := cfg.HostBasePath
	if hostBaseDir == "" {
		hostBaseDir = absBaseDir
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	// 验证 Docker 连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		cli.Close()
		return nil, fmt.Errorf("docker not available: %w", err)
	}

	cm := &ContainerManager{
		cli:         cli,
		containers:  make(map[string]*UserContainer),
		baseDir:     absBaseDir,
		hostBaseDir: hostBaseDir,
		config:      cfg,
		stopCh:      make(chan struct{}),
	}

	// 启动空闲容器清理 goroutine
	cm.wg.Add(1)
	go cm.cleanupLoop()

	log.Printf("[Container] Manager initialized: baseDir=%s, hostBaseDir=%s, image=%s, runtime=%s",
		absBaseDir, hostBaseDir, cfg.Image, cfg.Runtime)
	return cm, nil
}

// validateUserID 验证用户 ID 是否安全
func validateUserID(userID string) error {
	if userID == "" {
		return fmt.Errorf("empty user ID")
	}
	if !validUserID.MatchString(userID) {
		return fmt.Errorf("invalid user ID: contains disallowed characters")
	}
	if len(userID) > 64 {
		return fmt.Errorf("user ID too long")
	}
	return nil
}

// GetOrCreate 获取或创建用户容器 (全程持锁，避免 TOCTOU)
func (cm *ContainerManager) GetOrCreate(ctx context.Context, userID string) (*UserContainer, error) {
	if err := validateUserID(userID); err != nil {
		return nil, err
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	uc, exists := cm.containers[userID]
	if exists && uc.Status == "running" {
		// 验证容器仍在运行（持锁状态下检查）
		info, err := cm.cli.ContainerInspect(ctx, uc.ID)
		if err == nil && info.State.Running {
			uc.updateLastUsed()
			return uc, nil
		}
		// 容器已停止或不存在，清理记录
		delete(cm.containers, userID)
	}

	return cm.createLocked(ctx, userID)
}

// createLocked 创建新容器（调用者必须持有 cm.mu）
func (cm *ContainerManager) createLocked(ctx context.Context, userID string) (*UserContainer, error) {
	containerName := fmt.Sprintf("coworker-sandbox-%s", userID)

	// 用户工作空间路径
	// hostWorkDir: 宿主机路径，用于 bind mount
	// localWorkDir: 容器内路径，用于创建目录
	hostWorkDir := filepath.Join(cm.hostBaseDir, userID, "workspace")
	localWorkDir := filepath.Join(cm.baseDir, userID, "workspace")

	// 确保工作目录存在（在当前容器内创建，因为 userdata 已挂载）
	if err := os.MkdirAll(localWorkDir, 0755); err != nil {
		return nil, fmt.Errorf("create workspace dir: %w", err)
	}

	// 容器配置
	containerCfg := &container.Config{
		Image:      cm.config.Image,
		Cmd:        []string{"sleep", "infinity"},
		WorkingDir: "/workspace",
		User:       "sandbox",
		Labels: map[string]string{
			"coworker.user":    userID,
			"coworker.managed": "true",
		},
	}

	// 资源限制
	nanoCPUs := int64(cm.config.CPUQuota * 1e9)
	pidLimit := cm.config.PidLimit

	hostCfg := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: hostWorkDir,
				Target: "/workspace",
			},
		},
		NetworkMode: "none",
		Resources: container.Resources{
			Memory:    cm.config.MemoryMB * 1024 * 1024,
			NanoCPUs:  nanoCPUs,
			PidsLimit: &pidLimit,
		},
		AutoRemove: false,
	}

	// 设置 gVisor 运行时
	if cm.config.Runtime != "" {
		hostCfg.Runtime = cm.config.Runtime
	}

	// 先尝试删除同名容器（可能是残留的）
	cm.removeContainer(ctx, containerName)

	// 创建容器
	resp, err := cm.cli.ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, containerName)
	if err != nil {
		return nil, fmt.Errorf("create container for %s: %w", userID, err)
	}

	// 启动容器
	if err := cm.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		// 清理创建失败的容器
		cm.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("start container for %s: %w", userID, err)
	}

	uc := &UserContainer{
		ID:         resp.ID,
		UserID:     userID,
		Status:     "running",
		CreatedAt:  time.Now(),
		lastUsedAt: time.Now(),
	}
	cm.containers[userID] = uc

	log.Printf("[Container] Created container for user %s: id=%s", userID, resp.ID[:12])
	return uc, nil
}

// Exec 在用户容器中执行命令
func (cm *ContainerManager) Exec(ctx context.Context, userID, command string, timeout time.Duration) (string, error) {
	// 限制最大超时
	if timeout > MaxExecTimeout {
		timeout = MaxExecTimeout
	}

	uc, err := cm.GetOrCreate(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("get container: %w", err)
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 创建 exec 实例
	execCfg := container.ExecOptions{
		Cmd:          []string{"bash", "-c", command},
		WorkingDir:   "/workspace",
		User:         "sandbox",
		AttachStdout: true,
		AttachStderr: true,
	}

	execResp, err := cm.cli.ContainerExecCreate(execCtx, uc.ID, execCfg)
	if err != nil {
		return "", fmt.Errorf("create exec: %w", err)
	}

	// 附加到 exec
	attachResp, err := cm.cli.ContainerExecAttach(execCtx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", fmt.Errorf("attach exec: %w", err)
	}

	// 读取输出（使用独立 goroutine + 超时控制）
	var stdout, stderr bytes.Buffer
	doneCh := make(chan error, 1)
	go func() {
		_, err := stdcopy.StdCopy(&stdout, &stderr, attachResp.Reader)
		doneCh <- err
	}()

	var output string
	var execErr error

	select {
	case err := <-doneCh:
		// 正常完成，关闭连接
		attachResp.Close()
		if err != nil {
			execErr = fmt.Errorf("read output: %w", err)
		}
	case <-execCtx.Done():
		// 超时：关闭连接以中断 StdCopy goroutine
		attachResp.Close()
		// 等待 goroutine 退出（关闭 reader 后 StdCopy 会返回）
		<-doneCh
		output = stdout.String() + stderr.String()
		uc.updateLastUsed()
		return output, fmt.Errorf("command timed out after %v", timeout)
	}

	// 更新最后使用时间
	uc.updateLastUsed()

	// 合并输出
	output = stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	if execErr != nil {
		return output, execErr
	}

	// 检查退出码
	inspectResp, err := cm.cli.ContainerExecInspect(execCtx, execResp.ID)
	if err == nil && inspectResp.ExitCode != 0 {
		return output, fmt.Errorf("exit code %d", inspectResp.ExitCode)
	}

	return output, nil
}

// Stop 停止并删除用户容器
func (cm *ContainerManager) Stop(ctx context.Context, userID string) error {
	cm.mu.Lock()
	uc, exists := cm.containers[userID]
	if exists {
		delete(cm.containers, userID)
	}
	cm.mu.Unlock()

	if !exists {
		return nil
	}

	timeout := 10
	stopOptions := container.StopOptions{Timeout: &timeout}
	if err := cm.cli.ContainerStop(ctx, uc.ID, stopOptions); err != nil {
		log.Printf("[Container] Warning: failed to stop container %s: %v", uc.ID[:12], err)
	}
	if err := cm.cli.ContainerRemove(ctx, uc.ID, container.RemoveOptions{Force: true}); err != nil {
		log.Printf("[Container] Warning: failed to remove container %s: %v", uc.ID[:12], err)
	}

	log.Printf("[Container] Stopped container for user %s: id=%s", userID, uc.ID[:12])
	return nil
}

// StopAll 停止所有容器（用于优雅关闭）
func (cm *ContainerManager) StopAll() {
	// 通知 cleanup goroutine 退出
	close(cm.stopCh)
	// 等待 cleanup goroutine 完成
	cm.wg.Wait()

	// 收集所有用户 ID
	cm.mu.Lock()
	userIDs := make([]string, 0, len(cm.containers))
	for uid := range cm.containers {
		userIDs = append(userIDs, uid)
	}
	cm.mu.Unlock()

	// 停止所有容器
	ctx := context.Background()
	for _, uid := range userIDs {
		cm.Stop(ctx, uid)
	}

	// 关闭 Docker 客户端
	cm.cli.Close()
	log.Printf("[Container] All containers stopped")
}

// cleanupLoop 定期清理空闲容器
func (cm *ContainerManager) cleanupLoop() {
	defer cm.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-cm.stopCh:
			return
		case <-ticker.C:
			cm.cleanup()
		}
	}
}

// cleanup 清理空闲容器
func (cm *ContainerManager) cleanup() {
	cm.mu.Lock()
	var idleUsers []string
	for userID, uc := range cm.containers {
		if time.Since(uc.getLastUsed()) > cm.config.IdleTimeout {
			idleUsers = append(idleUsers, userID)
		}
	}
	cm.mu.Unlock()

	if len(idleUsers) == 0 {
		return
	}

	ctx := context.Background()
	for _, uid := range idleUsers {
		log.Printf("[Container] Cleaning idle container for user %s", uid)
		cm.Stop(ctx, uid)
	}
	log.Printf("[Container] Cleaned %d idle containers", len(idleUsers))
}

// removeContainer 移除同名容器（忽略错误）
func (cm *ContainerManager) removeContainer(ctx context.Context, name string) {
	stopTimeout := 5
	cm.cli.ContainerStop(ctx, name, container.StopOptions{Timeout: &stopTimeout})
	cm.cli.ContainerRemove(ctx, name, container.RemoveOptions{Force: true})
}

// ContainerCount 返回当前活跃容器数量
func (cm *ContainerManager) ContainerCount() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.containers)
}
