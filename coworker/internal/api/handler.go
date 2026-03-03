package api

import (
	"archive/zip"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/coworker/internal/client"
	"github.com/QuantumNous/new-api/coworker/internal/config"
	"github.com/QuantumNous/new-api/coworker/internal/job"
	"github.com/QuantumNous/new-api/coworker/internal/mcp"
	"github.com/QuantumNous/new-api/coworker/internal/memory"
	"github.com/QuantumNous/new-api/coworker/internal/session"
	"github.com/QuantumNous/new-api/coworker/internal/store"
	"github.com/QuantumNous/new-api/coworker/internal/task"
	"github.com/QuantumNous/new-api/coworker/internal/workspace"
	"github.com/QuantumNous/new-api/coworker/pkg/types"
	"github.com/gin-gonic/gin"
)

// RESTHandler REST API 处理器
type RESTHandler struct {
	sessions  *session.Manager
	tasks     *task.Manager
	workspace *workspace.Manager
	jobs      *job.Manager
	memories  *memory.Manager
	store     *store.Manager
	mcpMgr    *mcp.Manager
	config    *config.Config
	aiClient  *client.ClaudeClient
}

// SetAIClient 设置 AI 客户端（用于分类等）
func (h *RESTHandler) SetAIClient(c *client.ClaudeClient) {
	h.aiClient = c
}

// SetStoreManager 设置商店管理器
func (h *RESTHandler) SetStoreManager(sm *store.Manager) {
	h.store = sm
}

// NewRESTHandler 创建 REST 处理器
func NewRESTHandler(sm *session.Manager) *RESTHandler {
	return &RESTHandler{sessions: sm}
}

// SetTaskManager 设置任务管理器
func (h *RESTHandler) SetTaskManager(tm *task.Manager) {
	h.tasks = tm
}

// SetWorkspaceManager 设置工作空间管理器
func (h *RESTHandler) SetWorkspaceManager(wm *workspace.Manager) {
	h.workspace = wm
}

// SetJobManager 设置 Job 管理器
func (h *RESTHandler) SetJobManager(jm *job.Manager) {
	h.jobs = jm
}

// SetMemoryManager 设置记忆管理器
func (h *RESTHandler) SetMemoryManager(mm *memory.Manager) {
	h.memories = mm
}

// SetMCPManager 设置 MCP 管理器
func (h *RESTHandler) SetMCPManager(mgr *mcp.Manager) {
	h.mcpMgr = mgr
}

// SetConfig 设置配置
func (h *RESTHandler) SetConfig(cfg *config.Config) {
	h.config = cfg
}

// CreateSession 创建会话
func (h *RESTHandler) CreateSession(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sess := h.sessions.Create(req.UserID)
	c.JSON(http.StatusOK, gin.H{"session_id": sess.ID})
}

// GetSession 获取会话
func (h *RESTHandler) GetSession(c *gin.Context) {
	id := c.Param("id")
	sess := h.sessions.Get(id)
	if sess == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	c.JSON(http.StatusOK, sess)
}

// GetSessionHistory 获取会话历史消息（前端格式）
func (h *RESTHandler) GetSessionHistory(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"session_id": "",
			"messages":   []interface{}{},
		})
		return
	}

	sess := h.sessions.Get(id)
	if sess == nil {
		c.JSON(http.StatusOK, gin.H{
			"session_id": id,
			"messages":   []interface{}{},
			"not_found":  true,
		})
		return
	}

	// 获取会话消息并转换为前端格式
	messages := sess.GetMessages()
	frontendMessages := ConvertMessagesToFrontend(messages)

	c.JSON(http.StatusOK, gin.H{
		"session_id": id,
		"messages":   frontendMessages,
	})
}

// DeleteSession 删除会话
func (h *RESTHandler) DeleteSession(c *gin.Context) {
	id := c.Param("id")
	h.sessions.Delete(id)
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// Health 健康检查
func (h *RESTHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ========== 会话管理 API ==========

// ListSessions 获取会话列表
func (h *RESTHandler) ListSessions(c *gin.Context) {
	userID := c.Query("user_id")
	sessions := h.sessions.List(userID)

	var sessionList []map[string]interface{}
	for _, sess := range sessions {
		// 优先使用会话的 Title 字段
		title := sess.GetTitle()
		if title == "" {
			// 后备：获取第一条用户消息作为标题
			title = "新对话"
			messages := sess.GetMessages()
			for _, msg := range messages {
				if msg.Role == "user" {
					for _, block := range msg.Content {
						if textBlock, ok := block.(types.TextBlock); ok {
							title = textBlock.Text
							if runes := []rune(title); len(runes) > 50 {
								title = string(runes[:50]) + "..."
							}
							break
						}
						if blockMap, ok := block.(map[string]interface{}); ok {
							if blockMap["type"] == "text" {
								if text, ok := blockMap["text"].(string); ok {
									title = text
									if runes := []rune(title); len(runes) > 50 {
										title = string(runes[:50]) + "..."
									}
									break
								}
							}
						}
					}
					break
				}
			}
		}

		sessionList = append(sessionList, map[string]interface{}{
			"id":            sess.ID,
			"title":         title,
			"created_at":    sess.CreatedAt.Unix(),
			"updated_at":    sess.UpdatedAt.Unix(),
			"message_count": len(sess.GetMessages()),
		})
	}

	c.JSON(http.StatusOK, gin.H{"sessions": sessionList})
}

// ========== 任务管理 API ==========

// ListTasks 获取任务列表
func (h *RESTHandler) ListTasks(c *gin.Context) {
	userID := c.Query("user_id")
	listID := c.DefaultQuery("list_id", "default")

	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	if h.tasks == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "task manager not initialized"})
		return
	}

	tasks := h.tasks.List(userID, listID)
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

// CreateTask 创建任务
func (h *RESTHandler) CreateTask(c *gin.Context) {
	var req struct {
		UserID      string `json:"user_id"`
		ListID      string `json:"list_id"`
		Subject     string `json:"subject"`
		Description string `json:"description"`
		ActiveForm  string `json:"activeForm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	if req.ListID == "" {
		req.ListID = "default"
	}

	if h.tasks == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "task manager not initialized"})
		return
	}

	newTask, err := h.tasks.Create(req.UserID, req.ListID, req.Subject, req.Description, req.ActiveForm)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "task": newTask})
}

// UpdateTask 更新任务
func (h *RESTHandler) UpdateTask(c *gin.Context) {
	taskID := c.Param("id")
	var req struct {
		UserID      string `json:"user_id"`
		ListID      string `json:"list_id"`
		Status      string `json:"status,omitempty"`
		Subject     string `json:"subject,omitempty"`
		Description string `json:"description,omitempty"`
		ActiveForm  string `json:"activeForm,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	if req.ListID == "" {
		req.ListID = "default"
	}

	updates := make(map[string]interface{})
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Subject != "" {
		updates["subject"] = req.Subject
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.ActiveForm != "" {
		updates["activeForm"] = req.ActiveForm
	}

	updatedTask, err := h.tasks.Update(req.UserID, req.ListID, taskID, updates)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "task": updatedTask})
}

// DeleteTask 删除任务
func (h *RESTHandler) DeleteTask(c *gin.Context) {
	taskID := c.Param("id")
	userID := c.Query("user_id")
	listID := c.DefaultQuery("list_id", "default")

	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	updates := map[string]interface{}{"status": "deleted"}
	_, err := h.tasks.Update(userID, listID, taskID, updates)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ReorderTasks 批量排序任务
func (h *RESTHandler) ReorderTasks(c *gin.Context) {
	var req struct {
		UserID  string   `json:"user_id"`
		ListID  string   `json:"list_id"`
		TaskIDs []string `json:"task_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	if req.ListID == "" {
		req.ListID = "default"
	}

	err := h.tasks.UpdateOrder(req.UserID, req.ListID, req.TaskIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ========== 文件管理 API ==========

// ListFiles 获取文件列表
func (h *RESTHandler) ListFiles(c *gin.Context) {
	userID := c.Query("user_id")
	path := c.Query("path")

	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	if h.workspace == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "workspace not initialized"})
		return
	}

	if err := h.workspace.EnsureUserWorkspace(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	files, err := h.workspace.ListFiles(userID, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"files": files, "path": path})
}

// CreateFolder 创建文件夹
func (h *RESTHandler) CreateFolder(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id"`
		Path   string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == "" || req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id and path are required"})
		return
	}

	if err := h.workspace.CreateFolder(req.UserID, req.Path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "path": req.Path})
}

// DeleteFile 删除文件或文件夹
func (h *RESTHandler) DeleteFile(c *gin.Context) {
	userID := c.Query("user_id")
	path := c.Query("path")

	if userID == "" || path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id and path are required"})
		return
	}

	if err := h.workspace.DeleteFile(userID, path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RenameFile 重命名文件或文件夹
func (h *RESTHandler) RenameFile(c *gin.Context) {
	var req struct {
		UserID  string `json:"user_id"`
		Path    string `json:"path"`
		NewName string `json:"new_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == "" || req.Path == "" || req.NewName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id, path and new_name are required"})
		return
	}

	if err := h.workspace.RenameFile(req.UserID, req.Path, req.NewName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "new_name": req.NewName})
}

// GetWorkspaceStats 获取工作空间使用统计
func (h *RESTHandler) GetWorkspaceStats(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	if h.workspace == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "workspace not initialized"})
		return
	}

	stats, err := h.workspace.GetWorkspaceStats(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	cfg := config.Get()
	quotaMB := cfg.Security.WorkspaceQuotaMB
	quotaBytes := int64(quotaMB) * 1024 * 1024

	c.JSON(http.StatusOK, gin.H{
		"total_size":  stats["total_size"],
		"file_count":  stats["file_count"],
		"dir_count":   stats["dir_count"],
		"quota_bytes": quotaBytes,
		"quota_mb":    quotaMB,
	})
}

// ========== 配置管理 API ==========

// GetConfig 获取配置
func (h *RESTHandler) GetConfig(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	content, err := h.workspace.LoadConfig(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"content": ""})
		return
	}

	c.JSON(http.StatusOK, gin.H{"content": content})
}

// SaveConfig 保存配置
func (h *RESTHandler) SaveConfig(c *gin.Context) {
	var req struct {
		UserID  string `json:"user_id"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	if err := h.workspace.SaveConfig(req.UserID, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ========== 用户信息 API ==========

// GetUserInfo 获取用户信息
func (h *RESTHandler) GetUserInfo(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	info, err := h.workspace.LoadUserInfo(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_name":         info.UserName,
		"coworker_name":     info.CoworkerName,
		"phone":             info.Phone,
		"email":             info.Email,
		"api_token_key":     info.ApiTokenKey,
		"api_token_name":    info.ApiTokenName,
		"selected_model":    info.SelectedModel,
		"group":             info.Group,
		"assistant_avatar":  info.AssistantAvatar,
		"temperature":       info.Temperature,
		"top_p":             info.TopP,
		"frequency_penalty": info.FrequencyPenalty,
		"presence_penalty":  info.PresencePenalty,
	})
}

// SaveUserInfo 保存用户信息
func (h *RESTHandler) SaveUserInfo(c *gin.Context) {
	var req struct {
		UserID           string   `json:"user_id"`
		UserName         string   `json:"user_name"`
		CoworkerName     string   `json:"coworker_name"`
		Phone            string   `json:"phone"`
		Email            string   `json:"email"`
		ApiTokenKey      string   `json:"api_token_key"`
		ApiTokenName     string   `json:"api_token_name"`
		AssistantAvatar  string   `json:"assistant_avatar"`
		SelectedModel    string   `json:"selected_model"`
		Group            string   `json:"group"`
		Temperature      *float64 `json:"temperature"`
		TopP             *float64 `json:"top_p"`
		FrequencyPenalty *float64 `json:"frequency_penalty"`
		PresencePenalty  *float64 `json:"presence_penalty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	// 先加载已有数据，保留 InstalledItems 等非本接口管理的字段
	info, err := h.workspace.LoadUserInfo(req.UserID)
	if err != nil {
		info = &workspace.UserInfo{}
	}
	info.UserName = req.UserName
	info.CoworkerName = req.CoworkerName
	info.Phone = req.Phone
	info.Email = req.Email
	info.ApiTokenKey = req.ApiTokenKey
	info.ApiTokenName = req.ApiTokenName
	info.AssistantAvatar = req.AssistantAvatar
	info.SelectedModel = req.SelectedModel
	info.Group = req.Group
	info.Temperature = req.Temperature
	info.TopP = req.TopP
	info.FrequencyPenalty = req.FrequencyPenalty
	info.PresencePenalty = req.PresencePenalty

	if err := h.workspace.SaveUserInfo(req.UserID, info); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ========== Job 管理 API ==========

// ListJobs 获取 Job 列表
func (h *RESTHandler) ListJobs(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	if h.jobs == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "job manager not initialized"})
		return
	}

	jobs := h.jobs.List(userID)
	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

// CreateJob 创建 Job
func (h *RESTHandler) CreateJob(c *gin.Context) {
	var req struct {
		UserID          string `json:"user_id"`
		Name            string `json:"name"`
		Command         string `json:"command"`
		ScheduleType    string `json:"schedule_type"`
		Time            string `json:"time,omitempty"`
		Weekdays        []int  `json:"weekdays,omitempty"`
		IntervalMinutes int    `json:"interval_minutes,omitempty"`
		RunAt           string `json:"run_at,omitempty"`
		CronExpr        string `json:"cron_expr,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == "" || req.Name == "" || req.Command == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id, name and command are required"})
		return
	}

	if h.jobs == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "job manager not initialized"})
		return
	}

	// 默认调度类型
	scheduleType := job.ScheduleType(req.ScheduleType)
	if scheduleType == "" {
		if req.CronExpr != "" {
			scheduleType = job.ScheduleCron
		} else {
			scheduleType = job.ScheduleDaily
		}
	}

	// 解析 run_at（前端传 datetime-local 格式 "2026-02-15T09:00"）
	var runAt int64
	if req.RunAt != "" {
		if t, err := time.Parse("2006-01-02T15:04", req.RunAt); err == nil {
			runAt = t.UnixMilli()
		}
	}

	newJob, err := h.jobs.CreateWithSchedule(
		req.UserID, req.Name, req.Command,
		scheduleType, req.CronExpr, req.Time,
		req.Weekdays, req.IntervalMinutes, runAt,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "job": newJob})
}

// UpdateJob 更新 Job
func (h *RESTHandler) UpdateJob(c *gin.Context) {
	jobID := c.Param("id")
	var req struct {
		UserID          string `json:"user_id"`
		Name            string `json:"name,omitempty"`
		CronExpr        string `json:"cron_expr,omitempty"`
		Command         string `json:"command,omitempty"`
		Enabled         *bool  `json:"enabled,omitempty"`
		ScheduleType    string `json:"schedule_type,omitempty"`
		Time            string `json:"time,omitempty"`
		Weekdays        []int  `json:"weekdays,omitempty"`
		IntervalMinutes *int   `json:"interval_minutes,omitempty"`
		RunAt           string `json:"run_at,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	if h.jobs == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "job manager not initialized"})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.CronExpr != "" {
		updates["cron_expr"] = req.CronExpr
	}
	if req.Command != "" {
		updates["command"] = req.Command
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.ScheduleType != "" {
		updates["schedule_type"] = req.ScheduleType
	}
	if req.Time != "" {
		updates["time"] = req.Time
	}
	if len(req.Weekdays) > 0 {
		updates["weekdays"] = req.Weekdays
	}
	if req.IntervalMinutes != nil {
		updates["interval_minutes"] = *req.IntervalMinutes
	}
	if req.RunAt != "" {
		// 解析 datetime-local 格式
		if t, err := time.Parse("2006-01-02T15:04", req.RunAt); err == nil {
			updates["run_at"] = t.UnixMilli()
		}
	}

	updatedJob, err := h.jobs.Update(req.UserID, jobID, updates)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "job": updatedJob})
}

// DeleteJob 删除 Job
func (h *RESTHandler) DeleteJob(c *gin.Context) {
	jobID := c.Param("id")
	userID := c.Query("user_id")

	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	if h.jobs == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "job manager not initialized"})
		return
	}

	if err := h.jobs.Delete(userID, jobID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RunJob 手动触发 Job
func (h *RESTHandler) RunJob(c *gin.Context) {
	jobID := c.Param("id")
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	if h.jobs == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "job manager not initialized"})
		return
	}

	jobItem := h.jobs.Get(req.UserID, jobID)
	if jobItem == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	// 异步触发执行
	if err := h.jobs.RunNow(req.UserID, jobID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"job":     jobItem,
		"message": "Job triggered. AI is processing in background.",
	})
}

// ReorderJobs 批量排序 Jobs
func (h *RESTHandler) ReorderJobs(c *gin.Context) {
	var req struct {
		UserID string   `json:"user_id"`
		JobIDs []string `json:"job_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	if h.jobs == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "job manager not initialized"})
		return
	}

	if err := h.jobs.UpdateOrder(req.UserID, req.JobIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ========== 记忆管理 API ==========

// ListMemories 获取记忆列表（按 weight 降序）
func (h *RESTHandler) ListMemories(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	if h.memories == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "memory manager not initialized"})
		return
	}

	// 确保用户记忆已加载
	h.memories.LoadUserMemories(userID)

	memories := h.memories.List(userID)

	// 排序：weight 降序 → access_cnt 降序 → summary 字母序
	sort.Slice(memories, func(i, j int) bool {
		if memories[i].Weight != memories[j].Weight {
			return memories[i].Weight > memories[j].Weight
		}
		if memories[i].AccessCnt != memories[j].AccessCnt {
			return memories[i].AccessCnt > memories[j].AccessCnt
		}
		return memories[i].Summary < memories[j].Summary
	})

	c.JSON(http.StatusOK, gin.H{"memories": memories})
}

// GetMemory 获取单条记忆
func (h *RESTHandler) GetMemory(c *gin.Context) {
	userID := c.Query("user_id")
	memoryID := c.Param("id")

	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	if h.memories == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "memory manager not initialized"})
		return
	}

	mem, err := h.memories.Get(userID, memoryID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"memory": mem})
}

// CreateMemory 创建记忆
func (h *RESTHandler) CreateMemory(c *gin.Context) {
	var req struct {
		UserID  string   `json:"user_id"`
		Tags    []string `json:"tags"`
		Content string   `json:"content"`
		Summary string   `json:"summary"`
		Weight  float64  `json:"weight"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == "" || len(req.Tags) == 0 || req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id, tags and content are required"})
		return
	}

	if h.memories == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "memory manager not initialized"})
		return
	}

	if req.Weight <= 0 || req.Weight > 1 {
		req.Weight = 0.5
	}
	if req.Summary == "" {
		req.Summary = req.Content
		if len(req.Summary) > 50 {
			req.Summary = req.Summary[:50] + "..."
		}
	}

	mem := &memory.Memory{
		Tags:    req.Tags,
		Content: req.Content,
		Summary: req.Summary,
		Source:  "manual",
		Weight:  req.Weight,
	}

	created, err := h.memories.Create(req.UserID, mem)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "memory": created})
}

// UpdateMemory 更新记忆
func (h *RESTHandler) UpdateMemory(c *gin.Context) {
	memoryID := c.Param("id")
	var req struct {
		UserID  string   `json:"user_id"`
		Tags    []string `json:"tags,omitempty"`
		Content string   `json:"content,omitempty"`
		Summary string   `json:"summary,omitempty"`
		Weight  float64  `json:"weight,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	if h.memories == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "memory manager not initialized"})
		return
	}

	updates := make(map[string]interface{})
	if len(req.Tags) > 0 {
		updates["tags"] = req.Tags
	}
	if req.Content != "" {
		updates["content"] = req.Content
	}
	if req.Summary != "" {
		updates["summary"] = req.Summary
	}
	if req.Weight > 0 && req.Weight <= 1 {
		updates["weight"] = req.Weight
	}

	updated, err := h.memories.Update(req.UserID, memoryID, updates)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "memory": updated})
}

// DeleteMemory 删除记忆
func (h *RESTHandler) DeleteMemory(c *gin.Context) {
	memoryID := c.Param("id")
	userID := c.Query("user_id")

	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	if h.memories == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "memory manager not initialized"})
		return
	}

	if err := h.memories.Delete(userID, memoryID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// SearchMemories 搜索记忆
func (h *RESTHandler) SearchMemories(c *gin.Context) {
	userID := c.Query("user_id")
	query := c.Query("q")

	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	if h.memories == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "memory manager not initialized"})
		return
	}

	// 确保用户记忆已加载
	h.memories.LoadUserMemories(userID)

	if query == "" {
		// 无查询词时返回全部
		memories := h.memories.List(userID)
		sort.Slice(memories, func(i, j int) bool {
			if memories[i].Weight != memories[j].Weight {
				return memories[i].Weight > memories[j].Weight
			}
			if memories[i].AccessCnt != memories[j].AccessCnt {
				return memories[i].AccessCnt > memories[j].AccessCnt
			}
			return memories[i].Summary < memories[j].Summary
		})
		c.JSON(http.StatusOK, gin.H{"memories": memories})
		return
	}

	memories := h.memories.Retrieve(userID, query, 20)

	// 排序：weight 降序 → access_cnt 降序 → summary 字母序
	sort.Slice(memories, func(i, j int) bool {
		if memories[i].Weight != memories[j].Weight {
			return memories[i].Weight > memories[j].Weight
		}
		if memories[i].AccessCnt != memories[j].AccessCnt {
			return memories[i].AccessCnt > memories[j].AccessCnt
		}
		return memories[i].Summary < memories[j].Summary
	})

	c.JSON(http.StatusOK, gin.H{"memories": memories})
}

// ========== 技能商店 API ==========

// ListStoreItems 列出所有商店条目
func (h *RESTHandler) ListStoreItems(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusOK, gin.H{"items": []interface{}{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": h.store.List()})
}

// CreateStoreItem 创建商店条目（仅管理员）
func (h *RESTHandler) CreateStoreItem(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store not initialized"})
		return
	}
	var item store.StoreItem
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	created, err := h.store.Create(item)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "item": created})
}

// UpdateStoreItem 更新商店条目（仅管理员）
func (h *RESTHandler) UpdateStoreItem(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store not initialized"})
		return
	}
	id := c.Param("id")
	var item store.StoreItem
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updated, err := h.store.Update(id, item)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "item": updated})
}

// DeleteStoreItem 删除商店条目（仅管理员）
func (h *RESTHandler) DeleteStoreItem(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store not initialized"})
		return
	}
	id := c.Param("id")

	// 级联清理：从所有用户的已安装列表中移除该 item
	if err := h.store.RemoveItemFromAllUsers(id); err != nil {
		// 不阻塞删除，仅记录警告
		c.JSON(http.StatusOK, gin.H{"success": false, "warning": "cascade cleanup failed: " + err.Error()})
	}

	if err := h.store.Delete(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ImportStoreItems 从 GitHub 导入商店条目（仅管理员）
func (h *RESTHandler) ImportStoreItems(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store not initialized"})
		return
	}
	var req struct {
		RepoURL    string `json:"repo_url"`
		ImportType string `json:"import_type"` // "skill"(默认) 或 "agent" 或 "plugin"
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.RepoURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repo_url is required"})
		return
	}

	var items []store.StoreItem
	var err error
	switch req.ImportType {
	case "plugin":
		items, err = h.store.ImportPlugin(req.RepoURL)
	case "agent":
		items, err = h.store.ImportAgents(req.RepoURL)
	default:
		items, err = h.store.Import(req.RepoURL)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "items": items, "count": len(items)})
}

// GetUserStore 获取用户已安装的商店条目
func (h *RESTHandler) GetUserStore(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	if h.store == nil {
		c.JSON(http.StatusOK, gin.H{"installed": []string{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"installed": h.store.LoadUserInstalled(userID)})
}

// SaveUserStore 保存用户已安装的商店条目（批量替换，内部通过 diff 调用单项安装/卸载）
func (h *RESTHandler) SaveUserStore(c *gin.Context) {
	var req struct {
		UserID  string   `json:"user_id"`
		ItemIDs []string `json:"item_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	if h.store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store not initialized"})
		return
	}

	var skillDir string
	if h.workspace != nil {
		skillDir = h.workspace.GetUserSkillDir(req.UserID)
	}

	// diff：计算需要安装和卸载的 item
	oldIDs := h.store.LoadUserInstalled(req.UserID)
	oldSet := make(map[string]bool, len(oldIDs))
	for _, id := range oldIDs {
		oldSet[id] = true
	}
	newSet := make(map[string]bool, len(req.ItemIDs))
	for _, id := range req.ItemIDs {
		newSet[id] = true
	}

	// 卸载：旧列表中有但新列表中没有的
	for _, id := range oldIDs {
		if !newSet[id] {
			h.store.UninstallItemForUser(req.UserID, id, skillDir)
		}
	}

	// 安装：新列表中有但旧列表中没有的
	for _, id := range req.ItemIDs {
		if !oldSet[id] {
			h.store.InstallItemForUser(req.UserID, id, skillDir)
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// InstallStoreItem 用户安装单个商店条目
func (h *RESTHandler) InstallStoreItem(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store not initialized"})
		return
	}

	id := c.Param("id")
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	var skillDir string
	if h.workspace != nil {
		skillDir = h.workspace.GetUserSkillDir(req.UserID)
	}

	if err := h.store.InstallItemForUser(req.UserID, id, skillDir); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// UninstallStoreItem 用户卸载单个商店条目
func (h *RESTHandler) UninstallStoreItem(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store not initialized"})
		return
	}

	id := c.Param("id")
	// 支持 query param 或 body 传递 user_id
	userID := c.Query("user_id")
	if userID == "" {
		var req struct {
			UserID string `json:"user_id"`
		}
		if err := c.ShouldBindJSON(&req); err == nil {
			userID = req.UserID
		}
	}
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	var skillDir string
	if h.workspace != nil {
		skillDir = h.workspace.GetUserSkillDir(userID)
	}

	if err := h.store.UninstallItemForUser(userID, id, skillDir); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ImportFromModelScope 从魔搭 MCP 广场导入 MCP 服务器（仅管理员）
func (h *RESTHandler) ImportFromModelScope(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store not initialized"})
		return
	}
	var req struct {
		ModelScopeURL string `json:"modelscope_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ModelScopeURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "modelscope_url is required"})
		return
	}

	item, err := h.store.ImportFromModelScope(req.ModelScopeURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "item": item})
}

// ========== MCP 配置 API ==========

// GetUserMCPConfig 获取用户对某 MCP 条目的配置 JSON
func (h *RESTHandler) GetUserMCPConfig(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusOK, gin.H{"mcp_json": ""})
		return
	}

	itemID := c.Param("id")
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	mcpJson := h.store.GetUserMCPJson(userID, itemID)
	c.JSON(http.StatusOK, gin.H{"mcp_json": mcpJson})
}

// SaveUserMCPConfig 保存用户对某 MCP 条目的配置 JSON
func (h *RESTHandler) SaveUserMCPConfig(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store not initialized"})
		return
	}

	itemID := c.Param("id")
	var req struct {
		UserID  string `json:"user_id"`
		MCPJson string `json:"mcp_json"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	// 校验服务名是否匹配
	item := h.store.GetByID(itemID)
	if item == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "item not found"})
		return
	}
	if req.MCPJson != "" {
		if err := mcp.ValidateMCPJson(req.MCPJson, item.Name); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	if err := h.store.SaveUserMCPJson(req.UserID, itemID, req.MCPJson); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// TestMCPConnection 测试 MCP 连接
func (h *RESTHandler) TestMCPConnection(c *gin.Context) {
	var req struct {
		MCPJson      string `json:"mcp_json"`
		ExpectedName string `json:"expected_name"`
		Timeout      int    `json:"timeout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.MCPJson == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mcp_json is required"})
		return
	}
	if req.Timeout <= 0 {
		req.Timeout = 15
	}

	// 校验服务名
	if req.ExpectedName != "" {
		if err := mcp.ValidateMCPJson(req.MCPJson, req.ExpectedName); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
	}

	// 解析 MCP JSON
	_, serverCfg, err := mcp.ParseMCPJson(req.MCPJson)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(req.Timeout)*time.Second)
	defer cancel()

	cfg := &mcp.TransportConfig{
		URL:     serverCfg.URL,
		Headers: serverCfg.Headers,
		Timeout: req.Timeout,
	}

	// 使用临时 manager 测试连接
	testMgr := mcp.NewManager()

	conn, err := testMgr.Connect(ctx, "test", cfg)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 连接成功，返回服务器信息和工具数量
	result := gin.H{
		"success":    true,
		"server":     conn.Server,
		"tool_count": len(conn.Tools),
	}
	if len(conn.Tools) > 0 {
		toolNames := make([]string, 0, len(conn.Tools))
		for _, t := range conn.Tools {
			toolNames = append(toolNames, t.Name)
		}
		result["tools"] = toolNames
	}

	// 断开测试连接
	testMgr.Disconnect(conn.ID)

	c.JSON(http.StatusOK, result)
}

// ========== 收藏 API ==========

// FavoriteStoreItem toggle 收藏
func (h *RESTHandler) FavoriteStoreItem(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store not initialized"})
		return
	}
	id := c.Param("id")
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	added, err := h.store.FavoriteItem(req.UserID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "favorited": added})
}

// GetUserFavorites 获取用户收藏列表
func (h *RESTHandler) GetUserFavorites(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	if h.store == nil {
		c.JSON(http.StatusOK, gin.H{"favorites": []string{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"favorites": h.store.LoadUserFavorites(userID)})
}

// ========== 分类 API ==========

// GetStoreItem 获取商店条目详情（含 readme 和文件树）
func (h *RESTHandler) GetStoreItem(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store not initialized"})
		return
	}
	id := c.Param("id")
	item := h.store.GetByID(id)
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"item":           item,
		"readme_content": h.store.GetItemReadme(item),
		"file_tree":      h.store.GetItemFileTree(item),
	})
}

// DownloadStoreItem 下载商店条目为 zip 文件
func (h *RESTHandler) DownloadStoreItem(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store not initialized"})
		return
	}
	id := c.Param("id")
	item := h.store.GetByID(id)
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}
	dir := h.store.ItemDir(item)
	if dir == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "no local files for this item"})
		return
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "item directory not found"})
		return
	}

	zipName := item.Name + ".zip"
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipName))

	zw := zip.NewWriter(c.Writer)
	defer zw.Close()

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		w.Write(data)
		return nil
	})
}

// ========== 分类 API ==========

// ClassifyStoreItem AI 分类单个条目
func (h *RESTHandler) ClassifyStoreItem(c *gin.Context) {
	if h.store == nil || h.aiClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "not initialized"})
		return
	}
	id := c.Param("id")
	item := h.store.GetByID(id)
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}
	desc := item.Description
	if item.DisplayDesc != "" {
		desc = item.DisplayDesc
	}
	cat := store.ClassifyItem(h.aiClient, item.DisplayName, desc)
	item.Category = cat
	updated, err := h.store.Update(id, *item)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "item": updated})
}

// ClassifyAllStoreItems AI 分类所有未分类条目
func (h *RESTHandler) ClassifyAllStoreItems(c *gin.Context) {
	if h.store == nil || h.aiClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "not initialized"})
		return
	}
	items := h.store.List()
	classified := 0
	for _, item := range items {
		if item.Category != "" {
			continue
		}
		desc := item.Description
		if item.DisplayDesc != "" {
			desc = item.DisplayDesc
		}
		cat := store.ClassifyItem(h.aiClient, item.DisplayName, desc)
		item.Category = cat
		h.store.Update(item.ID, item)
		classified++
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "classified": classified})
}

// ========== 内置模型 API ==========

// GetBuiltinModel 获取当前内置模型设置
func (h *RESTHandler) GetBuiltinModel(c *gin.Context) {
	modelName := "gpt-4o-mini"
	if v := store.GetBuiltinModelOption(); v != "" {
		modelName = v
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "model": modelName})
}

// SaveBuiltinModel 保存内置模型设置
func (h *RESTHandler) SaveBuiltinModel(c *gin.Context) {
	var req struct {
		Model string `json:"model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model is required"})
		return
	}

	store.SaveBuiltinModelOption(req.Model)

	// 重建 store manager 的 AI client
	newClient := store.CreateInternalAIClient()
	if newClient != nil {
		h.store.SetAIClient(newClient)
		h.aiClient = newClient
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
