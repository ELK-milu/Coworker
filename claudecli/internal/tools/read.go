package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/claudecli/internal/sandbox"
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// ReadTool 文件读取工具
type ReadTool struct {
	workingDir string
}

type ReadInput struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

func NewReadTool(workingDir string) *ReadTool {
	return &ReadTool{workingDir: workingDir}
}

func (t *ReadTool) Name() string { return "Read" }

func (t *ReadTool) Description() string {
	return "Read file contents from the filesystem."
}

func (t *ReadTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path": map[string]interface{}{"type": "string"},
			"offset":    map[string]interface{}{"type": "integer"},
			"limit":     map[string]interface{}{"type": "integer"},
		},
		"required": []string{"file_path"},
	}
}

func (t *ReadTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	var in ReadInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	// 获取沙箱
	sb, _ := ctx.Value(types.SandboxKey).(*sandbox.Sandbox)

	// 使用沙箱解析路径
	path, err := t.resolvePathWithSandbox(ctx, in.FilePath, sb)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// 文件不存在，尝试提供模糊建议
		suggestions := t.suggestFiles(path)
		// 虚拟化建议路径
		if sb != nil {
			suggestions = sb.VirtualizePaths(suggestions)
		}
		errMsg := fmt.Sprintf("file not found: %s", in.FilePath)
		if len(suggestions) > 0 {
			errMsg += fmt.Sprintf("\n\nDid you mean one of these?\n- %s", strings.Join(suggestions, "\n- "))
		}
		return &types.ToolResult{Success: false, Error: errMsg}, nil
	}

	// 检查是否为二进制文件
	if isBinary, err := t.isBinaryFile(path); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	} else if isBinary {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("cannot read binary file: %s", in.FilePath),
		}, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	return &types.ToolResult{Success: true, Output: string(content)}, nil
}

func (t *ReadTool) resolvePath(ctx context.Context, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	workDir := types.GetWorkingDir(ctx, t.workingDir)
	return filepath.Join(workDir, path)
}

// resolvePathWithSandbox 使用沙箱解析路径
func (t *ReadTool) resolvePathWithSandbox(ctx context.Context, path string, sb *sandbox.Sandbox) (string, error) {
	if sb != nil {
		return sb.ToReal(path)
	}
	// 无沙箱时使用原有逻辑
	return t.resolvePath(ctx, path), nil
}

// isBinaryFile 检测文件是否为二进制文件
func (t *ReadTool) isBinaryFile(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	// 读取前 8000 字节进行检测
	buf := make([]byte, 8000)
	n, err := file.Read(buf)
	if err != nil && n == 0 {
		return false, err
	}

	// 检查是否包含 NULL 字节（二进制文件的特征）
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true, nil
		}
	}

	return false, nil
}

// suggestFiles 根据文件名提供模糊建议
func (t *ReadTool) suggestFiles(path string) []string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var suggestions []string
	baseLower := strings.ToLower(base)

	for _, entry := range entries {
		name := entry.Name()
		nameLower := strings.ToLower(name)

		// 检查是否包含基础名称
		if strings.Contains(nameLower, baseLower) ||
			strings.Contains(baseLower, nameLower) {
			suggestions = append(suggestions, filepath.Join(dir, name))
		}
	}

	// 最多返回 3 个建议
	if len(suggestions) > 3 {
		suggestions = suggestions[:3]
	}

	return suggestions
}
