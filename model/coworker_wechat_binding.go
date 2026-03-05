package model

// CoworkerWechatBinding 微信公众号 OpenID ↔ 系统用户 绑定
type CoworkerWechatBinding struct {
	ID        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	OpenID    string `json:"open_id" gorm:"uniqueIndex;type:varchar(64);not null"`
	UserID    int    `json:"user_id" gorm:"index;not null"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (CoworkerWechatBinding) TableName() string { return "coworker_wechat_bindings" }

// GetWechatBindingByOpenID 按 OpenID 查找绑定
func GetWechatBindingByOpenID(openID string) (*CoworkerWechatBinding, error) {
	var binding CoworkerWechatBinding
	err := DB.Where("open_id = ?", openID).First(&binding).Error
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

// GetWechatBindingByUserID 按 UserID 查找绑定
func GetWechatBindingByUserID(userID int) (*CoworkerWechatBinding, error) {
	var binding CoworkerWechatBinding
	err := DB.Where("user_id = ?", userID).First(&binding).Error
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

// CreateWechatBinding 创建绑定
func CreateWechatBinding(binding *CoworkerWechatBinding) error {
	return DB.Create(binding).Error
}

// DeleteWechatBindingByOpenID 按 OpenID 删除绑定
func DeleteWechatBindingByOpenID(openID string) error {
	return DB.Where("open_id = ?", openID).Delete(&CoworkerWechatBinding{}).Error
}

// DeleteWechatBindingByUserID 按 UserID 删除绑定
func DeleteWechatBindingByUserID(userID int) error {
	return DB.Where("user_id = ?", userID).Delete(&CoworkerWechatBinding{}).Error
}
