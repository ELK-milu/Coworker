package subagent

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/coworker/internal/agent"
	"github.com/QuantumNous/new-api/coworker/internal/client"
	"github.com/QuantumNous/new-api/coworker/internal/config"
	"github.com/QuantumNous/new-api/coworker/internal/loop"
	"github.com/QuantumNous/new-api/coworker/internal/sandbox"
	"github.com/QuantumNous/new-api/coworker/internal/session"
	"github.com/QuantumNous/new-api/coworker/internal/workspace"
	"github.com/QuantumNous/new-api/coworker/pkg/types"
)

// ConversationSubagentExecutor 基于 ConversationLoop 的子代理执行器
// 为每次子代理调用创建独立会话和对话循环
type ConversationSubagentExecutor struct {
	clientFactory func(userID string) *client.ClaudeClient
	sessions      *session.Manager
	workspace     *workspace.Manager
	config        *config.Config
}

// NewConversationSubagentExecutor 创建子代理执行器
func NewConversationSubagentExecutor(
	clientFactory func(userID string) *client.ClaudeClient,
	sessions *session.Manager,
	workspace *workspace.Manager,
	cfg *config.Config,
) *ConversationSubagentExecutor {
	return &ConversationSubagentExecutor{
		clientFactory: clientFactory,
		sessions:      sessions,
		workspace:     workspace,
		config:        cfg,
	}
}

// Execute 执行子代理任务
func (e *ConversationSubagentExecutor) Execute(
	ctx context.Context,
	agentType *agent.AgentType,
	prompt string,
	resumeSessionID string,
) (string, string, error) {
	userID := types.GetUserID(ctx)
	if userID == "" {
		return "", "", fmt.Errorf("user ID not found in context")
	}

	// 1. 创建或恢复子会话
	var sess *session.Session
	if resumeSessionID != "" {
		sess = e.sessions.Get(resumeSessionID)
		if sess != nil {
			log.Printf("[SubagentExecutor] Resuming session %s for agent %s", resumeSessionID, agentType.Name)
		}
	}
	if sess == nil {
		sess = e.sessions.Create(userID)
		parentSessionID := types.GetSessionID(ctx)
		sess.SetParentID(parentSessionID)
		log.Printf("[SubagentExecutor] Created new session %s (parent=%s) for agent %s",
			sess.ID, parentSessionID, agentType.Name)
	}

	// 2. 获取工具提供者（继承父级的 MCP 工具）
	var toolProvider types.ToolProvider
	if tp, ok := ctx.Value(types.ToolProviderKey).(types.ToolProvider); ok {
		toolProvider = tp
	}
	if toolProvider == nil {
		return "", sess.ID, fmt.Errorf("tool provider not found in context")
	}

	// 3. 构建子代理系统提示词（简化版）
	systemPrompt := buildSubagentPrompt(agentType, e.workspace, userID)

	// 4. 创建事件通道 + 收集文本
	subEventCh := make(chan loop.LoopEvent, 100)
	var finalText strings.Builder
	done := make(chan struct{})

	// 转发子代理事件到父级（如果可用）
	var parentEventCh chan<- loop.LoopEvent
	if ch, ok := ctx.Value(types.EventChKey).(chan<- loop.LoopEvent); ok {
		parentEventCh = ch
	}

	go func() {
		defer close(done)
		for event := range subEventCh {
			if event.Type == loop.EventTypeText {
				finalText.WriteString(event.Text)
			}
			// 转发有意义的事件到父级（工具开始/结束）
			if parentEventCh != nil {
				switch event.Type {
				case loop.EventTypeToolStart, loop.EventTypeToolInput, loop.EventTypeToolEnd:
					if event.Metadata == nil {
						event.Metadata = make(map[string]interface{})
					}
					event.Metadata["subagent"] = agentType.Name
					// 非阻塞发送，避免父级通道满时阻塞子代理
					select {
					case parentEventCh <- event:
					default:
					}
				}
			}
		}
	}()

	// 5. 创建并运行子代理 ConversationLoop
	userClient := e.clientFactory(userID)
	userWorkDir := e.workspace.GetUserWorkDir(userID)
	sb := sandbox.NewSandbox(userID, userWorkDir)

	subLoop := loop.NewConversationLoop(
		userClient, sess, toolProvider,
		systemPrompt, userID, sb, nil, agentType, subEventCh,
	)
	err := subLoop.ProcessMessage(ctx, prompt)
	close(subEventCh)
	<-done

	// 6. 保存子会话
	if saveErr := e.sessions.Save(sess.ID); saveErr != nil {
		log.Printf("[SubagentExecutor] Failed to save session %s: %v", sess.ID, saveErr)
	}

	if err != nil {
		return finalText.String(), sess.ID, fmt.Errorf("subagent loop error: %w", err)
	}

	return finalText.String(), sess.ID, nil
}

// buildSubagentPrompt 构建简化版子代理系统提示词
func buildSubagentPrompt(agentType *agent.AgentType, ws *workspace.Manager, userID string) string {
	var parts []string

	parts = append(parts, "You are a specialized subagent running in an isolated session.")
	parts = append(parts, "")

	// 环境信息
	workDir := "/workspace"
	if ws != nil {
		workDir = ws.GetUserWorkDir(userID)
	}
	parts = append(parts, fmt.Sprintf("Working directory: %s", workDir))
	parts = append(parts, fmt.Sprintf("Platform: %s", runtime.GOOS))
	parts = append(parts, fmt.Sprintf("Current date: %s", time.Now().Format("2006-01-02")))
	parts = append(parts, "")

	// Agent 专用提示词
	if agentType.Prompt != "" {
		parts = append(parts, agentType.Prompt)
		parts = append(parts, "")
	}

	parts = append(parts, "Important: Complete your task and provide a clear summary of what was done.")

	return strings.Join(parts, "\n")
}
