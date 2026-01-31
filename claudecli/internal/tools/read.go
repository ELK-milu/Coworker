package tools

import (
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

	path := t.resolvePath(in.FilePath)
	content, err := os.ReadFile(path)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	return &types.ToolResult{Success: true, Output: string(content)}, nil
}

func (t *ReadTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.workingDir, path)
}
