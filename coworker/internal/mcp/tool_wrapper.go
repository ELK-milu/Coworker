package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/coworker/pkg/types"
)

// MCPToolWrapper 将 MCP 工具包装为 types.Tool 接口
type MCPToolWrapper struct {
	serverName string
	toolInfo   ToolInfo
	manager    *Manager
	connID     string
}

// sanitizeName 将名称转为安全的工具名（非字母数字下划线横线 → 下划线）
var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func sanitizeName(s string) string {
	return unsafeChars.ReplaceAllString(s, "_")
}

func (t *MCPToolWrapper) Name() string {
	return fmt.Sprintf("mcp__%s__%s", sanitizeName(t.serverName), sanitizeName(t.toolInfo.Name))
}

func (t *MCPToolWrapper) Description() string {
	desc := t.toolInfo.Description
	if desc == "" {
		desc = fmt.Sprintf("MCP tool from %s", t.serverName)
	}
	return desc
}

func (t *MCPToolWrapper) InputSchema() map[string]interface{} {
	if t.toolInfo.InputSchema != nil {
		return t.toolInfo.InputSchema
	}
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *MCPToolWrapper) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	result, err := t.manager.CallTool(ctx, t.connID, t.toolInfo.Name, input)
	if err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("MCP tool %s/%s error: %v", t.serverName, t.toolInfo.Name, err),
		}, nil
	}

	// 尝试从 MCP 结果中提取文本内容
	output := extractMCPContent(result)
	return &types.ToolResult{
		Success: true,
		Output:  output,
		Metadata: map[string]interface{}{
			"mcp_server": t.serverName,
			"mcp_tool":   t.toolInfo.Name,
		},
	}, nil
}

// extractMCPContent 从 MCP 工具结果中提取文本内容
func extractMCPContent(result json.RawMessage) string {
	if result == nil {
		return ""
	}

	// MCP 结果可能是 {"content": [{"type":"text","text":"..."}]}
	var structured struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(result, &structured); err == nil && len(structured.Content) > 0 {
		var texts []string
		for _, c := range structured.Content {
			if c.Type == "text" && c.Text != "" {
				texts = append(texts, c.Text)
			}
		}
		if len(texts) > 0 {
			return strings.Join(texts, "\n")
		}
	}

	// 回退：直接返回 JSON 字符串
	return string(result)
}

// WrapConnectionTools 将连接的所有工具包装为 types.Tool 列表
func WrapConnectionTools(conn *Connection, mgr *Manager) []types.Tool {
	tools := make([]types.Tool, 0, len(conn.Tools))
	for _, ti := range conn.Tools {
		tools = append(tools, &MCPToolWrapper{
			serverName: conn.Name,
			toolInfo:   ti,
			manager:    mgr,
			connID:     conn.ID,
		})
	}
	return tools
}
