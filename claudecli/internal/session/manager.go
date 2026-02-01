package session

import (
	"log"
	"sync"

	"github.com/google/uuid"
)

// Manager 会话管理器
type Manager struct {
	sessions   map[string]*Session
	mu         sync.RWMutex
	workingDir string
}

// NewManager 创建会话管理器
func NewManager(workingDir string) *Manager {
	m := &Manager{
		sessions:   make(map[string]*Session),
		workingDir: workingDir,
	}
	// 启动时加载已有会话
	m.loadExistingSessions()
	return m
}

// loadExistingSessions 加载已有的会话文件
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
	session := NewSession(id, userID, m.workingDir)
	m.sessions[id] = session

	// 保存到文件
	if err := SaveSession(session); err != nil {
		log.Printf("[Session] Failed to save new session %s: %v", id, err)
	}

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

	// 尝试从文件加载
	sess, err := LoadSession(sessionID)
	if err != nil {
		return nil
	}

	// 加载成功，添加到内存
	m.mu.Lock()
	m.sessions[sessionID] = sess
	m.mu.Unlock()

	log.Printf("[Session] Loaded session from file: %s", sessionID)
	return sess
}

// Delete 删除会话
func (m *Manager) Delete(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)

	// 删除文件
	if err := DeleteSessionFile(sessionID); err != nil {
		log.Printf("[Session] Failed to delete session file %s: %v", sessionID, err)
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
