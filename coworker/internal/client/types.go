package client

// StreamEventType 流事件类型
type StreamEventType string

const (
	EventText      StreamEventType = "text"
	EventThinking  StreamEventType = "thinking"
	EventToolStart StreamEventType = "tool_start"
	EventToolDelta StreamEventType = "tool_delta"
	EventStop      StreamEventType = "stop"
	EventUsage     StreamEventType = "usage"
	EventError     StreamEventType = "error"
)

// StreamEvent 流事件
type StreamEvent struct {
	Type       StreamEventType `json:"type"`
	Text       string          `json:"text,omitempty"`
	ToolID     string          `json:"tool_id,omitempty"`
	ToolName   string          `json:"tool_name,omitempty"`
	ToolInput  string          `json:"tool_input,omitempty"`
	StopReason string          `json:"stop_reason,omitempty"`
	Usage      *UsageInfo      `json:"usage,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// UsageInfo 使用统计
type UsageInfo struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
