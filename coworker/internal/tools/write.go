package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/coworker/internal/sandbox"
	"github.com/QuantumNous/new-api/coworker/pkg/types"
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
	startTime := time.Now()

	var in WriteInput
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
	log.Printf("[Write] Input path: %s, Resolved path: %s", in.FilePath, path)

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

	// 读取旧内容（用于生成 diff）
	var oldContent string
	isNewFile := false
	if oldBytes, err := os.ReadFile(path); err == nil {
		oldContent = string(oldBytes)
	} else {
		isNewFile = true
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error(), ElapsedMs: time.Since(startTime).Milliseconds()}, nil
	}

	if err := os.WriteFile(path, []byte(in.Content), 0644); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error(), ElapsedMs: time.Since(startTime).Milliseconds()}, nil
	}

	// P2.5: 写入后更新 FileTime 记录
	if ft != nil {
		_ = ft.Update(path)
	}

	// 生成输出信息（含 diff 摘要）
	outputPath := in.FilePath
	output := t.buildOutput(outputPath, oldContent, in.Content, isNewFile)

	return &types.ToolResult{
		Success:   true,
		Output:    output,
		ElapsedMs: time.Since(startTime).Milliseconds(),
	}, nil
}

func (t *WriteTool) resolvePath(ctx context.Context, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	workDir := types.GetWorkingDir(ctx, t.workingDir)
	log.Printf("[Write] resolvePath: defaultWorkDir=%s, contextWorkDir=%s", t.workingDir, workDir)
	return filepath.Join(workDir, path)
}

// resolvePathWithSandbox 使用沙箱解析路径
func (t *WriteTool) resolvePathWithSandbox(ctx context.Context, path string, sb *sandbox.Sandbox) (string, error) {
	if sb != nil {
		return sb.ToReal(path)
	}
	return t.resolvePath(ctx, path), nil
}

// buildOutput 构建写入结果输出（含 diff 摘要）
func (t *WriteTool) buildOutput(filePath, oldContent, newContent string, isNewFile bool) string {
	var out strings.Builder

	if isNewFile {
		newLines := strings.Count(newContent, "\n") + 1
		fmt.Fprintf(&out, "Created new file %s (%d lines)\n", filePath, newLines)
		return out.String()
	}

	// 计算 diff 摘要
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	added, removed := diffLineCount(oldLines, newLines)

	fmt.Fprintf(&out, "File written: %s\n", filePath)
	fmt.Fprintf(&out, "Changes: %d lines added, %d lines removed (total: %d → %d lines)\n",
		added, removed, len(oldLines), len(newLines))

	return out.String()
}

// diffLineCount 计算新增和删除的行数（简单 LCS 近似）
func diffLineCount(oldLines, newLines []string) (added, removed int) {
	oldSet := make(map[string]int)
	for _, line := range oldLines {
		oldSet[line]++
	}

	newSet := make(map[string]int)
	for _, line := range newLines {
		newSet[line]++
	}

	// 计算删除的行
	for line, count := range oldSet {
		newCount := newSet[line]
		if newCount < count {
			removed += count - newCount
		}
	}

	// 计算新增的行
	for line, count := range newSet {
		oldCount := oldSet[line]
		if oldCount < count {
			added += count - oldCount
		}
	}

	return added, removed
}
