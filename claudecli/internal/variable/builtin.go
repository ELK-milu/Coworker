package variable

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// registerBuiltinVariables 注册所有内置变量
func (m *Manager) registerBuiltinVariables() {
	// 时间相关
	m.resolvers["{{current_time}}"] = resolveCurrentTime
	m.resolvers["{{current_date}}"] = resolveCurrentDate
	m.resolvers["{{current_datetime}}"] = resolveCurrentDateTime
	m.resolvers["{{timestamp}}"] = resolveTimestamp

	// 环境相关
	m.resolvers["{{working_dir}}"] = resolveWorkingDir
	m.resolvers["{{project_dir}}"] = resolveProjectDir
	m.resolvers["{{user_id}}"] = resolveUserID
	m.resolvers["{{session_id}}"] = resolveSessionID
	m.resolvers["{{platform}}"] = resolvePlatform
	m.resolvers["{{hostname}}"] = resolveHostname

	// Git 相关
	m.resolvers["{{git_branch}}"] = resolveGitBranch
	m.resolvers["{{git_status}}"] = resolveGitStatus
	m.resolvers["{{git_remote}}"] = resolveGitRemote

	// 项目相关
	m.resolvers["{{project_name}}"] = resolveProjectName
	m.resolvers["{{recent_files}}"] = resolveRecentFiles
}

// 时间相关解析器

func resolveCurrentTime(ctx *ResolveContext) string {
	return time.Now().Format("15:04:05")
}

func resolveCurrentDate(ctx *ResolveContext) string {
	return time.Now().Format("2006-01-02")
}

func resolveCurrentDateTime(ctx *ResolveContext) string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func resolveTimestamp(ctx *ResolveContext) string {
	return fmt.Sprintf("%d", time.Now().Unix())
}

// 环境相关解析器

func resolveWorkingDir(ctx *ResolveContext) string {
	if ctx.WorkDir != "" {
		return ctx.WorkDir
	}
	wd, _ := os.Getwd()
	return wd
}

func resolveProjectDir(ctx *ResolveContext) string {
	if ctx.ProjectDir != "" {
		return ctx.ProjectDir
	}
	return ctx.WorkDir
}

func resolveUserID(ctx *ResolveContext) string {
	return ctx.UserID
}

func resolveSessionID(ctx *ResolveContext) string {
	return ctx.SessionID
}

func resolvePlatform(ctx *ResolveContext) string {
	return runtime.GOOS
}

func resolveHostname(ctx *ResolveContext) string {
	hostname, _ := os.Hostname()
	return hostname
}

// Git 相关解析器

func resolveGitBranch(ctx *ResolveContext) string {
	workDir := ctx.WorkDir
	if workDir == "" {
		return ""
	}

	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func resolveGitStatus(ctx *ResolveContext) string {
	workDir := ctx.WorkDir
	if workDir == "" {
		return ""
	}

	cmd := exec.Command("git", "status", "--short")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	status := strings.TrimSpace(string(output))
	if status == "" {
		return "clean"
	}

	// 限制输出行数
	lines := strings.Split(status, "\n")
	if len(lines) > 10 {
		return strings.Join(lines[:10], "\n") + fmt.Sprintf("\n... and %d more files", len(lines)-10)
	}
	return status
}

func resolveGitRemote(ctx *ResolveContext) string {
	workDir := ctx.WorkDir
	if workDir == "" {
		return ""
	}

	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// 项目相关解析器

func resolveProjectName(ctx *ResolveContext) string {
	workDir := ctx.WorkDir
	if workDir == "" {
		return ""
	}
	return filepath.Base(workDir)
}

func resolveRecentFiles(ctx *ResolveContext) string {
	workDir := ctx.WorkDir
	if workDir == "" {
		return ""
	}

	// 获取最近修改的文件（最多5个）
	cmd := exec.Command("git", "diff", "--name-only", "HEAD~5", "HEAD")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	files := strings.TrimSpace(string(output))
	if files == "" {
		return ""
	}

	lines := strings.Split(files, "\n")
	if len(lines) > 5 {
		lines = lines[:5]
	}
	return strings.Join(lines, ", ")
}

// BuiltinVariableDescriptions 内置变量描述
var BuiltinVariableDescriptions = map[string]string{
	"{{current_time}}":     "当前时间 (HH:MM:SS)",
	"{{current_date}}":     "当前日期 (YYYY-MM-DD)",
	"{{current_datetime}}": "当前日期时间",
	"{{timestamp}}":        "Unix 时间戳",
	"{{working_dir}}":      "当前工作目录",
	"{{project_dir}}":      "项目根目录",
	"{{user_id}}":          "用户 ID",
	"{{session_id}}":       "会话 ID",
	"{{platform}}":         "操作系统平台",
	"{{hostname}}":         "主机名",
	"{{git_branch}}":       "当前 Git 分支",
	"{{git_status}}":       "Git 状态摘要",
	"{{git_remote}}":       "Git 远程仓库 URL",
	"{{project_name}}":     "项目名称",
	"{{recent_files}}":     "最近修改的文件",
}
