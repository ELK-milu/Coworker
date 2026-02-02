package tools

import (
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Registry 工具注册表
type Registry struct {
	tools map[string]types.Tool
	mu    sync.RWMutex
}

// NewRegistry 创建工具注册表
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]types.Tool),
	}
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
func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (*types.ToolResult, error) {
	tool, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return tool.Execute(ctx, input)
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

	soTool, ok := tool.(*StructuredOutputTool)
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

	soTool, ok := tool.(*StructuredOutputTool)
	if !ok {
		return
	}

	soTool.ClearSchema()
}
