package session

import (
	"sync"
	"time"

	"github.com/QuantumNous/new-api/claudecli/internal/context"
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
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
	Context    *context.Manager // 上下文管理器
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
		Context:    context.NewManager(nil),
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

// GetContextStats 获取上下文统计
func (s *Session) GetContextStats() context.Stats {
	if s.Context == nil {
		return context.Stats{}
	}
	return s.Context.GetStats()
}

// IsContextNearLimit 检查上下文是否接近限制
func (s *Session) IsContextNearLimit() bool {
	if s.Context == nil {
		return false
	}
	return s.Context.IsNearLimit()
}

// CompactContext 压缩上下文
func (s *Session) CompactContext() {
	if s.Context != nil {
		s.Context.Compact()
	}
}

// ClearMessages 清除消息
func (s *Session) ClearMessages() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = make([]types.Message, 0)
	if s.Context != nil {
		s.Context.Clear()
	}
}

// GetWorkingDir 获取工作目录
func (s *Session) GetWorkingDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.WorkingDir
}
