package tools

import (
	"github.com/QuantumNous/new-api/claudecli/internal/sandbox"
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
	Pattern string   `json:"pattern"`
	Path    string   `json:"path,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

// 默认排除模式
var defaultExclude = []string{
	"node_modules",
	".git",
	"vendor",
	"__pycache__",
	".venv",
	"dist",
	"build",
	".next",
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
			"exclude": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "string"},
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *GlobTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	var in GlobInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	// 获取沙箱
	sb, _ := ctx.Value(types.SandboxKey).(*sandbox.Sandbox)

	workDir := types.GetWorkingDir(ctx, t.workingDir)

	var pattern string
	if filepath.IsAbs(in.Pattern) {
		// 绝对路径需要通过沙箱验证
		if sb != nil {
			realPath, err := sb.ToReal(in.Pattern)
			if err != nil {
				return &types.ToolResult{Success: false, Error: err.Error()}, nil
			}
			pattern = realPath
		} else {
			pattern = in.Pattern
		}
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

	// 合并默认排除和用户指定的排除
	excludePatterns := append(defaultExclude, in.Exclude...)

	// 过滤排除的路径
	filtered := filterExcluded(matches, excludePatterns)

	// 虚拟化输出路径
	if sb != nil {
		filtered = sb.VirtualizePaths(filtered)
	}

	return &types.ToolResult{
		Success: true,
		Output:  strings.Join(filtered, "\n"),
	}, nil
}

// filterExcluded 过滤排除的路径
func filterExcluded(paths []string, excludePatterns []string) []string {
	var result []string
	for _, path := range paths {
		if !shouldExclude(path, excludePatterns) {
			result = append(result, path)
		}
	}
	return result
}

// shouldExclude 检查路径是否应该被排除
func shouldExclude(path string, excludePatterns []string) bool {
	// 将路径分割为各个部分
	parts := strings.Split(filepath.ToSlash(path), "/")

	for _, pattern := range excludePatterns {
		// 检查路径的任何部分是否匹配排除模式
		for _, part := range parts {
			if part == pattern {
				return true
			}
			// 支持简单的通配符匹配
			if strings.HasSuffix(pattern, "*") {
				prefix := strings.TrimSuffix(pattern, "*")
				if strings.HasPrefix(part, prefix) {
					return true
				}
			}
		}
	}
	return false
}
