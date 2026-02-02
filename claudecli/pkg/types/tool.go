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
)

// GetWorkingDir 从 context 中获取工作目录
func GetWorkingDir(ctx context.Context, defaultDir string) string {
	if dir, ok := ctx.Value(WorkingDirKey).(string); ok && dir != "" {
		return dir
	}
	return defaultDir
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

// WebSearchTool Server Tool 定义
type WebSearchTool struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
