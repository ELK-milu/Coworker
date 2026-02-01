package workspace

import (
	"os"
	"path/filepath"
	"sync"
)

// Manager 用户工作空间管理器
type Manager struct {
	baseDir string
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

// GetUserWorkspace 获取用户工作空间路径
func (m *Manager) GetUserWorkspace(userID string) string {
	return filepath.Join(m.baseDir, userID)
}

// GetUserWorkDir 获取用户工作目录（用于 Claude 工具操作）
func (m *Manager) GetUserWorkDir(userID string) string {
	return filepath.Join(m.baseDir, userID, "workspace")
}

// GetUserClaudeDir 获取用户的 .claude 目录
func (m *Manager) GetUserClaudeDir(userID string) string {
	return filepath.Join(m.baseDir, userID, ".claude")
}

// GetUserSessionsDir 获取用户的会话存储目录
func (m *Manager) GetUserSessionsDir(userID string) string {
	return filepath.Join(m.baseDir, userID, ".claude", "sessions")
}

// EnsureUserWorkspace 确保用户工作空间存在
func (m *Manager) EnsureUserWorkspace(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dirs := []string{
		m.GetUserWorkspace(userID),
		m.GetUserWorkDir(userID),
		m.GetUserClaudeDir(userID),
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
	targetPath := filepath.Join(basePath, subPath)

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

	// 确保目录存在
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return nil, err
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

		// 计算相对路径
		relPath := filepath.Join(subPath, entry.Name())

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
