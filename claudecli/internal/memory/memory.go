package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
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
	WindowID   string                 `json:"window_id"`   // 上下文窗口 ID (sessionID-wN)
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

// ContentHash 计算内容哈希（用于去重）
func ContentHash(content string) string {
	// 标准化内容：去除多余空白、转小写
	normalized := strings.ToLower(strings.TrimSpace(content))
	normalized = strings.Join(strings.Fields(normalized), " ")
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:8]) // 只取前8字节
}

// FindSimilar 查找相似记忆
// 基于标签重叠、内容哈希、摘要相似度判断
func (m *Manager) FindSimilar(userID string, mem *Memory) *Memory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userMems, ok := m.memories[userID]
	if !ok {
		return nil
	}

	newHash := ContentHash(mem.Content)

	var bestMatch *Memory
	bestScore := 0.0

	for _, existing := range userMems {
		// 1. 内容哈希完全匹配 = 重复
		if ContentHash(existing.Content) == newHash {
			return existing
		}

		// 2. 摘要高度相似 = 很可能是同一概念
		if mem.Summary != "" && existing.Summary != "" {
			summarySim := m.contentSimilarity(existing.Summary, mem.Summary)
			if summarySim >= 0.6 {
				return existing
			}
		}

		// 3. 综合评分：标签重叠 + 内容相似度（不再要求 source 相同）
		tagScore := m.tagOverlap(existing.Tags, mem.Tags)
		contentScore := m.contentSimilarity(existing.Content, mem.Content)

		// 加权综合分：标签 40% + 内容 60%
		combined := tagScore*0.4 + contentScore*0.6
		if combined > bestScore {
			bestScore = combined
			bestMatch = existing
		}
	}

	// 综合分超过 0.45 视为相似（比之前的双重 0.7+0.6 门槛宽松很多）
	if bestScore >= 0.45 {
		return bestMatch
	}

	return nil
}

// tagOverlap 计算标签重叠率
func (m *Manager) tagOverlap(tags1, tags2 []string) float64 {
	if len(tags1) == 0 || len(tags2) == 0 {
		return 0
	}

	set1 := make(map[string]bool)
	for _, t := range tags1 {
		set1[strings.ToLower(t)] = true
	}

	overlap := 0
	for _, t := range tags2 {
		if set1[strings.ToLower(t)] {
			overlap++
		}
	}

	// Jaccard 相似度
	union := len(tags1) + len(tags2) - overlap
	if union == 0 {
		return 0
	}
	return float64(overlap) / float64(union)
}

// contentSimilarity 计算内容相似度（词重叠）
func (m *Manager) contentSimilarity(content1, content2 string) float64 {
	words1 := strings.Fields(strings.ToLower(content1))
	words2 := strings.Fields(strings.ToLower(content2))

	if len(words1) == 0 || len(words2) == 0 {
		return 0
	}

	set1 := make(map[string]bool)
	for _, w := range words1 {
		set1[w] = true
	}

	overlap := 0
	for _, w := range words2 {
		if set1[w] {
			overlap++
		}
	}

	// Jaccard 相似度
	union := len(words1) + len(words2) - overlap
	if union == 0 {
		return 0
	}
	return float64(overlap) / float64(union)
}

// CreateOrMerge 创建新记忆或合并到现有相似记忆
// 返回: (记忆, 是否为新创建, 错误)
func (m *Manager) CreateOrMerge(userID string, mem *Memory) (*Memory, bool, error) {
	// 查找相似记忆
	similar := m.FindSimilar(userID, mem)

	if similar != nil {
		// 合并到现有记忆
		merged, err := m.mergeMemory(userID, similar, mem)
		if err != nil {
			return nil, false, err
		}
		return merged, false, nil
	}

	// 创建新记忆
	created, err := m.Create(userID, mem)
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

// mergeMemory 合并两条记忆
func (m *Manager) mergeMemory(userID string, existing, newMem *Memory) (*Memory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 合并标签（去重）
	tagSet := make(map[string]bool)
	for _, t := range existing.Tags {
		tagSet[strings.ToLower(t)] = true
	}
	for _, t := range newMem.Tags {
		if !tagSet[strings.ToLower(t)] {
			existing.Tags = append(existing.Tags, t)
			tagSet[strings.ToLower(t)] = true
		}
	}

	// 更新权重（取较高值）
	if newMem.Weight > existing.Weight {
		existing.Weight = newMem.Weight
	}

	// 增加访问计数
	existing.AccessCnt++
	existing.UpdatedAt = time.Now().Unix()
	existing.LastAccess = time.Now().Unix()

	// 更新标签索引
	m.removeFromTagIndex(userID, existing)
	for _, tag := range existing.Tags {
		m.tagIndex[userID][tag] = append(m.tagIndex[userID][tag], existing.ID)
	}

	// 持久化
	if err := m.saveMemory(userID, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

// FindByWindowID 根据窗口 ID 查找记忆
func (m *Manager) FindByWindowID(userID, windowID string) *Memory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if windowID == "" {
		return nil
	}

	userMems, ok := m.memories[userID]
	if !ok {
		return nil
	}

	for _, mem := range userMems {
		if mem.WindowID == windowID {
			return mem
		}
	}
	return nil
}

// UpsertByWindowID 按窗口 ID 创建或替换记忆
// 同一个窗口只维护一条记忆，后续提取直接替换标题和内容
func (m *Manager) UpsertByWindowID(userID string, mem *Memory) (*Memory, bool, error) {
	existing := m.FindByWindowID(userID, mem.WindowID)

	if existing != nil {
		// 替换内容和摘要
		m.mu.Lock()
		existing.Content = mem.Content
		existing.Summary = mem.Summary
		existing.Tags = mem.Tags
		if mem.Weight > existing.Weight {
			existing.Weight = mem.Weight
		}
		existing.AccessCnt++
		existing.UpdatedAt = time.Now().Unix()
		existing.LastAccess = time.Now().Unix()

		// 更新标签索引
		m.removeFromTagIndex(userID, existing)
		for _, tag := range existing.Tags {
			m.tagIndex[userID][tag] = append(m.tagIndex[userID][tag], existing.ID)
		}

		if err := m.saveMemory(userID, existing); err != nil {
			m.mu.Unlock()
			return nil, false, err
		}
		m.mu.Unlock()
		return existing, false, nil
	}

	// 新窗口，创建记忆
	created, err := m.Create(userID, mem)
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}
