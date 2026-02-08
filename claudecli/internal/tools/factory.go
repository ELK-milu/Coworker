package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// ToolFactory 工具工厂
// 参考 OpenCode tool/tool.ts 的 Tool.define() 模式
// 提供统一的工具创建、输入验证、输出截断功能
type ToolFactory struct {
	truncation *Truncation
}

// NewToolFactory 创建工具工厂
func NewToolFactory(truncation *Truncation) *ToolFactory {
	return &ToolFactory{
		truncation: truncation,
	}
}

// WrappedTool 包装后的工具，自动验证输入 + 截断输出
type WrappedTool struct {
	inner      types.Tool
	truncation *Truncation
}

// Wrap 包装一个工具，添加自动验证和截断
func (f *ToolFactory) Wrap(tool types.Tool) types.Tool {
	return &WrappedTool{
		inner:      tool,
		truncation: f.truncation,
	}
}

func (w *WrappedTool) Name() string        { return w.inner.Name() }
func (w *WrappedTool) Description() string  { return w.inner.Description() }
func (w *WrappedTool) InputSchema() map[string]interface{} { return w.inner.InputSchema() }

// Execute 执行工具（自动验证输入 + 截断输出）
func (w *WrappedTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	// 1. 验证输入参数
	if err := w.validateInput(input); err != nil {
		return &types.ToolResult{
			Success: false,
			Error: fmt.Sprintf(
				"The %s tool was called with invalid arguments: %s\nPlease rewrite the input so it satisfies the expected schema.",
				w.inner.Name(), err.Error(),
			),
		}, nil
	}

	// 2. 执行工具
	result, err := w.inner.Execute(ctx, input)
	if err != nil {
		return result, err
	}

	// 3. 对成功的输出应用截断
	if result != nil && result.Success && w.truncation != nil && result.Output != "" {
		tr := w.truncation.TruncateOutput(result.Output, "head")
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

// validateInput 验证输入参数是否符合 schema
func (w *WrappedTool) validateInput(input json.RawMessage) error {
	schema := w.inner.InputSchema()
	if schema == nil {
		return nil
	}

	// 解析输入 JSON
	var inputMap map[string]interface{}
	if err := json.Unmarshal(input, &inputMap); err != nil {
		return fmt.Errorf("invalid JSON input: %v", err)
	}

	// 检查 required 字段
	if required, ok := schema["required"].([]string); ok {
		for _, field := range required {
			if _, exists := inputMap[field]; !exists {
				return fmt.Errorf("missing required field: %s", field)
			}
		}
	}

	// 检查 required 字段（[]interface{} 类型，从 JSON 反序列化时可能是这种类型）
	if required, ok := schema["required"].([]interface{}); ok {
		for _, field := range required {
			fieldStr, ok := field.(string)
			if !ok {
				continue
			}
			if _, exists := inputMap[fieldStr]; !exists {
				return fmt.Errorf("missing required field: %s", fieldStr)
			}
		}
	}

	// 检查属性类型
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		for key, val := range inputMap {
			propSchema, exists := props[key]
			if !exists {
				continue // 允许额外字段
			}
			if err := validateFieldType(key, val, propSchema); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateFieldType 验证字段类型
func validateFieldType(name string, value interface{}, schema interface{}) error {
	propMap, ok := schema.(map[string]interface{})
	if !ok {
		return nil
	}

	expectedType, ok := propMap["type"].(string)
	if !ok {
		return nil
	}

	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field %s: expected string, got %T", name, value)
		}
	case "integer":
		switch v := value.(type) {
		case float64:
			if v != float64(int64(v)) {
				return fmt.Errorf("field %s: expected integer, got float", name)
			}
		case json.Number:
			if _, err := v.Int64(); err != nil {
				return fmt.Errorf("field %s: expected integer: %v", name, err)
			}
		default:
			return fmt.Errorf("field %s: expected integer, got %T", name, value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("field %s: expected boolean, got %T", name, value)
		}
	case "number":
		switch value.(type) {
		case float64, json.Number:
			// ok
		default:
			return fmt.Errorf("field %s: expected number, got %T", name, value)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("field %s: expected array, got %T", name, value)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("field %s: expected object, got %T", name, value)
		}
	}

	return nil
}

// RegisterWithFactory 使用工厂注册工具到注册表
// 工具会被自动包装，添加输入验证和输出截断
func RegisterWithFactory(registry *Registry, factory *ToolFactory, tool types.Tool) {
	wrapped := factory.Wrap(tool)
	registry.Register(wrapped)
	log.Printf("[ToolFactory] Registered tool: %s (with validation + truncation)", tool.Name())
}

// RegisterAllWithFactory 批量注册工具
func RegisterAllWithFactory(registry *Registry, factory *ToolFactory, tools []types.Tool) {
	for _, tool := range tools {
		RegisterWithFactory(registry, factory, tool)
	}
	log.Printf("[ToolFactory] Registered %d tools with factory wrapper", len(tools))
}

// GetInnerTool 获取包装工具的内部工具（用于类型断言）
func GetInnerTool(tool types.Tool) types.Tool {
	if w, ok := tool.(*WrappedTool); ok {
		return w.inner
	}
	return tool
}

// UnwrapAs 尝试将工具解包为指定类型
// 用法: if bashTool, ok := UnwrapAs[*BashTool](tool); ok { ... }
func UnwrapAs[T types.Tool](tool types.Tool) (T, bool) {
	// 先尝试直接断言
	if t, ok := tool.(T); ok {
		return t, true
	}
	// 再尝试解包后断言
	inner := GetInnerTool(tool)
	if t, ok := inner.(T); ok {
		return t, true
	}
	var zero T
	return zero, false
}

// ToolNames 返回工具名称列表（用于日志和调试）
func ToolNames(tools []types.Tool) string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name()
	}
	return strings.Join(names, ", ")
}
