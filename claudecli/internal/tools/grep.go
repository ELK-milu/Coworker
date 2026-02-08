package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/claudecli/internal/sandbox"
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// 参考 OpenCode grep.ts: 结果限制 + 行长截断 + 按文件分组
const (
	GrepMaxResults   = 100  // 最大匹配结果数
	GrepMaxLineChars = 2000 // 单行最大字符数
)

// GrepTool 内容搜索工具
type GrepTool struct {
	workingDir string
}

type GrepInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
	Glob    string `json:"glob,omitempty"`
}

// grepMatch 单条匹配结果
type grepMatch struct {
	FilePath string
	LineNum  int
	LineText string
}

func NewGrepTool(workingDir string) *GrepTool {
	return &GrepTool{workingDir: workingDir}
}

func (t *GrepTool) Name() string { return "Grep" }

func (t *GrepTool) Description() string {
	return "Search file contents using regex."
}

func (t *GrepTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{"type": "string"},
			"path":    map[string]interface{}{"type": "string"},
			"glob":    map[string]interface{}{"type": "string"},
		},
		"required": []string{"pattern"},
	}
}

func (t *GrepTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	startTime := time.Now()

	var in GrepInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error(), ElapsedMs: time.Since(startTime).Milliseconds()}, nil
	}

	re, err := regexp.Compile(in.Pattern)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error(), ElapsedMs: time.Since(startTime).Milliseconds()}, nil
	}

	// 获取沙箱
	sb, _ := ctx.Value(types.SandboxKey).(*sandbox.Sandbox)

	workDir := types.GetWorkingDir(ctx, t.workingDir)
	searchPath := workDir
	if in.Path != "" {
		searchPath = filepath.Join(workDir, in.Path)
	}

	// P3: 收集匹配结果（带结果限制），参考 OpenCode grep.ts
	var allMatches []grepMatch
	totalMatches := 0
	limitReached := false

	_ = filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil // 遇错继续，参考 OpenCode
		}
		if in.Glob != "" {
			matched, _ := filepath.Match(in.Glob, info.Name())
			if !matched {
				return nil
			}
		}
		matches := t.searchFile(path, re, sb)
		for _, m := range matches {
			totalMatches++
			if !limitReached {
				allMatches = append(allMatches, m)
				if len(allMatches) >= GrepMaxResults {
					limitReached = true
				}
			}
		}
		return nil
	})

	if len(allMatches) == 0 {
		return &types.ToolResult{
			Success:   true,
			Output:    "No matches found.",
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	// P3: 按文件分组输出，参考 OpenCode grep.ts
	output := t.formatGroupedOutput(allMatches, totalMatches, limitReached)

	return &types.ToolResult{
		Success:   true,
		Output:    output,
		ElapsedMs: time.Since(startTime).Milliseconds(),
	}, nil
}

func (t *GrepTool) searchFile(path string, re *regexp.Regexp, sb *sandbox.Sandbox) []grepMatch {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	// 虚拟化路径
	displayPath := path
	if sb != nil {
		displayPath = sb.ToVirtual(path)
	}

	var results []grepMatch
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			// P3: 单行截断（参考 OpenCode grep.ts: 2000 字符限制）
			if len(line) > GrepMaxLineChars {
				line = line[:GrepMaxLineChars] + "..."
			}
			results = append(results, grepMatch{
				FilePath: displayPath,
				LineNum:  lineNum,
				LineText: line,
			})
		}
	}
	return results
}

// formatGroupedOutput 按文件分组格式化输出
// 参考 OpenCode grep.ts: 按文件分组 + 缩进行号
func (t *GrepTool) formatGroupedOutput(matches []grepMatch, totalMatches int, truncated bool) string {
	var out strings.Builder
	currentFile := ""

	for _, m := range matches {
		if m.FilePath != currentFile {
			if currentFile != "" {
				out.WriteString("\n")
			}
			fmt.Fprintf(&out, "%s:\n", m.FilePath)
			currentFile = m.FilePath
		}
		fmt.Fprintf(&out, "  %d: %s\n", m.LineNum, m.LineText)
	}

	if truncated {
		fmt.Fprintf(&out, "\n(%d results shown of %d total matches. Use a more specific pattern or path to narrow results.)",
			len(matches), totalMatches)
	}

	return out.String()
}
