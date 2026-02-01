package tools

import (
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// WriteTool 文件写入工具
type WriteTool struct {
	workingDir string
}

type WriteInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

func NewWriteTool(workingDir string) *WriteTool {
	return &WriteTool{workingDir: workingDir}
}

func (t *WriteTool) Name() string { return "Write" }

func (t *WriteTool) Description() string {
	return "Write content to a file."
}

func (t *WriteTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path": map[string]interface{}{"type": "string"},
			"content":   map[string]interface{}{"type": "string"},
		},
		"required": []string{"file_path", "content"},
	}
}

func (t *WriteTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	var in WriteInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	path := t.resolvePath(ctx, in.FilePath)
	log.Printf("[Write] Input path: %s, Resolved path: %s", in.FilePath, path)

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	if err := os.WriteFile(path, []byte(in.Content), 0644); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	return &types.ToolResult{Success: true, Output: "File written successfully to " + path}, nil
}

func (t *WriteTool) resolvePath(ctx context.Context, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	workDir := types.GetWorkingDir(ctx, t.workingDir)
	log.Printf("[Write] resolvePath: defaultWorkDir=%s, contextWorkDir=%s", t.workingDir, workDir)
	return filepath.Join(workDir, path)
}
