package api

import (
	"net/http"

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
		title := "新对话"
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

		sessionList = append(sessionList, map[string]interface{}{
			"id":            sess.ID,
			"title":         title,
			"created_at":    sess.CreatedAt.Unix(),
			"updated_at":    sess.UpdatedAt.Unix(),
			"message_count": len(messages),
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
