package session

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

// Manager 会话管理器
type Manager struct {
	sessions   map[string]*Session
	mu         sync.RWMutex
	baseDir    string // 用户工作空间基础目录
}

// NewManager 创建会话管理器
func NewManager(baseDir string) *Manager {
	m := &Manager{
		sessions: make(map[string]*Session),
		baseDir:  baseDir,
	}
	// 启动时加载所有用户的会话
	m.loadAllUserSessions()
	return m
}

// GetUserWorkDir 获取用户的工作目录
func (m *Manager) GetUserWorkDir(userID string) string {
	if m.baseDir != "" && userID != "" {
		return filepath.Join(m.baseDir, userID, "workspace")
	}
	return m.baseDir
}

// GetUserSessionDir 获取用户的会话存储目录
func (m *Manager) GetUserSessionDir(userID string) string {
	if m.baseDir != "" && userID != "" {
		return filepath.Join(m.baseDir, userID, ".coworker", "sessions")
	}
	return getSessionDir()
}

// loadAllUserSessions 加载所有用户的会话
func (m *Manager) loadAllUserSessions() {
	if m.baseDir == "" {
		// 兼容旧版本，从全局目录加载
		m.loadExistingSessions()
		return
	}

	// 遍历所有用户目录
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		log.Printf("[Session] Failed to read base dir %s: %v", m.baseDir, err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		userID := entry.Name()
		m.loadUserSessions(userID)
	}
}

// loadUserSessions 加载指定用户的会话
func (m *Manager) loadUserSessions(userID string) {
	sessionIDs, err := ListUserSessionFiles(userID)
	if err != nil {
		log.Printf("[Session] Failed to list session files for user %s: %v", userID, err)
		return
	}

	for _, id := range sessionIDs {
		sess, err := LoadUserSession(userID, id)
		if err != nil {
			log.Printf("[Session] Failed to load session %s for user %s: %v", id, userID, err)
			continue
		}
		m.sessions[id] = sess
		log.Printf("[Session] Loaded session: %s (user: %s)", id, userID)
	}
}

// loadExistingSessions 加载已有的会话文件（兼容旧版本）
func (m *Manager) loadExistingSessions() {
	sessionIDs, err := ListSessionFiles()
	if err != nil {
		log.Printf("[Session] Failed to list session files: %v", err)
		return
	}

	for _, id := range sessionIDs {
		sess, err := LoadSession(id)
		if err != nil {
			log.Printf("[Session] Failed to load session %s: %v", id, err)
			continue
		}
		m.sessions[id] = sess
		log.Printf("[Session] Loaded session: %s", id)
	}
}

// Create 创建新会话
func (m *Manager) Create(userID string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New().String()
	// 使用用户特定的工作目录
	workDir := m.GetUserWorkDir(userID)

	// 确保用户目录存在
	if m.baseDir != "" && userID != "" {
		sessDir := m.GetUserSessionDir(userID)
		if err := os.MkdirAll(sessDir, 0755); err != nil {
			log.Printf("[Session] Failed to create session dir for user %s: %v", userID, err)
		}
		if err := os.MkdirAll(workDir, 0755); err != nil {
			log.Printf("[Session] Failed to create work dir for user %s: %v", userID, err)
		}
	}

	session := NewSession(id, userID, workDir)
	m.sessions[id] = session

	// 保存到文件
	if err := SaveSession(session); err != nil {
		log.Printf("[Session] Failed to save new session %s: %v", id, err)
	}

	log.Printf("[Session] Created session %s for user %s, workDir: %s", id, userID, workDir)
	return session
}

// Get 获取会话
func (m *Manager) Get(sessionID string) *Session {
	if sessionID == "" {
		return nil
	}

	m.mu.RLock()
	sess := m.sessions[sessionID]
	m.mu.RUnlock()

	if sess != nil {
		return sess
	}

	// 尝试从文件加载（需要遍历所有用户目录）
	if m.baseDir != "" {
		entries, err := os.ReadDir(m.baseDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				userID := entry.Name()
				sess, err = LoadUserSession(userID, sessionID)
				if err == nil && sess != nil {
					m.mu.Lock()
					m.sessions[sessionID] = sess
					m.mu.Unlock()
					log.Printf("[Session] Loaded session from file: %s (user: %s)", sessionID, userID)
					return sess
				}
			}
		}
	}

	// 兼容旧版本：尝试从全局目录加载
	sess, err := LoadSession(sessionID)
	if err != nil {
		return nil
	}

	m.mu.Lock()
	m.sessions[sessionID] = sess
	m.mu.Unlock()

	log.Printf("[Session] Loaded session from file: %s", sessionID)
	return sess
}

// Delete 删除会话
func (m *Manager) Delete(sessionID string) {
	m.mu.Lock()
	sess := m.sessions[sessionID]
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	// 删除文件
	if sess != nil && sess.UserID != "" && m.baseDir != "" {
		// 从用户目录删除
		if err := DeleteUserSessionFile(sess.UserID, sessionID); err != nil {
			log.Printf("[Session] Failed to delete session file %s: %v", sessionID, err)
		}
	} else {
		// 兼容旧版本：从全局目录删除
		if err := DeleteSessionFile(sessionID); err != nil {
			log.Printf("[Session] Failed to delete session file %s: %v", sessionID, err)
		}
	}
}

// Save 保存会话到文件
func (m *Manager) Save(sessionID string) error {
	m.mu.RLock()
	sess := m.sessions[sessionID]
	m.mu.RUnlock()

	if sess == nil {
		return nil
	}

	return SaveSession(sess)
}

// List 列出用户的所有会话
func (m *Manager) List(userID string) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Session
	for _, s := range m.sessions {
		if s.UserID == userID {
			result = append(result, s)
		}
	}
	return result
}

// ListAll 列出所有会话
func (m *Manager) ListAll() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Session
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}
