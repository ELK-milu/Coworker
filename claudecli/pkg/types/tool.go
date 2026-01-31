package types

import (
	"context"
	"encoding/json"
)

// ToolDefinition 工具定义
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ToolResult 工具执行结果
type ToolResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
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
