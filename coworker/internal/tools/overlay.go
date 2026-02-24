package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/QuantumNous/new-api/coworker/pkg/types"
)

// ToolOverlay 工具覆盖层
// 组合基础 ToolProvider（共享注册表）和附加工具（MCP 等），
// 使每用户可拥有独立的工具列表而不污染全局注册表。
type ToolOverlay struct {
	base       types.ToolProvider
	extra      map[string]types.Tool
	mu         sync.RWMutex
	truncation *Truncation
}

// NewToolOverlay 创建工具覆盖层
func NewToolOverlay(base types.ToolProvider) *ToolOverlay {
	return &ToolOverlay{
		base:  base,
		extra: make(map[string]types.Tool),
	}
}

// SetTruncation 设置截断器（用于未经工厂包装的附加工具）
func (o *ToolOverlay) SetTruncation(t *Truncation) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.truncation = t
}

// AddTool 添加附加工具（extra 优先于 base）
func (o *ToolOverlay) AddTool(tool types.Tool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.extra[tool.Name()] = tool
}

// RemoveTool 移除附加工具
func (o *ToolOverlay) RemoveTool(name string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.extra, name)
}

// Get 获取工具（extra 优先）
func (o *ToolOverlay) Get(name string) (types.Tool, bool) {
	o.mu.RLock()
	if tool, ok := o.extra[name]; ok {
		o.mu.RUnlock()
		return tool, true
	}
	o.mu.RUnlock()
	return o.base.Get(name)
}

// GetDefinitions 获取所有工具定义（base + extra 合并，extra 覆盖同名）
func (o *ToolOverlay) GetDefinitions() []types.ToolDefinition {
	baseDefs := o.base.GetDefinitions()

	o.mu.RLock()
	defer o.mu.RUnlock()

	if len(o.extra) == 0 {
		return baseDefs
	}

	// 收集 base 中未被 extra 覆盖的定义
	seen := make(map[string]bool, len(o.extra))
	for name := range o.extra {
		seen[name] = true
	}

	defs := make([]types.ToolDefinition, 0, len(baseDefs)+len(o.extra))
	for _, def := range baseDefs {
		if !seen[def.Name] {
			defs = append(defs, def)
		}
	}

	// 追加 extra 工具定义
	for _, tool := range o.extra {
		defs = append(defs, types.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}

	return defs
}

// Execute 执行工具（extra 优先，含后备截断）
func (o *ToolOverlay) Execute(ctx context.Context, name string, input json.RawMessage) (*types.ToolResult, error) {
	o.mu.RLock()
	tool, isExtra := o.extra[name]
	truncation := o.truncation
	o.mu.RUnlock()

	if isExtra {
		result, err := tool.Execute(ctx, input)
		if err != nil {
			return result, err
		}
		// 后备截断：附加工具未经工厂包装
		if result != nil && result.Success && truncation != nil && result.Output != "" {
			tr := truncation.TruncateOutput(result.Output, "head")
			result.Output = tr.Content
			if tr.Truncated {
				if result.Metadata == nil {
					result.Metadata = make(map[string]interface{})
				}
				result.Metadata["truncated"] = true
				result.Metadata["truncated_output_path"] = tr.OutputPath
			}
		}
		return result, nil
	}

	return o.base.Execute(ctx, name, input)
}

// Ensure ToolOverlay satisfies ToolProvider
var _ types.ToolProvider = (*ToolOverlay)(nil)

// Ensure Registry also satisfies ToolProvider (compile-time check)
var _ types.ToolProvider = (*Registry)(nil)

// SetStructuredOutputSchema 代理到 base（如果 base 是 Registry）
func (o *ToolOverlay) SetStructuredOutputSchema(schema map[string]interface{}) error {
	if reg, ok := o.base.(*Registry); ok {
		return reg.SetStructuredOutputSchema(schema)
	}
	return fmt.Errorf("base does not support SetStructuredOutputSchema")
}

// ClearStructuredOutputSchema 代理到 base（如果 base 是 Registry）
func (o *ToolOverlay) ClearStructuredOutputSchema() {
	if reg, ok := o.base.(*Registry); ok {
		reg.ClearStructuredOutputSchema()
	}
}
