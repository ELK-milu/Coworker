package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/claudecli/internal/skills"
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// SkillsTool 技能查询工具
type SkillsTool struct {
	registry *skills.Registry
}

// SkillsInput 输入参数
type SkillsInput struct {
	Action  string `json:"action"`  // list, load, search
	Name    string `json:"name"`    // 技能名称（load 时使用）
	Keyword string `json:"keyword"` // 搜索关键词（search 时使用）
}

func NewSkillsTool(registry *skills.Registry) *SkillsTool {
	return &SkillsTool{registry: registry}
}

func (t *SkillsTool) Name() string { return "Skills" }

func (t *SkillsTool) Description() string {
	return `Query and load domain knowledge skills.

Actions:
- list: List all available skills
- load: Load a specific skill's content by name
- search: Search skills by keyword in name/description

Use this to access specialized knowledge when needed.`
}

func (t *SkillsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"list", "load", "search"},
				"description": "Action to perform",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Skill name (for load action)",
			},
			"keyword": map[string]interface{}{
				"type":        "string",
				"description": "Search keyword (for search action)",
			},
		},
		"required": []string{"action"},
	}
}

func (t *SkillsTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	startTime := time.Now()

	var in SkillsInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("failed to parse input: %v", err),
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	if t.registry == nil {
		return &types.ToolResult{
			Success:   false,
			Error:     "skills registry not initialized",
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	switch in.Action {
	case "list":
		return t.handleList(startTime)
	case "load":
		return t.handleLoad(in.Name, startTime)
	case "search":
		return t.handleSearch(in.Keyword, startTime)
	default:
		return &types.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("unknown action: %s", in.Action),
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}
}

func (t *SkillsTool) handleList(startTime time.Time) (*types.ToolResult, error) {
	skillList := t.registry.GetAll()
	var output string
	if len(skillList) == 0 {
		output = "No skills available."
	} else {
		output = fmt.Sprintf("Available skills (%d):\n", len(skillList))
		for _, s := range skillList {
			output += fmt.Sprintf("- %s: %s\n", s.Name, s.Description)
		}
	}

	return &types.ToolResult{
		Success:   true,
		Output:    output,
		ElapsedMs: time.Since(startTime).Milliseconds(),
	}, nil
}

func (t *SkillsTool) handleLoad(name string, startTime time.Time) (*types.ToolResult, error) {
	if name == "" {
		return &types.ToolResult{
			Success:   false,
			Error:     "skill name is required for load action",
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	skill, ok := t.registry.Get(name)
	if !ok {
		return &types.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("skill not found: %s", name),
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	return &types.ToolResult{
		Success:   true,
		Output:    skill.Content,
		ElapsedMs: time.Since(startTime).Milliseconds(),
		Metadata: map[string]interface{}{
			"skill_name": name,
		},
	}, nil
}

func (t *SkillsTool) handleSearch(keyword string, startTime time.Time) (*types.ToolResult, error) {
	if keyword == "" {
		return &types.ToolResult{
			Success:   false,
			Error:     "keyword is required for search action",
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	keyword = strings.ToLower(keyword)
	skillList := t.registry.GetAll()
	var matches []*skills.Skill

	for _, s := range skillList {
		if strings.Contains(strings.ToLower(s.Name), keyword) ||
			strings.Contains(strings.ToLower(s.Description), keyword) {
			matches = append(matches, s)
		}
	}

	var output string
	if len(matches) == 0 {
		output = fmt.Sprintf("No skills found matching '%s'", keyword)
	} else {
		output = fmt.Sprintf("Skills matching '%s' (%d):\n", keyword, len(matches))
		for _, s := range matches {
			output += fmt.Sprintf("- %s: %s\n", s.Name, s.Description)
		}
	}

	return &types.ToolResult{
		Success:   true,
		Output:    output,
		ElapsedMs: time.Since(startTime).Milliseconds(),
	}, nil
}
