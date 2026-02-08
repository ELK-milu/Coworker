package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/claudecli/internal/sandbox"
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
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
	startTime := time.Now()

	var in EditInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error(), ElapsedMs: time.Since(startTime).Milliseconds()}, nil
	}

	// 获取沙箱
	sb, _ := ctx.Value(types.SandboxKey).(*sandbox.Sandbox)

	// 获取 FileTime 追踪器
	ft, _ := ctx.Value(types.FileTimeKey).(*FileTime)

	// 使用沙箱解析路径
	path, err := t.resolvePathWithSandbox(ctx, in.FilePath, sb)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error(), ElapsedMs: time.Since(startTime).Milliseconds()}, nil
	}

	// P2.5: FileTime 检查 — 检测文件是否被外部修改
	if ft != nil {
		if err := ft.AssertNotModified(path); err != nil {
			return &types.ToolResult{
				Success:   false,
				Error:     err.Error(),
				ElapsedMs: time.Since(startTime).Milliseconds(),
			}, nil
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error(), ElapsedMs: time.Since(startTime).Milliseconds()}, nil
	}

	oldContent := string(content)
	var newContent string
	var matchMethod string

	if in.ReplaceAll {
		newContent = strings.ReplaceAll(oldContent, in.OldString, in.NewString)
		matchMethod = "exact"
	} else {
		match, method := t.replacerChain.FindBestMatch(oldContent, in.OldString)
		if match != nil {
			newContent = oldContent[:match.Start] + in.NewString + oldContent[match.End:]
			matchMethod = method
		} else {
			newContent = oldContent
		}
	}

	if oldContent == newContent {
		return &types.ToolResult{Success: false, Error: "old_string not found", ElapsedMs: time.Since(startTime).Milliseconds()}, nil
	}

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error(), ElapsedMs: time.Since(startTime).Milliseconds()}, nil
	}

	// P2.5: 编辑后更新 FileTime 记录
	if ft != nil {
		_ = ft.Update(path)
	}

	// P3: 生成 diff 摘要（参考 OpenCode edit.ts 的 diffLines 输出）
	output := t.buildEditOutput(in.FilePath, oldContent, newContent, matchMethod)

	return &types.ToolResult{Success: true, Output: output, ElapsedMs: time.Since(startTime).Milliseconds()}, nil
}

func (t *EditTool) resolvePath(ctx context.Context, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	workDir := types.GetWorkingDir(ctx, t.workingDir)
	return filepath.Join(workDir, path)
}

// resolvePathWithSandbox 使用沙箱解析路径
func (t *EditTool) resolvePathWithSandbox(ctx context.Context, path string, sb *sandbox.Sandbox) (string, error) {
	if sb != nil {
		return sb.ToReal(path)
	}
	return t.resolvePath(ctx, path), nil
}

// buildEditOutput 构建编辑结果输出（含 diff 摘要）
// 参考 OpenCode edit.ts: diffLines() 统计增删行数
func (t *EditTool) buildEditOutput(filePath, oldContent, newContent, matchMethod string) string {
	var out strings.Builder

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")
	added, removed := diffLineCount(oldLines, newLines)

	fmt.Fprintf(&out, "Edited %s", filePath)
	if added > 0 || removed > 0 {
		fmt.Fprintf(&out, " (+%d/-%d lines)", added, removed)
	}
	out.WriteString("\n")

	if matchMethod != "" && matchMethod != "SimpleReplacer" {
		fmt.Fprintf(&out, "Matched via: %s\n", matchMethod)
	}

	return out.String()
}
