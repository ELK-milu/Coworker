package tools

import (
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
)

// GlobTool 文件模式匹配工具
type GlobTool struct {
	workingDir string
}

type GlobInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

func NewGlobTool(workingDir string) *GlobTool {
	return &GlobTool{workingDir: workingDir}
}

func (t *GlobTool) Name() string { return "Glob" }

func (t *GlobTool) Description() string {
	return "Find files matching a glob pattern."
}

func (t *GlobTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{"type": "string"},
			"path":    map[string]interface{}{"type": "string"},
		},
		"required": []string{"pattern"},
	}
}

func (t *GlobTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	var in GlobInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	workDir := types.GetWorkingDir(ctx, t.workingDir)

	var pattern string
	// 检查 pattern 是否是绝对路径
	if filepath.IsAbs(in.Pattern) {
		pattern = in.Pattern
	} else {
		searchPath := workDir
		if in.Path != "" {
			searchPath = filepath.Join(workDir, in.Path)
		}
		pattern = filepath.Join(searchPath, in.Pattern)
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	return &types.ToolResult{
		Success: true,
		Output:  strings.Join(matches, "\n"),
	}, nil
}
