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
// ref 为空时下载默认分支（HEAD），否则下载指定分支/tag
//
// 使用 codeload.github.com 直接下载，不走 API 端点，不受 API 限流约束。
// 格式: https://codeload.github.com/{owner}/{repo}/tar.gz/refs/heads/{branch}
//   或: https://codeload.github.com/{owner}/{repo}/tar.gz/{ref}  (tag/sha)
// ref 为空时 fallback 到 HEAD（GitHub 会自动解析为默认分支）
func downloadRepoArchive(owner, repo, ref string) (repoRoot string, cleanup func(), err error) {
	if ref == "" {
		ref = "HEAD"
	}
	archiveURL := fmt.Sprintf("https://codeload.github.com/%s/%s/tar.gz/%s", owner, repo, ref)
	req, _ := http.NewRequest("GET", archiveURL, nil)
	req.Header.Set("User-Agent", "Coworker-Store")

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
	parsed := parseSource(repoURL)
	if parsed.Owner == "" || parsed.Repo == "" {
		return nil, fmt.Errorf("invalid repo: %s", repoURL)
	}

	owner, repo := parsed.Owner, parsed.Repo
	ghURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	repoIdent := owner + "/" + repo

	// 一次性下载整个仓库到临时目录（单次 HTTP 请求，避免 API 限流）
	// 如果指定了 ref，下载指定分支/tag
	repoDir, cleanup, err := downloadRepoArchive(owner, repo, parsed.Ref)
	if err != nil {
		return nil, fmt.Errorf("download repo: %w", err)
	}
	defer cleanup()

	// discoverSkills 的搜索起点：如果有 subpath 则从子路径开始
	searchPath := "."
	if parsed.Subpath != "" {
		searchPath = parsed.Subpath
	}

	// 优先级: marketplace.json > plugin.json > discoverSkills > root SKILL.md
	// 1. 尝试 .claude-plugin/marketplace.json → 统一为 TypePlugin
	pluginsDir := filepath.Join(filepath.Dir(storeDir), "plugins")
	if items, err := importFromMarketplace(repoDir, repo, ghURL, pluginsDir); err == nil && len(items) > 0 {
		if parsed.SkillFilter != "" {
			items = filterSkillItemsByName(items, parsed.SkillFilter)
		}
		if len(items) > 0 {
			fillDefaultAuthor(items, repoIdent)
			return items, nil
		}
	}

	// 2. 尝试 .claude-plugin/plugin.json
	if items, err := tryPluginJSON(repoDir, ghURL, storeDir); err == nil && len(items) > 0 {
		if parsed.SkillFilter != "" {
			items = filterSkillItemsByName(items, parsed.SkillFilter)
		}
		if len(items) > 0 {
			fillDefaultAuthor(items, repoIdent)
			return items, nil
		}
	}

	// 3. discoverSkills 统一发现（模仿 npx skills 的优先级扫描）
	//    扫描: ./SKILL.md → ./skills/ → 直接子目录 → 递归(max 5)
	if items, err := tryDiscoverSkills(repoDir, repo, ghURL, storeDir, searchPath); err == nil && len(items) > 0 {
		if parsed.SkillFilter != "" {
			items = filterSkillItemsByName(items, parsed.SkillFilter)
		}
		if len(items) > 0 {
			fillDefaultAuthor(items, repoIdent)
			return items, nil
		}
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

	repoDir, cleanup, err := downloadRepoArchive(owner, repo, "")
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

	repoDir, cleanup, err := downloadRepoArchive(owner, repo, "")
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

// ParsedSource GitHub URL 解析结果（增强版，支持分支/子路径/技能过滤）
type ParsedSource struct {
	Owner       string // GitHub owner
	Repo        string // GitHub repo name
	Ref         string // 分支/tag（从 /tree/xxx 提取）
	Subpath     string // 子路径（从 /tree/branch/path/to/skill 提取）
	SkillFilter string // 技能过滤（从 owner/repo@skill-name 提取）
}

// parseSource 增强版 URL 解析，支持以下格式:
//   - https://github.com/owner/repo
//   - github.com/owner/repo/tree/main
//   - github.com/owner/repo/tree/main/skills/pdf
//   - owner/repo
//   - owner/repo/path/to/skill
//   - owner/repo@find-skills
func parseSource(input string) ParsedSource {
	input = strings.TrimSpace(input)
	input = strings.TrimSuffix(input, "/")
	input = strings.TrimSuffix(input, ".git")

	var ps ParsedSource

	// 提取 @skillFilter（在任何 URL 处理之前）
	if idx := strings.LastIndex(input, "@"); idx > 0 {
		// 确保 @ 不是 URL scheme 的一部分（如 git@github.com）
		before := input[:idx]
		if !strings.Contains(before, "://") && !strings.HasPrefix(before, "git@") {
			ps.SkillFilter = input[idx+1:]
			input = before
		}
	}

	// 移除协议前缀
	input = strings.TrimPrefix(input, "https://")
	input = strings.TrimPrefix(input, "http://")

	// 移除 github.com 前缀
	if strings.HasPrefix(input, "github.com/") {
		input = strings.TrimPrefix(input, "github.com/")
	}

	// 此时 input 格式为: owner/repo[/tree/ref[/subpath]] 或 owner/repo[/subpath]
	parts := strings.SplitN(input, "/", 3)
	if len(parts) < 2 {
		return ps
	}

	ps.Owner = parts[0]
	ps.Repo = parts[1]

	if len(parts) == 3 {
		rest := parts[2] // tree/main/skills/pdf 或 path/to/skill

		if strings.HasPrefix(rest, "tree/") {
			// /tree/ref[/subpath] 格式
			treeParts := strings.SplitN(rest[5:], "/", 2) // 去掉 "tree/"
			ps.Ref = treeParts[0]
			if len(treeParts) == 2 && treeParts[1] != "" {
				ps.Subpath = treeParts[1]
			}
		} else if strings.HasPrefix(rest, "blob/") {
			// /blob/ref/path 格式（也支持）
			blobParts := strings.SplitN(rest[5:], "/", 2)
			ps.Ref = blobParts[0]
			if len(blobParts) == 2 && blobParts[1] != "" {
				ps.Subpath = blobParts[1]
			}
		} else {
			// 简写格式 owner/repo/path/to/skill
			ps.Subpath = rest
		}
	}

	return ps
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

	// skills: 有显式 Skills 数组 → scanSkillPathsMD；否则 → discoverSkills 统一发现
	if len(p.Skills) > 0 {
		subItems = append(subItems, scanSkillPathsMD(repoDir, p.Skills)...)
	} else {
		searchPath := sourcePath
		if searchPath == "" {
			searchPath = "."
		}
		subItems = append(subItems, discoverSkills(repoDir, searchPath)...)
	}

	// agents: 使用 discoverAgents 统一发现
	agentSearchPath := sourcePath
	if agentSearchPath == "" {
		agentSearchPath = "."
	}
	subItems = append(subItems, discoverAgents(repoDir, agentSearchPath)...)

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

	searchPath := remotePath
	if searchPath == "" {
		searchPath = "."
	}

	// 扫描 agents — 使用 discoverAgents 统一发现
	subItems = append(subItems, discoverAgents(repoDir, searchPath)...)

	// 扫描 skills：优先使用显式路径列表，否则 discoverSkills 统一发现
	if len(skillPaths) > 0 {
		subItems = append(subItems, scanSkillPathsMD(repoDir, skillPaths)...)
	} else {
		subItems = append(subItems, discoverSkills(repoDir, searchPath)...)
	}

	// 扫描 commands（保持原有逻辑）
	cmdDir := "commands"
	if remotePath != "" {
		cmdDir = remotePath + "/commands"
	}
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

// ═══ Discover Skills Import ═══════════════════════════════════════

// tryDiscoverSkills 使用 discoverSkills 统一发现仓库中的 skills
// 覆盖: 根 SKILL.md、skills/ 目录、直接子目录、递归搜索
// searchPath: 搜索起点路径（"." 表示仓库根目录）
func tryDiscoverSkills(repoDir, repo, ghURL, storeDir, searchPath string) ([]StoreItem, error) {
	subs := discoverSkills(repoDir, searchPath)
	if len(subs) == 0 {
		return nil, fmt.Errorf("no skills discovered")
	}

	var items []StoreItem
	for _, sub := range subs {
		item := StoreItem{
			Name:        sub.Name,
			Description: sub.Description,
			Type:        TypeSkill,
			GithubURL:   ghURL,
			Content:     sub.Content,
		}
		// 复制 skill 目录到 store/skills/{name}/
		if storeDir != "" && sub.LocalDir != "" {
			localDir := sanitizeDirName(sub.Name)
			destDir := filepath.Join(storeDir, localDir)
			// sub.LocalDir 格式为 "skills/xxx"，从仓库根找源目录
			srcRelPath := filepath.Base(sub.LocalDir)
			// 尝试在常见位置查找源目录
			for _, candidate := range []string{
				"skills/" + srcRelPath,    // skills/find-skills/
				srcRelPath,                // find-skills/
				sub.LocalDir,              // skills/find-skills (原始值)
			} {
				if err := copyLocalDir(repoDir, candidate, destDir); err == nil {
					item.LocalDir = localDir
					break
				}
			}
		}
		items = append(items, item)
	}
	return items, nil
}

// ═══ Unified Discovery (模仿 npx skills 的优先级扫描) ═══════════════

// skipDirs 在递归搜索时跳过的目录名
var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"dist":         true,
	"build":        true,
	".next":        true,
	".cache":       true,
	"vendor":       true,
	"__pycache__":  true,
	".venv":        true,
	".claude":      true,
}

// discoverSkills 统一 skill 发现函数，模仿 npx skills 的优先级扫描逻辑
// 返回找到的所有 skill SubItem
//
// 扫描优先级：
//  1. searchPath/SKILL.md — searchPath 本身就是一个 skill
//  2. 优先级目录 — skills/, skills/.curated, .claude/skills/ 等
//  3. searchPath/ 直接子目录 — 每个子目录含 SKILL.md
//  4. 递归搜索 searchPath/ — 最大深度 5，跳过 skipDirs
func discoverSkills(repoDir, searchPath string) []SubItem {
	// 优先级 1: searchPath 本身就是一个 skill（直接含 SKILL.md）
	if sub := readSkillAt(repoDir, searchPath); sub != nil {
		return []SubItem{*sub}
	}

	// 优先级 2: 按优先级扫描标准目录（对齐 npx skills v1.4.1 的 prioritySearchDirs）
	priorityDirs := []string{
		searchPath + "/skills",
		searchPath + "/skills/.curated",
		searchPath + "/skills/.experimental",
		searchPath + "/skills/.system",
		searchPath + "/.agent/skills",
		searchPath + "/.agents/skills",
		searchPath + "/.claude/skills",
		searchPath + "/.cline/skills",
		searchPath + "/.codebuddy/skills",
		searchPath + "/.codex/skills",
		searchPath + "/.commandcode/skills",
		searchPath + "/.continue/skills",
		searchPath + "/.github/skills",
		searchPath + "/.goose/skills",
		searchPath + "/.iflow/skills",
		searchPath + "/.junie/skills",
		searchPath + "/.kilocode/skills",
		searchPath + "/.kiro/skills",
		searchPath + "/.mux/skills",
		searchPath + "/.neovate/skills",
		searchPath + "/.opencode/skills",
		searchPath + "/.openhands/skills",
		searchPath + "/.pi/skills",
		searchPath + "/.qoder/skills",
		searchPath + "/.roo/skills",
		searchPath + "/.trae/skills",
		searchPath + "/.windsurf/skills",
		searchPath + "/.zencoder/skills",
	}
	// 追加 plugin manifest 中声明的 skill 搜索目录
	priorityDirs = append(priorityDirs, getPluginSkillPaths(repoDir, searchPath)...)
	seenNames := make(map[string]bool)
	var subs []SubItem
	for _, dir := range priorityDirs {
		for _, sub := range scanDirForSkills(repoDir, dir) {
			if !seenNames[sub.Name] {
				subs = append(subs, sub)
				seenNames[sub.Name] = true
			}
		}
	}
	if len(subs) > 0 {
		return subs
	}

	// 优先级 3: searchPath/ 直接子目录（每个子目录含 SKILL.md）
	if subs := scanDirForSkills(repoDir, searchPath); len(subs) > 0 {
		return subs
	}

	// 优先级 4: 递归搜索（max depth 5）
	var results []SubItem
	findSkillsRecursive(repoDir, searchPath, 0, 5, &results)
	return results
}

// discoverAgents 统一 agent 发现函数
// 扫描优先级：
//  1. searchPath/agents/ — 标准 agents 子目录
//  2. 递归搜索 searchPath/ — 在子目录中查找 agents/ 目录，最大深度 5
func discoverAgents(repoDir, searchPath string) []SubItem {
	// 优先级 1: searchPath/agents/ 标准布局
	if subs := scanAgentsMD(repoDir, searchPath+"/agents"); len(subs) > 0 {
		return subs
	}

	// 优先级 2: 递归搜索
	var results []SubItem
	findAgentsRecursive(repoDir, searchPath, 0, 5, &results)
	return results
}

// readSkillAt 尝试从指定路径直接读取 SKILL.md，返回单个 SubItem
func readSkillAt(repoDir, dirPath string) *SubItem {
	for _, fname := range []string{"SKILL.md", "skill.md"} {
		content, err := localReadFile(repoDir, dirPath+"/"+fname)
		if err != nil {
			continue
		}
		dirBase := filepath.Base(dirPath)
		if dirBase == "." || dirBase == "" {
			dirBase = "root"
		}
		skillName := dirBase
		description := "Skill: " + skillName

		if strings.HasPrefix(content, "---") {
			parts := strings.SplitN(content[3:], "---", 2)
			if len(parts) == 2 {
				fmName := extractYAMLField(parts[0], "name")
				fmDesc := extractYAMLField(parts[0], "description")
				// 验证: name 和 description 都必须非空才视为有效 frontmatter
				// （对齐 npx skills 的 readSkill 验证逻辑）
				if fmName != "" && fmDesc != "" {
					skillName = fmName
					description = fmDesc
				} else if fmName != "" {
					skillName = fmName
				} else if fmDesc != "" {
					description = fmDesc
				}
				// 如果 name 为空，保留 fallback 到目录名（已在上面设置）
			}
		}

		return &SubItem{
			Type:        SubTypeSkill,
			Name:        skillName,
			Description: description,
			Content:     content,
			LocalDir:    "skills/" + dirBase,
		}
	}
	return nil
}

// scanDirForSkills 扫描目录的直接子目录，每个子目录如果含 SKILL.md 则作为一个 skill
func scanDirForSkills(repoDir, dirPath string) []SubItem {
	entries, err := localListDir(repoDir, dirPath)
	if err != nil {
		return nil
	}

	var subs []SubItem
	for _, e := range entries {
		if !e.IsDir {
			continue
		}
		if sub := readSkillAt(repoDir, dirPath+"/"+e.Name); sub != nil {
			subs = append(subs, *sub)
		}
	}
	return subs
}

// findSkillsRecursive 递归搜索含 SKILL.md 的目录，最大深度 maxDepth
func findSkillsRecursive(repoDir, dirPath string, depth, maxDepth int, results *[]SubItem) {
	if depth >= maxDepth {
		return
	}
	entries, err := localListDir(repoDir, dirPath)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir {
			continue
		}
		if skipDirs[e.Name] {
			continue
		}
		subPath := dirPath + "/" + e.Name
		if sub := readSkillAt(repoDir, subPath); sub != nil {
			*results = append(*results, *sub)
		} else {
			findSkillsRecursive(repoDir, subPath, depth+1, maxDepth, results)
		}
	}
}

// findAgentsRecursive 递归搜索名为 "agents" 的目录，在其中扫描 .md agent 文件
func findAgentsRecursive(repoDir, dirPath string, depth, maxDepth int, results *[]SubItem) {
	if depth >= maxDepth {
		return
	}
	entries, err := localListDir(repoDir, dirPath)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir {
			continue
		}
		if skipDirs[e.Name] {
			continue
		}
		subPath := dirPath + "/" + e.Name
		// 找到 agents 目录，扫描其中的 .md 文件
		if e.Name == "agents" {
			if subs := scanAgentsMD(repoDir, subPath); len(subs) > 0 {
				*results = append(*results, subs...)
			}
		} else {
			findAgentsRecursive(repoDir, subPath, depth+1, maxDepth, results)
		}
	}
}

// ═══ Directory Scanning ═════════════════════════════════════════════

// scanSkillsDir 扫描指定目录的 skills，返回独立的 StoreItem（用于 ImportFromGithub 的 plugin.json 导入路径）
func scanSkillsDir(repoDir, dirPath, author, ghURL, storeDir string) ([]StoreItem, error) {
	subs := discoverSkills(repoDir, dirPath)
	if len(subs) == 0 {
		return nil, fmt.Errorf("no skills found in %s", dirPath)
	}

	var items []StoreItem
	for _, sub := range subs {
		item := StoreItem{
			Name:        sub.Name,
			Description: sub.Description,
			Type:        TypeSkill,
			GithubURL:   ghURL,
			Content:     sub.Content,
			Author:      author,
		}
		// 复制整个 skill 目录到 store/skills/{name}/
		if storeDir != "" && sub.LocalDir != "" {
			localDir := sanitizeDirName(sub.Name)
			destDir := filepath.Join(storeDir, localDir)
			// sub.LocalDir 格式为 "skills/xxx"，取最后部分找源目录
			srcRelPath := dirPath + "/" + filepath.Base(sub.LocalDir)
			if err := copyLocalDir(repoDir, srcRelPath, destDir); err == nil {
				item.LocalDir = localDir
			}
		}
		items = append(items, item)
	}
	return items, nil
}

// scanSkillPathsMD 根据显式路径列表逐个获取 skill（用于 plugin 导入，返回 SubItem）
func scanSkillPathsMD(repoDir string, paths []string) []SubItem {
	var subs []SubItem
	for _, p := range paths {
		p = strings.TrimPrefix(p, "./")
		if sub := readSkillAt(repoDir, p); sub != nil {
			subs = append(subs, *sub)
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

// ═══ Parsing & Utilities ════════════════════════════════════════════

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

// filterSkillsByName 按名称过滤 SubItem（大小写不敏感）
func filterSkillsByName(items []SubItem, name string) []SubItem {
	target := strings.ToLower(name)
	var result []SubItem
	for _, item := range items {
		if strings.ToLower(item.Name) == target {
			result = append(result, item)
		}
	}
	return result
}

// filterSkillItemsByName 按名称过滤 StoreItem（大小写不敏感）
// 对于 TypePlugin，过滤其 SubItems 中匹配的 skill；对于 TypeSkill，直接按名称匹配
func filterSkillItemsByName(items []StoreItem, name string) []StoreItem {
	target := strings.ToLower(name)
	var result []StoreItem
	for _, item := range items {
		if item.Type == TypePlugin {
			// 过滤 plugin 的子条目
			var filteredSubs []SubItem
			for _, sub := range item.SubItems {
				if sub.Type == SubTypeSkill && strings.ToLower(sub.Name) == target {
					filteredSubs = append(filteredSubs, sub)
				}
			}
			if len(filteredSubs) > 0 {
				filtered := item
				filtered.SubItems = filteredSubs
				result = append(result, filtered)
			}
		} else if strings.ToLower(item.Name) == target {
			result = append(result, item)
		}
	}
	return result
}

// getPluginSkillPaths 从 plugin manifest 中提取额外的 skill 搜索目录
// 扫描 .claude-plugin/marketplace.json 和 .claude-plugin/plugin.json 中声明的 skill 路径
func getPluginSkillPaths(repoDir, searchPath string) []string {
	var dirs []string
	seen := make(map[string]bool)

	addDir := func(skillPath string) {
		skillPath = strings.TrimPrefix(skillPath, "./")
		if skillPath == "" {
			return
		}
		// 取 skill 路径的父目录作为搜索目录
		parent := filepath.ToSlash(filepath.Dir(skillPath))
		if parent == "." {
			parent = searchPath
		} else if searchPath != "." {
			parent = searchPath + "/" + parent
		}
		if !seen[parent] {
			dirs = append(dirs, parent)
			seen[parent] = true
		}
	}

	// 1. marketplace.json → 每个 plugin 的 skills 数组
	if raw, err := localReadFile(repoDir, ".claude-plugin/marketplace.json"); err == nil {
		var mp MarketplaceJSON
		if json.Unmarshal([]byte(raw), &mp) == nil {
			for _, p := range mp.Plugins {
				for _, sp := range p.Skills {
					addDir(sp)
				}
			}
		}
	}

	// 2. plugin.json → skills 数组（如果有的话）
	if raw, err := localReadFile(repoDir, ".claude-plugin/plugin.json"); err == nil {
		var obj map[string]interface{}
		if json.Unmarshal([]byte(raw), &obj) == nil {
			if skills, ok := obj["skills"].([]interface{}); ok {
				for _, s := range skills {
					if sp, ok := s.(string); ok {
						addDir(sp)
					}
				}
			}
		}
	}

	return dirs
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
