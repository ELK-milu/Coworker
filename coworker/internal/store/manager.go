package store

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/model"
)

// Manager 技能商店管理器
type Manager struct {
	dataDir string
	useDB   bool
	mu      sync.RWMutex
	items   []StoreItem
}

// NewManager 创建商店管理器
func NewManager(baseDir string) *Manager {
	m := &Manager{dataDir: filepath.Join(baseDir, "store")}
	m.load()
	return m
}

// SetUseDB 设置是否使用数据库持久化
func (m *Manager) SetUseDB(useDB bool) {
	m.useDB = useDB
	if useDB {
		// 从 DB 重新加载
		m.loadFromDB()
	}
}

// storeItemToDBModel 将 StoreItem 转为 DB 模型
func storeItemToDBModel(item StoreItem) *model.CoworkerStoreItem {
	configJSON, _ := json.Marshal(item.ConfigSchema)
	subItemsJSON, _ := json.Marshal(item.SubItems)
	return &model.CoworkerStoreItem{
		ID:           item.ID,
		Name:         item.Name,
		Description:  item.Description,
		DisplayName:  item.DisplayName,
		DisplayDesc:  item.DisplayDesc,
		Type:         string(item.Type),
		Icon:         item.Icon,
		Author:       item.Author,
		GithubURL:    item.GithubURL,
		Content:      item.Content,
		LocalDir:     item.LocalDir,
		ServerURL:    item.ServerURL,
		ConfigSchema: string(configJSON),
		SubItems:     string(subItemsJSON),
		CreatedAt:    item.CreatedAt.Unix(),
		UpdatedAt:    item.UpdatedAt.Unix(),
	}
}

// dbModelToStoreItem 将 DB 模型转为 StoreItem
func dbModelToStoreItem(dbItem *model.CoworkerStoreItem) StoreItem {
	var configSchema []ConfigField
	json.Unmarshal([]byte(dbItem.ConfigSchema), &configSchema)

	var subItems []SubItem
	json.Unmarshal([]byte(dbItem.SubItems), &subItems)

	return StoreItem{
		ID:           dbItem.ID,
		Name:         dbItem.Name,
		Description:  dbItem.Description,
		DisplayName:  dbItem.DisplayName,
		DisplayDesc:  dbItem.DisplayDesc,
		Type:         ItemType(dbItem.Type),
		Icon:         dbItem.Icon,
		Author:       dbItem.Author,
		GithubURL:    dbItem.GithubURL,
		Content:      dbItem.Content,
		LocalDir:     dbItem.LocalDir,
		ServerURL:    dbItem.ServerURL,
		ConfigSchema: configSchema,
		SubItems:     subItems,
		CreatedAt:    time.Unix(dbItem.CreatedAt, 0),
		UpdatedAt:    time.Unix(dbItem.UpdatedAt, 0),
	}
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

// loadFromDB 从数据库加载所有条目到内存
func (m *Manager) loadFromDB() {
	dbItems, err := model.ListCoworkerStoreItems()
	if err != nil {
		log.Printf("[Store] DB load failed: %v", err)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.items = make([]StoreItem, 0, len(dbItems))
	for _, dbItem := range dbItems {
		m.items = append(m.items, dbModelToStoreItem(dbItem))
	}
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

	// DB 路径
	if m.useDB {
		dbItem := storeItemToDBModel(item)
		if err := model.CreateCoworkerStoreItem(dbItem); err != nil {
			log.Printf("[Store] DB create failed: %v", err)
		}
		return item, nil
	}

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

			// DB 路径
			if m.useDB {
				dbItem := storeItemToDBModel(item)
				if err := model.UpdateCoworkerStoreItem(dbItem); err != nil {
					log.Printf("[Store] DB update failed: %v", err)
				}
				return item, nil
			}

			return item, m.save()
		}
	}
	return StoreItem{}, fmt.Errorf("item not found: %s", id)
}

// Delete 删除条目（同时清理本地 skill 目录）
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, s := range m.items {
		if s.ID == id {
			// 清理本地 skill 目录
			if s.LocalDir != "" {
				skillDir := filepath.Join(m.dataDir, "skills", s.LocalDir)
				os.RemoveAll(skillDir)
			}
			// 清理 plugin 目录
			if s.Type == TypePlugin && s.LocalDir != "" {
				pluginDir := filepath.Join(m.dataDir, "plugins", s.LocalDir)
				os.RemoveAll(pluginDir)
			}
			m.items = append(m.items[:i], m.items[i+1:]...)

			// DB 路径
			if m.useDB {
				if err := model.DeleteCoworkerStoreItem(id); err != nil {
					log.Printf("[Store] DB delete failed: %v", err)
				}
				return nil
			}

			return m.save()
		}
	}
	return fmt.Errorf("item not found: %s", id)
}

// SkillDir 返回 skill 的全局目录绝对路径
func (m *Manager) SkillDir(item *StoreItem) string {
	if item == nil || item.LocalDir == "" {
		return ""
	}
	return filepath.Join(m.dataDir, "skills", item.LocalDir)
}

// PluginDir 返回 plugin 的全局目录绝对路径
func (m *Manager) PluginDir(item *StoreItem) string {
	if item == nil || item.LocalDir == "" || item.Type != TypePlugin {
		return ""
	}
	return filepath.Join(m.dataDir, "plugins", item.LocalDir)
}

// CopySkillsToWorkspace 将用户已安装的 skill 复制到 workspace/.skills/
func (m *Manager) CopySkillsToWorkspace(userID, workspaceDir string) error {
	ids := m.LoadUserInstalled(userID)
	if len(ids) == 0 {
		return nil
	}

	targetBase := filepath.Join(workspaceDir, ".skills")

	// 收集需要复制的 skill
	type skillCopy struct {
		srcDir string
		name   string
	}
	var toCopy []skillCopy

	m.mu.RLock()
	for _, id := range ids {
		for _, item := range m.items {
			if item.ID != id {
				continue
			}
			if item.Type == TypeSkill && item.LocalDir != "" {
				srcDir := filepath.Join(m.dataDir, "skills", item.LocalDir)
				if info, err := os.Stat(srcDir); err == nil && info.IsDir() {
					toCopy = append(toCopy, skillCopy{srcDir: srcDir, name: item.LocalDir})
				}
			}
			if item.Type == TypePlugin && item.LocalDir != "" {
				// 扫描 plugin 目录下的 skills/ 子目录
				pluginSkillsDir := filepath.Join(m.dataDir, "plugins", item.LocalDir, "skills")
				if info, err := os.Stat(pluginSkillsDir); err == nil && info.IsDir() {
					entries, _ := os.ReadDir(pluginSkillsDir)
					for _, entry := range entries {
						if entry.IsDir() {
							srcDir := filepath.Join(pluginSkillsDir, entry.Name())
							toCopy = append(toCopy, skillCopy{srcDir: srcDir, name: entry.Name()})
						}
					}
				}
			}
			break
		}
	}
	m.mu.RUnlock()

	if len(toCopy) == 0 {
		return nil
	}

	// 清空 .skills/ 再复制（保证一致性）
	os.RemoveAll(targetBase)
	os.MkdirAll(targetBase, 0755)

	for _, sc := range toCopy {
		dst := filepath.Join(targetBase, sc.name)
		if err := copyDir(sc.srcDir, dst); err != nil {
			return fmt.Errorf("copy skill %s: %w", sc.name, err)
		}
	}
	return nil
}

// copyDir 递归复制目录
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
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

// Import 从 GitHub 导入 skills，跳过已存在的同名条目
func (m *Manager) Import(repoURL string) ([]StoreItem, error) {
	skillsDir := filepath.Join(m.dataDir, "skills")
	parsed, err := ImportFromGithub(repoURL, skillsDir)
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

		// DB 路径
		if m.useDB {
			dbItem := storeItemToDBModel(item)
			if err := model.CreateCoworkerStoreItem(dbItem); err != nil {
				log.Printf("[Store] DB create failed for import item %s: %v", item.Name, err)
			}
		}

		time.Sleep(time.Millisecond) // 确保 ID 唯一
	}

	if len(added) > 0 && !m.useDB {
		if err := m.save(); err != nil {
			return nil, err
		}
	}
	return added, nil
}

// ImportAgents 从 GitHub 导入独立 agents
func (m *Manager) ImportAgents(repoURL string) ([]StoreItem, error) {
	parsed, err := ImportAgentsFromGithub(repoURL)
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

		// DB 路径
		if m.useDB {
			dbItem := storeItemToDBModel(item)
			if err := model.CreateCoworkerStoreItem(dbItem); err != nil {
				log.Printf("[Store] DB create failed for import agent %s: %v", item.Name, err)
			}
		}

		time.Sleep(time.Millisecond) // 确保 ID 唯一
	}

	if len(added) > 0 && !m.useDB {
		if err := m.save(); err != nil {
			return nil, err
		}
	}
	return added, nil
}

// ImportPlugin 从 GitHub 导入插件（完整的 agents + skills + commands）
func (m *Manager) ImportPlugin(repoURL string) ([]StoreItem, error) {
	pluginsDir := filepath.Join(m.dataDir, "plugins")
	parsed, err := ImportPluginFromGithub(repoURL, pluginsDir)
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

		// DB 路径
		if m.useDB {
			dbItem := storeItemToDBModel(item)
			if err := model.CreateCoworkerStoreItem(dbItem); err != nil {
				log.Printf("[Store] DB create failed for import plugin %s: %v", item.Name, err)
			}
		}

		time.Sleep(time.Millisecond) // 确保 ID 唯一
	}

	if len(added) > 0 && !m.useDB {
		if err := m.save(); err != nil {
			return nil, err
		}
	}
	return added, nil
}

// installedItemRef 用户已安装条目引用（DB JSON 格式）
type installedItemRef struct {
	ItemID  string `json:"item_id"`
	Enabled bool   `json:"enabled"`
}

// UserInstalled 用户已安装的技能 ID 列表
type UserInstalled struct {
	ItemIDs []string `json:"item_ids"`
}

func (m *Manager) userInstalledPath(userID string) string {
	return filepath.Join(m.dataDir, "installed", userID+".json")
}

// LoadUserInstalled 加载用户已安装的技能 ID 列表
func (m *Manager) LoadUserInstalled(userID string) []string {
	// DB 路径
	if m.useDB {
		if dbUserID, err := strconv.Atoi(userID); err == nil {
			dbProfile, err := model.GetCoworkerUserProfile(dbUserID)
			if err == nil && dbProfile.InstalledItems != "" {
				var refs []installedItemRef
				if err := json.Unmarshal([]byte(dbProfile.InstalledItems), &refs); err == nil {
					ids := make([]string, 0, len(refs))
					for _, ref := range refs {
						if ref.Enabled {
							ids = append(ids, ref.ItemID)
						}
					}
					return ids
				}
			}
		}
		return []string{}
	}

	// 文件路径
	data, err := os.ReadFile(m.userInstalledPath(userID))
	if err != nil {
		return []string{}
	}
	var u UserInstalled
	if err := json.Unmarshal(data, &u); err != nil {
		return []string{}
	}
	return u.ItemIDs
}

// SaveUserInstalled 保存用户已安装的技能 ID 列表
func (m *Manager) SaveUserInstalled(userID string, itemIDs []string) error {
	// DB 路径
	if m.useDB {
		if dbUserID, err := strconv.Atoi(userID); err == nil {
			// 转换为 installedItemRef 格式
			refs := make([]installedItemRef, 0, len(itemIDs))
			for _, id := range itemIDs {
				refs = append(refs, installedItemRef{ItemID: id, Enabled: true})
			}
			refsJSON, _ := json.Marshal(refs)

			// 读取已有 profile，更新 InstalledItems 字段
			existing, _ := model.GetCoworkerUserProfile(dbUserID)
			if existing == nil {
				// 创建新 profile（仅 InstalledItems）
				return model.UpsertCoworkerUserProfile(&model.CoworkerUserProfile{
					UserID:         dbUserID,
					InstalledItems: string(refsJSON),
				})
			}
			existing.InstalledItems = string(refsJSON)
			return model.UpdateCoworkerUserProfile(existing)
		}
	}

	// 文件路径
	p := m.userInstalledPath(userID)
	os.MkdirAll(filepath.Dir(p), 0755)
	data, err := json.MarshalIndent(UserInstalled{ItemIDs: itemIDs}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// GetByID 根据 ID 获取条目
func (m *Manager) GetByID(id string) *StoreItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, item := range m.items {
		if item.ID == id {
			cp := item
			return &cp
		}
	}

	// DB 降级（缓存未命中）
	if m.useDB {
		dbItem, err := model.GetCoworkerStoreItem(id)
		if err == nil {
			item := dbModelToStoreItem(dbItem)
			return &item
		}
	}

	return nil
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
