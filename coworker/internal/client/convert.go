package client

import (
	"fmt"

	"github.com/QuantumNous/new-api/coworker/pkg/types"

	"github.com/anthropics/anthropic-sdk-go"
)

// convertMessages 转换消息格式
func convertMessages(messages []types.Message) []anthropic.MessageParam {
	result := make([]anthropic.MessageParam, 0, len(messages))
	for _, msg := range messages {
		result = append(result, convertMessage(msg))
	}
	return result
}

func convertMessage(msg types.Message) anthropic.MessageParam {
	blocks := make([]anthropic.ContentBlockParamUnion, 0)
	for _, c := range msg.Content {
		switch v := c.(type) {
		case types.TextBlock:
			blocks = append(blocks, anthropic.NewTextBlock(v.Text))
		case types.SystemBlock:
			// SystemBlock → 包裹 <system-reminder> 标签后作为文本块发送
			wrapped := fmt.Sprintf("<system-reminder>\n%s\n</system-reminder>", v.Text)
			blocks = append(blocks, anthropic.NewTextBlock(wrapped))
		case types.ToolResultBlock:
			blocks = append(blocks, anthropic.NewToolResultBlock(v.ToolUseID, v.Content, v.IsError))
		}
	}
	return anthropic.MessageParam{
		Role:    anthropic.MessageParamRole(msg.Role),
		Content: blocks,
	}
}

// convertBetaMessages 转换消息格式（Beta API）
func convertBetaMessages(messages []types.Message) []anthropic.BetaMessageParam {
	result := make([]anthropic.BetaMessageParam, 0, len(messages))
	for _, msg := range messages {
		result = append(result, convertBetaMessage(msg))
	}
	return result
}

func convertBetaMessage(msg types.Message) anthropic.BetaMessageParam {
	blocks := make([]anthropic.BetaContentBlockParamUnion, 0)
	for _, c := range msg.Content {
		switch v := c.(type) {
		case types.TextBlock:
			blocks = append(blocks, anthropic.BetaContentBlockParamUnion{
				OfText: &anthropic.BetaTextBlockParam{
					Type: "text",
					Text: v.Text,
				},
			})
		case types.SystemBlock:
			// SystemBlock → 包裹 <system-reminder> 标签后作为文本块发送
			wrapped := fmt.Sprintf("<system-reminder>\n%s\n</system-reminder>", v.Text)
			blocks = append(blocks, anthropic.BetaContentBlockParamUnion{
				OfText: &anthropic.BetaTextBlockParam{
					Type: "text",
					Text: wrapped,
				},
			})
		case types.ToolUseBlock:
			blocks = append(blocks, anthropic.BetaContentBlockParamUnion{
				OfToolUse: &anthropic.BetaToolUseBlockParam{
					Type:  "tool_use",
					ID:    v.ID,
					Name:  v.Name,
					Input: v.Input,
				},
			})
		case types.ToolResultBlock:
			blocks = append(blocks, anthropic.BetaContentBlockParamUnion{
				OfToolResult: &anthropic.BetaToolResultBlockParam{
					Type:      "tool_result",
					ToolUseID: v.ToolUseID,
					Content: []anthropic.BetaToolResultBlockParamContentUnion{
						{OfText: &anthropic.BetaTextBlockParam{
							Type: "text",
							Text: v.Content,
						}},
					},
					IsError: anthropic.Bool(v.IsError),
				},
			})
		case map[string]interface{}:
			// 处理从 JSON 反序列化后的 map 类型
			block := convertMapToBlock(v)
			if block != nil {
				blocks = append(blocks, *block)
			}
		}
	}
	return anthropic.BetaMessageParam{
		Role:    anthropic.BetaMessageParamRole(msg.Role),
		Content: blocks,
	}
}

// convertMapToBlock 将 map 转换为内容块
func convertMapToBlock(m map[string]interface{}) *anthropic.BetaContentBlockParamUnion {
	blockType, _ := m["type"].(string)
	switch blockType {
	case "text":
		text, _ := m["text"].(string)
		return &anthropic.BetaContentBlockParamUnion{
			OfText: &anthropic.BetaTextBlockParam{
				Type: "text",
				Text: text,
			},
		}
	case "system_block":
		text, _ := m["text"].(string)
		wrapped := fmt.Sprintf("<system-reminder>\n%s\n</system-reminder>", text)
		return &anthropic.BetaContentBlockParamUnion{
			OfText: &anthropic.BetaTextBlockParam{
				Type: "text",
				Text: wrapped,
			},
		}
	case "tool_use":
		id, _ := m["id"].(string)
		name, _ := m["name"].(string)
		input := m["input"]
		return &anthropic.BetaContentBlockParamUnion{
			OfToolUse: &anthropic.BetaToolUseBlockParam{
				Type:  "tool_use",
				ID:    id,
				Name:  name,
				Input: input,
			},
		}
	case "tool_result":
		toolUseID, _ := m["tool_use_id"].(string)
		content, _ := m["content"].(string)
		isError, _ := m["is_error"].(bool)
		return &anthropic.BetaContentBlockParamUnion{
			OfToolResult: &anthropic.BetaToolResultBlockParam{
				Type:      "tool_result",
				ToolUseID: toolUseID,
				Content: []anthropic.BetaToolResultBlockParamContentUnion{
					{OfText: &anthropic.BetaTextBlockParam{
						Type: "text",
						Text: content,
					}},
				},
				IsError: anthropic.Bool(isError),
			},
		}
	}
	return nil
}
