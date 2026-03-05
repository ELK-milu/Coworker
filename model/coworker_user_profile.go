package model

// CoworkerUserProfile 用户画像（合并 UserInfo + Profile，coworker_user_profiles 表）
type CoworkerUserProfile struct {
	ID               int      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID           int      `json:"user_id" gorm:"uniqueIndex;not null"` // 关联 users.id
	UserName         string   `json:"user_name" gorm:"type:varchar(128)"`
	CoworkerName     string   `json:"coworker_name" gorm:"type:varchar(128)"`
	Phone            string   `json:"phone" gorm:"type:varchar(32)"`
	Email            string   `json:"email" gorm:"type:varchar(128)"`
	WechatID         string   `json:"wechat_id" gorm:"type:varchar(64)"`
	ApiTokenKey      string   `json:"api_token_key" gorm:"type:varchar(128)"`
	ApiTokenName     string   `json:"api_token_name" gorm:"type:varchar(64)"`
	SelectedModel    string   `json:"selected_model" gorm:"type:varchar(64)"`
	Group            string   `json:"group" gorm:"type:varchar(64)"`
	AssistantAvatar  string   `json:"assistant_avatar" gorm:"type:text"`  // base64
	InstalledItems   string   `json:"installed_items" gorm:"type:text"`   // JSON array
	FavoriteItems    string   `json:"favorite_items" gorm:"type:text"`   // JSON array of item IDs
	Temperature      *float64 `json:"temperature"`
	TopP             *float64 `json:"top_p"`
	FrequencyPenalty *float64 `json:"frequency_penalty"`
	PresencePenalty  *float64 `json:"presence_penalty"`
	// Profile 字段（合并自 profile.json）
	Languages       string `json:"languages" gorm:"type:text"`          // JSON array
	Frameworks      string `json:"frameworks" gorm:"type:text"`         // JSON array
	CodingStyle     string `json:"coding_style" gorm:"type:text"`       // JSON object
	ResponseStyle   string `json:"response_style" gorm:"type:varchar(32)"`
	UILanguage      string `json:"ui_language" gorm:"type:varchar(16)"` // zh-CN|en-US
	CurrentProjects string `json:"current_projects" gorm:"type:text"`   // JSON array (max 10)
	TotalSessions   int    `json:"total_sessions" gorm:"default:0"`
	TotalMessages   int    `json:"total_messages" gorm:"default:0"`
	TopTools        string `json:"top_tools" gorm:"type:text"` // JSON object {"bash":234}
	CreatedAt       int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CoworkerUserProfile) TableName() string { return "coworker_user_profiles" }

// CreateCoworkerUserProfile 创建用户画像
func CreateCoworkerUserProfile(profile *CoworkerUserProfile) error {
	return DB.Create(profile).Error
}

// GetCoworkerUserProfile 按 UserID 获取画像
func GetCoworkerUserProfile(userID int) (*CoworkerUserProfile, error) {
	var profile CoworkerUserProfile
	err := DB.Where("user_id = ?", userID).First(&profile).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// UpsertCoworkerUserProfile 创建或更新用户画像
func UpsertCoworkerUserProfile(profile *CoworkerUserProfile) error {
	var existing CoworkerUserProfile
	result := DB.Where("user_id = ?", profile.UserID).First(&existing)
	if result.Error != nil {
		// 不存在，创建
		return DB.Create(profile).Error
	}
	// 已存在，更新（保留 ID）
	profile.ID = existing.ID
	return DB.Save(profile).Error
}

// UpdateCoworkerUserProfile 更新用户画像
func UpdateCoworkerUserProfile(profile *CoworkerUserProfile) error {
	return DB.Save(profile).Error
}

// DeleteCoworkerUserProfile 删除用户画像
func DeleteCoworkerUserProfile(userID int) error {
	return DB.Where("user_id = ?", userID).Delete(&CoworkerUserProfile{}).Error
}

// IncrementCoworkerUserProfileSessions 原子增加会话计数
func IncrementCoworkerUserProfileSessions(userID int) error {
	return DB.Model(&CoworkerUserProfile{}).Where("user_id = ?", userID).
		UpdateColumn("total_sessions", DB.Raw("total_sessions + 1")).Error
}

// IncrementCoworkerUserProfileMessages 原子增加消息计数
func IncrementCoworkerUserProfileMessages(userID int) error {
	return DB.Model(&CoworkerUserProfile{}).Where("user_id = ?", userID).
		UpdateColumn("total_messages", DB.Raw("total_messages + 1")).Error
}

// ListAllCoworkerUserProfiles 列出所有用户画像（用于级联清理）
func ListAllCoworkerUserProfiles() ([]*CoworkerUserProfile, error) {
	var profiles []*CoworkerUserProfile
	err := DB.Find(&profiles).Error
	return profiles, err
}
