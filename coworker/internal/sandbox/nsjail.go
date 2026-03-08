package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CommandResult 命令执行结果
type CommandResult struct {
	Output   string
	Error    string
	ExitCode int
	Success  bool
}

// NsjailExecutor 通过 Docker exec 调用 nsjail 容器
type NsjailExecutor struct {
	config NsjailConfig
}

// NsjailConfig nsjail 配置
type NsjailConfig struct {
	ContainerName string        // nsjail 容器名称
	MemoryMB      int           // 内存限制 (MB)
	ExecTimeout   time.Duration // 执行超时
}

// NewNsjailExecutor 创建 nsjail 执行器
func NewNsjailExecutor(config NsjailConfig) *NsjailExecutor {
	if config.ContainerName == "" {
		config.ContainerName = "nsjail-sandbox"
	}
	if config.MemoryMB <= 0 {
		config.MemoryMB = 512
	}
	if config.ExecTimeout <= 0 {
		config.ExecTimeout = 2 * time.Minute
	}
	return &NsjailExecutor{config: config}
}

// Exec 在 nsjail 沙箱中执行命令
// workspacePath: 后端容器中的路径 (如 /app/userdata/user123/workspace)
// 会转换为 nsjail 容器中的路径 (如 /userdata/user123/workspace)
func (e *NsjailExecutor) Exec(ctx context.Context, workspacePath, command string, timeout time.Duration, extraMounts []Mount) (*CommandResult, error) {
	if timeout <= 0 {
		timeout = e.config.ExecTimeout
	}

	// 路径转换: /app/userdata/xxx -> /userdata/xxx
	nsjailPath := convertToNsjailPath(workspacePath)

	// 构建 nsjail 命令参数 (-q 抑制日志输出, --cwd 设置工作目录)
	nsjailCmd := fmt.Sprintf(
		"nsjail -Mo -q --user 99999 --group 99999 --hostname sandbox "+
			"--bindmount %s:/workspace:rw "+
			"--bindmount /bin:/bin:ro "+
			"--bindmount /lib:/lib:ro "+
			"--bindmount /lib64:/lib64:ro "+
			"--bindmount /usr:/usr:ro "+
			"--bindmount /etc:/etc:ro "+
			"--bindmount /tmp:/tmp "+
			"--bindmount /dev/null:/dev/null "+
			"--bindmount /dev/zero:/dev/zero:ro "+
			"--bindmount /dev/urandom:/dev/urandom:ro ",
		nsjailPath,
	)

	// 追加额外挂载
	for _, m := range extraMounts {
		mountRealPath := convertToNsjailPath(m.RealPath)
		mode := "rw"
		if m.ReadOnly {
			mode = "ro"
		}
		nsjailCmd += fmt.Sprintf("--bindmount %s:%s:%s ", mountRealPath, m.VirtualPath, mode)
	}

	nsjailCmd += fmt.Sprintf(
		"--cwd /workspace "+
			"--time_limit %d "+
			"--rlimit_as %d "+
			"--disable_proc "+
			"-- /bin/bash -c '%s'",
		int(timeout.Seconds()),
		e.config.MemoryMB,
		shellEscapeSingleQuote(command),
	)

	// 通过 docker exec 调用
	execCtx, cancel := context.WithTimeout(ctx, timeout+10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "docker", "exec", e.config.ContainerName, "sh", "-c", nsjailCmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// 解析结果
	result := &CommandResult{
		Output:   filterNsjailLogs(strings.TrimRight(stdout.String(), "\n")),
		Error:    filterNsjailLogs(strings.TrimRight(stderr.String(), "\n")),
		ExitCode: 0,
		Success:  true,
	}

	if err != nil {
		result.Success = false
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Error = err.Error()
		}
	}

	return result, nil
}

// Ping 检查 nsjail 容器是否可用
func (e *NsjailExecutor) Ping() error {
	// nsjail 不支持 --version，使用 --help 检测
	cmd := exec.Command("docker", "exec", e.config.ContainerName, "nsjail", "--help")
	return cmd.Run()
}

// convertToNsjailPath 将后端路径转换为 nsjail 容器路径
// /app/userdata/xxx -> /userdata/xxx
func convertToNsjailPath(backendPath string) string {
	// 清理路径
	cleanPath := filepath.Clean(backendPath)
	// 替换前缀
	if strings.HasPrefix(cleanPath, "/app/userdata") {
		return strings.Replace(cleanPath, "/app/userdata", "/userdata", 1)
	}
	return cleanPath
}

// shellEscapeSingleQuote 使用单引号安全转义 shell 命令
// 将 command 中的 ' 替换为 '\'' (结束单引号、转义单引号、重新开始单引号)
// 这可以防止 $()、``、$VAR 等 shell 展开（单引号内不展开变量和子命令）
func shellEscapeSingleQuote(command string) string {
	return strings.ReplaceAll(command, "'", `'\''`)
}

// filterNsjailLogs 过滤 nsjail 的日志输出
// 移除 [W], [I], [E], [F], [D] 开头的日志行
func filterNsjailLogs(output string) string {
	if output == "" {
		return output
	}

	lines := strings.Split(output, "\n")
	filtered := make([]string, 0, len(lines))

	for _, line := range lines {
		// 跳过 nsjail 日志行 (格式: [W][timestamp][pid] message)
		if len(line) > 3 && line[0] == '[' &&
			(line[1] == 'W' || line[1] == 'I' || line[1] == 'E' || line[1] == 'F' || line[1] == 'D') &&
			line[2] == ']' {
			continue
		}
		filtered = append(filtered, line)
	}

	return strings.Join(filtered, "\n")
}
