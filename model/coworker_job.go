package model

// CoworkerJob 定时事项（coworker_jobs 表）
type CoworkerJob struct {
	ID              string `json:"id" gorm:"primaryKey;type:varchar(32)"` // UnixNano string
	UserID          int    `json:"user_id" gorm:"index;not null"`
	Name            string `json:"name" gorm:"type:varchar(256);not null"`
	Command         string `json:"command" gorm:"type:text"`
	Enabled         bool   `json:"enabled" gorm:"default:true"`
	LastRun         int64  `json:"last_run"`                                        // Milliseconds
	NextRun         int64  `json:"next_run" gorm:"index"`                           // Milliseconds, 调度器查询关键索引
	Status          string `json:"status" gorm:"type:varchar(16);default:'idle'"`   // idle|running|failed
	LastError       string `json:"last_error" gorm:"type:text"`
	LastResult      string `json:"last_result" gorm:"type:text"`
	SortOrder       int    `json:"sort_order" gorm:"default:0"`
	ScheduleType    string `json:"schedule_type" gorm:"type:varchar(16)"`           // once|daily|weekly|interval|cron
	Time            string `json:"time" gorm:"type:varchar(8)"`                     // HH:MM
	Weekdays        string `json:"weekdays" gorm:"type:varchar(32)"`                // JSON array [0,1,5]
	IntervalMinutes int    `json:"interval_minutes" gorm:"default:0"`
	RunAt           int64  `json:"run_at"`
	CronExpr        string `json:"cron_expr" gorm:"type:varchar(64)"`
	Metadata        string `json:"metadata" gorm:"type:text"` // JSON object
	CreatedAt       int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CoworkerJob) TableName() string { return "coworker_jobs" }

// CreateCoworkerJob 创建 Job
func CreateCoworkerJob(job *CoworkerJob) error {
	return DB.Create(job).Error
}

// GetCoworkerJob 按 ID + UserID 获取 Job
func GetCoworkerJob(id string, userID int) (*CoworkerJob, error) {
	var job CoworkerJob
	err := DB.Where("id = ? AND user_id = ?", id, userID).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// ListCoworkerJobs 列出用户所有 Jobs（按 sort_order 排序）
func ListCoworkerJobs(userID int) ([]*CoworkerJob, error) {
	var jobs []*CoworkerJob
	err := DB.Where("user_id = ?", userID).Order("sort_order ASC, created_at ASC").Find(&jobs).Error
	return jobs, err
}

// UpdateCoworkerJob 更新 Job
func UpdateCoworkerJob(job *CoworkerJob) error {
	return DB.Save(job).Error
}

// DeleteCoworkerJob 删除 Job
func DeleteCoworkerJob(id string, userID int) error {
	return DB.Where("id = ? AND user_id = ?", id, userID).Delete(&CoworkerJob{}).Error
}

// GetDueCoworkerJobs 获取所有到期需要执行的 Jobs
func GetDueCoworkerJobs(nowMillis int64) ([]*CoworkerJob, error) {
	var jobs []*CoworkerJob
	err := DB.Where("enabled = ? AND status = ? AND next_run <= ?", true, "idle", nowMillis).
		Find(&jobs).Error
	return jobs, err
}

// BatchCreateCoworkerJobs 批量创建 Jobs（迁移用，跳过已存在的）
func BatchCreateCoworkerJobs(jobs []*CoworkerJob) (int, error) {
	created := 0
	for _, j := range jobs {
		var existing CoworkerJob
		result := DB.Where("id = ? AND user_id = ?", j.ID, j.UserID).First(&existing)
		if result.Error != nil {
			if err := DB.Create(j).Error; err != nil {
				continue
			}
			created++
		}
	}
	return created, nil
}
