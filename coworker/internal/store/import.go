package store

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ImportFromGithub 从 GitHub 仓库导入 skills/agents
// storeDir: 技能文件全局存储目录（store/skills/），空字符串表示不下载文件
func ImportFromGithub(repoURL string, storeDir string) ([]StoreItem, error) {
	owner, repo := parseRepo(repoURL)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid repo: %s", repoURL)
	}

	ghURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)

	// 1. 尝试 .claude-plugin/plugin.json
	if items, err := tryPluginJSON(owner, repo, ghURL, storeDir); err == nil && len(items) > 0 {
		return items, nil
	}

	// 2. 尝试 .claude-plugin/marketplace.json
	if items, err := tryMarketplaceJSON(owner, repo, ghURL, storeDir); err == nil && len(items) > 0 {
		return items, nil
	}

	// 3. 尝试根目录 SKILL.md / skill.md
	if items, err := tryRootSkill(owner, repo, ghURL, storeDir); err == nil && len(items) > 0 {
		return items, nil
	}

	return nil, fmt.Errorf("no plugin/skill found in %s/%s", owner, repo)
}

func parseRepo(input string) (string, string) {
	input = strings.TrimSpace(input)
	input = strings.TrimSuffix(input, "/")
	input = strings.TrimSuffix(input, ".git")

	// 完整 URL
	if strings.Contains(input, "github.com") {
		parts := strings.Split(input, "github.com/")
		if len(parts) == 2 {
			input = parts[1]
		}
	}
	// https:// 前缀清理
	input = strings.TrimPrefix(input, "https://")
	input = strings.TrimPrefix(input, "http://")

	parts := strings.SplitN(input, "/", 3)
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

// ghAPIGet 调用 GitHub API 获取内容
func ghAPIGet(path string) ([]byte, error) {
	url := "https://api.github.com/repos/" + path
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Coworker-Store")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API %d: %s", resp.StatusCode, path)
	}
	return io.ReadAll(resp.Body)
}

// ghRawGet 获取 raw 文件内容
func ghRawGet(owner, repo, path string) (string, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/HEAD/%s", owner, repo, path)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("not found: %s", path)
	}
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

func tryPluginJSON(owner, repo, ghURL, storeDir string) ([]StoreItem, error) {
	raw, err := ghRawGet(owner, repo, ".claude-plugin/plugin.json")
	if err != nil {
		return nil, err
	}
	var plugin PluginJSON
	if err := json.Unmarshal([]byte(raw), &plugin); err != nil {
		return nil, err
	}

	return scanSkillsDir(owner, repo, "skills", plugin.Author, ghURL, storeDir)
}

func tryMarketplaceJSON(owner, repo, ghURL, storeDir string) ([]StoreItem, error) {
	raw, err := ghRawGet(owner, repo, ".claude-plugin/marketplace.json")
	if err != nil {
		return nil, err
	}
	var mp MarketplaceJSON
	if err := json.Unmarshal([]byte(raw), &mp); err != nil {
		return nil, err
	}

	var allItems []StoreItem
	for _, p := range mp.Plugins {
		localPath, ok := p.Source.(string)
		if !ok {
			continue
		}
		localPath = strings.TrimPrefix(localPath, "./")
		skillsPath := localPath + "/skills"
		items, err := scanSkillsDir(owner, repo, skillsPath, p.Name, ghURL, storeDir)
		if err != nil {
			continue
		}
		allItems = append(allItems, items...)
	}
	if len(allItems) == 0 {
		return nil, fmt.Errorf("no skills found in marketplace plugins")
	}
	return allItems, nil
}

func tryRootSkill(owner, repo, ghURL, storeDir string) ([]StoreItem, error) {
	for _, name := range []string{"SKILL.md", "skill.md"} {
		content, err := ghRawGet(owner, repo, name)
		if err != nil {
			continue
		}
		item := parseSkillMD(content, repo, ghURL)
		// 下载整个仓库根目录到 store/skills/{name}/
		if storeDir != "" {
			localDir := sanitizeDirName(item.Name)
			destDir := filepath.Join(storeDir, localDir)
			if err := downloadSkillDir(owner, repo, "", destDir); err == nil {
				item.LocalDir = localDir
			}
		}
		return []StoreItem{item}, nil
	}
	return nil, fmt.Errorf("no root skill.md")
}

// scanSkillsDir 扫描 skills/ 目录下的子目录
func scanSkillsDir(owner, repo, dirPath, author, ghURL, storeDir string) ([]StoreItem, error) {
	apiPath := fmt.Sprintf("%s/%s/contents/%s", owner, repo, dirPath)
	data, err := ghAPIGet(apiPath)
	if err != nil {
		return nil, err
	}

	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}

	var items []StoreItem
	for _, e := range entries {
		if e.Type != "dir" {
			continue
		}
		// 尝试 SKILL.md 或 skill.md
		for _, fname := range []string{"SKILL.md", "skill.md"} {
			content, err := ghRawGet(owner, repo, dirPath+"/"+e.Name+"/"+fname)
			if err != nil {
				continue
			}
			item := parseSkillMD(content, e.Name, ghURL)
			if author != "" {
				item.Author = author
			}
			// 下载整个 skill 子目录到 store/skills/{name}/
			if storeDir != "" {
				localDir := sanitizeDirName(item.Name)
				destDir := filepath.Join(storeDir, localDir)
				remotePath := dirPath + "/" + e.Name
				if err := downloadSkillDir(owner, repo, remotePath, destDir); err == nil {
					item.LocalDir = localDir
				}
			}
			items = append(items, item)
			break
		}
	}
	return items, nil
}

// parseSkillMD 解析 SKILL.md 的 YAML frontmatter
func parseSkillMD(content, fallbackName, ghURL string) StoreItem {
	item := StoreItem{
		Type:      TypeSkill,
		GithubURL: ghURL,
		Content:   content,
	}

	// 解析 YAML frontmatter (--- ... ---)
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content[3:], "---", 2)
		if len(parts) == 2 {
			fm := parts[0]
			item.Content = strings.TrimSpace(parts[1])
			item.Name = extractYAMLField(fm, "name")
			item.Description = extractYAMLField(fm, "description")
		}
	}

	if item.Name == "" {
		item.Name = fallbackName
	}
	if item.Description == "" {
		item.Description = "Imported from GitHub"
	}
	return item
}

func extractYAMLField(yaml, field string) string {
	for _, line := range strings.Split(yaml, "\n") {
		line = strings.TrimSpace(line)
		prefix := field + ":"
		if strings.HasPrefix(line, prefix) {
			val := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			val = strings.Trim(val, "\"'")
			return val
		}
	}
	return ""
}

// downloadSkillDir 递归下载 GitHub 目录到本地
func downloadSkillDir(owner, repo, remotePath, localDir string) error {
	apiPath := fmt.Sprintf("%s/%s/contents", owner, repo)
	if remotePath != "" {
		apiPath += "/" + remotePath
	}

	data, err := ghAPIGet(apiPath)
	if err != nil {
		return err
	}

	var entries []struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		Path        string `json:"path"`
		DownloadURL string `json:"download_url"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	os.MkdirAll(localDir, 0755)

	for _, e := range entries {
		localPath := filepath.Join(localDir, e.Name)
		if e.Type == "dir" {
			// 递归下载子目录
			subRemote := e.Path
			if err := downloadSkillDir(owner, repo, subRemote, localPath); err != nil {
				return err
			}
		} else if e.Type == "file" {
			// 下载文件
			content, err := ghRawGet(owner, repo, e.Path)
			if err != nil {
				return fmt.Errorf("download %s: %w", e.Path, err)
			}
			os.MkdirAll(filepath.Dir(localPath), 0755)
			if err := os.WriteFile(localPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("write %s: %w", localPath, err)
			}
		}
	}
	return nil
}

// sanitizeDirName 将 skill 名称转为安全的目录名
func sanitizeDirName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ToLower(name)
	return name
}

// --- Agent Import ---

// ImportAgentsFromGithub 从 GitHub 导入独立 agents（遍历 agents/ 目录中的 .md 文件）
func ImportAgentsFromGithub(repoURL string) ([]StoreItem, error) {
	owner, repo := parseRepo(repoURL)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid repo: %s", repoURL)
	}

	ghURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)

	// 尝试根目录 agents/
	items := scanAgentsToStoreItems(owner, repo, "agents", ghURL)
	if len(items) > 0 {
		return items, nil
	}

	return nil, fmt.Errorf("no agents/ directory found in %s/%s", owner, repo)
}

// scanAgentsToStoreItems 扫描 agents/ 目录，每个 .md 文件创建独立的 TypeAgent StoreItem
func scanAgentsToStoreItems(owner, repo, dirPath, ghURL string) []StoreItem {
	apiPath := fmt.Sprintf("%s/%s/contents/%s", owner, repo, dirPath)
	data, err := ghAPIGet(apiPath)
	if err != nil {
		return nil
	}

	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}

	var items []StoreItem
	for _, e := range entries {
		if e.Type != "file" || !strings.HasSuffix(strings.ToLower(e.Name), ".md") {
			continue
		}
		content, err := ghRawGet(owner, repo, dirPath+"/"+e.Name)
		if err != nil {
			continue
		}
		agentName := strings.TrimSuffix(e.Name, filepath.Ext(e.Name))
		description := "Agent: " + agentName

		// 解析 YAML frontmatter
		if strings.HasPrefix(content, "---") {
			parts := strings.SplitN(content[3:], "---", 2)
			if len(parts) == 2 {
				fm := parts[0]
				if n := extractYAMLField(fm, "name"); n != "" {
					agentName = n
				}
				if d := extractYAMLField(fm, "description"); d != "" {
					description = d
				}
			}
		}

		items = append(items, StoreItem{
			Name:        agentName,
			Description: description,
			Type:        TypeAgent,
			GithubURL:   ghURL,
			Content:     content,
		})
	}
	return items
}

// --- Plugin Import ---

// ImportPluginFromGithub 从 GitHub 导入插件（完整的 agents + skills + commands）
func ImportPluginFromGithub(repoURL string, pluginsDir string) ([]StoreItem, error) {
	owner, repo := parseRepo(repoURL)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid repo: %s", repoURL)
	}

	ghURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)

	// 1. 尝试 marketplace.json → 多个插件
	if items, err := tryMarketplacePlugins(owner, repo, ghURL, pluginsDir); err == nil && len(items) > 0 {
		return items, nil
	}

	// 2. 尝试 plugin.json → 单个插件
	if items, err := trySinglePlugin(owner, repo, ghURL, pluginsDir); err == nil && len(items) > 0 {
		return items, nil
	}

	// 3. 尝试根目录直接扫描 agents/ + skills/ + commands/
	if items, err := tryRootPlugin(owner, repo, ghURL, pluginsDir); err == nil && len(items) > 0 {
		return items, nil
	}

	return nil, fmt.Errorf("no plugin found in %s/%s", owner, repo)
}

func tryMarketplacePlugins(owner, repo, ghURL, pluginsDir string) ([]StoreItem, error) {
	raw, err := ghRawGet(owner, repo, ".claude-plugin/marketplace.json")
	if err != nil {
		return nil, err
	}
	var mp MarketplaceJSON
	if err := json.Unmarshal([]byte(raw), &mp); err != nil {
		return nil, err
	}

	var allItems []StoreItem
	for _, p := range mp.Plugins {
		localPath, ok := p.Source.(string)
		if !ok {
			continue
		}
		localPath = strings.TrimPrefix(localPath, "./")

		item := buildPluginItem(owner, repo, localPath, p.Name, p.Description, ghURL, pluginsDir)
		if item != nil {
			allItems = append(allItems, *item)
		}
	}
	if len(allItems) == 0 {
		return nil, fmt.Errorf("no plugins found in marketplace")
	}
	return allItems, nil
}

func trySinglePlugin(owner, repo, ghURL, pluginsDir string) ([]StoreItem, error) {
	raw, err := ghRawGet(owner, repo, ".claude-plugin/plugin.json")
	if err != nil {
		return nil, err
	}
	var plugin PluginJSON
	if err := json.Unmarshal([]byte(raw), &plugin); err != nil {
		return nil, err
	}

	name := plugin.Name
	if name == "" {
		name = repo
	}
	desc := plugin.Description
	if desc == "" {
		desc = "Imported plugin from GitHub"
	}

	item := buildPluginItem(owner, repo, "", name, desc, ghURL, pluginsDir)
	if item == nil {
		return nil, fmt.Errorf("no sub-items found in plugin")
	}
	if plugin.Author != "" {
		item.Author = plugin.Author
	}
	return []StoreItem{*item}, nil
}

func tryRootPlugin(owner, repo, ghURL, pluginsDir string) ([]StoreItem, error) {
	item := buildPluginItem(owner, repo, "", repo, "Imported plugin from GitHub", ghURL, pluginsDir)
	if item == nil {
		return nil, fmt.Errorf("no sub-items found in root")
	}
	return []StoreItem{*item}, nil
}

func buildPluginItem(owner, repo, remotePath, name, desc, ghURL, pluginsDir string) *StoreItem {
	var subItems []SubItem

	agentDir := "agents"
	skillDir := "skills"
	cmdDir := "commands"
	if remotePath != "" {
		agentDir = remotePath + "/agents"
		skillDir = remotePath + "/skills"
		cmdDir = remotePath + "/commands"
	}

	// 扫描 agents/
	agentSubs := scanAgentsMD(owner, repo, agentDir)
	subItems = append(subItems, agentSubs...)

	// 扫描 skills/
	skillSubs := scanSkillsMD(owner, repo, skillDir)
	subItems = append(subItems, skillSubs...)

	// 扫描 commands/
	cmdSubs := scanCommandsMD(owner, repo, cmdDir)
	subItems = append(subItems, cmdSubs...)

	if len(subItems) == 0 {
		return nil
	}

	// 下载整个插件目录到 plugins/{name}/
	localDir := sanitizeDirName(name)
	if pluginsDir != "" {
		destDir := filepath.Join(pluginsDir, localDir)
		downloadPath := remotePath
		downloadSkillDir(owner, repo, downloadPath, destDir)
	}

	return &StoreItem{
		Name:        name,
		Description: desc,
		Type:        TypePlugin,
		GithubURL:   ghURL,
		SubItems:    subItems,
		LocalDir:    localDir,
	}
}

// scanAgentsMD 扫描 agents/ 目录下的 .md 文件
func scanAgentsMD(owner, repo, dirPath string) []SubItem {
	apiPath := fmt.Sprintf("%s/%s/contents/%s", owner, repo, dirPath)
	data, err := ghAPIGet(apiPath)
	if err != nil {
		return nil
	}

	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}

	var subs []SubItem
	for _, e := range entries {
		if e.Type != "file" || !strings.HasSuffix(strings.ToLower(e.Name), ".md") {
			continue
		}
		content, err := ghRawGet(owner, repo, dirPath+"/"+e.Name)
		if err != nil {
			continue
		}
		agentName := strings.TrimSuffix(e.Name, filepath.Ext(e.Name))
		description := "Agent: " + agentName
		model := ""
		var tools []string

		// 解析 YAML frontmatter
		if strings.HasPrefix(content, "---") {
			parts := strings.SplitN(content[3:], "---", 2)
			if len(parts) == 2 {
				fm := parts[0]
				if n := extractYAMLField(fm, "name"); n != "" {
					agentName = n
				}
				if d := extractYAMLField(fm, "description"); d != "" {
					description = d
				}
				if m := extractYAMLField(fm, "model"); m != "" {
					model = m
				}
				if t := extractYAMLField(fm, "tools"); t != "" {
					tools = NormalizeToolNames(t)
				}
			}
		}

		subs = append(subs, SubItem{
			Type:        SubTypeAgent,
			Name:        agentName,
			Description: description,
			Content:     content,
			Model:       model,
			Tools:       tools,
		})
	}
	return subs
}

// scanSkillsMD 扫描 skills/ 目录下的子目录（每个含 SKILL.md）
func scanSkillsMD(owner, repo, dirPath string) []SubItem {
	apiPath := fmt.Sprintf("%s/%s/contents/%s", owner, repo, dirPath)
	data, err := ghAPIGet(apiPath)
	if err != nil {
		return nil
	}

	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}

	var subs []SubItem
	for _, e := range entries {
		if e.Type != "dir" {
			continue
		}
		// 尝试 SKILL.md 或 skill.md
		for _, fname := range []string{"SKILL.md", "skill.md"} {
			content, err := ghRawGet(owner, repo, dirPath+"/"+e.Name+"/"+fname)
			if err != nil {
				continue
			}
			skillName := e.Name
			description := "Skill: " + skillName

			if strings.HasPrefix(content, "---") {
				parts := strings.SplitN(content[3:], "---", 2)
				if len(parts) == 2 {
					fm := parts[0]
					if n := extractYAMLField(fm, "name"); n != "" {
						skillName = n
					}
					if d := extractYAMLField(fm, "description"); d != "" {
						description = d
					}
				}
			}

			subs = append(subs, SubItem{
				Type:        SubTypeSkill,
				Name:        skillName,
				Description: description,
				Content:     content,
				LocalDir:    "skills/" + e.Name,
			})
			break
		}
	}
	return subs
}

// scanCommandsMD 扫描 commands/ 目录下的 .md 文件
func scanCommandsMD(owner, repo, dirPath string) []SubItem {
	apiPath := fmt.Sprintf("%s/%s/contents/%s", owner, repo, dirPath)
	data, err := ghAPIGet(apiPath)
	if err != nil {
		return nil
	}

	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}

	var subs []SubItem
	for _, e := range entries {
		if e.Type != "file" || !strings.HasSuffix(strings.ToLower(e.Name), ".md") {
			continue
		}
		content, err := ghRawGet(owner, repo, dirPath+"/"+e.Name)
		if err != nil {
			continue
		}
		cmdName := strings.TrimSuffix(e.Name, filepath.Ext(e.Name))
		description := "Command: " + cmdName

		if strings.HasPrefix(content, "---") {
			parts := strings.SplitN(content[3:], "---", 2)
			if len(parts) == 2 {
				fm := parts[0]
				if d := extractYAMLField(fm, "description"); d != "" {
					description = d
				}
				// command name from argument-hint or filename
				if ah := extractYAMLField(fm, "argument-hint"); ah != "" {
					description += " (" + ah + ")"
				}
			}
		}

		subs = append(subs, SubItem{
			Type:        SubTypeCommand,
			Name:        cmdName,
			Description: description,
			Content:     content,
		})
	}
	return subs
}

// toolNameMap 外部 agent md 中的工具名 → Coworker 内部工具名映射
var toolNameMap = map[string]string{
	"ls":        "LS",
	"read":      "Read",
	"write":     "Write",
	"edit":      "Edit",
	"glob":      "Glob",
	"grep":      "Grep",
	"bash":      "Bash",
	"webfetch":  "WebFetch",
	"websearch": "WebSearch",
	// 大小写变体
	"LS":        "LS",
	"Read":      "Read",
	"Write":     "Write",
	"Edit":      "Edit",
	"Glob":      "Glob",
	"Grep":      "Grep",
	"Bash":      "Bash",
	"WebFetch":  "WebFetch",
	"WebSearch": "WebSearch",
}

// NormalizeToolNames 将逗号/空格分隔的 tools 字符串解析并映射为 Coworker 内部工具名
// 输入示例: "LS, Read, Grep, Glob, Bash"
// 输出: ["LS", "Read", "Grep", "Glob", "Bash"]
// 未识别的名称保留原样（可能是 MCP 工具等）
func NormalizeToolNames(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "*" {
		return nil // nil = 全部工具
	}

	// 支持逗号或空格分隔
	var parts []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}

	seen := make(map[string]bool)
	var result []string
	for _, p := range parts {
		if mapped, ok := toolNameMap[p]; ok {
			if !seen[mapped] {
				result = append(result, mapped)
				seen[mapped] = true
			}
		} else if !seen[p] {
			// 未识别的保留原样
			result = append(result, p)
			seen[p] = true
		}
	}
	return result
}

// ParseAgentTools 从 agent markdown 内容的 frontmatter 中解析 tools 字段
// 用于独立 TypeAgent 条目（非 plugin 子条目）在注册时解析
func ParseAgentTools(content string) []string {
	if !strings.HasPrefix(content, "---") {
		return nil
	}
	parts := strings.SplitN(content[3:], "---", 2)
	if len(parts) < 2 {
		return nil
	}
	fm := parts[0]
	t := extractYAMLField(fm, "tools")
	if t == "" {
		return nil
	}
	return NormalizeToolNames(t)
}
