package client

import (
	"github.com/QuantumNous/new-api/claudecli/pkg/types"

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
		case types.ToolUseBlock:
			// 处理 assistant 消息中的 tool_use 块
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
		}
	}
	return anthropic.BetaMessageParam{
		Role:    anthropic.BetaMessageParamRole(msg.Role),
		Content: blocks,
	}
}
