package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// MigrateCoworkerDataFromFiles 从 JSON 文件迁移数据到数据库
// baseDir 是 userdata 根目录（如 ./userdata）
// 幂等：跳过已存在的记录，不删除原 JSON 文件
func MigrateCoworkerDataFromFiles(baseDir string) error {
	log.Println("[CoworkerMigrate] Starting JSON → DB migration from:", baseDir)
	startTime := time.Now()

	// 扫描 baseDir 下所有用户目录
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		log.Printf("[CoworkerMigrate] WARNING: cannot read baseDir %s: %v", baseDir, err)
		return nil // 不阻塞启动
	}

	totalMemories := 0
	totalProfiles := 0
	totalJobs := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		userDirName := entry.Name()

		// 跳过特殊目录
		if userDirName == "store" || strings.HasPrefix(userDirName, ".") {
			continue
		}

		// 尝试将目录名转为 int（作为 DB UserID）
		dbUserID, err := strconv.Atoi(userDirName)
		if err != nil {
			log.Printf("[CoworkerMigrate] Skipping non-numeric user dir: %s", userDirName)
			continue
		}

		// 迁移记忆
		n, err := migrateMemories(baseDir, userDirName, dbUserID)
		if err != nil {
			log.Printf("[CoworkerMigrate] WARNING: memories migration failed for user %s: %v", userDirName, err)
		}
		totalMemories += n

		// 迁移用户画像（UserInfo + Profile）
		ok, err := migrateUserProfile(baseDir, userDirName, dbUserID)
		if err != nil {
			log.Printf("[CoworkerMigrate] WARNING: profile migration failed for user %s: %v", userDirName, err)
		}
		if ok {
			totalProfiles++
		}

		// 迁移 Jobs
		n, err = migrateJobs(baseDir, userDirName, dbUserID)
		if err != nil {
			log.Printf("[CoworkerMigrate] WARNING: jobs migration failed for user %s: %v", userDirName, err)
		}
		totalJobs += n
	}

	// 迁移全局商店条目
	totalStore, err := migrateStoreItems(baseDir)
	if err != nil {
		log.Printf("[CoworkerMigrate] WARNING: store migration failed: %v", err)
	}

	// 标记迁移完成
	UpdateOption("coworker_migration_done", "true")

	elapsed := time.Since(startTime)
	log.Printf("[CoworkerMigrate] Migration completed in %v: memories=%d, profiles=%d, jobs=%d, store_items=%d",
		elapsed, totalMemories, totalProfiles, totalJobs, totalStore)

	return nil
}

// IsCoworkerMigrationDone 检查迁移是否已完成
func IsCoworkerMigrationDone() bool {
	var option Option
	result := DB.Where(commonKeyCol+" = ?", "coworker_migration_done").First(&option)
	if result.Error != nil {
		return false
	}
	return option.Value == "true"
}

// migrateMemories 迁移用户记忆
func migrateMemories(baseDir, userDirName string, dbUserID int) (int, error) {
	memoriesDir := filepath.Join(baseDir, userDirName, ".claude", "memories")
	entries, err := os.ReadDir(memoriesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	var memories []*CoworkerMemory
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(memoriesDir, entry.Name()))
		if err != nil {
			continue
		}

		var raw memoryJSON
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}

		// 转换为 DB 模型
		tagsJSON, _ := json.Marshal(raw.Tags)
		metadataJSON, _ := json.Marshal(raw.Metadata)

		mem := &CoworkerMemory{
			ID:          raw.ID,
			UserID:      dbUserID,
			Tags:        string(tagsJSON),
			Content:     raw.Content,
			Summary:     raw.Summary,
			Source:      raw.Source,
			SessionID:   raw.SessionID,
			WindowID:    raw.WindowID,
			ContentHash: computeContentHash(raw.Content),
			Weight:      raw.Weight,
			AccessCnt:   raw.AccessCnt,
			Metadata:    string(metadataJSON),
			CreatedAt:   raw.CreatedAt,
			UpdatedAt:   raw.UpdatedAt,
			LastAccess:  raw.LastAccess,
		}
		memories = append(memories, mem)
	}

	if len(memories) == 0 {
		return 0, nil
	}

	return BatchCreateCoworkerMemories(memories)
}

// migrateUserProfile 迁移用户画像
func migrateUserProfile(baseDir, userDirName string, dbUserID int) (bool, error) {
	claudeDir := filepath.Join(baseDir, userDirName, ".claude")

	// 读取 userinfo.json
	var userInfo userInfoJSON
	if data, err := os.ReadFile(filepath.Join(claudeDir, "userinfo.json")); err == nil {
		json.Unmarshal(data, &userInfo)
	}

	// 读取 profile.json
	var profile profileJSON
	if data, err := os.ReadFile(filepath.Join(claudeDir, "profile.json")); err == nil {
		json.Unmarshal(data, &profile)
	}

	// 如果两个文件都不存在，跳过
	if userInfo.UserName == "" && profile.UserID == "" && userInfo.Email == "" {
		return false, nil
	}

	// 转换 InstalledItems
	installedJSON, _ := json.Marshal(userInfo.InstalledItems)
	languagesJSON, _ := json.Marshal(profile.Languages)
	frameworksJSON, _ := json.Marshal(profile.Frameworks)
	codingStyleJSON, _ := json.Marshal(profile.CodingStyle)
	projectsJSON, _ := json.Marshal(profile.CurrentProjects)
	topToolsJSON, _ := json.Marshal(profile.TopTools)

	dbProfile := &CoworkerUserProfile{
		UserID:           dbUserID,
		UserName:         userInfo.UserName,
		CoworkerName:     userInfo.CoworkerName,
		Phone:            userInfo.Phone,
		Email:            userInfo.Email,
		ApiTokenKey:      userInfo.ApiTokenKey,
		ApiTokenName:     userInfo.ApiTokenName,
		SelectedModel:    userInfo.SelectedModel,
		Group:            userInfo.Group,
		AssistantAvatar:  userInfo.AssistantAvatar,
		InstalledItems:   string(installedJSON),
		Temperature:      userInfo.Temperature,
		TopP:             userInfo.TopP,
		FrequencyPenalty: userInfo.FrequencyPenalty,
		PresencePenalty:  userInfo.PresencePenalty,
		Languages:        string(languagesJSON),
		Frameworks:       string(frameworksJSON),
		CodingStyle:      string(codingStyleJSON),
		ResponseStyle:    profile.ResponseStyle,
		UILanguage:       profile.Language,
		CurrentProjects:  string(projectsJSON),
		TotalSessions:    profile.TotalSessions,
		TotalMessages:    profile.TotalMessages,
		TopTools:         string(topToolsJSON),
	}

	return true, UpsertCoworkerUserProfile(dbProfile)
}

// migrateJobs 迁移 Jobs
func migrateJobs(baseDir, userDirName string, dbUserID int) (int, error) {
	jobsDir := filepath.Join(baseDir, userDirName, ".claude", "jobs")
	entries, err := os.ReadDir(jobsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	var jobs []*CoworkerJob
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(jobsDir, entry.Name()))
		if err != nil {
			continue
		}

		var raw jobJSON
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}

		weekdaysJSON, _ := json.Marshal(raw.Weekdays)
		metadataJSON, _ := json.Marshal(raw.Metadata)

		job := &CoworkerJob{
			ID:              raw.ID,
			UserID:          dbUserID,
			Name:            raw.Name,
			Command:         raw.Command,
			Enabled:         raw.Enabled,
			LastRun:         raw.LastRun,
			NextRun:         raw.NextRun,
			Status:          string(raw.Status),
			LastError:       raw.LastError,
			SortOrder:       raw.Order,
			ScheduleType:    string(raw.ScheduleType),
			Time:            raw.Time,
			Weekdays:        string(weekdaysJSON),
			IntervalMinutes: raw.IntervalMinutes,
			RunAt:           raw.RunAt,
			CronExpr:        raw.CronExpr,
			Metadata:        string(metadataJSON),
			CreatedAt:       raw.CreatedAt,
			UpdatedAt:       raw.UpdatedAt,
		}
		jobs = append(jobs, job)
	}

	if len(jobs) == 0 {
		return 0, nil
	}

	return BatchCreateCoworkerJobs(jobs)
}

// migrateStoreItems 迁移全局商店条目
func migrateStoreItems(baseDir string) (int, error) {
	storeFile := filepath.Join(baseDir, "store", "items.json")
	data, err := os.ReadFile(storeFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	var rawItems []storeItemJSON
	if err := json.Unmarshal(data, &rawItems); err != nil {
		return 0, err
	}

	var items []*CoworkerStoreItem
	for _, raw := range rawItems {
		configJSON, _ := json.Marshal(raw.ConfigSchema)
		item := &CoworkerStoreItem{
			ID:           raw.ID,
			Name:         raw.Name,
			Description:  raw.Description,
			Type:         string(raw.Type),
			Icon:         raw.Icon,
			Author:       raw.Author,
			GithubURL:    raw.GithubURL,
			Content:      raw.Content,
			LocalDir:     raw.LocalDir,
			ServerURL:    raw.ServerURL,
			ConfigSchema: string(configJSON),
			CreatedAt:    raw.CreatedAt.Unix(),
			UpdatedAt:    raw.UpdatedAt.Unix(),
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		return 0, nil
	}

	return BatchCreateCoworkerStoreItems(items)
}

// computeContentHash 计算内容哈希（与 memory.ContentHash 一致）
func computeContentHash(content string) string {
	// 标准化内容：去除多余空白、转小写
	normalized := strings.ToLower(strings.TrimSpace(content))
	normalized = strings.Join(strings.Fields(normalized), " ")
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:8]) // 只取前8字节
}

// ========== JSON 文件结构体（仅迁移用） ==========

type memoryJSON struct {
	ID         string                 `json:"id"`
	UserID     string                 `json:"user_id"`
	Tags       []string               `json:"tags"`
	Content    string                 `json:"content"`
	Summary    string                 `json:"summary"`
	Source     string                 `json:"source"`
	SessionID  string                 `json:"session_id"`
	WindowID   string                 `json:"window_id"`
	Weight     float64                `json:"weight"`
	AccessCnt  int                    `json:"access_cnt"`
	Metadata   map[string]interface{} `json:"metadata"`
	CreatedAt  int64                  `json:"created_at"`
	UpdatedAt  int64                  `json:"updated_at"`
	LastAccess int64                  `json:"last_access"`
}

type userInfoJSON struct {
	UserName         string                  `json:"user_name"`
	CoworkerName     string                  `json:"coworker_name"`
	Phone            string                  `json:"phone"`
	Email            string                  `json:"email"`
	ApiTokenKey      string                  `json:"api_token_key"`
	ApiTokenName     string                  `json:"api_token_name"`
	SelectedModel    string                  `json:"selected_model"`
	Group            string                  `json:"group"`
	AssistantAvatar  string                  `json:"assistant_avatar"`
	InstalledItems   []userStoreItemJSON     `json:"installed_items"`
	Temperature      *float64                `json:"temperature"`
	TopP             *float64                `json:"top_p"`
	FrequencyPenalty *float64                `json:"frequency_penalty"`
	PresencePenalty  *float64                `json:"presence_penalty"`
}

type userStoreItemJSON struct {
	ItemID  string            `json:"item_id"`
	Enabled bool              `json:"enabled"`
	Config  map[string]string `json:"config,omitempty"`
}

type profileJSON struct {
	UserID          string            `json:"user_id"`
	Languages       []string          `json:"languages"`
	Frameworks      []string          `json:"frameworks"`
	CodingStyle     map[string]string `json:"coding_style"`
	ResponseStyle   string            `json:"response_style"`
	Language        string            `json:"language"`
	CurrentProjects []projectJSON     `json:"current_projects"`
	TotalSessions   int               `json:"total_sessions"`
	TotalMessages   int               `json:"total_messages"`
	TopTools        map[string]int    `json:"top_tools"`
	CreatedAt       int64             `json:"created_at"`
	UpdatedAt       int64             `json:"updated_at"`
}

type projectJSON struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	TechStack   []string `json:"tech_stack"`
	Description string   `json:"description"`
	LastAccess  int64    `json:"last_access"`
}

type jobJSON struct {
	ID              string                 `json:"id"`
	UserID          string                 `json:"user_id"`
	Name            string                 `json:"name"`
	Command         string                 `json:"command"`
	Enabled         bool                   `json:"enabled"`
	LastRun         int64                  `json:"last_run"`
	NextRun         int64                  `json:"next_run"`
	Status          string                 `json:"status"`
	LastError       string                 `json:"last_error"`
	Order           int                    `json:"order"`
	ScheduleType    string                 `json:"schedule_type"`
	Time            string                 `json:"time"`
	Weekdays        []int                  `json:"weekdays"`
	IntervalMinutes int                    `json:"interval_minutes"`
	RunAt           int64                  `json:"run_at"`
	CronExpr        string                 `json:"cron_expr"`
	Metadata        map[string]interface{} `json:"metadata"`
	CreatedAt       int64                  `json:"created_at"`
	UpdatedAt       int64                  `json:"updated_at"`
}

type storeItemJSON struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Type         string          `json:"type"`
	Icon         string          `json:"icon"`
	Author       string          `json:"author"`
	GithubURL    string          `json:"github_url"`
	Content      string          `json:"content"`
	LocalDir     string          `json:"local_dir"`
	ServerURL    string          `json:"server_url"`
	ConfigSchema json.RawMessage `json:"config_schema"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
