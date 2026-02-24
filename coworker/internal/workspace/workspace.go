package workspace

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/model"
)

// Manager 用户工作空间管理器
type Manager struct {
	baseDir string
	useDB   bool
	mu      sync.RWMutex
}

// NewManager 创建工作空间管理器
func NewManager(baseDir string) *Manager {
	if baseDir == "" {
		baseDir = "./data/users"
	}
	return &Manager{
		baseDir: baseDir,
	}
}

// SetUseDB 设置是否使用数据库持久化
func (m *Manager) SetUseDB(useDB bool) {
	m.useDB = useDB
}

// GetUserWorkspace 获取用户工作空间路径
func (m *Manager) GetUserWorkspace(userID string) string {
	return filepath.Join(m.baseDir, userID)
}

// GetUserWorkDir 获取用户工作目录（用于 Claude 工具操作）
func (m *Manager) GetUserWorkDir(userID string) string {
	return filepath.Join(m.baseDir, userID, "workspace")
}

// GetUserCoworkerDir 获取用户的 .coworker 目录
func (m *Manager) GetUserCoworkerDir(userID string) string {
	return filepath.Join(m.baseDir, userID, ".coworker")
}

// GetUserSessionsDir 获取用户的会话存储目录
func (m *Manager) GetUserSessionsDir(userID string) string {
	return filepath.Join(m.baseDir, userID, ".coworker", "sessions")
}

// EnsureUserWorkspace 确保用户工作空间存在
func (m *Manager) EnsureUserWorkspace(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dirs := []string{
		m.GetUserWorkspace(userID),
		m.GetUserWorkDir(userID),
		m.GetUserCoworkerDir(userID),
		m.GetUserSessionsDir(userID),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// FileInfo 文件信息
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time"`
}

// ListFiles 列出用户工作空间中的文件
func (m *Manager) ListFiles(userID string, subPath string) ([]FileInfo, error) {
	basePath := m.GetUserWorkDir(userID)

	// 保存原始路径用于返回（使用正斜杠）
	originalSubPath := subPath

	// 将前端的正斜杠路径转换为系统路径
	systemSubPath := strings.ReplaceAll(subPath, "/", string(filepath.Separator))
	targetPath := filepath.Join(basePath, systemSubPath)

	// 安全检查：确保路径在用户工作空间内
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, err
	}
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return nil, err
	}

	// 检查是否在基础路径内
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil || len(rel) > 2 && rel[:2] == ".." {
		return nil, os.ErrPermission
	}

	// 检查目录是否存在，不存在则创建
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		if err := os.MkdirAll(targetPath, 0755); err != nil {
			return nil, err
		}
	}

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 计算相对路径，使用正斜杠以保持跨平台一致性
		var relPath string
		if originalSubPath == "" {
			relPath = entry.Name()
		} else {
			relPath = originalSubPath + "/" + entry.Name()
		}

		files = append(files, FileInfo{
			Name:    entry.Name(),
			Path:    relPath,
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Unix(),
		})
	}

	return files, nil
}

// GetWorkspaceStats 获取工作空间统计信息
func (m *Manager) GetWorkspaceStats(userID string) (map[string]interface{}, error) {
	workDir := m.GetUserWorkDir(userID)

	var totalSize int64
	var fileCount int
	var dirCount int

	err := filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误，继续遍历
		}
		if info.IsDir() {
			dirCount++
		} else {
			fileCount++
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_size":  totalSize,
		"file_count":  fileCount,
		"dir_count":   dirCount,
		"workspace":   workDir,
	}, nil
}

// CreateFolder 创建文件夹
func (m *Manager) CreateFolder(userID string, subPath string) error {
	basePath := m.GetUserWorkDir(userID)
	targetPath := filepath.Join(basePath, subPath)

	// 安全检查
	if err := m.validatePath(basePath, targetPath); err != nil {
		return err
	}

	return os.MkdirAll(targetPath, 0755)
}

// DeleteFile 删除文件或文件夹
func (m *Manager) DeleteFile(userID string, subPath string) error {
	basePath := m.GetUserWorkDir(userID)
	targetPath := filepath.Join(basePath, subPath)

	// 安全检查
	if err := m.validatePath(basePath, targetPath); err != nil {
		return err
	}

	// 检查文件是否存在
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", subPath)
	}

	return os.RemoveAll(targetPath)
}

// RenameFile 重命名文件或文件夹
func (m *Manager) RenameFile(userID string, oldPath string, newName string) error {
	basePath := m.GetUserWorkDir(userID)
	oldFullPath := filepath.Join(basePath, oldPath)

	// 安全检查
	if err := m.validatePath(basePath, oldFullPath); err != nil {
		return err
	}

	// 检查文件是否存在
	if _, err := os.Stat(oldFullPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", oldPath)
	}

	// 构建新路径
	dir := filepath.Dir(oldFullPath)
	newFullPath := filepath.Join(dir, newName)

	// 安全检查新路径
	if err := m.validatePath(basePath, newFullPath); err != nil {
		return err
	}

	return os.Rename(oldFullPath, newFullPath)
}

// ReadFile 读取文件内容（用于下载）
func (m *Manager) ReadFile(userID string, subPath string) ([]byte, error) {
	basePath := m.GetUserWorkDir(userID)
	targetPath := filepath.Join(basePath, subPath)

	// 安全检查
	if err := m.validatePath(basePath, targetPath); err != nil {
		return nil, err
	}

	// 检查是否是文件
	info, err := os.Stat(targetPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("cannot read directory")
	}

	return os.ReadFile(targetPath)
}

// WriteFile 写入文件内容（用于上传）
func (m *Manager) WriteFile(userID string, subPath string, content []byte) error {
	basePath := m.GetUserWorkDir(userID)
	targetPath := filepath.Join(basePath, subPath)

	// 安全检查
	if err := m.validatePath(basePath, targetPath); err != nil {
		return err
	}

	// 确保父目录存在
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(targetPath, content, 0644)
}

// SaveUploadedFile 保存上传的文件（从 io.Reader）
func (m *Manager) SaveUploadedFile(userID string, subPath string, reader io.Reader) error {
	basePath := m.GetUserWorkDir(userID)
	targetPath := filepath.Join(basePath, subPath)

	// 安全检查
	if err := m.validatePath(basePath, targetPath); err != nil {
		return err
	}

	// 确保父目录存在
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 创建目标文件
	file, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 复制内容
	_, err = io.Copy(file, reader)
	return err
}

// GetFileInfo 获取单个文件信息
func (m *Manager) GetFileInfo(userID string, subPath string) (*FileInfo, error) {
	basePath := m.GetUserWorkDir(userID)
	targetPath := filepath.Join(basePath, subPath)

	// 安全检查
	if err := m.validatePath(basePath, targetPath); err != nil {
		return nil, err
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return nil, err
	}

	return &FileInfo{
		Name:    info.Name(),
		Path:    subPath,
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		ModTime: info.ModTime().Unix(),
	}, nil
}

// GetAbsolutePath 获取文件的绝对路径（用于下载）
func (m *Manager) GetAbsolutePath(userID string, subPath string) (string, error) {
	basePath := m.GetUserWorkDir(userID)
	targetPath := filepath.Join(basePath, subPath)

	// 安全检查
	if err := m.validatePath(basePath, targetPath); err != nil {
		return "", err
	}

	return targetPath, nil
}

// validatePath 验证路径安全性
func (m *Manager) validatePath(basePath, targetPath string) error {
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return err
	}
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return err
	}

	// 检查是否在基础路径内
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return os.ErrPermission
	}
	if strings.HasPrefix(rel, "..") {
		return os.ErrPermission
	}

	return nil
}

// LoadConfig 加载用户配置文件 (COWORKER.md)
func (m *Manager) LoadConfig(userID string) (string, error) {
	configPath := filepath.Join(m.GetUserCoworkerDir(userID), "COWORKER.md")

	// 确保用户目录存在
	if err := m.EnsureUserWorkspace(userID); err != nil {
		return "", err
	}

	// 检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", nil // 文件不存在返回空字符串
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// SaveConfig 保存用户配置文件 (COWORKER.md)
func (m *Manager) SaveConfig(userID string, content string) error {
	// 确保用户目录存在
	if err := m.EnsureUserWorkspace(userID); err != nil {
		return err
	}

	configPath := filepath.Join(m.GetUserCoworkerDir(userID), "COWORKER.md")
	return os.WriteFile(configPath, []byte(content), 0644)
}

// UserStoreItem 用户已安装的商店条目
type UserStoreItem struct {
	ItemID  string            `json:"item_id"`
	Enabled bool              `json:"enabled"`
	Config  map[string]string `json:"config,omitempty"`
}

// UserInfo 用户信息
type UserInfo struct {
	UserName      string `json:"user_name"`
	CoworkerName  string `json:"coworker_name"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	ApiTokenKey   string `json:"api_token_key,omitempty"`
	ApiTokenName  string `json:"api_token_name,omitempty"`
	SelectedModel string `json:"selected_model,omitempty"`
	Group         string `json:"group,omitempty"`
	AssistantAvatar  string          `json:"assistant_avatar,omitempty"`
	InstalledItems   []UserStoreItem `json:"installed_items,omitempty"`
	// 采样参数（nil 表示使用默认值）
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"top_p,omitempty"`
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`
}

// LoadUserInfo 加载用户信息
func (m *Manager) LoadUserInfo(userID string) (*UserInfo, error) {
	// DB 路径
	if m.useDB {
		if dbUserID, err := strconv.Atoi(userID); err == nil {
			dbProfile, err := model.GetCoworkerUserProfile(dbUserID)
			if err == nil {
				return dbProfileToUserInfo(dbProfile), nil
			}
			// DB 未找到，尝试文件降级
		}
	}

	// 文件路径
	infoPath := filepath.Join(m.GetUserCoworkerDir(userID), "userinfo.json")

	// 确保用户目录存在
	if err := m.EnsureUserWorkspace(userID); err != nil {
		return nil, err
	}

	// 检查文件是否存在
	if _, err := os.Stat(infoPath); os.IsNotExist(err) {
		return &UserInfo{}, nil // 文件不存在返回空结构
	}

	content, err := os.ReadFile(infoPath)
	if err != nil {
		return nil, err
	}

	var info UserInfo
	if err := json.Unmarshal(content, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// SaveUserInfo 保存用户信息
func (m *Manager) SaveUserInfo(userID string, info *UserInfo) error {
	// DB 路径
	if m.useDB {
		if dbUserID, err := strconv.Atoi(userID); err == nil {
			dbProfile := userInfoToDBProfile(dbUserID, info)
			if err := model.UpsertCoworkerUserProfile(dbProfile); err != nil {
				log.Printf("[Workspace] DB save failed, falling back to file: %v", err)
			} else {
				return nil
			}
		}
	}

	// 文件路径
	// 确保用户目录存在
	if err := m.EnsureUserWorkspace(userID); err != nil {
		return err
	}

	content, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}

	infoPath := filepath.Join(m.GetUserCoworkerDir(userID), "userinfo.json")
	return os.WriteFile(infoPath, content, 0644)
}

// dbProfileToUserInfo 将 DB 模型转为 UserInfo
func dbProfileToUserInfo(p *model.CoworkerUserProfile) *UserInfo {
	var installedItems []UserStoreItem
	json.Unmarshal([]byte(p.InstalledItems), &installedItems)

	return &UserInfo{
		UserName:         p.UserName,
		CoworkerName:     p.CoworkerName,
		Phone:            p.Phone,
		Email:            p.Email,
		ApiTokenKey:      p.ApiTokenKey,
		ApiTokenName:     p.ApiTokenName,
		SelectedModel:    p.SelectedModel,
		Group:            p.Group,
		AssistantAvatar:  p.AssistantAvatar,
		InstalledItems:   installedItems,
		Temperature:      p.Temperature,
		TopP:             p.TopP,
		FrequencyPenalty: p.FrequencyPenalty,
		PresencePenalty:  p.PresencePenalty,
	}
}

// userInfoToDBProfile 将 UserInfo 转为 DB 模型（保留已有的 Profile 字段）
func userInfoToDBProfile(dbUserID int, info *UserInfo) *model.CoworkerUserProfile {
	installedJSON, _ := json.Marshal(info.InstalledItems)

	// 先读取已有的 DB profile 以保留 Profile 字段
	existing, _ := model.GetCoworkerUserProfile(dbUserID)

	p := &model.CoworkerUserProfile{
		UserID:           dbUserID,
		UserName:         info.UserName,
		CoworkerName:     info.CoworkerName,
		Phone:            info.Phone,
		Email:            info.Email,
		ApiTokenKey:      info.ApiTokenKey,
		ApiTokenName:     info.ApiTokenName,
		SelectedModel:    info.SelectedModel,
		Group:            info.Group,
		AssistantAvatar:  info.AssistantAvatar,
		InstalledItems:   string(installedJSON),
		Temperature:      info.Temperature,
		TopP:             info.TopP,
		FrequencyPenalty: info.FrequencyPenalty,
		PresencePenalty:  info.PresencePenalty,
	}

	// 保留已有的 Profile 字段
	if existing != nil {
		p.ID = existing.ID
		p.Languages = existing.Languages
		p.Frameworks = existing.Frameworks
		p.CodingStyle = existing.CodingStyle
		p.ResponseStyle = existing.ResponseStyle
		p.UILanguage = existing.UILanguage
		p.CurrentProjects = existing.CurrentProjects
		p.TotalSessions = existing.TotalSessions
		p.TotalMessages = existing.TotalMessages
		p.TopTools = existing.TopTools
	}

	return p
}
