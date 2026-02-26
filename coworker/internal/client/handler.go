package client

import (
	"log"

	"github.com/anthropics/anthropic-sdk-go"
)

// handleBetaStreamEvent 处理 Beta 流事件
func (c *ClaudeClient) handleBetaStreamEvent(event anthropic.BetaRawMessageStreamEventUnion, eventCh chan<- StreamEvent) {
	switch event.Type {
	case "message_start":
		// 捕获 input_tokens（Anthropic API 在 message_start 中返回）
		if event.Message.Usage.InputTokens > 0 {
			eventCh <- StreamEvent{
				Type: EventUsage,
				Usage: &UsageInfo{
					InputTokens: int(event.Message.Usage.InputTokens),
				},
			}
		}
	case "content_block_start":
		if event.ContentBlock.Type == "tool_use" {
			log.Printf("[API] Tool use start: id=%s, name=%s", event.ContentBlock.ID, event.ContentBlock.Name)
			eventCh <- StreamEvent{
				Type:     EventToolStart,
				ToolID:   event.ContentBlock.ID,
				ToolName: event.ContentBlock.Name,
			}
		} else if event.ContentBlock.Type == "thinking" {
			eventCh <- StreamEvent{
				Type: EventThinking,
				Text: "", // 开始标记
			}
		}
	case "content_block_delta":
		if event.Delta.Type == "text_delta" {
			eventCh <- StreamEvent{
				Type: EventText,
				Text: event.Delta.Text,
			}
		} else if event.Delta.Type == "thinking_delta" {
			eventCh <- StreamEvent{
				Type: EventThinking,
				Text: event.Delta.Thinking,
			}
		} else if event.Delta.Type == "input_json_delta" {
			eventCh <- StreamEvent{
				Type:      EventToolDelta,
				ToolInput: event.Delta.PartialJSON,
			}
		}
	case "message_stop":
		// noop
	case "message_delta":
		// stop_reason 在 message_delta 事件的 Delta 中
		if event.Delta.StopReason != "" {
			eventCh <- StreamEvent{
				Type:       EventStop,
				StopReason: string(event.Delta.StopReason),
			}
		}
		if event.Usage.OutputTokens > 0 {
			eventCh <- StreamEvent{
				Type: EventUsage,
				Usage: &UsageInfo{
					OutputTokens: int(event.Usage.OutputTokens),
				},
			}
		}
	}
}
