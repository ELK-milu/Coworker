package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
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

// Get 获取用户画像
func (m *Manager) Get(userID string) (*UserProfile, error) {
	m.mu.RLock()
	if p, ok := m.profiles[userID]; ok {
		m.mu.RUnlock()
		return p, nil
	}
	m.mu.RUnlock()

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

	return m.save(userID, p)
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

	m.save(userID, p)
}

// RecordSession 记录会话
func (m *Manager) RecordSession(userID string) {
	p := m.GetOrCreate(userID)

	m.mu.Lock()
	defer m.mu.Unlock()

	p.TotalSessions++
	p.UpdatedAt = time.Now().Unix()

	m.save(userID, p)
}

// RecordMessage 记录消息
func (m *Manager) RecordMessage(userID string) {
	p := m.GetOrCreate(userID)

	m.mu.Lock()
	defer m.mu.Unlock()

	p.TotalMessages++
	p.UpdatedAt = time.Now().Unix()

	m.save(userID, p)
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
			m.save(userID, p)
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
	m.save(userID, p)
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

// Save 保存画像到文件
func (m *Manager) Save(userID string, p *UserProfile) error {
	m.mu.Lock()
	m.profiles[userID] = p
	m.mu.Unlock()

	return m.save(userID, p)
}

// save 内部保存方法（不加锁）
func (m *Manager) save(userID string, p *UserProfile) error {
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
