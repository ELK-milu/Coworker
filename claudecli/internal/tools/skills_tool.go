package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/claudecli/internal/store"
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// skillEntry 统一的技能条目（来自 registry 或 store）
type skillEntry struct {
	Name        string
	Description string
	Content     string
}

// SkillsTool 技能工具 — 渐进式披露（Progressive Disclosure）
// 参考 OpenCode tool/skill.ts:
//   - Description() 动态列出 <available_skills>（仅 name + description）
//   - Execute() 按需加载完整 <skill_content>
type SkillsTool struct {
	store *store.Manager

	mu           sync.RWMutex
	userID       string       // 当前用户 ID
	cachedDesc   string       // 缓存的动态 description
	cachedHint   string       // 缓存的 name 示例（用于 InputSchema）
	cachedSkills []skillEntry // 缓存的可用技能列表
}

func NewSkillsTool(storeMgr *store.Manager) *SkillsTool {
	t := &SkillsTool{store: storeMgr}
	t.rebuildCache("")
	return t
}

// RefreshForUser 刷新当前用户的可用技能列表（每次对话前调用）
// 参考 OpenCode tool/skill.ts: Tool.define("skill", async (ctx) => { ... })
func (t *SkillsTool) RefreshForUser(userID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.userID = userID
	t.rebuildCache(userID)
}

func (t *SkillsTool) Name() string { return "Skills" }

// Description 动态返回包含 <available_skills> 的描述
// 参考 OpenCode tool/skill.ts:22-46
func (t *SkillsTool) Description() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.cachedDesc
}

func (t *SkillsTool) InputSchema() map[string]interface{} {
	t.mu.RLock()
	hint := t.cachedHint
	t.mu.RUnlock()

	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": fmt.Sprintf("The name of the skill from available_skills%s", hint),
			},
		},
		"required": []string{"name"},
	}
}

func (t *SkillsTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	startTime := time.Now()

	var in struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("failed to parse input: %v", err),
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	if in.Name == "" {
		return &types.ToolResult{
			Success:   false,
			Error:     "skill name is required",
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 查找技能（registry 优先，store 其次）
	entry := t.findSkill(in.Name)
	if entry == nil {
		available := t.listNames()
		return &types.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("Skill %q not found. Available skills: %s", in.Name, strings.Join(available, ", ")),
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 参考 OpenCode tool/skill.ts:99-115 — 输出 <skill_content> 块
	output := strings.Join([]string{
		fmt.Sprintf(`<skill_content name="%s">`, entry.Name),
		fmt.Sprintf("# Skill: %s", entry.Name),
		"",
		strings.TrimSpace(entry.Content),
		"</skill_content>",
	}, "\n")

	return &types.ToolResult{
		Success:   true,
		Output:    output,
		ElapsedMs: time.Since(startTime).Milliseconds(),
		Metadata:  map[string]interface{}{"skill_name": entry.Name},
	}, nil
}

// rebuildCache 重建缓存的 description 和技能列表（调用方需持有写锁）
func (t *SkillsTool) rebuildCache(userID string) {
	entries := t.collectSkills(userID)
	t.cachedSkills = entries

	if len(entries) == 0 {
		t.cachedDesc = "Load a specialized skill that provides domain-specific instructions and workflows. No skills are currently available."
		t.cachedHint = ""
		return
	}

	// 构建 <available_skills> XML — 参考 OpenCode tool/skill.ts:36-46
	var lines []string
	lines = append(lines,
		"Load a specialized skill that provides domain-specific instructions and workflows.",
		"",
		"When you recognize that a task matches one of the available skills listed below, use this tool to load the full skill instructions.",
		"",
		"The skill will inject detailed instructions, workflows, and access to bundled resources into the conversation context.",
		"",
		`Tool output includes a <skill_content name="..."> block with the loaded content.`,
		"",
		"Invoke this tool to load a skill when a task matches one of the available skills listed below:",
		"",
		"<available_skills>",
	)
	for _, e := range entries {
		lines = append(lines,
			"  <skill>",
			fmt.Sprintf("    <name>%s</name>", e.Name),
			fmt.Sprintf("    <description>%s</description>", e.Description),
			"  </skill>",
		)
	}
	lines = append(lines, "</available_skills>")
	t.cachedDesc = strings.Join(lines, "\n")

	// 构建 hint（最多 3 个示例）
	examples := make([]string, 0, 3)
	for i, e := range entries {
		if i >= 3 {
			break
		}
		examples = append(examples, fmt.Sprintf("'%s'", e.Name))
	}
	t.cachedHint = fmt.Sprintf(" (e.g., %s, ...)", strings.Join(examples, ", "))
}

// collectSkills 收集用户已安装的 store 技能
func (t *SkillsTool) collectSkills(userID string) []skillEntry {
	if t.store == nil || userID == "" {
		return nil
	}
	var entries []skillEntry
	seen := make(map[string]bool)
	for _, id := range t.store.LoadUserInstalled(userID) {
		item := t.store.GetByID(id)
		if item == nil || item.Type != store.TypeSkill || seen[item.Name] {
			continue
		}
		entries = append(entries, skillEntry{Name: item.Name, Description: item.Description, Content: item.Content})
		seen[item.Name] = true
	}
	return entries
}

// findSkill 从 store 查找技能
func (t *SkillsTool) findSkill(name string) *skillEntry {
	if t.store == nil {
		return nil
	}
	t.mu.RLock()
	userID := t.userID
	t.mu.RUnlock()

	if userID == "" {
		return nil
	}
	for _, id := range t.store.LoadUserInstalled(userID) {
		item := t.store.GetByID(id)
		if item != nil && item.Type == store.TypeSkill && item.Name == name && item.Content != "" {
			return &skillEntry{Name: item.Name, Description: item.Description, Content: item.Content}
		}
	}
	return nil
}

// listNames 列出所有可用技能名称
func (t *SkillsTool) listNames() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	names := make([]string, len(t.cachedSkills))
	for i, e := range t.cachedSkills {
		names[i] = e.Name
	}
	return names
}
