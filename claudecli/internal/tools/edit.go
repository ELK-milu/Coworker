package tools

import (
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// EditTool 文件编辑工具
type EditTool struct {
	workingDir    string
	replacerChain *ReplacerChain
}

type EditInput struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

func NewEditTool(workingDir string) *EditTool {
	return &EditTool{
		workingDir:    workingDir,
		replacerChain: NewReplacerChain(),
	}
}

func (t *EditTool) Name() string { return "Edit" }

func (t *EditTool) Description() string {
	return "Edit file by replacing old_string with new_string."
}

func (t *EditTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path":   map[string]interface{}{"type": "string"},
			"old_string":  map[string]interface{}{"type": "string"},
			"new_string":  map[string]interface{}{"type": "string"},
			"replace_all": map[string]interface{}{"type": "boolean"},
		},
		"required": []string{"file_path", "old_string", "new_string"},
	}
}

func (t *EditTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	var in EditInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	path := t.resolvePath(ctx, in.FilePath)
	content, err := os.ReadFile(path)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	oldContent := string(content)
	var newContent string
	var matchMethod string

	if in.ReplaceAll {
		// ReplaceAll 模式仍使用精确匹配
		newContent = strings.ReplaceAll(oldContent, in.OldString, in.NewString)
		matchMethod = "exact"
	} else {
		// 使用 ReplacerChain 进行模糊匹配
		match, method := t.replacerChain.FindBestMatch(oldContent, in.OldString)
		if match != nil {
			newContent = oldContent[:match.Start] + in.NewString + oldContent[match.End:]
			matchMethod = method
		} else {
			newContent = oldContent
		}
	}

	if oldContent == newContent {
		return &types.ToolResult{Success: false, Error: "old_string not found"}, nil
	}

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	output := "File edited successfully"
	if matchMethod != "" && matchMethod != "SimpleReplacer" {
		output = "File edited successfully (matched via " + matchMethod + ")"
	}

	return &types.ToolResult{Success: true, Output: output}, nil
}

func (t *EditTool) resolvePath(ctx context.Context, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	workDir := types.GetWorkingDir(ctx, t.workingDir)
	return filepath.Join(workDir, path)
}
