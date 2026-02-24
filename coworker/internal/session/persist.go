package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/QuantumNous/new-api/coworker/internal/context"
	"github.com/QuantumNous/new-api/coworker/pkg/types"
)

// SessionData 会话持久化数据结构
type SessionData struct {
	ID         string          `json:"id"`
	UserID     string          `json:"user_id"`
	Title      string          `json:"title,omitempty"` // 会话标题
	Messages   []types.Message `json:"messages"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	TotalCost  float64         `json:"total_cost"`
	WorkingDir string          `json:"working_dir"`
	TokenUsage TokenUsage      `json:"token_usage"`
	// 记忆提取状态
	LastExtractedAt       int64 `json:"last_extracted_at,omitempty"`
	LastExtractedMsgCount int   `json:"last_extracted_msg_count,omitempty"`
	// 上下文窗口索引（压缩次数）
	WindowIndex int `json:"window_index,omitempty"`
}

// TokenUsage token使用统计
type TokenUsage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
	Total  int `json:"total"`
}

// ToSessionData 将Session转换为可持久化的数据
func (s *Session) ToSessionData() *SessionData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 从 Context 同步最新窗口索引
	if s.Context != nil {
		s.WindowIndex = s.Context.GetWindowIndex()
	}

	return &SessionData{
		ID:                    s.ID,
		UserID:                s.UserID,
		Title:                 s.Title,
		Messages:              s.Messages,
		CreatedAt:             s.CreatedAt,
		UpdatedAt:             s.UpdatedAt,
		TotalCost:             s.TotalCost,
		WorkingDir:            s.WorkingDir,
		LastExtractedAt:       s.LastExtractedAt,
		LastExtractedMsgCount: s.LastExtractedMsgCount,
		WindowIndex:           s.WindowIndex,
	}
}

// FromSessionData 从持久化数据恢复Session
func FromSessionData(data *SessionData) *Session {
	workingDir := data.WorkingDir
	// 如果 WorkingDir 为空，根据 UserID 重新计算
	if workingDir == "" && userBaseDir != "" && data.UserID != "" {
		workingDir = filepath.Join(userBaseDir, data.UserID, "workspace")
	}
	// 恢复上下文管理器及窗口索引
	ctx := context.NewManager(nil)
	if data.WindowIndex > 0 {
		ctx.SetWindowIndex(data.WindowIndex)
	}

	return &Session{
		ID:                    data.ID,
		UserID:                data.UserID,
		Title:                 data.Title,
		Messages:              data.Messages,
		CreatedAt:             data.CreatedAt,
		UpdatedAt:             data.UpdatedAt,
		TotalCost:             data.TotalCost,
		WorkingDir:            workingDir,
		Context:               ctx,
		WindowIndex:           data.WindowIndex,
		LastExtractedAt:       data.LastExtractedAt,
		LastExtractedMsgCount: data.LastExtractedMsgCount,
	}
}

// 用户工作空间基础目录（可通过 SetUserBaseDir 设置）
var userBaseDir = ""

// SetUserBaseDir 设置用户工作空间基础目录
func SetUserBaseDir(dir string) {
	userBaseDir = dir
}

// getSessionDir 获取会话存储目录（兼容旧版本）
func getSessionDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	return filepath.Join(homeDir, ".coworker", "sessions")
}

// getUserSessionDir 获取用户的会话存储目录
func getUserSessionDir(userID string) string {
	if userBaseDir != "" && userID != "" {
		return filepath.Join(userBaseDir, userID, ".coworker", "sessions")
	}
	return getSessionDir()
}

// ensureSessionDir 确保会话目录存在
func ensureSessionDir() error {
	dir := getSessionDir()
	return os.MkdirAll(dir, 0755)
}

// ensureUserSessionDir 确保用户会话目录存在
func ensureUserSessionDir(userID string) error {
	dir := getUserSessionDir(userID)
	return os.MkdirAll(dir, 0755)
}

// getSessionPath 获取会话文件路径
func getSessionPath(sessionID string) string {
	return filepath.Join(getSessionDir(), sessionID+".json")
}

// getUserSessionPath 获取用户会话文件路径
func getUserSessionPath(userID, sessionID string) string {
	return filepath.Join(getUserSessionDir(userID), sessionID+".json")
}

// SaveSession 保存会话到文件
func SaveSession(sess *Session) error {
	var dir string
	var path string

	if userBaseDir != "" && sess.UserID != "" {
		if err := ensureUserSessionDir(sess.UserID); err != nil {
			return err
		}
		path = getUserSessionPath(sess.UserID, sess.ID)
	} else {
		if err := ensureSessionDir(); err != nil {
			return err
		}
		path = getSessionPath(sess.ID)
	}
	_ = dir

	data := sess.ToSessionData()
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, jsonData, 0644)
}

// LoadSession 从文件加载会话
func LoadSession(sessionID string) (*Session, error) {
	path := getSessionPath(sessionID)
	jsonData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var data SessionData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, err
	}

	return FromSessionData(&data), nil
}

// LoadUserSession 从用户目录加载会话
func LoadUserSession(userID, sessionID string) (*Session, error) {
	path := getUserSessionPath(userID, sessionID)
	jsonData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var data SessionData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, err
	}

	return FromSessionData(&data), nil
}

// ListSessionFiles 列出所有会话文件
func ListSessionFiles() ([]string, error) {
	dir := getSessionDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var sessionIDs []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			sessionID := entry.Name()[:len(entry.Name())-5]
			sessionIDs = append(sessionIDs, sessionID)
		}
	}
	return sessionIDs, nil
}

// ListUserSessionFiles 列出用户的所有会话文件
func ListUserSessionFiles(userID string) ([]string, error) {
	dir := getUserSessionDir(userID)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var sessionIDs []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			sessionID := entry.Name()[:len(entry.Name())-5]
			sessionIDs = append(sessionIDs, sessionID)
		}
	}
	return sessionIDs, nil
}

// DeleteSessionFile 删除会话文件
func DeleteSessionFile(sessionID string) error {
	path := getSessionPath(sessionID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(path)
}

// DeleteUserSessionFile 删除用户会话文件
func DeleteUserSessionFile(userID, sessionID string) error {
	path := getUserSessionPath(userID, sessionID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(path)
}
