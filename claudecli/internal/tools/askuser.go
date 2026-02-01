package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

const (
	askUserTimeout = 5 * time.Minute
)

// AskUserContextKey context key for ask user callback
type AskUserContextKey string

const (
	// AskUserCallbackKey callback function key
	AskUserCallbackKey AskUserContextKey = "ask_user_callback"
)

// AskUserCallback 发送问题到前端的回调函数类型
type AskUserCallback func(requestID string, questions []Question) error

// AskUserQuestionTool 向用户提问工具
type AskUserQuestionTool struct {
	pendingMu   sync.Mutex
	pendingReqs map[string]chan *UserResponse
}

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

// UserResponse 用户响应
type UserResponse struct {
	RequestID string            `json:"request_id"`
	Answers   map[string]string `json:"answers"`
	Cancelled bool              `json:"cancelled"`
}

// NewAskUserQuestionTool 创建工具实例
func NewAskUserQuestionTool() *AskUserQuestionTool {
	return &AskUserQuestionTool{
		pendingReqs: make(map[string]chan *UserResponse),
	}
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

	// 从 context 获取回调函数
	callback, ok := ctx.Value(AskUserCallbackKey).(AskUserCallback)
	if !ok || callback == nil {
		return &types.ToolResult{Success: false, Error: "ask user callback not available"}, nil
	}

	// 生成请求 ID
	requestID := fmt.Sprintf("ask_%d", time.Now().UnixNano())

	// 创建响应通道
	respCh := make(chan *UserResponse, 1)
	t.pendingMu.Lock()
	t.pendingReqs[requestID] = respCh
	t.pendingMu.Unlock()

	defer func() {
		t.pendingMu.Lock()
		delete(t.pendingReqs, requestID)
		t.pendingMu.Unlock()
	}()

	// 发送问题到前端
	if err := callback(requestID, in.Questions); err != nil {
		return &types.ToolResult{Success: false, Error: "failed to send: " + err.Error()}, nil
	}

	// 等待用户响应
	select {
	case <-ctx.Done():
		return &types.ToolResult{Success: false, Error: "cancelled"}, nil
	case <-time.After(askUserTimeout):
		return &types.ToolResult{Success: false, Error: "timeout"}, nil
	case resp := <-respCh:
		if resp.Cancelled {
			return &types.ToolResult{Success: false, Error: "user cancelled"}, nil
		}
		return &types.ToolResult{Success: true, Output: formatAnswers(in.Questions, resp.Answers)}, nil
	}
}

// HandleResponse 处理用户响应
func (t *AskUserQuestionTool) HandleResponse(resp *UserResponse) bool {
	t.pendingMu.Lock()
	ch, ok := t.pendingReqs[resp.RequestID]
	t.pendingMu.Unlock()

	if !ok {
		return false
	}

	select {
	case ch <- resp:
		return true
	default:
		return false
	}
}

func formatAnswers(questions []Question, answers map[string]string) string {
	result := "User responses:\n"
	for i, q := range questions {
		key := fmt.Sprintf("q%d", i)
		answer := answers[key]
		if answer == "" {
			answer = "(no answer)"
		}
		result += fmt.Sprintf("- %s: %s\n", q.Header, answer)
	}
	return result
}
