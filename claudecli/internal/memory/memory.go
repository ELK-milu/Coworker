package memory

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Memory 记忆条目
type Memory struct {
	ID         string                 `json:"id"`
	UserID     string                 `json:"user_id"`
	Tags       []string               `json:"tags"`        // 标签列表
	Content    string                 `json:"content"`     // 记忆内容
	Summary    string                 `json:"summary"`     // 简短摘要
	Source     string                 `json:"source"`      // 来源: manual, conversation, extracted
	SessionID  string                 `json:"session_id"`  // 关联会话
	Weight     float64                `json:"weight"`      // 重要性权重 (0-1)
	AccessCnt  int                    `json:"access_cnt"`  // 访问次数
	Metadata   map[string]interface{} `json:"metadata"`
	CreatedAt  int64                  `json:"created_at"`
	UpdatedAt  int64                  `json:"updated_at"`
	LastAccess int64                  `json:"last_access"`
}

// CalculateWeight 计算动态权重
// 公式: weight = base * recency * frequency
func (m *Memory) CalculateWeight() float64 {
	days := float64(time.Now().Unix()-m.LastAccess) / 86400
	recency := math.Exp(-days / 30) // 30天衰减
	frequency := math.Log(1 + float64(m.AccessCnt))
	return m.Weight * recency * (1 + 0.1*frequency)
}

// Manager 记忆管理器
type Manager struct {
	baseDir  string
	memories map[string]map[string]*Memory // userID -> memoryID -> Memory
	tagIndex map[string]map[string][]string // userID -> tag -> memoryIDs
	mu       sync.RWMutex
}

// NewManager 创建记忆管理器
func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir:  baseDir,
		memories: make(map[string]map[string]*Memory),
		tagIndex: make(map[string]map[string][]string),
	}
}

// Create 创建新记忆
func (m *Manager) Create(userID string, mem *Memory) (*Memory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 生成 ID
	if mem.ID == "" {
		mem.ID = uuid.New().String()[:8]
	}

	mem.UserID = userID
	now := time.Now().Unix()
	mem.CreatedAt = now
	mem.UpdatedAt = now
	mem.LastAccess = now

	if mem.Weight == 0 {
		mem.Weight = 0.5 // 默认权重
	}

	// 初始化用户记忆映射
	if m.memories[userID] == nil {
		m.memories[userID] = make(map[string]*Memory)
	}
	if m.tagIndex[userID] == nil {
		m.tagIndex[userID] = make(map[string][]string)
	}

	m.memories[userID][mem.ID] = mem

	// 更新标签索引
	for _, tag := range mem.Tags {
		m.tagIndex[userID][tag] = append(m.tagIndex[userID][tag], mem.ID)
	}

	// 持久化
	if err := m.saveMemory(userID, mem); err != nil {
		return nil, err
	}

	return mem, nil
}

// Get 获取记忆
func (m *Manager) Get(userID, memoryID string) (*Memory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if userMems, ok := m.memories[userID]; ok {
		if mem, ok := userMems[memoryID]; ok {
			return mem, nil
		}
	}
	return nil, fmt.Errorf("memory not found: %s", memoryID)
}

// GetByID 根据 ID 获取记忆（不返回错误）
func (m *Manager) GetByID(userID, memoryID string) *Memory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if userMems, ok := m.memories[userID]; ok {
		if mem, ok := userMems[memoryID]; ok {
			return mem
		}
	}
	return nil
}

// Update 更新记忆
func (m *Manager) Update(userID, memoryID string, updates map[string]interface{}) (*Memory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mem, ok := m.memories[userID][memoryID]
	if !ok {
		return nil, fmt.Errorf("memory not found: %s", memoryID)
	}

	// 应用更新
	if content, ok := updates["content"].(string); ok {
		mem.Content = content
	}
	if summary, ok := updates["summary"].(string); ok {
		mem.Summary = summary
	}
	if tags, ok := updates["tags"].([]string); ok {
		// 更新标签索引
		m.removeFromTagIndex(userID, mem)
		mem.Tags = tags
		for _, tag := range tags {
			m.tagIndex[userID][tag] = append(m.tagIndex[userID][tag], mem.ID)
		}
	}
	if weight, ok := updates["weight"].(float64); ok {
		mem.Weight = weight
	}

	mem.UpdatedAt = time.Now().Unix()

	// 持久化
	if err := m.saveMemory(userID, mem); err != nil {
		return nil, err
	}

	return mem, nil
}

// Delete 删除记忆
func (m *Manager) Delete(userID, memoryID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mem, ok := m.memories[userID][memoryID]
	if !ok {
		return fmt.Errorf("memory not found: %s", memoryID)
	}

	// 从标签索引中移除
	m.removeFromTagIndex(userID, mem)

	// 从内存中删除
	delete(m.memories[userID], memoryID)

	// 删除文件
	return m.deleteMemoryFile(userID, memoryID)
}

// List 列出用户所有记忆
func (m *Manager) List(userID string) []*Memory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Memory
	if userMems, ok := m.memories[userID]; ok {
		for _, mem := range userMems {
			result = append(result, mem)
		}
	}
	return result
}

// RecordAccess 记录访问
func (m *Manager) RecordAccess(userID, memoryID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if mem, ok := m.memories[userID][memoryID]; ok {
		mem.AccessCnt++
		mem.LastAccess = time.Now().Unix()
		m.saveMemory(userID, mem)
	}
}

// removeFromTagIndex 从标签索引中移除
func (m *Manager) removeFromTagIndex(userID string, mem *Memory) {
	for _, tag := range mem.Tags {
		if ids, ok := m.tagIndex[userID][tag]; ok {
			var newIDs []string
			for _, id := range ids {
				if id != mem.ID {
					newIDs = append(newIDs, id)
				}
			}
			m.tagIndex[userID][tag] = newIDs
		}
	}
}

// GetMemoriesDir 获取记忆存储目录
func (m *Manager) GetMemoriesDir(userID string) string {
	return filepath.Join(m.baseDir, userID, ".claude", "memories")
}

// saveMemory 保存单条记忆
func (m *Manager) saveMemory(userID string, mem *Memory) error {
	dir := m.GetMemoriesDir(userID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	path := filepath.Join(dir, mem.ID+".json")
	data, err := json.MarshalIndent(mem, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// deleteMemoryFile 删除记忆文件
func (m *Manager) deleteMemoryFile(userID, memoryID string) error {
	path := filepath.Join(m.GetMemoriesDir(userID), memoryID+".json")
	return os.Remove(path)
}

// LoadUserMemories 加载用户所有记忆
func (m *Manager) LoadUserMemories(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dir := m.GetMemoriesDir(userID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if m.memories[userID] == nil {
		m.memories[userID] = make(map[string]*Memory)
	}
	if m.tagIndex[userID] == nil {
		m.tagIndex[userID] = make(map[string][]string)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var mem Memory
		if err := json.Unmarshal(data, &mem); err != nil {
			continue
		}

		m.memories[userID][mem.ID] = &mem

		// 重建标签索引
		for _, tag := range mem.Tags {
			m.tagIndex[userID][tag] = append(m.tagIndex[userID][tag], mem.ID)
		}
	}

	return nil
}

// Retrieve 检索相关记忆（便捷方法）
func (m *Manager) Retrieve(userID, query string, limit int) []*Memory {
	retriever := NewRetriever(m, m.baseDir)
	return retriever.Retrieve(userID, query, limit)
}

// FormatForPrompt 格式化记忆用于系统提示词（便捷方法）
func (m *Manager) FormatForPrompt(memories []*Memory) string {
	return FormatForPrompt(memories)
}
