package store

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manager 技能商店管理器
type Manager struct {
	dataDir string
	mu      sync.RWMutex
	items   []StoreItem
}

// NewManager 创建商店管理器
func NewManager(baseDir string) *Manager {
	m := &Manager{dataDir: filepath.Join(baseDir, "store")}
	m.load()
	return m
}

func (m *Manager) filePath() string {
	return filepath.Join(m.dataDir, "items.json")
}

func (m *Manager) load() {
	data, err := os.ReadFile(m.filePath())
	if err != nil {
		m.items = []StoreItem{}
		return
	}
	json.Unmarshal(data, &m.items)
}

func (m *Manager) save() error {
	os.MkdirAll(m.dataDir, 0755)
	data, err := json.MarshalIndent(m.items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath(), data, 0644)
}

// List 列出所有条目
func (m *Manager) List() []StoreItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]StoreItem, len(m.items))
	copy(result, m.items)
	return result
}

// Create 创建条目
func (m *Manager) Create(item StoreItem) (StoreItem, error) {
	if item.GithubURL != "" && item.Content == "" {
		if content, err := fetchGithubContent(item.GithubURL); err == nil {
			item.Content = content
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	item.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()
	m.items = append(m.items, item)
	return item, m.save()
}

// Update 更新条目
func (m *Manager) Update(id string, item StoreItem) (StoreItem, error) {
	if item.GithubURL != "" && item.Content == "" {
		if content, err := fetchGithubContent(item.GithubURL); err == nil {
			item.Content = content
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, s := range m.items {
		if s.ID == id {
			item.ID = id
			item.CreatedAt = s.CreatedAt
			item.UpdatedAt = time.Now()
			m.items[i] = item
			return item, m.save()
		}
	}
	return StoreItem{}, fmt.Errorf("item not found: %s", id)
}

// Delete 删除条目
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, s := range m.items {
		if s.ID == id {
			m.items = append(m.items[:i], m.items[i+1:]...)
			return m.save()
		}
	}
	return fmt.Errorf("item not found: %s", id)
}

// Import 从 GitHub 导入 skills，跳过已存在的同名条目
func (m *Manager) Import(repoURL string) ([]StoreItem, error) {
	parsed, err := ImportFromGithub(repoURL)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing := make(map[string]bool)
	for _, item := range m.items {
		existing[item.Name] = true
	}

	var added []StoreItem
	for _, item := range parsed {
		if existing[item.Name] {
			continue
		}
		item.ID = fmt.Sprintf("%d", time.Now().UnixNano())
		item.CreatedAt = time.Now()
		item.UpdatedAt = time.Now()
		m.items = append(m.items, item)
		added = append(added, item)
		existing[item.Name] = true
		time.Sleep(time.Millisecond) // 确保 ID 唯一
	}

	if len(added) > 0 {
		if err := m.save(); err != nil {
			return nil, err
		}
	}
	return added, nil
}

// fetchGithubContent 从 GitHub URL 获取内容
func fetchGithubContent(githubURL string) (string, error) {
	rawURL := githubURL
	if strings.Contains(githubURL, "github.com") && strings.Contains(githubURL, "/blob/") {
		rawURL = strings.Replace(githubURL, "github.com", "raw.githubusercontent.com", 1)
		rawURL = strings.Replace(rawURL, "/blob/", "/", 1)
	}
	resp, err := http.Get(rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
