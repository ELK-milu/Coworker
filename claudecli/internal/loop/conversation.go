package loop

import (
	"github.com/QuantumNous/new-api/claudecli/internal/client"
	"github.com/QuantumNous/new-api/claudecli/internal/session"
	"github.com/QuantumNous/new-api/claudecli/internal/tools"
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
	"context"
	"encoding/json"
	"log"
	"time"
)

// ConversationLoop 对话循环
type ConversationLoop struct {
	client    *client.ClaudeClient
	session   *session.Session
	tools     *tools.Registry
	eventCh   chan<- LoopEvent
	system    string
	mode      string // normal, plan, acceptEdits, bypassPermissions
	startTime int64
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
	// 状态信息
	Status *StatusInfo `json:"status,omitempty"`
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
	EventTypeText       = "text"
	EventTypeToolStart  = "tool_start"
	EventTypeToolEnd    = "tool_end"
	EventTypeDone       = "done"
	EventTypeError      = "error"
	EventTypeStatus     = "status"
)

// NewConversationLoop 创建对话循环
func NewConversationLoop(
	c *client.ClaudeClient,
	sess *session.Session,
	registry *tools.Registry,
	systemPrompt string,
	eventCh chan<- LoopEvent,
) *ConversationLoop {
	return &ConversationLoop{
		client:  c,
		session: sess,
		tools:   registry,
		system:  systemPrompt,
		eventCh: eventCh,
		mode:    "normal",
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

	for {
		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			l.sendStatusEvent()
			l.eventCh <- LoopEvent{Type: EventTypeDone, Done: true}
			return ctx.Err()
		default:
		}

		// 检查上下文是否接近限制，如果是则压缩
		if l.session.IsContextNearLimit() {
			log.Printf("[Loop] Context near limit, compacting...")
			l.session.CompactContext()
		}

		// 发送状态事件（开始处理）
		l.sendStatusEvent()

		// 调用 Claude API
		streamCh, err := l.client.CreateMessageStream(
			ctx,
			l.session.Messages,
			l.tools.GetDefinitions(),
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

		// 如果没有工具调用，结束循环
		if stopReason != "tool_use" || len(toolCalls) == 0 {
			l.eventCh <- LoopEvent{Type: EventTypeDone, Done: true}
			return nil
		}

		// 执行工具调用
		if err := l.executeTools(ctx, toolCalls); err != nil {
			return err
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
		content = append(content, types.ToolUseBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Name,
			Input: json.RawMessage(tc.Input),
		})
	}
	if len(content) > 0 {
		l.session.AddMessage(types.Message{Role: "assistant", Content: content})
	}
}

// executeTools 执行工具调用
func (l *ConversationLoop) executeTools(ctx context.Context, calls []toolCall) error {
	results := make([]interface{}, 0, len(calls))

	for _, tc := range calls {
		log.Printf("[Tool] Executing: name=%s, id=%s", tc.Name, tc.ID)
		log.Printf("[Tool] Input: %s", tc.Input)

		result, err := l.tools.Execute(ctx, tc.Name, json.RawMessage(tc.Input))

		var toolResult types.ToolResultBlock
		if err != nil {
			log.Printf("[Tool] Error: %v", err)
			toolResult = types.ToolResultBlock{
				Type:      "tool_result",
				ToolUseID: tc.ID,
				Content:   err.Error(),
				IsError:   true,
			}
		} else {
			log.Printf("[Tool] Success: %v, Output length: %d", result.Success, len(result.Output))
			toolResult = types.ToolResultBlock{
				Type:      "tool_result",
				ToolUseID: tc.ID,
				Content:   result.Output,
				IsError:   !result.Success,
			}
		}

		results = append(results, toolResult)
		l.eventCh <- LoopEvent{
			Type:       EventTypeToolEnd,
			ToolID:     tc.ID,
			ToolName:   tc.Name,
			ToolResult: toolResult.Content,
			IsError:    toolResult.IsError,
		}
	}

	// 添加工具结果消息
	l.session.AddMessage(types.Message{Role: "user", Content: results})
	return nil
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
