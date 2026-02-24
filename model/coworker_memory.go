package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// CoworkerMemory 记忆条目（coworker_memories 表）
type CoworkerMemory struct {
	ID          string  `json:"id" gorm:"primaryKey;type:varchar(64)"`
	UserID      int     `json:"user_id" gorm:"index;not null"`
	Tags        string  `json:"tags" gorm:"type:text"`                     // JSON array: ["tag1","tag2"]
	Content     string  `json:"content" gorm:"type:text;not null"`
	Summary     string  `json:"summary" gorm:"type:varchar(512)"`
	Source      string  `json:"source" gorm:"type:varchar(32)"`            // manual|conversation|extracted|ai_extracted|context_window_summary
	SessionID   string  `json:"session_id" gorm:"type:varchar(64);index"`
	WindowID    string  `json:"window_id" gorm:"type:varchar(80);index"`   // sessionID-wN
	ContentHash string  `json:"content_hash" gorm:"type:varchar(16);index"` // SHA256 前 8 字节，去重用
	Weight      float64 `json:"weight" gorm:"type:decimal(4,3);default:0.5"`
	AccessCnt   int     `json:"access_cnt" gorm:"default:0"`
	Metadata    string  `json:"metadata" gorm:"type:text"` // JSON object
	CreatedAt   int64   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   int64   `json:"updated_at" gorm:"autoUpdateTime"`
	LastAccess  int64   `json:"last_access" gorm:"index"`
}

func (CoworkerMemory) TableName() string { return "coworker_memories" }

// CreateCoworkerMemory 创建记忆
func CreateCoworkerMemory(memory *CoworkerMemory) error {
	return DB.Create(memory).Error
}

// GetCoworkerMemory 按 ID + UserID 获取记忆
func GetCoworkerMemory(id string, userID int) (*CoworkerMemory, error) {
	var mem CoworkerMemory
	err := DB.Where("id = ? AND user_id = ?", id, userID).First(&mem).Error
	if err != nil {
		return nil, err
	}
	return &mem, nil
}

// ListCoworkerMemories 列出用户所有记忆
func ListCoworkerMemories(userID int) ([]*CoworkerMemory, error) {
	var memories []*CoworkerMemory
	err := DB.Where("user_id = ?", userID).Order("last_access DESC").Find(&memories).Error
	return memories, err
}

// UpdateCoworkerMemory 更新记忆
func UpdateCoworkerMemory(memory *CoworkerMemory) error {
	return DB.Save(memory).Error
}

// DeleteCoworkerMemory 删除记忆
func DeleteCoworkerMemory(id string, userID int) error {
	return DB.Where("id = ? AND user_id = ?", id, userID).Delete(&CoworkerMemory{}).Error
}

// UpsertCoworkerMemory 创建或更新记忆（按 ID 去重）
func UpsertCoworkerMemory(memory *CoworkerMemory) error {
	var existing CoworkerMemory
	result := DB.Where("id = ? AND user_id = ?", memory.ID, memory.UserID).First(&existing)
	if result.Error != nil {
		// 不存在，创建
		return DB.Create(memory).Error
	}
	// 已存在，更新
	return DB.Save(memory).Error
}

// FindCoworkerMemoryByWindowID 按窗口 ID 查找记忆
func FindCoworkerMemoryByWindowID(userID int, windowID string) (*CoworkerMemory, error) {
	var mem CoworkerMemory
	err := DB.Where("user_id = ? AND window_id = ?", userID, windowID).First(&mem).Error
	if err != nil {
		return nil, err
	}
	return &mem, nil
}

// FindCoworkerMemoryByContentHash 按内容哈希查找（去重）
func FindCoworkerMemoryByContentHash(userID int, contentHash string) (*CoworkerMemory, error) {
	var mem CoworkerMemory
	err := DB.Where("user_id = ? AND content_hash = ?", userID, contentHash).First(&mem).Error
	if err != nil {
		return nil, err
	}
	return &mem, nil
}

// SearchCoworkerMemories 搜索记忆（全文搜索）
// PostgreSQL 使用 ILIKE，MySQL 使用 LIKE，SQLite 使用 LIKE
func SearchCoworkerMemories(userID int, query string, limit int) ([]*CoworkerMemory, error) {
	var memories []*CoworkerMemory

	if limit <= 0 {
		limit = 20
	}

	// 分词搜索：将查询拆分为关键词，每个都要匹配 content 或 summary 或 tags
	keywords := strings.Fields(strings.TrimSpace(query))
	if len(keywords) == 0 {
		return ListCoworkerMemories(userID)
	}

	tx := DB.Where("user_id = ?", userID)

	if common.UsingPostgreSQL {
		// PostgreSQL: 使用 ILIKE
		for _, kw := range keywords {
			pattern := "%" + kw + "%"
			tx = tx.Where("(content ILIKE ? OR summary ILIKE ? OR tags ILIKE ?)", pattern, pattern, pattern)
		}
	} else {
		// MySQL / SQLite: 使用 LIKE
		for _, kw := range keywords {
			pattern := "%" + kw + "%"
			tx = tx.Where("(content LIKE ? OR summary LIKE ? OR tags LIKE ?)", pattern, pattern, pattern)
		}
	}

	err := tx.Order("weight DESC, last_access DESC").Limit(limit).Find(&memories).Error
	return memories, err
}

// BatchCreateCoworkerMemories 批量创建记忆（迁移用，跳过已存在的）
func BatchCreateCoworkerMemories(memories []*CoworkerMemory) (int, error) {
	created := 0
	for _, mem := range memories {
		var existing CoworkerMemory
		result := DB.Where("id = ? AND user_id = ?", mem.ID, mem.UserID).First(&existing)
		if result.Error != nil {
			// 不存在，创建
			if err := DB.Create(mem).Error; err != nil {
				continue
			}
			created++
		}
	}
	return created, nil
}
