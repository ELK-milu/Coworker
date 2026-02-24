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
