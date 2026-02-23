package tools

import (
	"context"
	"encoding/json"

	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// AskUserQuestionTool 向用户提问工具（非阻塞模式）
// 工具立即返回，前端从 tool_end 事件中解析 questions 并展示交互面板
// 用户回答后作为普通聊天消息发送，AI 在下一轮对话中接收
type AskUserQuestionTool struct{}

// AskUserInput 输入参数
type AskUserInput struct {
	Questions []Question `json:"questions"`
}

// Question 问题定义
type Question struct {
	Question    string   `json:"question"`
	Header      string   `json:"header"`
	Options     []Option `json:"options"`
	MultiSelect bool     `json:"multiSelect"`
}

// Option 选项定义
type Option struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// NewAskUserQuestionTool 创建工具实例
func NewAskUserQuestionTool() *AskUserQuestionTool {
	return &AskUserQuestionTool{}
}

func (t *AskUserQuestionTool) Name() string { return "AskUserQuestion" }

func (t *AskUserQuestionTool) Description() string {
	return "Ask the user questions to gather preferences or clarify instructions."
}

func (t *AskUserQuestionTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"questions": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"question":    map[string]interface{}{"type": "string"},
						"header":      map[string]interface{}{"type": "string"},
						"multiSelect": map[string]interface{}{"type": "boolean"},
						"options": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"label":       map[string]interface{}{"type": "string"},
									"description": map[string]interface{}{"type": "string"},
								},
							},
						},
					},
				},
			},
		},
		"required": []string{"questions"},
	}
}

func (t *AskUserQuestionTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	var in AskUserInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	if len(in.Questions) == 0 {
		return &types.ToolResult{Success: false, Error: "at least one question required"}, nil
	}

	// 非阻塞模式：立即返回，告知 AI 问题已展示给用户
	// 前端从 tool_end 事件中解析 input.questions 并展示交互面板
	// 用户回答后作为普通聊天消息发送
	return &types.ToolResult{
		Success: true,
		Output:  "Questions have been presented to the user. Wait for their response in the next message.",
	}, nil
}
