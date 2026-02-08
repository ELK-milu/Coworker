package tools

import (
	"context"
	"encoding/json"
	"fmt"
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
	FilePath   string      `json:"file_path"`
	OldString  string      `json:"old_string"`
	NewString  string      `json:"new_string"`
	ReplaceAll bool        `json:"replace_all,omitempty"`
	Edits      []EditPair  `json:"edits,omitempty"` // 批量替换
}

// EditPair 单个替换对
type EditPair struct {
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
	return `Edit file by replacing old_string with new_string. Supports batch mode: pass an "edits" array with multiple {old_string, new_string} pairs to apply several replacements in a single call.`
}

func (t *EditTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path":  map[string]interface{}{"type": "string", "description": "The file to edit"},
			"old_string": map[string]interface{}{"type": "string", "description": "The text to replace (single edit mode)"},
			"new_string": map[string]interface{}{"type": "string", "description": "The replacement text (single edit mode)"},
			"replace_all": map[string]interface{}{"type": "boolean", "description": "Replace all occurrences (default false)"},
			"edits": map[string]interface{}{
				"type":        "array",
				"description": "Batch mode: array of {old_string, new_string, replace_all} pairs. When provided, old_string/new_string at top level are ignored.",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"old_string":  map[string]interface{}{"type": "string"},
						"new_string":  map[string]interface{}{"type": "string"},
						"replace_all": map[string]interface{}{"type": "boolean"},
					},
					"required": []string{"old_string", "new_string"},
				},
			},
		},
		"required": []string{"file_path"},
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

	// 构建替换对列表（批量模式 or 单次模式）
	pairs := t.buildEditPairs(&in)
	if len(pairs) == 0 {
		return &types.ToolResult{Success: false, Error: "No edit pairs provided. Supply old_string/new_string or edits array.", ElapsedMs: time.Since(startTime).Milliseconds()}, nil
	}

	// 逐个应用替换
	currentContent := oldContent
	var results []string
	failCount := 0

	for i, pair := range pairs {
		before := currentContent
		var matchMethod string

		if pair.ReplaceAll {
			currentContent = strings.ReplaceAll(currentContent, pair.OldString, pair.NewString)
			matchMethod = "exact"
		} else {
			match, method := t.replacerChain.FindBestMatch(currentContent, pair.OldString)
			if match != nil {
				currentContent = currentContent[:match.Start] + pair.NewString + currentContent[match.End:]
				matchMethod = method
			}
		}

		if before == currentContent {
			failCount++
			results = append(results, fmt.Sprintf("Edit %d/%d: old_string not found", i+1, len(pairs)))
		} else {
			added, removed := diffLineCount(strings.Split(before, "\n"), strings.Split(currentContent, "\n"))
			msg := fmt.Sprintf("Edit %d/%d: +%d/-%d lines", i+1, len(pairs), added, removed)
			if matchMethod != "" && matchMethod != "SimpleReplacer" {
				msg += fmt.Sprintf(" (matched via: %s)", matchMethod)
			}
			results = append(results, msg)
		}
	}

	// 全部失败
	if failCount == len(pairs) {
		return &types.ToolResult{Success: false, Error: "All edits failed: old_string not found", ElapsedMs: time.Since(startTime).Milliseconds()}, nil
	}

	// 无实际变更
	if oldContent == currentContent {
		return &types.ToolResult{Success: false, Error: "old_string not found", ElapsedMs: time.Since(startTime).Milliseconds()}, nil
	}

	if err := os.WriteFile(path, []byte(currentContent), 0644); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error(), ElapsedMs: time.Since(startTime).Milliseconds()}, nil
	}

	// P2.5: 编辑后更新 FileTime 记录
	if ft != nil {
		_ = ft.Update(path)
	}

	// 生成输出摘要
	output := t.buildBatchEditOutput(in.FilePath, oldContent, currentContent, results)

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

// buildEditPairs 构建替换对列表（批量模式优先，否则单次模式）
func (t *EditTool) buildEditPairs(in *EditInput) []EditPair {
	// 批量模式：edits 数组优先
	if len(in.Edits) > 0 {
		return in.Edits
	}
	// 单次模式：old_string + new_string
	if in.OldString != "" {
		return []EditPair{{
			OldString:  in.OldString,
			NewString:  in.NewString,
			ReplaceAll: in.ReplaceAll,
		}}
	}
	return nil
}

// buildBatchEditOutput 构建批量编辑结果输出
func (t *EditTool) buildBatchEditOutput(filePath, oldContent, newContent string, editResults []string) string {
	var out strings.Builder

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")
	added, removed := diffLineCount(oldLines, newLines)

	fmt.Fprintf(&out, "Edited %s", filePath)
	if added > 0 || removed > 0 {
		fmt.Fprintf(&out, " (+%d/-%d lines)", added, removed)
	}
	out.WriteString("\n")

	// 批量模式：逐条输出每个替换的结果
	if len(editResults) > 1 {
		for _, r := range editResults {
			fmt.Fprintf(&out, "  %s\n", r)
		}
	} else if len(editResults) == 1 {
		// 单次模式：如果有非精确匹配信息，附加显示
		if strings.Contains(editResults[0], "matched via") {
			fmt.Fprintf(&out, "  %s\n", editResults[0])
		}
	}

	return out.String()
}
