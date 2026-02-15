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
	Title      string // 会话标题（AI 生成）
	Messages   []types.Message
	CreatedAt  time.Time
	UpdatedAt  time.Time
	TotalCost  float64
	WorkingDir string
	Context     *context.Manager // 上下文管理器
	WindowIndex int              // 上下文窗口索引（压缩次数），持久化字段
	// 记忆提取状态追踪
	LastExtractedAt       int64 // 上次提取记忆的时间戳
	LastExtractedMsgCount int   // 上次提取时的消息数量
	mu                    sync.RWMutex
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

// SetWorkingDir 设置工作目录
func (s *Session) SetWorkingDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.WorkingDir = dir
}

// GetTitle 获取会话标题
func (s *Session) GetTitle() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Title
}

// SetTitle 设置会话标题
func (s *Session) SetTitle(title string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Title = title
	s.UpdatedAt = time.Now()
}

// NeedsMemoryExtraction 检查是否需要提取记忆
// 只有当会话有新消息时才需要提取
func (s *Session) NeedsMemoryExtraction() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 消息数量少于2条不提取
	if len(s.Messages) < 2 {
		return false
	}

	// 从未提取过，需要提取
	if s.LastExtractedAt == 0 {
		return true
	}

	// 消息数量增加了，需要提取
	if len(s.Messages) > s.LastExtractedMsgCount {
		return true
	}

	return false
}

// MarkMemoryExtracted 标记记忆已提取
func (s *Session) MarkMemoryExtracted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastExtractedAt = time.Now().Unix()
	s.LastExtractedMsgCount = len(s.Messages)
}

// GetExtractionState 获取提取状态（用于持久化）
func (s *Session) GetExtractionState() (int64, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastExtractedAt, s.LastExtractedMsgCount
}

// SetExtractionState 设置提取状态（从持久化恢复）
func (s *Session) SetExtractionState(extractedAt int64, msgCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastExtractedAt = extractedAt
	s.LastExtractedMsgCount = msgCount
}
