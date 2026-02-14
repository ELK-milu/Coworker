package types

// MessageRole 消息角色
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
)

// ContentBlock 内容块接口
type ContentBlock interface {
	GetType() string
}

// TextBlock 文本内容块
type TextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (t TextBlock) GetType() string { return "text" }

// ToolUseBlock 工具调用块
type ToolUseBlock struct {
	Type  string      `json:"type"`
	ID    string      `json:"id"`
	Name  string      `json:"name"`
	Input interface{} `json:"input"`
}

func (t ToolUseBlock) GetType() string { return "tool_use" }

// ToolResultBlock 工具结果块
type ToolResultBlock struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
	// 执行时间信息（用于前端显示）
	ElapsedMs int64 `json:"elapsed_ms,omitempty"`
	TimeoutMs int64 `json:"timeout_ms,omitempty"`
	TimedOut  bool  `json:"timed_out,omitempty"`
	// 执行环境 (local, microsandbox, nsjail)
	ExecEnv string `json:"exec_env,omitempty"`
}

func (t ToolResultBlock) GetType() string { return "tool_result" }

// SystemBlock 系统注入内容块
// 用于在 user 消息中嵌入系统级指令（如 reminder、continue prompt 等）
// 内部存储时不包含 <system-reminder> 标签，仅在 convert.go 转换为 API 格式时包裹标签
// 这样可以防止用户伪造 <system-reminder> 标签进行提示词注入
type SystemBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (t SystemBlock) GetType() string { return "system_block" }

// Message 消息
type Message struct {
	Role    MessageRole   `json:"role"`
	Content []interface{} `json:"content"`
}

// NewTextMessage 创建文本消息
func NewTextMessage(role MessageRole, text string) Message {
	return Message{
		Role: role,
		Content: []interface{}{
			TextBlock{Type: "text", Text: text},
		},
	}
}

// Usage token 使用统计
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StopReason 停止原因
type StopReason string

const (
	StopReasonEndTurn      StopReason = "end_turn"
	StopReasonMaxTokens    StopReason = "max_tokens"
	StopReasonStopSequence StopReason = "stop_sequence"
	StopReasonToolUse      StopReason = "tool_use"
)
