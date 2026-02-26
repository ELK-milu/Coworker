package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/coworker/internal/store"
	"github.com/QuantumNous/new-api/coworker/pkg/types"
)

// skillEntry 统一的技能条目（来自 registry 或 store）
type skillEntry struct {
	Name        string
	Description string
	Content     string
	LocalDir    string // 虚拟目录名（用于 /.skill/ 路径）
	realDir     string // 实际磁盘路径（用于文件列表扫描）
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
	var lines []string
	lines = append(lines,
		fmt.Sprintf(`<skill_content name="%s">`, entry.Name),
		fmt.Sprintf("# Skill: %s", entry.Name),
		"",
		strings.TrimSpace(entry.Content),
	)

	// 如果有本地目录，添加 base directory 和文件列表
	if entry.LocalDir != "" {
		skillBasePath := "/.skill/" + entry.LocalDir + "/"
		lines = append(lines,
			"",
			fmt.Sprintf("Base directory for this skill: %s", skillBasePath),
			"Relative paths in this skill (e.g., scripts/, reference/) are relative to this base directory.",
		)
		// 扫描 store 数据目录中的 skill 文件
		if entry.realDir != "" {
			files := listSkillFiles(entry.realDir, 10)
			if len(files) > 0 {
				lines = append(lines, "", "<skill_files>")
				for _, f := range files {
					virtualPath := skillBasePath + f
					lines = append(lines, fmt.Sprintf("<file>%s</file>", virtualPath))
				}
				lines = append(lines, "</skill_files>")
			}
		}
	}

	lines = append(lines, "</skill_content>")
	output := strings.Join(lines, "\n")

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

// resolveSubItemDir 在 store/plugins 目录中查找 sub_item 的实际目录
// 尝试三种候选路径（对应三种插件磁盘布局）：
//  1. {plugin}/{sub.Name}/SKILL.md         — 模式 A（扁平）
//  2. {plugin}/{sub.LocalDir}/SKILL.md     — 模式 B（skills/ 子目录）
//  3. {plugin}/SKILL.md                    — 模式 C（根目录即技能）
func (t *SkillsTool) resolveSubItemDir(pluginDirName string, sub store.SubItem) string {
	pluginsBase := filepath.Join(t.store.DataDir(), "plugins")
	candidates := []string{
		pluginDirName + "/" + sub.Name,
	}
	if sub.LocalDir != "" && sub.LocalDir != sub.Name {
		candidates = append(candidates, pluginDirName+"/"+sub.LocalDir)
	}
	for _, c := range candidates {
		for _, fname := range []string{"SKILL.md", "skill.md"} {
			if _, err := os.Stat(filepath.Join(pluginsBase, c, fname)); err == nil {
				return c
			}
		}
	}
	// 模式 C 回退：插件根目录即技能
	for _, fname := range []string{"SKILL.md", "skill.md"} {
		if _, err := os.Stat(filepath.Join(pluginsBase, pluginDirName, fname)); err == nil {
			return pluginDirName
		}
	}
	return pluginDirName + "/" + sub.Name // 默认
}

// collectSkills 收集用户已安装的 store 技能
func (t *SkillsTool) collectSkills(userID string) []skillEntry {
	if t.store == nil || userID == "" {
		return nil
	}
	var entries []skillEntry
	seen := make(map[string]bool)
	dataDir := t.store.DataDir()
	for _, id := range t.store.LoadUserInstalled(userID) {
		item := t.store.GetByID(id)
		if item == nil {
			continue
		}
		if item.Type == store.TypePlugin {
			pluginDirName := item.LocalDir
			if pluginDirName == "" {
				pluginDirName = item.Name
			}
			for _, sub := range item.SubItems {
				if (sub.Type == store.SubTypeSkill || sub.Type == store.SubTypeCommand) && !seen[sub.Name] {
					localDir := t.resolveSubItemDir(pluginDirName, sub)
					entries = append(entries, skillEntry{
						Name:        sub.Name,
						Description: sub.Description,
						Content:     sub.Content,
						LocalDir:    localDir,
						realDir:     filepath.Join(dataDir, "plugins", localDir),
					})
					seen[sub.Name] = true
				}
			}
			continue
		}
		if item.Type != store.TypeSkill || seen[item.Name] {
			continue
		}
		entries = append(entries, skillEntry{
			Name:        item.Name,
			Description: item.Description,
			Content:     item.Content,
			LocalDir:    item.LocalDir,
			realDir:     filepath.Join(dataDir, "skills", item.LocalDir),
		})
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
	dataDir := t.store.DataDir()
	for _, id := range t.store.LoadUserInstalled(userID) {
		item := t.store.GetByID(id)
		if item == nil {
			continue
		}
		if item.Type == store.TypePlugin {
			pluginDirName := item.LocalDir
			if pluginDirName == "" {
				pluginDirName = item.Name
			}
			for _, sub := range item.SubItems {
				if (sub.Type == store.SubTypeSkill || sub.Type == store.SubTypeCommand) && sub.Name == name && sub.Content != "" {
					localDir := t.resolveSubItemDir(pluginDirName, sub)
					return &skillEntry{
						Name:        sub.Name,
						Description: sub.Description,
						Content:     sub.Content,
						LocalDir:    localDir,
						realDir:     filepath.Join(dataDir, "plugins", localDir),
					}
				}
			}
			continue
		}
		if item.Type == store.TypeSkill && item.Name == name && item.Content != "" {
			return &skillEntry{
				Name:        item.Name,
				Description: item.Description,
				Content:     item.Content,
				LocalDir:    item.LocalDir,
				realDir:     filepath.Join(dataDir, "skills", item.LocalDir),
			}
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

// listSkillFiles 递归列出目录下的文件（排除 SKILL.md 和非技能资源），最多 limit 个
func listSkillFiles(dir string, limit int) []string {
	excludeDirs := map[string]bool{
		".claude-plugin": true, ".git": true, ".github": true,
		".cursor-plugin": true, ".codex": true, ".opencode": true,
		"node_modules": true,
	}
	excludeFiles := map[string]bool{
		"skill.md": true, "readme.md": true, "claude.md": true,
		"license": true, "license.txt": true, "license.md": true,
	}
	var files []string
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if excludeDirs[strings.ToLower(d.Name())] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		lower := strings.ToLower(name)
		if excludeFiles[lower] {
			return nil
		}
		// 跳过压缩包等大文件
		if strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		if len(files) >= limit {
			return filepath.SkipAll
		}
		return nil
	})
	return files
}
