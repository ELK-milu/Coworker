package session

import (
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
	"sync"
	"time"
)

// Session 会话
type Session struct {
	ID         string
	UserID     string
	Messages   []types.Message
	CreatedAt  time.Time
	UpdatedAt  time.Time
	TotalCost  float64
	WorkingDir string
	mu         sync.RWMutex
}

// NewSession 创建新会话
func NewSession(id, userID, workingDir string) *Session {
	return &Session{
		ID:         id,
		UserID:     userID,
		Messages:   make([]types.Message, 0),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		WorkingDir: workingDir,
	}
}

// AddMessage 添加消息
func (s *Session) AddMessage(msg types.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

// GetMessages 获取所有消息
func (s *Session) GetMessages() []types.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]types.Message, len(s.Messages))
	copy(result, s.Messages)
	return result
}
