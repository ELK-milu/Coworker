package tools

import (
	"github.com/QuantumNous/new-api/coworker/pkg/types"
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Registry 工具注册表
type Registry struct {
	tools      map[string]types.Tool
	mu         sync.RWMutex
	truncation *Truncation // 统一输出截断器
}

// NewRegistry 创建工具注册表
func NewRegistry() *Registry {
	return &Registry{
		tools:      make(map[string]types.Tool),
		truncation: nil,
	}
}

// SetTruncation 设置截断器
func (r *Registry) SetTruncation(t *Truncation) {
	r.truncation = t
}

// Register 注册工具
func (r *Registry) Register(tool types.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

// Get 获取工具
func (r *Registry) Get(name string) (types.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// GetDefinitions 获取所有工具定义
func (r *Registry) GetDefinitions() []types.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]types.ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, types.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}
	return defs
}

// Execute 执行工具
// 注意：输入验证和输出截断已由 ToolFactory 包装器处理
// 如果工具未经工厂包装，此处作为后备截断
func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (*types.ToolResult, error) {
	tool, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	result, err := tool.Execute(ctx, input)
	if err != nil {
		return result, err
	}

	// 后备截断：如果工具未经工厂包装，在此处截断
	// 已包装的工具（WrappedTool）会在内部处理截断，不会重复
	if _, isWrapped := tool.(*WrappedTool); !isWrapped {
		if result != nil && result.Success && r.truncation != nil && result.Output != "" {
			tr := r.truncation.TruncateOutput(result.Output, "head")
			result.Output = tr.Content
			if tr.Truncated {
				if result.Metadata == nil {
					result.Metadata = make(map[string]interface{})
				}
				result.Metadata["truncated"] = true
				result.Metadata["truncated_output_path"] = tr.OutputPath
			}
		}
	}

	return result, nil
}

// Unregister 注销工具
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// SetStructuredOutputSchema 设置结构化输出 schema
func (r *Registry) SetStructuredOutputSchema(schema map[string]interface{}) error {
	tool, ok := r.Get("StructuredOutput")
	if !ok {
		return fmt.Errorf("StructuredOutput tool not registered")
	}

	// 支持工厂包装的工具：先解包再断言
	inner := GetInnerTool(tool)
	soTool, ok := inner.(*StructuredOutputTool)
	if !ok {
		return fmt.Errorf("invalid StructuredOutput tool type")
	}

	return soTool.SetSchema(schema)
}

// ClearStructuredOutputSchema 清除结构化输出 schema
func (r *Registry) ClearStructuredOutputSchema() {
	tool, ok := r.Get("StructuredOutput")
	if !ok {
		return
	}

	// 支持工厂包装的工具：先解包再断言
	inner := GetInnerTool(tool)
	soTool, ok := inner.(*StructuredOutputTool)
	if !ok {
		return
	}

	soTool.ClearSchema()
}
