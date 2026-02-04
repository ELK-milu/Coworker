package tools

import (
	"context"
	"encoding/json"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/claudecli/internal/container"
	"github.com/QuantumNous/new-api/claudecli/internal/sandbox"
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// BashTool Bash 命令执行工具
type BashTool struct {
	workingDir      string
	blockedCommands []string
	timeout         time.Duration
	containerMgr    *container.ContainerManager // 容器管理器 (nil 则使用本地执行)
}

// BashInput Bash 工具输入
type BashInput struct {
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
	Timeout     int    `json:"timeout,omitempty"`
}

// NewBashTool 创建 Bash 工具
func NewBashTool(workingDir string, blockedCommands []string) *BashTool {
	return &BashTool{
		workingDir:      workingDir,
		blockedCommands: blockedCommands,
		timeout:         2 * time.Minute,
	}
}

// SetContainerManager 设置容器管理器（启用容器隔离模式）
func (t *BashTool) SetContainerManager(cm *container.ContainerManager) {
	t.containerMgr = cm
}

func (t *BashTool) Name() string { return "Bash" }

func (t *BashTool) Description() string {
	return "Execute shell commands. Use for git, npm, system operations."
}

func (t *BashTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command":     map[string]interface{}{"type": "string", "description": "Command to execute"},
			"description": map[string]interface{}{"type": "string", "description": "What this command does"},
			"timeout":     map[string]interface{}{"type": "integer", "description": "Timeout in ms"},
		},
		"required": []string{"command"},
	}
}

func (t *BashTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	var in BashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	// 检查危险命令
	for _, blocked := range t.blockedCommands {
		if strings.Contains(in.Command, blocked) {
			return &types.ToolResult{Success: false, Error: "blocked command"}, nil
		}
	}

	userID, _ := ctx.Value(types.UserIDKey).(string)

	// 容器模式：在 gVisor 容器中执行
	if t.containerMgr != nil && userID != "" {
		return t.executeInContainer(ctx, userID, in)
	}

	// 本地模式：直接在主机上执行
	return t.executeLocal(ctx, in)
}

// executeInContainer 在容器中执行命令
func (t *BashTool) executeInContainer(ctx context.Context, userID string, in BashInput) (*types.ToolResult, error) {
	// 检查磁盘配额
	if err := t.containerMgr.CheckDiskQuota(userID); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	timeout := t.timeout
	if in.Timeout > 0 {
		timeout = time.Duration(in.Timeout) * time.Millisecond
	}
	timeoutMs := timeout.Milliseconds()

	startTime := time.Now()
	output, err := t.containerMgr.Exec(ctx, userID, in.Command, timeout)
	elapsedMs := time.Since(startTime).Milliseconds()

	if err != nil {
		// 区分超时和其他错误
		timedOut := strings.Contains(err.Error(), "timed out")
		return &types.ToolResult{
			Success:   false,
			Output:    output,
			Error:     err.Error(),
			ElapsedMs: elapsedMs,
			TimeoutMs: timeoutMs,
			TimedOut:  timedOut,
		}, nil
	}

	return &types.ToolResult{
		Success:   true,
		Output:    output,
		ElapsedMs: elapsedMs,
		TimeoutMs: timeoutMs,
		TimedOut:  false,
	}, nil
}

// executeLocal 在本地执行命令（开发模式 / 未启用容器时的后备方案）
func (t *BashTool) executeLocal(ctx context.Context, in BashInput) (*types.ToolResult, error) {
	// 获取沙箱
	sb, _ := ctx.Value(types.SandboxKey).(*sandbox.Sandbox)

	// 检查命令中的危险系统路径
	if sb != nil {
		dangerousPaths := []string{"/etc/", "/var/", "/usr/", "/bin/", "/sbin/", "/root/", "/proc/", "/sys/"}
		for _, path := range dangerousPaths {
			if strings.Contains(in.Command, path) {
				return &types.ToolResult{Success: false, Error: "access to system paths not allowed"}, nil
			}
		}
	}

	timeout := t.timeout
	if in.Timeout > 0 {
		timeout = time.Duration(in.Timeout) * time.Millisecond
	}
	// 限制最大超时 10 分钟
	if timeout > 10*time.Minute {
		timeout = 10 * time.Minute
	}
	timeoutMs := timeout.Milliseconds()

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(execCtx, "cmd", "/C", in.Command)
	} else {
		cmd = exec.CommandContext(execCtx, "bash", "-c", in.Command)
	}
	// 从 context 获取工作目录，如果没有则使用默认值
	cmd.Dir = types.GetWorkingDir(ctx, t.workingDir)

	startTime := time.Now()
	output, err := cmd.CombinedOutput()
	elapsedMs := time.Since(startTime).Milliseconds()

	// 检查是否超时
	timedOut := execCtx.Err() == context.DeadlineExceeded

	// 虚拟化输出中的路径
	outputStr := string(output)
	if sb != nil {
		outputStr = sb.VirtualizeOutput(outputStr)
	}

	if err != nil {
		return &types.ToolResult{
			Success:   false,
			Output:    outputStr,
			Error:     err.Error(),
			ElapsedMs: elapsedMs,
			TimeoutMs: timeoutMs,
			TimedOut:  timedOut,
		}, nil
	}

	return &types.ToolResult{
		Success:   true,
		Output:    outputStr,
		ElapsedMs: elapsedMs,
		TimeoutMs: timeoutMs,
		TimedOut:  false,
	}, nil
}
