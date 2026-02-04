package tools

import (
	"bufio"
	"github.com/QuantumNous/new-api/claudecli/internal/sandbox"
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	var in GrepInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	re, err := regexp.Compile(in.Pattern)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	// 获取沙箱
	sb, _ := ctx.Value(types.SandboxKey).(*sandbox.Sandbox)

	workDir := types.GetWorkingDir(ctx, t.workingDir)
	searchPath := workDir
	if in.Path != "" {
		searchPath = filepath.Join(workDir, in.Path)
	}

	var results []string
	err = filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if in.Glob != "" {
			matched, _ := filepath.Match(in.Glob, info.Name())
			if !matched {
				return nil
			}
		}
		matches := t.searchFile(path, re, sb)
		results = append(results, matches...)
		return nil
	})

	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	return &types.ToolResult{Success: true, Output: strings.Join(results, "\n")}, nil
}

func (t *GrepTool) searchFile(path string, re *regexp.Regexp, sb *sandbox.Sandbox) []string {
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

	var results []string
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			results = append(results, fmt.Sprintf("%s:%d:%s", displayPath, lineNum, line))
		}
	}
	return results
}
