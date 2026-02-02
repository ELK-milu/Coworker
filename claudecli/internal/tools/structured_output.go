package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/QuantumNous/new-api/claudecli/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// StructuredOutputTool JSON Schema 验证工具
type StructuredOutputTool struct {
	schema     *jsonschema.Schema
	schemaJSON map[string]interface{}
	mu         sync.RWMutex
}

// StructuredOutputInput 结构化输出工具输入
type StructuredOutputInput struct {
	Data interface{} `json:"data"`
}

// NewStructuredOutputTool 创建结构化输出工具
func NewStructuredOutputTool() *StructuredOutputTool {
	return &StructuredOutputTool{}
}

func (t *StructuredOutputTool) Name() string { return "StructuredOutput" }

func (t *StructuredOutputTool) Description() string {
	return "Validate and return structured JSON output according to a predefined schema."
}

func (t *StructuredOutputTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{
				"description": "The structured data to validate and return",
			},
		},
		"required": []string{"data"},
	}
}

// SetSchema 设置 JSON Schema
func (t *StructuredOutputTool) SetSchema(schemaJSON map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 将 schema 转换为 JSON 字符串
	schemaBytes, err := json.Marshal(schemaJSON)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	// 编译 schema
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", bytes.NewReader(schemaBytes)); err != nil {
		return fmt.Errorf("failed to add schema resource: %w", err)
	}

	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("failed to compile schema: %w", err)
	}

	t.schema = schema
	t.schemaJSON = schemaJSON
	return nil
}

// GetSchema 获取当前 schema
func (t *StructuredOutputTool) GetSchema() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.schemaJSON
}

// ClearSchema 清除 schema
func (t *StructuredOutputTool) ClearSchema() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.schema = nil
	t.schemaJSON = nil
}

// HasSchema 检查是否设置了 schema
func (t *StructuredOutputTool) HasSchema() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.schema != nil
}

func (t *StructuredOutputTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	var in StructuredOutputInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to parse input: %v", err),
		}, nil
	}

	t.mu.RLock()
	schema := t.schema
	t.mu.RUnlock()

	// 如果没有设置 schema，直接返回数据
	if schema == nil {
		outputBytes, err := json.MarshalIndent(in.Data, "", "  ")
		if err != nil {
			return &types.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to marshal output: %v", err),
			}, nil
		}
		return &types.ToolResult{
			Success: true,
			Output:  string(outputBytes),
		}, nil
	}

	// 验证数据
	if err := schema.Validate(in.Data); err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("validation failed: %v", err),
		}, nil
	}

	// 返回验证通过的数据
	outputBytes, err := json.MarshalIndent(in.Data, "", "  ")
	if err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to marshal output: %v", err),
		}, nil
	}

	return &types.ToolResult{
		Success: true,
		Output:  string(outputBytes),
	}, nil
}
