package session

import (
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
	return &Manager{
		sessions:   make(map[string]*Session),
		workingDir: workingDir,
	}
}

// Create 创建新会话
func (m *Manager) Create(userID string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New().String()
	session := NewSession(id, userID, m.workingDir)
	m.sessions[id] = session
	return session
}

// Get 获取会话
func (m *Manager) Get(sessionID string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[sessionID]
}

// Delete 删除会话
func (m *Manager) Delete(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
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
