package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/coworker/internal/sandbox"
	"github.com/QuantumNous/new-api/coworker/pkg/types"
)

const (
	LSMaxEntries = 200 // 最大返回条目数
)

// LSTool 目录列表工具（沙箱隔离）
// 参考 Claude Code LS tool: 列出目录内容
type LSTool struct {
	workingDir string
}

type LSInput struct {
	Path string `json:"path,omitempty"` // 要列出的目录路径，默认为工作目录
}

func NewLSTool(workingDir string) *LSTool {
	return &LSTool{workingDir: workingDir}
}

func (t *LSTool) Name() string { return "LS" }

func (t *LSTool) Description() string {
	return `List directory contents. Returns file/directory names with type indicators and sizes.
Use this to explore directory structure and discover files.`
}

func (t *LSTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Directory path to list. Defaults to the current working directory if not specified.",
			},
		},
		"required": []string{},
	}
}

func (t *LSTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	startTime := time.Now()

	var in LSInput
	json.Unmarshal(input, &in)

	// 通过沙箱解析路径
	sb, _ := ctx.Value(types.SandboxKey).(*sandbox.Sandbox)
	if sb == nil {
		return &types.ToolResult{
			Success:   false,
			Error:     "sandbox not available",
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	targetPath := in.Path
	if targetPath == "" {
		targetPath = "."
	}

	realPath, err := sb.ToReal(targetPath)
	if err != nil {
		return &types.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("path error: %v", err),
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	info, err := os.Stat(realPath)
	if err != nil {
		return &types.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("cannot access %s: %v", targetPath, err),
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}
	if !info.IsDir() {
		return &types.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("%s is not a directory", targetPath),
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	entries, err := os.ReadDir(realPath)
	if err != nil {
		return &types.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("cannot read directory %s: %v", targetPath, err),
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 按目录优先、然后字母排序
	sort.Slice(entries, func(i, j int) bool {
		iDir := entries[i].IsDir()
		jDir := entries[j].IsDir()
		if iDir != jDir {
			return iDir // 目录排在前面
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	var lines []string
	count := 0
	for _, entry := range entries {
		if count >= LSMaxEntries {
			lines = append(lines, fmt.Sprintf("... (%d more entries truncated)", len(entries)-LSMaxEntries))
			break
		}

		name := entry.Name()
		if entry.IsDir() {
			lines = append(lines, fmt.Sprintf("%s/", name))
		} else {
			fi, err := entry.Info()
			if err != nil {
				lines = append(lines, name)
			} else {
				lines = append(lines, fmt.Sprintf("%s  (%s)", name, formatSize(fi.Size())))
			}
		}
		count++
	}

	if len(lines) == 0 {
		lines = append(lines, "(empty directory)")
	}

	// 虚拟化显示路径
	displayPath := sb.ToVirtual(realPath)
	header := fmt.Sprintf("%s  (%d entries)", displayPath, len(entries))

	output := header + "\n" + strings.Join(lines, "\n")

	return &types.ToolResult{
		Success:   true,
		Output:    output,
		ElapsedMs: time.Since(startTime).Milliseconds(),
		Metadata: map[string]interface{}{
			"path":        displayPath,
			"entry_count": len(entries),
		},
	}, nil
}

func formatSize(size int64) string {
	switch {
	case size >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(size)/float64(1<<30))
	case size >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(1<<20))
	case size >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(size)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", size)
	}
}
