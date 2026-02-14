package loop

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/QuantumNous/new-api/claudecli/internal/agent"
	"github.com/QuantumNous/new-api/claudecli/internal/client"
	"github.com/QuantumNous/new-api/claudecli/internal/prompt"
	"github.com/QuantumNous/new-api/claudecli/internal/sandbox"
	"github.com/QuantumNous/new-api/claudecli/internal/session"
	"github.com/QuantumNous/new-api/claudecli/internal/tools"
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
	"context"
	"encoding/json"
	"log"
	"time"
)

// 循环控制常量
const (
	DefaultMaxSteps       = 50  // 默认最大步数
	DoomLoopThreshold     = 3   // Doom Loop 检测阈值：连续相同调用次数
	DoomLoopHistorySize   = 5   // Doom Loop 历史记录大小
)

// toolCallRecord 工具调用记录（用于 Doom Loop 检测）
type toolCallRecord struct {
	Name      string
	InputHash string
}

// ConversationLoop 对话循环
type ConversationLoop struct {
	client    *client.ClaudeClient
	session   *session.Session
	tools     *tools.Registry
	eventCh   chan<- LoopEvent
	system    string
	mode      string // normal, plan, acceptEdits, bypassPermissions
	userID    string // 用户 ID，用于任务工具
	sandbox   *sandbox.Sandbox // 沙箱，用于路径隔离
	fileTime  *tools.FileTime  // 文件修改时间追踪
	startTime int64

	// P2.4: Agent 分层系统
	agent *agent.AgentType // 当前 Agent（nil = 使用默认 build agent）

	// P0.2: Doom Loop 检测
	recentCalls []toolCallRecord // 最近的工具调用记录

	// P0.3: 循环步数限制
	maxSteps    int  // 最大步数（0 = 使用默认值）
	currentStep int  // 当前步数
}

// LoopEvent 循环事件
type LoopEvent struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	ToolName   string `json:"tool_name,omitempty"`
	ToolID     string `json:"tool_id,omitempty"`
	ToolInput  string `json:"tool_input,omitempty"`
	ToolResult string `json:"tool_result,omitempty"`
	IsError    bool   `json:"is_error,omitempty"`
	Error      string `json:"error,omitempty"`
	Done       bool   `json:"done,omitempty"`
	// 工具执行时间信息
	ElapsedMs int64 `json:"elapsed_ms,omitempty"`
	TimeoutMs int64 `json:"timeout_ms,omitempty"`
	TimedOut  bool  `json:"timed_out,omitempty"`
	// 工具元数据 (exec_env 等)
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// 状态信息
	Status *StatusInfo `json:"status,omitempty"`
	// 任务变更信息
	TaskAction string      `json:"task_action,omitempty"`
	TaskData   interface{} `json:"task_data,omitempty"`
	// 会话信息
	SessionID   string `json:"session_id,omitempty"`
	SessionInfo *SessionInfo `json:"session_info,omitempty"`
	// 标题更新
	Title string `json:"title,omitempty"`
}

// SessionInfo 会话信息
type SessionInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// StatusInfo 状态信息
type StatusInfo struct {
	Model           string  `json:"model"`
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	TotalTokens     int     `json:"total_tokens"`
	ContextUsed     int     `json:"context_used"`
	ContextMax      int     `json:"context_max"`
	ContextPercent  float64 `json:"context_percent"`
	ElapsedMs       int64   `json:"elapsed_ms"`
	Mode            string  `json:"mode"` // normal, plan, acceptEdits, bypassPermissions
}

const (
	EventTypeText           = "text"
	EventTypeThinking       = "thinking"
	EventTypeToolStart      = "tool_start"
	EventTypeToolInput      = "tool_input" // 工具输入完成，执行前发送
	EventTypeToolEnd        = "tool_end"
	EventTypeDone           = "done"
	EventTypeError          = "error"
	EventTypeStatus         = "status"
	EventTypeTaskChanged    = "task_changed"
	EventTypeSessionCreated = "session_created"
	EventTypeTitleUpdated   = "title_updated"
)

// NewConversationLoop 创建对话循环
func NewConversationLoop(
	c *client.ClaudeClient,
	sess *session.Session,
	registry *tools.Registry,
	systemPrompt string,
	userID string,
	sb *sandbox.Sandbox,
	ft *tools.FileTime,
	ag *agent.AgentType,
	eventCh chan<- LoopEvent,
) *ConversationLoop {
	// 使用 Agent 的 MaxTurns 作为默认步数限制
	maxSteps := DefaultMaxSteps
	if ag != nil && ag.MaxTurns > 0 {
		maxSteps = ag.MaxTurns
	}

	// 如果 Agent 有自定义提示词，追加到系统提示词
	if ag != nil && ag.Prompt != "" {
		systemPrompt = systemPrompt + "\n\n" + ag.Prompt
	}

	return &ConversationLoop{
		client:      c,
		session:     sess,
		tools:       registry,
		system:      systemPrompt,
		eventCh:     eventCh,
		mode:        "normal",
		userID:      userID,
		sandbox:     sb,
		fileTime:    ft,
		agent:       ag,
		recentCalls: make([]toolCallRecord, 0, DoomLoopHistorySize),
		maxSteps:    maxSteps,
		currentStep: 0,
	}
}

// SetMaxSteps 设置最大步数（0 = 使用默认值）
func (l *ConversationLoop) SetMaxSteps(steps int) {
	if steps <= 0 {
		l.maxSteps = DefaultMaxSteps
	} else {
		l.maxSteps = steps
	}
}

// SetMode 设置模式
func (l *ConversationLoop) SetMode(mode string) {
	l.mode = mode
}

// ProcessMessage 处理用户消息
func (l *ConversationLoop) ProcessMessage(ctx context.Context, userInput string) error {
	// 添加用户消息到会话
	l.session.AddMessage(types.Message{
		Role:    "user",
		Content: []interface{}{types.TextBlock{Type: "text", Text: userInput}},
	})

	// 开始对话循环
	return l.runLoop(ctx)
}

// runLoop 运行对话循环
func (l *ConversationLoop) runLoop(ctx context.Context) error {
	l.startTime = time.Now().UnixMilli()
	l.currentStep = 0
	l.recentCalls = l.recentCalls[:0]

	for {
		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			l.sendStatusEvent()
			l.eventCh <- LoopEvent{Type: EventTypeDone, Done: true}
			return ctx.Err()
		default:
		}

		// P0.3: 检查步数限制
		l.currentStep++
		if l.currentStep > l.maxSteps {
			log.Printf("[Loop] Max steps reached (%d), stopping loop", l.maxSteps)
			// 注入 OpenCode 风格的 max-steps 提示词（强制文本回复）
			// 参考 OpenCode: packages/opencode/src/session/prompt/max-steps.txt
			l.session.AddMessage(types.Message{
				Role: "user",
				Content: []interface{}{types.TextBlock{
					Type: "text",
					Text: prompt.MaxStepsReached,
				}},
			})
			// 做最后一次 API 调用（不带工具定义，强制文本回复）
			streamCh, err := l.client.CreateMessageStream(ctx, l.session.Messages, nil, l.system)
			if err != nil {
				l.eventCh <- LoopEvent{Type: EventTypeError, Error: err.Error()}
				return err
			}
			_, _, _ = l.processStream(ctx, streamCh)
			l.sendStatusEvent()
			l.eventCh <- LoopEvent{Type: EventTypeDone, Done: true}
			return nil
		}

		// 检查上下文是否接近限制，如果是则压缩
		if l.session.IsContextNearLimit() {
			log.Printf("[Loop] Context near limit, compacting...")
			l.session.CompactContext()
		}

		// 发送状态事件（开始处理）
		l.sendStatusEvent()

		// P2.4: 根据 Agent 过滤工具定义
		toolDefs := l.getFilteredToolDefinitions()

		// 调用 Claude API
		streamCh, err := l.client.CreateMessageStream(
			ctx,
			l.session.Messages,
			toolDefs,
			l.system,
		)
		if err != nil {
			l.eventCh <- LoopEvent{Type: EventTypeError, Error: err.Error()}
			return err
		}

		// 处理流事件
		toolCalls, stopReason, err := l.processStream(ctx, streamCh)
		if err != nil {
			return err
		}

		// 发送状态事件（处理完成）
		l.sendStatusEvent()

		// P2.1: Finish Reason 精确检查
		switch types.StopReason(stopReason) {
		case types.StopReasonToolUse:
			if len(toolCalls) == 0 {
				log.Printf("[Loop] stop_reason=tool_use but no tool calls, ending")
				l.eventCh <- LoopEvent{Type: EventTypeDone, Done: true}
				return nil
			}
			// 继续执行工具
		case types.StopReasonMaxTokens:
			log.Printf("[Loop] max_tokens reached, injecting continue prompt")
			// 使用 SystemBlock：转换为 API 格式时自动包裹 <system-reminder> 标签
			l.session.AddMessage(types.Message{
				Role: "user",
				Content: []interface{}{types.SystemBlock{
					Type: "system_block",
					Text: "Your response was cut off because it exceeded the maximum token limit. " +
						"Please continue from where you left off.",
				}},
			})
			continue
		case types.StopReasonEndTurn:
			log.Printf("[Loop] Conversation ended: end_turn")
			l.eventCh <- LoopEvent{Type: EventTypeDone, Done: true}
			return nil
		default:
			log.Printf("[Loop] Conversation ended: stopReason=%s, toolCalls=%d", stopReason, len(toolCalls))
			l.eventCh <- LoopEvent{Type: EventTypeDone, Done: true}
			return nil
		}

		// 记录 Claude 请求的工具调用
		log.Printf("[Loop] Step %d/%d: Claude requested %d tool calls:", l.currentStep, l.maxSteps, len(toolCalls))
		for i, tc := range toolCalls {
			log.Printf("[Loop]   [%d] %s (id=%s)", i+1, tc.Name, tc.ID)
		}

		// 执行工具调用
		if err := l.executeTools(ctx, toolCalls); err != nil {
			return err
		}

		// P3.1: 工具执行后检测 context overflow（参考 OpenCode SessionCompaction.isOverflow）
		// OpenCode 在每步结束后检测，而非仅在循环开始前
		if l.session.IsContextNearLimit() {
			log.Printf("[Loop] Context overflow detected after tool execution (step %d), compacting...", l.currentStep)
			l.session.CompactContext()
		}
	}
}

// toolCall 工具调用信息
type toolCall struct {
	ID    string
	Name  string
	Input string
}

// processStream 处理流事件
func (l *ConversationLoop) processStream(ctx context.Context, streamCh <-chan client.StreamEvent) ([]toolCall, string, error) {
	var toolCalls []toolCall
	var currentTool *toolCall
	var textContent string
	var stopReason string

	for {
		select {
		case <-ctx.Done():
			// 上下文被取消，保存已有内容并返回
			// 如果有正在处理的工具调用，也要保存
			if currentTool != nil {
				toolCalls = append(toolCalls, *currentTool)
			}
			if textContent != "" || len(toolCalls) > 0 {
				l.saveAssistantMessage(textContent, toolCalls)
			}
			return nil, "", ctx.Err()

		case event, ok := <-streamCh:
			if !ok {
				// 通道关闭，保存助手消息
				l.saveAssistantMessage(textContent, toolCalls)
				return toolCalls, stopReason, nil
			}

			switch event.Type {
			case client.EventText:
				textContent += event.Text
				l.eventCh <- LoopEvent{Type: EventTypeText, Text: event.Text}

			case client.EventThinking:
				// 转发 thinking 事件到前端
				l.eventCh <- LoopEvent{Type: EventTypeThinking, Text: event.Text}

			case client.EventToolStart:
				currentTool = &toolCall{ID: event.ToolID, Name: event.ToolName}
				l.eventCh <- LoopEvent{
					Type:     EventTypeToolStart,
					ToolID:   event.ToolID,
					ToolName: event.ToolName,
				}

			case client.EventToolDelta:
				if currentTool != nil {
					currentTool.Input += event.ToolInput
				}

			case client.EventStop:
				stopReason = event.StopReason
				if currentTool != nil {
					toolCalls = append(toolCalls, *currentTool)
					currentTool = nil
				}

			case client.EventError:
				return nil, "", &loopError{msg: event.Error}
			}
		}
	}
}

// loopError 循环错误
type loopError struct {
	msg string
}

func (e *loopError) Error() string {
	return e.msg
}

// saveAssistantMessage 保存助手消息
func (l *ConversationLoop) saveAssistantMessage(text string, calls []toolCall) {
	content := make([]interface{}, 0)
	if text != "" {
		content = append(content, types.TextBlock{Type: "text", Text: text})
	}
	for _, tc := range calls {
		// 验证 Input 是否为有效 JSON，如果无效则使用空对象
		inputJSON := tc.Input
		if inputJSON == "" || !json.Valid([]byte(inputJSON)) {
			inputJSON = "{}"
			log.Printf("[Loop] Warning: Invalid tool input for %s, using empty object", tc.Name)
		}
		content = append(content, types.ToolUseBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Name,
			Input: json.RawMessage(inputJSON),
		})
	}
	if len(content) > 0 {
		l.session.AddMessage(types.Message{Role: "assistant", Content: content})
	}
}

// executeTools 执行工具调用
func (l *ConversationLoop) executeTools(ctx context.Context, calls []toolCall) error {
	results := make([]interface{}, 0, len(calls))

	// 将会话的工作目录、用户 ID、会话 ID、沙箱和文件时间追踪放入 context
	workDir := l.session.GetWorkingDir()
	toolCtx := context.WithValue(ctx, types.WorkingDirKey, workDir)
	toolCtx = context.WithValue(toolCtx, types.UserIDKey, l.userID)
	toolCtx = context.WithValue(toolCtx, types.SessionIDKey, l.session.ID)
	toolCtx = context.WithValue(toolCtx, types.SandboxKey, l.sandbox)
	toolCtx = context.WithValue(toolCtx, types.FileTimeKey, l.fileTime)

	for _, tc := range calls {
		log.Printf("[Tool] Executing: name=%s, id=%s, workDir=%s", tc.Name, tc.ID, workDir)
		log.Printf("[Tool] Input: %s", tc.Input)

		// P0.2: Doom Loop 检测
		if l.isDoomLoop(tc.Name, tc.Input) {
			log.Printf("[Tool] DOOM LOOP detected: %s called %d times with same input", tc.Name, DoomLoopThreshold)
			doomResult := types.ToolResultBlock{
				Type:      "tool_result",
				ToolUseID: tc.ID,
				Content: fmt.Sprintf(
					"[Doom Loop Detected] The tool '%s' has been called %d consecutive times with identical arguments. "+
						"This indicates a repetitive pattern. Please try a different approach:\n"+
						"- Use a different tool\n"+
						"- Change the arguments\n"+
						"- Reconsider your strategy\n"+
						"- If stuck, explain the issue to the user",
					tc.Name, DoomLoopThreshold,
				),
				IsError: true,
			}
			results = append(results, doomResult)
			l.recordToolCall(tc.Name, tc.Input)
			l.eventCh <- LoopEvent{
				Type:       EventTypeToolEnd,
				ToolID:     tc.ID,
				ToolName:   tc.Name,
				ToolInput:  tc.Input,
				ToolResult: doomResult.Content,
				IsError:    true,
			}
			continue
		}

		// 记录本次工具调用
		l.recordToolCall(tc.Name, tc.Input)

		// 在执行前发送工具输入，让前端可以显示完整的输入参数
		log.Printf("[Tool] Sending tool_input event for %s (id=%s)", tc.Name, tc.ID)
		l.eventCh <- LoopEvent{
			Type:      EventTypeToolInput,
			ToolID:    tc.ID,
			ToolName:  tc.Name,
			ToolInput: tc.Input,
		}

		result, err := l.tools.Execute(toolCtx, tc.Name, json.RawMessage(tc.Input))

		var toolResult types.ToolResultBlock
		var elapsedMs, timeoutMs int64
		var timedOut bool

		if err != nil {
			log.Printf("[Tool] %s failed: %v", tc.Name, err)
			toolResult = types.ToolResultBlock{
				Type:      "tool_result",
				ToolUseID: tc.ID,
				Content:   err.Error(),
				IsError:   true,
			}
		} else {
			elapsedMs = result.ElapsedMs
			timeoutMs = result.TimeoutMs
			timedOut = result.TimedOut

			if !result.Success {
				log.Printf("[Tool] %s failed: %s (elapsed: %dms, timedOut: %v)",
					tc.Name, result.Error, elapsedMs, timedOut)
			} else {
				log.Printf("[Tool] %s success (elapsed: %dms, output length: %d)",
					tc.Name, elapsedMs, len(result.Output))
			}

			// 提取执行环境
			execEnv := ""
			if result.Metadata != nil {
				if env, ok := result.Metadata["exec_env"].(string); ok {
					execEnv = env
				}
			}

			toolResult = types.ToolResultBlock{
				Type:      "tool_result",
				ToolUseID: tc.ID,
				Content:   result.Output,
				IsError:   !result.Success,
				ElapsedMs: elapsedMs,
				TimeoutMs: timeoutMs,
				TimedOut:  timedOut,
				ExecEnv:   execEnv,
			}
		}

		results = append(results, toolResult)

		// 提取工具元数据 (exec_env 等)
		var metadata map[string]interface{}
		if result != nil && result.Metadata != nil {
			metadata = result.Metadata
		}

		l.eventCh <- LoopEvent{
			Type:       EventTypeToolEnd,
			ToolID:     tc.ID,
			ToolName:   tc.Name,
			ToolInput:  tc.Input,
			ToolResult: toolResult.Content,
			IsError:    toolResult.IsError,
			ElapsedMs:  elapsedMs,
			TimeoutMs:  timeoutMs,
			TimedOut:   timedOut,
			Metadata:   metadata,
		}

		// 检测任务变更并发送事件
		if result != nil && result.Metadata != nil {
			if taskChanged, ok := result.Metadata["task_changed"].(bool); ok && taskChanged {
				action, _ := result.Metadata["action"].(string)
				taskData := result.Metadata["task"]
				l.eventCh <- LoopEvent{
					Type:       EventTypeTaskChanged,
					TaskAction: action,
					TaskData:   taskData,
				}
			}
		}
	}

	// 添加工具结果消息
	l.session.AddMessage(types.Message{Role: "user", Content: results})
	return nil
}

// hashToolCall 计算工具调用的哈希值（用于 Doom Loop 检测）
func hashToolCall(name string, input string) string {
	h := sha256.New()
	h.Write([]byte(name))
	h.Write([]byte(":"))
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// recordToolCall 记录工具调用（用于 Doom Loop 检测）
func (l *ConversationLoop) recordToolCall(name string, input string) {
	record := toolCallRecord{
		Name:      name,
		InputHash: hashToolCall(name, input),
	}
	l.recentCalls = append(l.recentCalls, record)
	// 保持历史记录在限定大小内
	if len(l.recentCalls) > DoomLoopHistorySize {
		l.recentCalls = l.recentCalls[len(l.recentCalls)-DoomLoopHistorySize:]
	}
}

// isDoomLoop 检测是否陷入 Doom Loop
// 检查最近 N 次调用是否为相同工具+相同输入
func (l *ConversationLoop) isDoomLoop(name string, input string) bool {
	if len(l.recentCalls) < DoomLoopThreshold-1 {
		return false
	}

	currentHash := hashToolCall(name, input)

	// 检查最近 DoomLoopThreshold-1 次调用是否都相同
	count := 0
	for i := len(l.recentCalls) - 1; i >= 0 && count < DoomLoopThreshold-1; i-- {
		if l.recentCalls[i].Name == name && l.recentCalls[i].InputHash == currentHash {
			count++
		} else {
			break
		}
	}

	return count >= DoomLoopThreshold-1
}

// getFilteredToolDefinitions 根据 Agent 权限过滤工具定义
// 参考 OpenCode: PermissionNext.disabled() + tool registry filtering
func (l *ConversationLoop) getFilteredToolDefinitions() []types.ToolDefinition {
	allDefs := l.tools.GetDefinitions()

	// 无 Agent 或 Agent 允许所有工具 → 返回全部
	if l.agent == nil {
		return allDefs
	}

	var filtered []types.ToolDefinition
	for _, def := range allDefs {
		if l.agent.IsToolAllowed(def.Name) {
			filtered = append(filtered, def)
		}
	}

	if len(filtered) == 0 {
		log.Printf("[Loop] WARNING: Agent '%s' has no allowed tools, returning all", l.agent.Name)
		return allDefs
	}

	log.Printf("[Loop] Agent '%s' filtered tools: %d/%d", l.agent.Name, len(filtered), len(allDefs))
	return filtered
}

// SetAgent 设置当前 Agent
func (l *ConversationLoop) SetAgent(ag *agent.AgentType) {
	l.agent = ag
	if ag != nil && ag.MaxTurns > 0 {
		l.maxSteps = ag.MaxTurns
	}
}

// sendStatusEvent 发送状态事件
func (l *ConversationLoop) sendStatusEvent() {
	stats := l.session.GetContextStats()
	elapsed := time.Now().UnixMilli() - l.startTime

	contextPercent := 0.0
	if stats.ContextMax > 0 {
		contextPercent = float64(stats.ContextUsed) / float64(stats.ContextMax) * 100
	}

	l.eventCh <- LoopEvent{
		Type: EventTypeStatus,
		Status: &StatusInfo{
			Model:          l.client.GetModel(),
			InputTokens:    stats.InputTokens,
			OutputTokens:   stats.OutputTokens,
			TotalTokens:    stats.TotalTokens,
			ContextUsed:    stats.ContextUsed,
			ContextMax:     stats.ContextMax,
			ContextPercent: contextPercent,
			ElapsedMs:      elapsed,
			Mode:           l.mode,
		},
	}
}
