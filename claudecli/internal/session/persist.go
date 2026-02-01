package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// SessionData 会话持久化数据结构
type SessionData struct {
	ID         string          `json:"id"`
	UserID     string          `json:"user_id"`
	Messages   []types.Message `json:"messages"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	TotalCost  float64         `json:"total_cost"`
	WorkingDir string          `json:"working_dir"`
	TokenUsage TokenUsage      `json:"token_usage"`
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

	return &SessionData{
		ID:         s.ID,
		UserID:     s.UserID,
		Messages:   s.Messages,
		CreatedAt:  s.CreatedAt,
		UpdatedAt:  s.UpdatedAt,
		TotalCost:  s.TotalCost,
		WorkingDir: s.WorkingDir,
	}
}

// FromSessionData 从持久化数据恢复Session
func FromSessionData(data *SessionData) *Session {
	return &Session{
		ID:         data.ID,
		UserID:     data.UserID,
		Messages:   data.Messages,
		CreatedAt:  data.CreatedAt,
		UpdatedAt:  data.UpdatedAt,
		TotalCost:  data.TotalCost,
		WorkingDir: data.WorkingDir,
	}
}

// getSessionDir 获取会话存储目录
func getSessionDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	return filepath.Join(homeDir, ".claude", "sessions")
}

// ensureSessionDir 确保会话目录存在
func ensureSessionDir() error {
	dir := getSessionDir()
	return os.MkdirAll(dir, 0755)
}

// getSessionPath 获取会话文件路径
func getSessionPath(sessionID string) string {
	return filepath.Join(getSessionDir(), sessionID+".json")
}

// SaveSession 保存会话到文件
func SaveSession(sess *Session) error {
	if err := ensureSessionDir(); err != nil {
		return err
	}

	data := sess.ToSessionData()
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(getSessionPath(sess.ID), jsonData, 0644)
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

// DeleteSessionFile 删除会话文件
func DeleteSessionFile(sessionID string) error {
	path := getSessionPath(sessionID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(path)
}
