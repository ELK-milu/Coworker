package types

import (
	"context"
	"encoding/json"
)

// ContextKey 用于 context 传递值的 key 类型
type ContextKey string

const (
	// WorkingDirKey 工作目录的 context key
	WorkingDirKey ContextKey = "working_dir"
	// UserIDKey 用户 ID 的 context key
	UserIDKey ContextKey = "user_id"
	// SessionIDKey 会话 ID 的 context key
	SessionIDKey ContextKey = "session_id"
	// MessageIDKey 当前消息 ID 的 context key
	MessageIDKey ContextKey = "message_id"
	// SandboxKey 沙箱的 context key
	SandboxKey ContextKey = "sandbox"
	// FileTimeKey 文件时间追踪器的 context key
	FileTimeKey ContextKey = "filetime"
	// MetadataCallbackKey 元数据回调函数的 context key
	MetadataCallbackKey ContextKey = "metadata_callback"
	// EventChKey 事件通道的 context key（用于子代理转发）
	EventChKey ContextKey = "event_ch"
	// ToolProviderKey 工具提供者的 context key（用于子代理继承 MCP 工具）
	ToolProviderKey ContextKey = "tool_provider"
)

// MetadataCallback 元数据回调函数类型
// 工具执行过程中可通过此回调实时更新元数据（如进度、中间输出等）
type MetadataCallback func(key string, value interface{})

// GetWorkingDir 从 context 中获取工作目录
func GetWorkingDir(ctx context.Context, defaultDir string) string {
	if dir, ok := ctx.Value(WorkingDirKey).(string); ok && dir != "" {
		return dir
	}
	return defaultDir
}

// GetUserID 从 context 中获取用户 ID
func GetUserID(ctx context.Context) string {
	if id, ok := ctx.Value(UserIDKey).(string); ok {
		return id
	}
	return ""
}

// GetSessionID 从 context 中获取会话 ID
func GetSessionID(ctx context.Context) string {
	if id, ok := ctx.Value(SessionIDKey).(string); ok {
		return id
	}
	return ""
}

// GetMetadataCallback 从 context 中获取元数据回调
func GetMetadataCallback(ctx context.Context) MetadataCallback {
	if cb, ok := ctx.Value(MetadataCallbackKey).(MetadataCallback); ok {
		return cb
	}
	return nil
}

// SendMetadata 通过 context 中的回调发送元数据
func SendMetadata(ctx context.Context, key string, value interface{}) {
	if cb := GetMetadataCallback(ctx); cb != nil {
		cb(key, value)
	}
}

// ToolDefinition 工具定义
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ToolResult 工具执行结果
type ToolResult struct {
	Success   bool                   `json:"success"`
	Output    string                 `json:"output,omitempty"`
	Error     string                 `json:"error,omitempty"`
	ElapsedMs int64                  `json:"elapsed_ms,omitempty"`
	TimeoutMs int64                  `json:"timeout_ms,omitempty"`
	TimedOut  bool                   `json:"timed_out,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Tool 工具接口
type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]interface{}
	Execute(ctx context.Context, input json.RawMessage) (*ToolResult, error)
}

// ToolProvider 工具提供者接口（抽象工具注册表）
// *tools.Registry 和 *tools.ToolOverlay 均满足此接口
type ToolProvider interface {
	Get(name string) (Tool, bool)
	GetDefinitions() []ToolDefinition
	Execute(ctx context.Context, name string, input json.RawMessage) (*ToolResult, error)
}

// WebSearchTool Server Tool 定义
type WebSearchTool struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
