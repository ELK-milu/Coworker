package profile

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/model"
)

// UserProfile 用户画像
type UserProfile struct {
	UserID string `json:"user_id"`

	// 编程偏好
	Languages   []string          `json:"languages"`    // 常用语言
	Frameworks  []string          `json:"frameworks"`   // 常用框架
	CodingStyle map[string]string `json:"coding_style"` // 代码风格偏好

	// 交互偏好
	ResponseStyle string `json:"response_style"` // concise/detailed/balanced
	Language      string `json:"language"`       // 交互语言 (zh-CN, en-US)

	// 项目上下文
	CurrentProjects []ProjectContext `json:"current_projects"`

	// 统计信息
	TotalSessions int            `json:"total_sessions"`
	TotalMessages int            `json:"total_messages"`
	TopTools      map[string]int `json:"top_tools"` // 工具使用统计

	// 元数据
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

// ProjectContext 项目上下文
type ProjectContext struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	TechStack   []string `json:"tech_stack"`
	Description string   `json:"description"`
	LastAccess  int64    `json:"last_access"`
}

// Manager 用户画像管理器
type Manager struct {
	baseDir  string
	useDB    bool
	profiles map[string]*UserProfile
	mu       sync.RWMutex
}

// NewManager 创建管理器
func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir:  baseDir,
		profiles: make(map[string]*UserProfile),
	}
}

// SetUseDB 设置是否使用数据库持久化
func (m *Manager) SetUseDB(useDB bool) {
	m.useDB = useDB
}

// parseProfileUserID 将 string UserID 转为 int（用于 DB）
func parseProfileUserID(userID string) (int, bool) {
	id, err := strconv.Atoi(userID)
	if err != nil {
		return 0, false
	}
	return id, true
}

// dbModelToProfile 将 DB 模型转为 UserProfile
func dbModelToProfile(dbProfile *model.CoworkerUserProfile, userID string) *UserProfile {
	var languages []string
	json.Unmarshal([]byte(dbProfile.Languages), &languages)
	if languages == nil {
		languages = []string{}
	}

	var frameworks []string
	json.Unmarshal([]byte(dbProfile.Frameworks), &frameworks)
	if frameworks == nil {
		frameworks = []string{}
	}

	var codingStyle map[string]string
	json.Unmarshal([]byte(dbProfile.CodingStyle), &codingStyle)
	if codingStyle == nil {
		codingStyle = make(map[string]string)
	}

	var projects []ProjectContext
	json.Unmarshal([]byte(dbProfile.CurrentProjects), &projects)
	if projects == nil {
		projects = []ProjectContext{}
	}

	var topTools map[string]int
	json.Unmarshal([]byte(dbProfile.TopTools), &topTools)
	if topTools == nil {
		topTools = make(map[string]int)
	}

	return &UserProfile{
		UserID:          userID,
		Languages:       languages,
		Frameworks:      frameworks,
		CodingStyle:     codingStyle,
		ResponseStyle:   dbProfile.ResponseStyle,
		Language:        dbProfile.UILanguage,
		CurrentProjects: projects,
		TotalSessions:   dbProfile.TotalSessions,
		TotalMessages:   dbProfile.TotalMessages,
		TopTools:        topTools,
		CreatedAt:       dbProfile.CreatedAt,
		UpdatedAt:       dbProfile.UpdatedAt,
	}
}

// saveProfileToDB 保存 Profile 字段到 DB（保留 UserInfo 字段）
func (m *Manager) saveProfileToDB(userID string, p *UserProfile) error {
	dbUserID, ok := parseProfileUserID(userID)
	if !ok {
		return nil // 非数字 userID，跳过 DB
	}

	languagesJSON, _ := json.Marshal(p.Languages)
	frameworksJSON, _ := json.Marshal(p.Frameworks)
	codingStyleJSON, _ := json.Marshal(p.CodingStyle)
	projectsJSON, _ := json.Marshal(p.CurrentProjects)
	topToolsJSON, _ := json.Marshal(p.TopTools)

	// 读取已有的 DB profile 以保留 UserInfo 字段
	existing, _ := model.GetCoworkerUserProfile(dbUserID)
	if existing == nil {
		// 不存在，创建新记录（仅 Profile 字段）
		return model.UpsertCoworkerUserProfile(&model.CoworkerUserProfile{
			UserID:          dbUserID,
			Languages:       string(languagesJSON),
			Frameworks:      string(frameworksJSON),
			CodingStyle:     string(codingStyleJSON),
			ResponseStyle:   p.ResponseStyle,
			UILanguage:      p.Language,
			CurrentProjects: string(projectsJSON),
			TotalSessions:   p.TotalSessions,
			TotalMessages:   p.TotalMessages,
			TopTools:        string(topToolsJSON),
		})
	}

	// 已存在，只更新 Profile 字段
	existing.Languages = string(languagesJSON)
	existing.Frameworks = string(frameworksJSON)
	existing.CodingStyle = string(codingStyleJSON)
	existing.ResponseStyle = p.ResponseStyle
	existing.UILanguage = p.Language
	existing.CurrentProjects = string(projectsJSON)
	existing.TotalSessions = p.TotalSessions
	existing.TotalMessages = p.TotalMessages
	existing.TopTools = string(topToolsJSON)

	return model.UpdateCoworkerUserProfile(existing)
}

// Get 获取用户画像
func (m *Manager) Get(userID string) (*UserProfile, error) {
	m.mu.RLock()
	if p, ok := m.profiles[userID]; ok {
		m.mu.RUnlock()
		return p, nil
	}
	m.mu.RUnlock()

	// DB 路径
	if m.useDB {
		if dbUserID, ok := parseProfileUserID(userID); ok {
			dbProfile, err := model.GetCoworkerUserProfile(dbUserID)
			if err == nil {
				p := dbModelToProfile(dbProfile, userID)
				m.mu.Lock()
				m.profiles[userID] = p
				m.mu.Unlock()
				return p, nil
			}
		}
	}

	// 尝试从文件加载
	return m.Load(userID)
}

// GetOrCreate 获取或创建用户画像
func (m *Manager) GetOrCreate(userID string) *UserProfile {
	p, err := m.Get(userID)
	if err != nil || p == nil {
		p = m.createDefault(userID)
		m.Save(userID, p)
	}
	return p
}

// createDefault 创建默认画像
func (m *Manager) createDefault(userID string) *UserProfile {
	now := time.Now().Unix()
	return &UserProfile{
		UserID:          userID,
		Languages:       []string{},
		Frameworks:      []string{},
		CodingStyle:     make(map[string]string),
		ResponseStyle:   "balanced",
		Language:        "zh-CN",
		CurrentProjects: []ProjectContext{},
		TopTools:        make(map[string]int),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

// Update 更新用户画像
func (m *Manager) Update(userID string, updates map[string]interface{}) error {
	p := m.GetOrCreate(userID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// 应用更新
	if langs, ok := updates["languages"].([]string); ok {
		p.Languages = langs
	}
	if frameworks, ok := updates["frameworks"].([]string); ok {
		p.Frameworks = frameworks
	}
	if style, ok := updates["response_style"].(string); ok {
		p.ResponseStyle = style
	}
	if lang, ok := updates["language"].(string); ok {
		p.Language = lang
	}
	if codingStyle, ok := updates["coding_style"].(map[string]string); ok {
		p.CodingStyle = codingStyle
	}

	p.UpdatedAt = time.Now().Unix()
	m.profiles[userID] = p

	// DB 路径
	if m.useDB {
		if err := m.saveProfileToDB(userID, p); err != nil {
			log.Printf("[Profile] DB save failed, falling back to file: %v", err)
		} else {
			return nil
		}
	}

	return m.saveToFile(userID, p)
}

// RecordToolUsage 记录工具使用
func (m *Manager) RecordToolUsage(userID, toolName string) {
	p := m.GetOrCreate(userID)

	m.mu.Lock()
	defer m.mu.Unlock()

	if p.TopTools == nil {
		p.TopTools = make(map[string]int)
	}
	p.TopTools[toolName]++
	p.UpdatedAt = time.Now().Unix()

	if m.useDB {
		if err := m.saveProfileToDB(userID, p); err != nil {
			log.Printf("[Profile] DB save failed for tool usage: %v", err)
		}
		return
	}

	m.saveToFile(userID, p)
}

// RecordSession 记录会话
func (m *Manager) RecordSession(userID string) {
	p := m.GetOrCreate(userID)

	m.mu.Lock()
	defer m.mu.Unlock()

	p.TotalSessions++
	p.UpdatedAt = time.Now().Unix()

	// DB 路径（使用原子递增）
	if m.useDB {
		if dbUserID, ok := parseProfileUserID(userID); ok {
			if err := model.IncrementCoworkerUserProfileSessions(dbUserID); err != nil {
				log.Printf("[Profile] DB increment sessions failed: %v", err)
			}
		}
		return
	}

	m.saveToFile(userID, p)
}

// RecordMessage 记录消息
func (m *Manager) RecordMessage(userID string) {
	p := m.GetOrCreate(userID)

	m.mu.Lock()
	defer m.mu.Unlock()

	p.TotalMessages++
	p.UpdatedAt = time.Now().Unix()

	// DB 路径（使用原子递增）
	if m.useDB {
		if dbUserID, ok := parseProfileUserID(userID); ok {
			if err := model.IncrementCoworkerUserProfileMessages(dbUserID); err != nil {
				log.Printf("[Profile] DB increment messages failed: %v", err)
			}
		}
		return
	}

	m.saveToFile(userID, p)
}

// AddProject 添加项目
func (m *Manager) AddProject(userID string, project ProjectContext) {
	p := m.GetOrCreate(userID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在
	for i, proj := range p.CurrentProjects {
		if proj.Path == project.Path {
			p.CurrentProjects[i] = project
			p.CurrentProjects[i].LastAccess = time.Now().Unix()
			m.persistProfile(userID, p)
			return
		}
	}

	// 添加新项目
	project.LastAccess = time.Now().Unix()
	p.CurrentProjects = append(p.CurrentProjects, project)

	// 限制项目数量
	if len(p.CurrentProjects) > 10 {
		p.CurrentProjects = p.CurrentProjects[len(p.CurrentProjects)-10:]
	}

	p.UpdatedAt = time.Now().Unix()
	m.persistProfile(userID, p)
}

// persistProfile 持久化 Profile（选择 DB 或文件）
func (m *Manager) persistProfile(userID string, p *UserProfile) {
	if m.useDB {
		if err := m.saveProfileToDB(userID, p); err != nil {
			log.Printf("[Profile] DB save failed, falling back to file: %v", err)
			m.saveToFile(userID, p)
		}
		return
	}
	m.saveToFile(userID, p)
}

// getProfilePath 获取画像文件路径
func (m *Manager) getProfilePath(userID string) string {
	return filepath.Join(m.baseDir, userID, ".claude", "profile.json")
}

// Load 从文件加载画像
func (m *Manager) Load(userID string) (*UserProfile, error) {
	path := m.getProfilePath(userID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var p UserProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.profiles[userID] = &p
	m.mu.Unlock()

	return &p, nil
}

// Save 保存画像
func (m *Manager) Save(userID string, p *UserProfile) error {
	m.mu.Lock()
	m.profiles[userID] = p
	m.mu.Unlock()

	if m.useDB {
		if err := m.saveProfileToDB(userID, p); err != nil {
			log.Printf("[Profile] DB save failed, falling back to file: %v", err)
		} else {
			return nil
		}
	}

	return m.saveToFile(userID, p)
}

// saveToFile 保存画像到文件（内部方法，不加锁）
func (m *Manager) saveToFile(userID string, p *UserProfile) error {
	path := m.getProfilePath(userID)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// RenderForPrompt 渲染用户画像用于系统提示词
func (m *Manager) RenderForPrompt(p *UserProfile) string {
	if p == nil {
		return ""
	}

	var parts []string

	if len(p.Languages) > 0 {
		parts = append(parts, "- Preferred Languages: "+strings.Join(p.Languages, ", "))
	}
	if len(p.Frameworks) > 0 {
		parts = append(parts, "- Preferred Frameworks: "+strings.Join(p.Frameworks, ", "))
	}
	if p.ResponseStyle != "" {
		parts = append(parts, "- Response Style: "+p.ResponseStyle)
	}
	if p.Language != "" {
		parts = append(parts, "- UI Language: "+p.Language)
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n")
}
