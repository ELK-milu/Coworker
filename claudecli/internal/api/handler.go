package api

import (
	"net/http"

	"github.com/QuantumNous/new-api/claudecli/internal/job"
	"github.com/QuantumNous/new-api/claudecli/internal/session"
	"github.com/QuantumNous/new-api/claudecli/internal/task"
	"github.com/QuantumNous/new-api/claudecli/internal/workspace"
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
	"github.com/gin-gonic/gin"
)

// RESTHandler REST API 处理器
type RESTHandler struct {
	sessions  *session.Manager
	tasks     *task.Manager
	workspace *workspace.Manager
	jobs      *job.Manager
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
							if len(title) > 50 {
								title = title[:50] + "..."
							}
							break
						}
						if blockMap, ok := block.(map[string]interface{}); ok {
							if blockMap["type"] == "text" {
								if text, ok := blockMap["text"].(string); ok {
									title = text
									if len(title) > 50 {
										title = title[:50] + "..."
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
		UserID   string `json:"user_id"`
		Name     string `json:"name"`
		CronExpr string `json:"cron_expr"`
		Command  string `json:"command"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == "" || req.Name == "" || req.CronExpr == "" || req.Command == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id, name, cron_expr and command are required"})
		return
	}

	if h.jobs == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "job manager not initialized"})
		return
	}

	newJob, err := h.jobs.Create(req.UserID, req.Name, req.CronExpr, req.Command)
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
		UserID   string `json:"user_id"`
		Name     string `json:"name,omitempty"`
		CronExpr string `json:"cron_expr,omitempty"`
		Command  string `json:"command,omitempty"`
		Enabled  *bool  `json:"enabled,omitempty"`
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

	// 标记为运行中
	h.jobs.MarkRunning(req.UserID, jobID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"job":     jobItem,
		"message": "Job triggered manually. Check WebSocket for execution events.",
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
