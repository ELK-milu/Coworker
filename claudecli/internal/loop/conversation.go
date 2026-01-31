package loop

import (
	"github.com/QuantumNous/new-api/claudecli/internal/client"
	"github.com/QuantumNous/new-api/claudecli/internal/session"
	"github.com/QuantumNous/new-api/claudecli/internal/tools"
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
	"context"
	"encoding/json"
	"log"
)

// ConversationLoop 对话循环
type ConversationLoop struct {
	client   *client.ClaudeClient
	session  *session.Session
	tools    *tools.Registry
	eventCh  chan<- LoopEvent
	system   string
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
}

const (
	EventTypeText       = "text"
	EventTypeToolStart  = "tool_start"
	EventTypeToolEnd    = "tool_end"
	EventTypeDone       = "done"
	EventTypeError      = "error"
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
	}
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
	for {
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
		toolCalls, stopReason, err := l.processStream(streamCh)
		if err != nil {
			return err
		}

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
func (l *ConversationLoop) processStream(streamCh <-chan client.StreamEvent) ([]toolCall, string, error) {
	var toolCalls []toolCall
	var currentTool *toolCall
	var textContent string
	var stopReason string

	for event := range streamCh {
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

	// 保存助手消息
	l.saveAssistantMessage(textContent, toolCalls)
	return toolCalls, stopReason, nil
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
