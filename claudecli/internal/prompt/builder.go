package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// PromptContext 提示词上下文
type PromptContext struct {
	WorkingDir       string         // 工作目录
	Model            string         // 当前模型
	PermissionMode   string         // 权限模式: normal, acceptEdits, planMode, bypassPermissions
	IsGitRepo        bool           // 是否为 git 仓库
	Platform         string         // 平台
	TodayDate        string         // 今天日期
	GitStatus        *GitStatusInfo // Git 状态
	ClaudeMdPath     string         // CLAUDE.md 路径
	CustomRules      string         // 自定义规则
	TasksRender      string         // 任务列表渲染（嵌入系统提示词）
	RelevantMemories string         // 相关记忆（功能4）
	SessionMemory    string         // 会话记忆（功能7）
	// 用户信息
	UserName     string // 用户称呼
	CoworkerName string // Coworker 称呼
	UserPhone    string // 用户手机号
	UserEmail    string // 用户邮箱
}

// GitStatusInfo Git 状态信息
type GitStatusInfo struct {
	Branch        string   // 当前分支
	MainBranch    string   // 主分支
	IsClean       bool     // 是否干净
	Staged        []string // 已暂存文件
	Unstaged      []string // 未暂存文件
	Untracked     []string // 未跟踪文件
	Ahead         int      // 领先提交数
	Behind        int      // 落后提交数
	RecentCommits []string // 最近提交
}

// BuildOptions 构建选项
type BuildOptions struct {
	IncludeIdentity       bool // 包含核心身份
	IncludeToolGuidelines bool // 包含工具指南
	IncludeGitGuidelines  bool // 包含 Git 指南
	IncludeClaudeMd       bool // 包含 CLAUDE.md
	MaxTokens             int  // 最大 token 数
}

// DefaultBuildOptions 默认构建选项
func DefaultBuildOptions() BuildOptions {
	return BuildOptions{
		IncludeIdentity:       true,
		IncludeToolGuidelines: true,
		IncludeGitGuidelines:  true,
		IncludeClaudeMd:       true,
		MaxTokens:             180000,
	}
}

// SystemPromptBuilder 系统提示词构建器
type SystemPromptBuilder struct {
	debug bool
}

// NewSystemPromptBuilder 创建构建器
func NewSystemPromptBuilder() *SystemPromptBuilder {
	return &SystemPromptBuilder{debug: false}
}

// SetDebug 设置调试模式
func (b *SystemPromptBuilder) SetDebug(debug bool) {
	b.debug = debug
}

// Build 构建完整的系统提示词
func (b *SystemPromptBuilder) Build(ctx *PromptContext, opts BuildOptions) string {
	var parts []string

	// 核心身份
	if opts.IncludeIdentity {
		parts = append(parts, CoreIdentity)
	}

	// 帮助信息
	parts = append(parts, getHelpInfo())

	// 输出风格
	parts = append(parts, OutputStyle)

	// 任务管理
	parts = append(parts, TaskManagement)

	// 代码编写指南
	parts = append(parts, CodingGuidelines)

	// 工具使用指南
	if opts.IncludeToolGuidelines {
		parts = append(parts, ToolGuidelines)
	}

	// 沙箱执行环境说明
	parts = append(parts, SandboxEnvironment)

	// Git 操作指南
	if opts.IncludeGitGuidelines {
		parts = append(parts, GitGuidelines)
	}

	// 代码引用
	parts = append(parts, CodeReferences)

	// 任务边界约束（防止自主扩展任务）
	parts = append(parts, TaskBoundary)

	// 记忆工具指南
	parts = append(parts, MemoryGuidelines)

	// 权限模式
	parts = append(parts, b.getPermissionMode(ctx.PermissionMode))

	// 用户信息（称呼、联系方式）
	if userInfo := b.getUserInfo(ctx); userInfo != "" {
		parts = append(parts, userInfo)
	}

	// 环境信息
	parts = append(parts, b.getEnvironmentInfo(ctx))

	// CLAUDE.md 内容
	if opts.IncludeClaudeMd && ctx.ClaudeMdPath != "" {
		if claudeMd := b.loadClaudeMd(ctx.ClaudeMdPath); claudeMd != "" {
			parts = append(parts, claudeMd)
		}
	}

	// Git 状态
	if ctx.GitStatus != nil {
		parts = append(parts, b.getGitStatusInfo(ctx.GitStatus))
	}

	// 当前任务列表
	if ctx.TasksRender != "" {
		parts = append(parts, b.getTasksInfo(ctx.TasksRender))
	}

	// 相关记忆
	if ctx.RelevantMemories != "" {
		parts = append(parts, b.getMemoriesInfo(ctx.RelevantMemories))
	}

	// 会话记忆
	if ctx.SessionMemory != "" {
		parts = append(parts, ctx.SessionMemory)
	}

	// 自定义规则
	if ctx.CustomRules != "" {
		parts = append(parts, ctx.CustomRules)
	}

	return strings.Join(parts, "\n\n")
}

// getHelpInfo 获取帮助信息
func getHelpInfo() string {
	return `If the user asks for help or wants to give feedback inform them of the following:
- /help: Get help with using Claude Code
- To give feedback, users should report the issue at https://github.com/anthropics/claude-code/issues`
}

// getPermissionMode 获取权限模式说明
func (b *SystemPromptBuilder) getPermissionMode(mode string) string {
	switch mode {
	case "acceptEdits":
		return PermissionModeAcceptEdits
	case "planMode":
		return PermissionModePlan
	case "bypassPermissions":
		return PermissionModeBypass
	default:
		return PermissionModeDefault
	}
}

// getEnvironmentInfo 获取环境信息
func (b *SystemPromptBuilder) getEnvironmentInfo(ctx *PromptContext) string {
	workingDir := ctx.WorkingDir
	if workingDir == "" {
		workingDir, _ = os.Getwd()
	}

	platform := ctx.Platform
	if platform == "" {
		platform = runtime.GOOS
	}

	todayDate := ctx.TodayDate
	if todayDate == "" {
		todayDate = time.Now().Format("2006-01-02")
	}

	isGitRepo := "No"
	if ctx.IsGitRepo {
		isGitRepo = "Yes"
	}

	lines := []string{
		"Here is useful information about the environment you are running in:",
		"<env>",
		fmt.Sprintf("Working directory: %s", workingDir),
		fmt.Sprintf("Is directory a git repo: %s", isGitRepo),
		fmt.Sprintf("Platform: %s", platform),
		fmt.Sprintf("Today's date: %s", todayDate),
		"</env>",
	}

	// 添加模型信息
	if ctx.Model != "" {
		displayName := getModelDisplayName(ctx.Model)
		lines = append(lines, fmt.Sprintf("You are powered by the model named %s.", displayName))
	}

	return strings.Join(lines, "\n")
}

// getModelDisplayName 获取模型显示名称
func getModelDisplayName(modelID string) string {
	modelID = strings.ToLower(modelID)
	switch {
	case strings.Contains(modelID, "opus-4-5"):
		return "Opus 4.5"
	case strings.Contains(modelID, "sonnet-4-5"):
		return "Sonnet 4.5"
	case strings.Contains(modelID, "sonnet-4"):
		return "Sonnet 4"
	case strings.Contains(modelID, "haiku"):
		return "Haiku 3.5"
	case strings.Contains(modelID, "opus"):
		return "Opus 4"
	default:
		return modelID
	}
}

// getUserInfo 获取用户信息
func (b *SystemPromptBuilder) getUserInfo(ctx *PromptContext) string {
	// 如果没有任何用户信息，返回空
	if ctx.UserName == "" && ctx.CoworkerName == "" && ctx.UserPhone == "" && ctx.UserEmail == "" {
		return ""
	}

	var lines []string
	lines = append(lines, "# User Information")
	lines = append(lines, "")

	if ctx.UserName != "" {
		lines = append(lines, fmt.Sprintf("- User's preferred name: %s (address the user by this name)", ctx.UserName))
	}
	if ctx.CoworkerName != "" {
		lines = append(lines, fmt.Sprintf("- Your name: %s (the user calls you by this name)", ctx.CoworkerName))
	}
	if ctx.UserPhone != "" {
		lines = append(lines, fmt.Sprintf("- User's phone: %s", ctx.UserPhone))
	}
	if ctx.UserEmail != "" {
		lines = append(lines, fmt.Sprintf("- User's email: %s", ctx.UserEmail))
	}

	return strings.Join(lines, "\n")
}

// loadClaudeMd 加载 CLAUDE.md 文件
func (b *SystemPromptBuilder) loadClaudeMd(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	// 获取相对路径用于显示
	displayPath := path
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, path); err == nil && !strings.HasPrefix(rel, "..") {
			displayPath = rel
		}
	}

	return fmt.Sprintf(`<system-reminder>
As you answer the user's questions, you can use the following context:
# claudeMd
Codebase and user instructions are shown below. Be sure to adhere to these instructions. IMPORTANT: These instructions OVERRIDE any default behavior and you MUST follow them exactly as written.

Contents of %s:

%s

      IMPORTANT: this context may or may not be relevant to your tasks. You should not respond to this context unless it is highly relevant to your task.
</system-reminder>`, displayPath, string(content))
}

// getGitStatusInfo 获取 Git 状态信息
func (b *SystemPromptBuilder) getGitStatusInfo(status *GitStatusInfo) string {
	var lines []string

	lines = append(lines, "gitStatus: This is the git status at the start of the conversation. Note that this status is a snapshot in time, and will not update during the conversation.")
	lines = append(lines, fmt.Sprintf("Current branch: %s", status.Branch))

	if status.MainBranch != "" {
		lines = append(lines, fmt.Sprintf("\nMain branch (you will usually use this for PRs): %s", status.MainBranch))
	}

	lines = append(lines, "\nStatus:")
	if status.IsClean {
		lines = append(lines, "(clean)")
	} else {
		// 已暂存
		for _, f := range status.Staged {
			lines = append(lines, fmt.Sprintf("A  %s", f))
		}
		// 已修改
		for _, f := range status.Unstaged {
			lines = append(lines, fmt.Sprintf(" M %s", f))
		}
		// 未跟踪
		for _, f := range status.Untracked {
			lines = append(lines, fmt.Sprintf("?? %s", f))
		}
	}

	// 最近提交
	if len(status.RecentCommits) > 0 {
		lines = append(lines, "\nRecent commits:")
		for _, commit := range status.RecentCommits {
			lines = append(lines, commit)
		}
	}

	return strings.Join(lines, "\n")
}

// getTasksInfo 获取当前任务信息
func (b *SystemPromptBuilder) getTasksInfo(tasksRender string) string {
	return fmt.Sprintf(`# Current Tasks

The following is your current task list. Use the TaskUpdate tool to update task status as you work.

%s`, tasksRender)
}

// getMemoriesInfo 获取相关记忆信息
func (b *SystemPromptBuilder) getMemoriesInfo(memories string) string {
	return fmt.Sprintf(`# Relevant Memories

The following memories may be relevant to the current conversation:

%s`, memories)
}

// FindClaudeMd 查找 CLAUDE.md 文件
func FindClaudeMd(workingDir string) string {
	// 查找顺序：
	// 1. 当前目录的 CLAUDE.md
	// 2. .claude/CLAUDE.md
	// 3. 用户目录的 ~/.claude/CLAUDE.md

	candidates := []string{
		filepath.Join(workingDir, "CLAUDE.md"),
		filepath.Join(workingDir, ".claude", "CLAUDE.md"),
	}

	// 添加用户目录
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".claude", "CLAUDE.md"))
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// 全局构建器实例
var defaultBuilder = NewSystemPromptBuilder()

// BuildSystemPrompt 便捷函数：构建系统提示词
func BuildSystemPrompt(ctx *PromptContext) string {
	return defaultBuilder.Build(ctx, DefaultBuildOptions())
}

// BuildSystemPromptWithOptions 便捷函数：使用自定义选项构建系统提示词
func BuildSystemPromptWithOptions(ctx *PromptContext, opts BuildOptions) string {
	return defaultBuilder.Build(ctx, opts)
}
