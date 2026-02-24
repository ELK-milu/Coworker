package store

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ═══ GitHub Archive Download ════════════════════════════════════════
//
// 通过一次 HTTP 请求下载整个仓库的 tarball，解压到临时目录，
// 后续所有扫描和文件读取都在本地完成，避免大量 GitHub API 调用导致限流。

// downloadRepoArchive 下载 GitHub 仓库的 tarball 并解压到临时目录
// 返回解压后的仓库根目录路径和清理函数
func downloadRepoArchive(owner, repo string) (repoRoot string, cleanup func(), err error) {
	archiveURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/tarball", owner, repo)
	req, _ := http.NewRequest("GET", archiveURL, nil)
	req.Header.Set("User-Agent", "Coworker-Store")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	log.Printf("[Store] downloading repo archive: %s/%s", owner, repo)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("download repo archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("download repo archive HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	tempDir, err := os.MkdirTemp("", "gh-import-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}
	cleanup = func() {
		os.RemoveAll(tempDir)
		log.Printf("[Store] cleaned up temp dir: %s", tempDir)
	}

	// 解压 tar.gz
	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	var rootDir string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			cleanup()
			return "", nil, fmt.Errorf("tar reader: %w", err)
		}

		// 跳过 PAX 全局扩展头（git archive 产生的 pax_global_header）
		if header.Typeflag == tar.TypeXGlobalHeader || header.Typeflag == tar.TypeXHeader {
			continue
		}

		// GitHub tarball 的根目录格式: {owner}-{repo}-{sha}/
		// 从含 "/" 的路径中提取第一级目录名
		name := header.Name
		if rootDir == "" && strings.Contains(name, "/") {
			parts := strings.SplitN(name, "/", 2)
			rootDir = parts[0]
		}

		// 安全: 防止路径穿越
		target := filepath.Join(tempDir, filepath.FromSlash(name))
		cleanTarget := filepath.Clean(target)
		cleanTemp := filepath.Clean(tempDir)
		if !strings.HasPrefix(cleanTarget, cleanTemp+string(os.PathSeparator)) && cleanTarget != cleanTemp {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			f, err := os.Create(target)
			if err != nil {
				continue
			}
			io.Copy(f, io.LimitReader(tr, 50<<20)) // 单文件 50MB 限制
			f.Close()
		}
	}

	if rootDir == "" {
		cleanup()
		return "", nil, fmt.Errorf("empty archive for %s/%s", owner, repo)
	}

	repoRoot = filepath.Join(tempDir, rootDir)
	log.Printf("[Store] extracted repo to: %s", repoRoot)
	return repoRoot, cleanup, nil
}

// ═══ Local Filesystem Helpers ═══════════════════════════════════════

// localReadFile 从本地解压的仓库目录读取文件
func localReadFile(repoDir, path string) (string, error) {
	fullPath := filepath.Join(repoDir, filepath.FromSlash(path))
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// repoEntry 表示本地目录中的一个条目
type repoEntry struct {
	Name  string
	IsDir bool
}

// localListDir 列出本地解压仓库中某个目录的条目
func localListDir(repoDir, dirPath string) ([]repoEntry, error) {
	fullPath := repoDir
	if dirPath != "" {
		fullPath = filepath.Join(repoDir, filepath.FromSlash(dirPath))
	}
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}
	var result []repoEntry
	for _, e := range entries {
		result = append(result, repoEntry{Name: e.Name(), IsDir: e.IsDir()})
	}
	return result, nil
}

// copyLocalDir 从解压的仓库目录复制子目录到目标位置
func copyLocalDir(repoDir, remotePath, destDir string) error {
	srcDir := repoDir
	if remotePath != "" {
		srcDir = filepath.Join(repoDir, filepath.FromSlash(remotePath))
	}

	info, err := os.Stat(srcDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("source dir not found: %s", remotePath)
	}

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(srcDir, path)
		target := filepath.Join(destDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		os.MkdirAll(filepath.Dir(target), 0755)
		return os.WriteFile(target, data, 0644)
	})
}

// ═══ Import Entry Points ════════════════════════════════════════════

// ImportFromGithub 从 GitHub 仓库导入 skills/agents
// storeDir: 技能文件全局存储目录（store/skills/），空字符串表示不下载文件
func ImportFromGithub(repoURL string, storeDir string) ([]StoreItem, error) {
	owner, repo := parseRepo(repoURL)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid repo: %s", repoURL)
	}

	ghURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	repoIdent := owner + "/" + repo

	// 一次性下载整个仓库到临时目录（单次 HTTP 请求，避免 API 限流）
	repoDir, cleanup, err := downloadRepoArchive(owner, repo)
	if err != nil {
		return nil, fmt.Errorf("download repo: %w", err)
	}
	defer cleanup()

	// 优先级: marketplace.json > plugin.json > root SKILL.md
	// 1. 尝试 .claude-plugin/marketplace.json → 统一为 TypePlugin
	pluginsDir := filepath.Join(filepath.Dir(storeDir), "plugins")
	if items, err := importFromMarketplace(repoDir, repo, ghURL, pluginsDir); err == nil && len(items) > 0 {
		fillDefaultAuthor(items, repoIdent)
		return items, nil
	}

	// 2. 尝试 .claude-plugin/plugin.json
	if items, err := tryPluginJSON(repoDir, ghURL, storeDir); err == nil && len(items) > 0 {
		fillDefaultAuthor(items, repoIdent)
		return items, nil
	}

	// 3. 尝试根目录 SKILL.md / skill.md
	if items, err := tryRootSkill(repoDir, repo, ghURL, storeDir); err == nil && len(items) > 0 {
		fillDefaultAuthor(items, repoIdent)
		return items, nil
	}

	return nil, fmt.Errorf("no plugin/skill found in %s/%s", owner, repo)
}

// ImportAgentsFromGithub 从 GitHub 导入独立 agents（遍历 agents/ 目录中的 .md 文件）
func ImportAgentsFromGithub(repoURL string) ([]StoreItem, error) {
	owner, repo := parseRepo(repoURL)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid repo: %s", repoURL)
	}

	ghURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	repoIdent := owner + "/" + repo

	repoDir, cleanup, err := downloadRepoArchive(owner, repo)
	if err != nil {
		return nil, fmt.Errorf("download repo: %w", err)
	}
	defer cleanup()

	// 尝试根目录 agents/
	items := scanAgentsToStoreItems(repoDir, "agents", ghURL)
	if len(items) > 0 {
		fillDefaultAuthor(items, repoIdent)
		return items, nil
	}

	return nil, fmt.Errorf("no agents/ directory found in %s/%s", owner, repo)
}

// ImportPluginFromGithub 从 GitHub 导入插件（完整的 agents + skills + commands）
func ImportPluginFromGithub(repoURL string, pluginsDir string) ([]StoreItem, error) {
	owner, repo := parseRepo(repoURL)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid repo: %s", repoURL)
	}

	ghURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	repoIdent := owner + "/" + repo

	repoDir, cleanup, err := downloadRepoArchive(owner, repo)
	if err != nil {
		return nil, fmt.Errorf("download repo: %w", err)
	}
	defer cleanup()

	// 1. 尝试 marketplace.json → 多个插件
	if items, err := importFromMarketplace(repoDir, repo, ghURL, pluginsDir); err == nil && len(items) > 0 {
		fillDefaultAuthor(items, repoIdent)
		return items, nil
	}

	// 2. 尝试 plugin.json → 单个插件
	if items, err := trySinglePlugin(repoDir, repo, ghURL, pluginsDir); err == nil && len(items) > 0 {
		fillDefaultAuthor(items, repoIdent)
		return items, nil
	}

	// 3. 尝试根目录直接扫描 agents/ + skills/ + commands/
	if items, err := tryRootPlugin(repoDir, repo, ghURL, pluginsDir); err == nil && len(items) > 0 {
		fillDefaultAuthor(items, repoIdent)
		return items, nil
	}

	return nil, fmt.Errorf("no plugin found in %s/%s", owner, repo)
}

// ═══ URL Parsing ════════════════════════════════════════════════════

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

// ═══ Marketplace Import ═════════════════════════════════════════════

// importFromMarketplace 统一处理 marketplace.json → 每个 plugin 条目生成一个 TypePlugin StoreItem
func importFromMarketplace(repoDir, repo, ghURL, pluginsDir string) ([]StoreItem, error) {
	raw, err := localReadFile(repoDir, ".claude-plugin/marketplace.json")
	if err != nil {
		return nil, err
	}
	var mp MarketplaceJSON
	if err := json.Unmarshal([]byte(raw), &mp); err != nil {
		return nil, err
	}

	var allItems []StoreItem
	for i := range mp.Plugins {
		item := buildMarketplacePluginItem(repoDir, repo, ghURL, pluginsDir, &mp, &mp.Plugins[i])
		if item != nil {
			allItems = append(allItems, *item)
		}
	}
	if len(allItems) == 0 {
		return nil, fmt.Errorf("no plugins found in marketplace")
	}
	return allItems, nil
}

// buildMarketplacePluginItem 将单个 marketplace plugin 条目构建为 TypePlugin StoreItem
func buildMarketplacePluginItem(repoDir, repo, ghURL, pluginsDir string, mp *MarketplaceJSON, p *MarketplacePlugin) *StoreItem {
	// 1. 解析 source 路径
	sourcePath := ""
	if s, ok := p.Source.(string); ok {
		sourcePath = strings.TrimPrefix(s, "./")
	}

	// 2. 尝试读取 {source}/.claude-plugin/plugin.json 补充元数据
	name := p.Name
	desc := p.Description
	if sourcePath != "" {
		pluginJSONPath := sourcePath + "/.claude-plugin/plugin.json"
		if raw, err := localReadFile(repoDir, pluginJSONPath); err == nil {
			var pj PluginJSON
			if json.Unmarshal([]byte(raw), &pj) == nil {
				if name == "" && pj.Name != "" {
					name = pj.Name
				}
				if desc == "" && pj.Description != "" {
					desc = pj.Description
				}
			}
		}
	}
	if name == "" {
		name = repo
	}
	if desc == "" {
		desc = "Imported plugin from GitHub"
	}

	// 3. 构建 SubItems
	var subItems []SubItem

	// skills: 有显式 Skills 数组 → scanSkillPathsMD；否则 → 三层降级扫描
	if len(p.Skills) > 0 {
		subItems = append(subItems, scanSkillPathsMD(repoDir, p.Skills)...)
	} else if sourcePath != "" {
		// 降级1: sourcePath/skills/ （标准布局）
		if subs := scanSkillsMD(repoDir, sourcePath+"/skills"); len(subs) > 0 {
			subItems = append(subItems, subs...)
		} else if subs := scanSkillsMD(repoDir, sourcePath); len(subs) > 0 {
			// 降级2: sourcePath/ 本身包含 skill 子目录（如 marketing-skill/content-creator/SKILL.md）
			subItems = append(subItems, subs...)
		} else {
			// 降级3: sourcePath/ 本身就是一个 skill（直接含 SKILL.md）
			subItems = append(subItems, tryReadSingleSkill(repoDir, sourcePath)...)
		}
	} else {
		subItems = append(subItems, scanSkillsMD(repoDir, "skills")...)
	}

	// agents: sourcePath/agents/ → 降级到根 agents/
	agentDir := "agents"
	if sourcePath != "" {
		agentDir = sourcePath + "/agents"
	}
	subItems = append(subItems, scanAgentsMD(repoDir, agentDir)...)

	// commands: sourcePath/commands/ → 降级到根 commands/
	cmdDir := "commands"
	if sourcePath != "" {
		cmdDir = sourcePath + "/commands"
	}
	subItems = append(subItems, scanCommandsMD(repoDir, cmdDir)...)

	if len(subItems) == 0 {
		return nil
	}

	// 4. 解析 author
	author := resolveMarketplaceAuthor(mp, p)

	// 5. 复制文件到目标目录
	localDir := sanitizeDirName(name)
	if pluginsDir != "" {
		if len(p.Skills) > 0 && (sourcePath == "" || sourcePath == ".") {
			// anthropics/skills 模式: source="./" + 显式 skills 数组 → 逐个复制 skill 目录
			for _, sp := range p.Skills {
				sp = strings.TrimPrefix(sp, "./")
				skillName := filepath.Base(sp)
				destDir := filepath.Join(pluginsDir, localDir, "skills", skillName)
				copyLocalDir(repoDir, sp, destDir)
			}
		} else if sourcePath != "" {
			// wshobson/agents 模式: source 指向子目录 → 复制整个 source 目录
			destDir := filepath.Join(pluginsDir, localDir)
			copyLocalDir(repoDir, sourcePath, destDir)
		}
	}

	return &StoreItem{
		Name:        name,
		Description: desc,
		Type:        TypePlugin,
		GithubURL:   ghURL,
		Author:      author,
		SubItems:    subItems,
		LocalDir:    localDir,
	}
}

// resolveMarketplaceAuthor 解析 author：plugin 级 author → marketplace 级 owner → 空
func resolveMarketplaceAuthor(mp *MarketplaceJSON, p *MarketplacePlugin) string {
	// plugin 级 author
	if p.Author != nil {
		switch v := p.Author.(type) {
		case string:
			if v != "" {
				return v
			}
		case map[string]interface{}:
			if name, ok := v["name"].(string); ok && name != "" {
				return name
			}
		}
	}
	// marketplace 级 owner
	if mp.Owner != nil && mp.Owner.Name != "" {
		return mp.Owner.Name
	}
	return ""
}

// ═══ Plugin JSON Import ═════════════════════════════════════════════

func tryPluginJSON(repoDir, ghURL, storeDir string) ([]StoreItem, error) {
	raw, err := localReadFile(repoDir, ".claude-plugin/plugin.json")
	if err != nil {
		return nil, err
	}
	var plugin PluginJSON
	if err := json.Unmarshal([]byte(raw), &plugin); err != nil {
		return nil, err
	}

	return scanSkillsDir(repoDir, "skills", plugin.Author, ghURL, storeDir)
}

func trySinglePlugin(repoDir, repo, ghURL, pluginsDir string) ([]StoreItem, error) {
	raw, err := localReadFile(repoDir, ".claude-plugin/plugin.json")
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

	item := buildPluginItem(repoDir, "", name, desc, ghURL, pluginsDir, nil)
	if item == nil {
		return nil, fmt.Errorf("no sub-items found in plugin")
	}
	if plugin.Author != "" {
		item.Author = plugin.Author
	}
	return []StoreItem{*item}, nil
}

func tryRootPlugin(repoDir, repo, ghURL, pluginsDir string) ([]StoreItem, error) {
	item := buildPluginItem(repoDir, "", repo, "Imported plugin from GitHub", ghURL, pluginsDir, nil)
	if item == nil {
		return nil, fmt.Errorf("no sub-items found in root")
	}
	return []StoreItem{*item}, nil
}

func buildPluginItem(repoDir, remotePath, name, desc, ghURL, pluginsDir string, skillPaths []string) *StoreItem {
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
	agentSubs := scanAgentsMD(repoDir, agentDir)
	subItems = append(subItems, agentSubs...)

	// 扫描 skills/：优先使用显式路径列表
	var skillSubs []SubItem
	if len(skillPaths) > 0 {
		skillSubs = scanSkillPathsMD(repoDir, skillPaths)
	} else {
		skillSubs = scanSkillsMD(repoDir, skillDir)
	}
	subItems = append(subItems, skillSubs...)

	// 扫描 commands/
	cmdSubs := scanCommandsMD(repoDir, cmdDir)
	subItems = append(subItems, cmdSubs...)

	if len(subItems) == 0 {
		return nil
	}

	// 复制整个插件目录到 plugins/{name}/
	localDir := sanitizeDirName(name)
	if pluginsDir != "" {
		destDir := filepath.Join(pluginsDir, localDir)
		copyLocalDir(repoDir, remotePath, destDir)
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

// ═══ Root Skill Import ══════════════════════════════════════════════

func tryRootSkill(repoDir, repo, ghURL, storeDir string) ([]StoreItem, error) {
	for _, name := range []string{"SKILL.md", "skill.md"} {
		content, err := localReadFile(repoDir, name)
		if err != nil {
			continue
		}
		item := parseSkillMD(content, repo, ghURL)
		// 复制整个仓库根目录到 store/skills/{name}/
		if storeDir != "" {
			localDir := sanitizeDirName(item.Name)
			destDir := filepath.Join(storeDir, localDir)
			if err := copyLocalDir(repoDir, "", destDir); err == nil {
				item.LocalDir = localDir
			}
		}
		return []StoreItem{item}, nil
	}
	return nil, fmt.Errorf("no root skill.md")
}

// ═══ Directory Scanning ═════════════════════════════════════════════

// scanSkillsDir 扫描 skills/ 目录下的子目录
func scanSkillsDir(repoDir, dirPath, author, ghURL, storeDir string) ([]StoreItem, error) {
	entries, err := localListDir(repoDir, dirPath)
	if err != nil {
		return nil, err
	}

	var items []StoreItem
	for _, e := range entries {
		if !e.IsDir {
			continue
		}
		// 尝试 SKILL.md 或 skill.md
		for _, fname := range []string{"SKILL.md", "skill.md"} {
			content, err := localReadFile(repoDir, dirPath+"/"+e.Name+"/"+fname)
			if err != nil {
				continue
			}
			item := parseSkillMD(content, e.Name, ghURL)
			if author != "" {
				item.Author = author
			}
			// 复制整个 skill 子目录到 store/skills/{name}/
			if storeDir != "" {
				localDir := sanitizeDirName(item.Name)
				destDir := filepath.Join(storeDir, localDir)
				remotePath := dirPath + "/" + e.Name
				if err := copyLocalDir(repoDir, remotePath, destDir); err == nil {
					item.LocalDir = localDir
				}
			}
			items = append(items, item)
			break
		}
	}
	return items, nil
}

// scanSkillsMD 扫描 skills/ 目录下的子目录（每个含 SKILL.md）
func scanSkillsMD(repoDir, dirPath string) []SubItem {
	entries, err := localListDir(repoDir, dirPath)
	if err != nil {
		return nil
	}

	var subs []SubItem
	for _, e := range entries {
		if !e.IsDir {
			continue
		}
		// 尝试 SKILL.md 或 skill.md
		for _, fname := range []string{"SKILL.md", "skill.md"} {
			content, err := localReadFile(repoDir, dirPath+"/"+e.Name+"/"+fname)
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

// scanSkillPathsMD 根据显式路径列表逐个获取 skill（用于 plugin 导入，返回 SubItem）
func scanSkillPathsMD(repoDir string, paths []string) []SubItem {
	var subs []SubItem
	for _, p := range paths {
		p = strings.TrimPrefix(p, "./")
		dirName := filepath.Base(p)
		for _, fname := range []string{"SKILL.md", "skill.md"} {
			content, err := localReadFile(repoDir, p+"/"+fname)
			if err != nil {
				continue
			}
			skillName := dirName
			description := "Imported from GitHub"
			if strings.HasPrefix(content, "---") {
				parts := strings.SplitN(content[3:], "---", 2)
				if len(parts) == 2 {
					if n := extractYAMLField(parts[0], "name"); n != "" {
						skillName = n
					}
					if d := extractYAMLField(parts[0], "description"); d != "" {
						description = d
					}
				}
			}
			subs = append(subs, SubItem{
				Type:        SubTypeSkill,
				Name:        skillName,
				Description: description,
				Content:     content,
				LocalDir:    "skills/" + dirName,
			})
			break
		}
	}
	return subs
}

// scanAgentsMD 扫描 agents/ 目录下的 .md 文件
func scanAgentsMD(repoDir, dirPath string) []SubItem {
	entries, err := localListDir(repoDir, dirPath)
	if err != nil {
		return nil
	}

	var subs []SubItem
	for _, e := range entries {
		if e.IsDir || !strings.HasSuffix(strings.ToLower(e.Name), ".md") {
			continue
		}
		content, err := localReadFile(repoDir, dirPath+"/"+e.Name)
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

// scanAgentsToStoreItems 递归扫描 agents/ 目录，每个 .md 文件创建独立的 TypeAgent StoreItem
func scanAgentsToStoreItems(repoDir, dirPath, ghURL string) []StoreItem {
	entries, err := localListDir(repoDir, dirPath)
	if err != nil {
		log.Printf("[Store] scanAgents: failed to list %s: %v", dirPath, err)
		return nil
	}

	log.Printf("[Store] scanAgents: %s → %d entries", dirPath, len(entries))

	var items []StoreItem
	for _, e := range entries {
		// 递归扫描子目录
		if e.IsDir {
			sub := scanAgentsToStoreItems(repoDir, dirPath+"/"+e.Name, ghURL)
			items = append(items, sub...)
			continue
		}
		if !strings.HasSuffix(strings.ToLower(e.Name), ".md") {
			continue
		}
		content, err := localReadFile(repoDir, dirPath+"/"+e.Name)
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

// scanCommandsMD 扫描 commands/ 目录下的 .md 文件
func scanCommandsMD(repoDir, dirPath string) []SubItem {
	entries, err := localListDir(repoDir, dirPath)
	if err != nil {
		return nil
	}

	var subs []SubItem
	for _, e := range entries {
		if e.IsDir || !strings.HasSuffix(strings.ToLower(e.Name), ".md") {
			continue
		}
		content, err := localReadFile(repoDir, dirPath+"/"+e.Name)
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

// tryReadSingleSkill 尝试从 sourcePath/ 直接读取 SKILL.md（sourcePath 本身就是一个 skill）
func tryReadSingleSkill(repoDir, sourcePath string) []SubItem {
	for _, fname := range []string{"SKILL.md", "skill.md"} {
		content, err := localReadFile(repoDir, sourcePath+"/"+fname)
		if err != nil {
			continue
		}
		skillName := filepath.Base(sourcePath)
		description := "Skill: " + skillName

		if strings.HasPrefix(content, "---") {
			parts := strings.SplitN(content[3:], "---", 2)
			if len(parts) == 2 {
				if n := extractYAMLField(parts[0], "name"); n != "" {
					skillName = n
				}
				if d := extractYAMLField(parts[0], "description"); d != "" {
					description = d
				}
			}
		}

		return []SubItem{{
			Type:        SubTypeSkill,
			Name:        skillName,
			Description: description,
			Content:     content,
			LocalDir:    "skills/" + filepath.Base(sourcePath),
		}}
	}
	return nil
}

// ═══ Parsing & Utilities ════════════════════════════════════════════

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

// fillDefaultAuthor 为没有 Author 的 StoreItem 填充默认值（owner/repo 格式）
func fillDefaultAuthor(items []StoreItem, defaultAuthor string) {
	for i := range items {
		if items[i].Author == "" {
			items[i].Author = defaultAuthor
		}
	}
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

// ═══ Tool Name Mapping ══════════════════════════════════════════════

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
