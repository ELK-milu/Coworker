package job

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/claudecli/internal/client"
	"github.com/QuantumNous/new-api/claudecli/internal/config"
	"github.com/QuantumNous/new-api/claudecli/internal/eventbus"
	"github.com/QuantumNous/new-api/claudecli/internal/loop"
	"github.com/QuantumNous/new-api/claudecli/internal/memory"
	"github.com/QuantumNous/new-api/claudecli/internal/prompt"
	"github.com/QuantumNous/new-api/claudecli/internal/sandbox"
	"github.com/QuantumNous/new-api/claudecli/internal/session"
	"github.com/QuantumNous/new-api/claudecli/internal/task"
	"github.com/QuantumNous/new-api/claudecli/internal/tools"
	"github.com/QuantumNous/new-api/claudecli/internal/workspace"
)

// JobExecutorDeps 执行器依赖
type JobExecutorDeps struct {
	Client    *client.ClaudeClient
	Sessions  *session.Manager
	Tools     *tools.Registry
	Workspace *workspace.Manager
	Tasks     *task.Manager
	Memories  *memory.Manager
	Config    *config.Config
	Bus       *eventbus.Bus
	FileTime  *tools.FileTime
	// BusySessions 用于检查会话是否正在被 WebSocket 使用
	BusySessions *BusySessionChecker
}

// BusySessionChecker 检查会话是否忙碌（由 WSHandler 的 busySessions 提供）
type BusySessionChecker struct {
	checkFn func(sessionID string) bool
}

// NewBusySessionChecker 创建忙碌检查器
func NewBusySessionChecker(fn func(sessionID string) bool) *BusySessionChecker {
	return &BusySessionChecker{checkFn: fn}
}

// IsBusy 检查会话是否忙碌
func (c *BusySessionChecker) IsBusy(sessionID string) bool {
	if c == nil || c.checkFn == nil {
		return false
	}
	return c.checkFn(sessionID)
}

// NewAIExecutor 创建 AI 执行器
func NewAIExecutor(deps *JobExecutorDeps) JobExecutor {
	return func(job *Job) error {
		return executeJobWithAI(deps, job)
	}
}

// getClientForJobUser 根据用户配置创建 per-user 客户端
func getClientForJobUser(deps *JobExecutorDeps, userID string) *client.ClaudeClient {
	if deps.Workspace == nil {
		return deps.Client
	}
	info, err := deps.Workspace.LoadUserInfo(userID)
	if err != nil || info == nil {
		return deps.Client
	}
	model := deps.Config.Claude.Model
	if info.SelectedModel != "" {
		model = info.SelectedModel
	}
	tokenKey := info.ApiTokenKey
	if tokenKey == "" {
		tokenKey = "playground-default"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	relayBaseURL := "http://127.0.0.1:" + port
	log.Printf("[JobExecutor] User %s using Relay, token: %s, model: %s", userID, tokenKey, model)
	c := client.NewClaudeClient(
		tokenKey, "", relayBaseURL,
		model, int(deps.Config.Claude.MaxTokens),
	)
	c.SetSamplingParams(info.Temperature, info.TopP)
	return c
}

// executeJobWithAI 使用 AI 执行 Job
func executeJobWithAI(deps *JobExecutorDeps, j *Job) error {
	log.Printf("[JobExecutor] Starting AI execution for job %s (%s), user: %s", j.ID, j.Name, j.UserID)

	// 1. 获取或创建会话
	sess, isNew := getOrCreateLatestSession(deps, j)
	log.Printf("[JobExecutor] Using session %s (new: %v) for job %s", sess.ID, isNew, j.ID)

	// 2. 构建沙箱
	realWorkDir := deps.Sessions.GetUserWorkDir(j.UserID)
	sb := sandbox.NewSandbox(j.UserID, realWorkDir)

	// 3. 构建系统提示词（简化版，不需要 git status）
	systemPrompt := buildJobSystemPrompt(deps, j.UserID, sb, j.Command)

	// 4. 构建用户消息
	userMessage := fmt.Sprintf("[定时事项: %s]\n\n%s", j.Name, j.Command)

	// 5. 创建 eventCh 并启动对话循环
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	eventCh := make(chan loop.LoopEvent, 100)

	// 注入 EventBus 到会话的 Context Manager（如果有的话）
	if deps.Bus != nil && sess.Context != nil {
		sess.Context.SetEventBus(deps.Bus)
		sess.Context.SetEventContext(j.UserID, sess.ID)
	}

	// 异步运行对话循环
	go func() {
		defer close(eventCh)

		userClient := getClientForJobUser(deps, j.UserID)
		l := loop.NewConversationLoop(
			userClient, sess, deps.Tools, systemPrompt,
			j.UserID, sb, deps.FileTime, nil, eventCh,
		)
		l.ProcessMessage(ctx, userMessage)

		// 保存会话
		if err := deps.Sessions.Save(sess.ID); err != nil {
			log.Printf("[JobExecutor] Failed to save session %s: %v", sess.ID, err)
		}

		// 通过 EventBus 通知记忆系统
		if deps.Bus != nil {
			windowIndex := 0
			if sess.Context != nil {
				windowIndex = sess.Context.GetWindowIndex()
			}
			deps.Bus.Emit(eventbus.Event{
				Type:      eventbus.EventTurnCompleted,
				UserID:    j.UserID,
				SessionID: sess.ID,
				Data: map[string]interface{}{
					"session":      sess,
					"window_index": windowIndex,
				},
			})
		}
	}()

	// 6. 消费 eventCh（收集日志，不需要 WebSocket 转发）
	var lastError string
	for event := range eventCh {
		switch event.Type {
		case loop.EventTypeText:
			// 日志记录 AI 回复片段（截断）
			if len(event.Text) > 200 {
				log.Printf("[JobExecutor] AI text: %s...", event.Text[:200])
			}
		case loop.EventTypeError:
			lastError = event.Error
			log.Printf("[JobExecutor] Error: %s", event.Error)
		case loop.EventTypeDone:
			log.Printf("[JobExecutor] Job %s conversation done", j.ID)
		case loop.EventTypeToolStart:
			log.Printf("[JobExecutor] Tool start: %s", event.ToolName)
		case loop.EventTypeToolEnd:
			if event.IsError {
				log.Printf("[JobExecutor] Tool %s failed: %s", event.ToolName, event.ToolResult)
			}
		}
	}

	if lastError != "" {
		return fmt.Errorf("AI execution error: %s", lastError)
	}

	log.Printf("[JobExecutor] Job %s completed successfully", j.ID)
	return nil
}

// getOrCreateLatestSession 获取用户最新会话或创建新会话
func getOrCreateLatestSession(deps *JobExecutorDeps, j *Job) (*session.Session, bool) {
	sessions := deps.Sessions.List(j.UserID)

	if len(sessions) > 0 {
		// 按 UpdatedAt 降序排序，找最新的
		sort.Slice(sessions, func(i, k int) bool {
			return sessions[i].UpdatedAt.After(sessions[k].UpdatedAt)
		})

		latest := sessions[0]

		// 检查是否正在被 WebSocket 使用
		if deps.BusySessions != nil && deps.BusySessions.IsBusy(latest.ID) {
			log.Printf("[JobExecutor] Latest session %s is busy, creating new session", latest.ID)
		} else {
			return latest, false
		}
	}

	// 创建新会话
	sess := deps.Sessions.Create(j.UserID)
	sess.SetTitle(fmt.Sprintf("[定时] %s", j.Name))
	return sess, true
}

// buildJobSystemPrompt 为 Job 构建简化版系统提示词
func buildJobSystemPrompt(deps *JobExecutorDeps, userID string, sb *sandbox.Sandbox, command string) string {
	virtualWorkDir := sb.GetVirtualWorkingDir()

	// 获取任务列表渲染
	tasksRender := ""
	if deps.Tasks != nil {
		tasksRender = deps.Tasks.RenderCompact(userID, "default", 10)
	}

	// 获取相关记忆
	relevantMemories := ""
	if deps.Memories != nil {
		mems := deps.Memories.Retrieve(userID, command, 5)
		if len(mems) > 0 {
			relevantMemories = deps.Memories.FormatForPrompt(mems)
		}
	}

	// 加载用户信息
	var userName, coworkerName, userPhone, userEmail string
	if deps.Workspace != nil {
		if userInfo, err := deps.Workspace.LoadUserInfo(userID); err == nil && userInfo != nil {
			userName = userInfo.UserName
			coworkerName = userInfo.CoworkerName
			userPhone = userInfo.Phone
			userEmail = userInfo.Email
		}
	}

	// 加载用户自定义提示词 (COWORKER.md)
	customRules := ""
	if deps.Workspace != nil {
		if content, err := deps.Workspace.LoadConfig(userID); err == nil && content != "" {
			customRules = content
		}
	}

	// 确定平台
	platform := "linux"
	if deps.Config.Nsjail.Enabled {
		platform = "linux (nsjail sandbox)"
	}

	promptCtx := &prompt.PromptContext{
		WorkingDir:       virtualWorkDir,
		Model:            deps.Config.Claude.Model,
		PermissionMode:   "bypassPermissions", // 定时任务自动执行，跳过权限确认
		Platform:         platform,
		TasksRender:      tasksRender,
		RelevantMemories: relevantMemories,
		CustomRules:      customRules,
		UserName:         userName,
		CoworkerName:     coworkerName,
		UserPhone:        userPhone,
		UserEmail:        userEmail,
	}

	// 检查 git 状态（简化版，不阻塞）
	realWorkDir := sb.GetRealWorkingDir()
	promptCtx.IsGitRepo = prompt.IsGitRepo(realWorkDir)
	promptCtx.ClaudeMdPath = prompt.FindClaudeMd(realWorkDir)

	return prompt.BuildSystemPrompt(promptCtx)
}
