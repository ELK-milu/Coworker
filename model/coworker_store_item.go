package model

// CoworkerStoreItem 技能商店条目（coworker_store_items 表，全局共享）
type CoworkerStoreItem struct {
	ID           string `json:"id" gorm:"primaryKey;type:varchar(32)"`
	Name         string `json:"name" gorm:"type:varchar(128);not null;uniqueIndex"`
	Description  string `json:"description" gorm:"type:text"`
	Type         string `json:"type" gorm:"type:varchar(16)"`          // skill|agent|mcp
	Icon         string `json:"icon" gorm:"type:text"`                 // base64 or icon name
	Author       string `json:"author" gorm:"type:varchar(64)"`
	GithubURL    string `json:"github_url" gorm:"type:varchar(512)"`
	Content      string `json:"content" gorm:"type:text"`              // markdown
	LocalDir     string `json:"local_dir" gorm:"type:varchar(256)"`
	ServerURL    string `json:"server_url" gorm:"type:varchar(512)"`
	ConfigSchema string `json:"config_schema" gorm:"type:text"` // JSON array
	SubItems     string `json:"sub_items" gorm:"type:text"`     // JSON array of SubItem
	CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CoworkerStoreItem) TableName() string { return "coworker_store_items" }

// CreateCoworkerStoreItem 创建商店条目
func CreateCoworkerStoreItem(item *CoworkerStoreItem) error {
	return DB.Create(item).Error
}

// GetCoworkerStoreItem 按 ID 获取商店条目
func GetCoworkerStoreItem(id string) (*CoworkerStoreItem, error) {
	var item CoworkerStoreItem
	err := DB.Where("id = ?", id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// GetCoworkerStoreItemByName 按 Name 获取商店条目
func GetCoworkerStoreItemByName(name string) (*CoworkerStoreItem, error) {
	var item CoworkerStoreItem
	err := DB.Where("name = ?", name).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// ListCoworkerStoreItems 列出所有商店条目
func ListCoworkerStoreItems() ([]*CoworkerStoreItem, error) {
	var items []*CoworkerStoreItem
	err := DB.Order("created_at DESC").Find(&items).Error
	return items, err
}

// UpdateCoworkerStoreItem 更新商店条目
func UpdateCoworkerStoreItem(item *CoworkerStoreItem) error {
	return DB.Save(item).Error
}

// DeleteCoworkerStoreItem 删除商店条目
func DeleteCoworkerStoreItem(id string) error {
	return DB.Where("id = ?", id).Delete(&CoworkerStoreItem{}).Error
}

// BatchCreateCoworkerStoreItems 批量创建商店条目（迁移用，跳过已存在的）
func BatchCreateCoworkerStoreItems(items []*CoworkerStoreItem) (int, error) {
	created := 0
	for _, item := range items {
		var existing CoworkerStoreItem
		result := DB.Where("id = ?", item.ID).First(&existing)
		if result.Error != nil {
			if err := DB.Create(item).Error; err != nil {
				continue
			}
			created++
		}
	}
	return created, nil
}
