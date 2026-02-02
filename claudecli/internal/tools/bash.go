package tools

import (
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
	"context"
	"encoding/json"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// BashTool Bash 命令执行工具
type BashTool struct {
	workingDir      string
	blockedCommands []string
	timeout         time.Duration
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

	timeout := t.timeout
	if in.Timeout > 0 {
		timeout = time.Duration(in.Timeout) * time.Millisecond
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

	if err != nil {
		return &types.ToolResult{
			Success:   false,
			Output:    string(output),
			Error:     err.Error(),
			ElapsedMs: elapsedMs,
			TimeoutMs: timeoutMs,
			TimedOut:  timedOut,
		}, nil
	}

	return &types.ToolResult{
		Success:   true,
		Output:    string(output),
		ElapsedMs: elapsedMs,
		TimeoutMs: timeoutMs,
		TimedOut:  false,
	}, nil
}
