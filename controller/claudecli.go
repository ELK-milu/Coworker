package controller

import (
	"github.com/QuantumNous/new-api/claudecli"
	"github.com/gin-gonic/gin"
)

// ClaudeCLIController claudecli 模块控制器
type ClaudeCLIController struct {
	module *claudecli.Module
}

// NewClaudeCLIController 创建控制器
func NewClaudeCLIController() *ClaudeCLIController {
	return &ClaudeCLIController{
		module: claudecli.Init(),
	}
}

// Health 健康检查
func (ctrl *ClaudeCLIController) Health(c *gin.Context) {
	ctrl.module.RESTHandler.Health(c)
}

// CreateSession 创建会话
func (ctrl *ClaudeCLIController) CreateSession(c *gin.Context) {
	ctrl.module.RESTHandler.CreateSession(c)
}

// GetSession 获取会话
func (ctrl *ClaudeCLIController) GetSession(c *gin.Context) {
	ctrl.module.RESTHandler.GetSession(c)
}

// DeleteSession 删除会话
func (ctrl *ClaudeCLIController) DeleteSession(c *gin.Context) {
	ctrl.module.RESTHandler.DeleteSession(c)
}

// HandleWebSocket 处理 WebSocket 连接
func (ctrl *ClaudeCLIController) HandleWebSocket(c *gin.Context) {
	ctrl.module.WSHandler.Handle(c)
}

// UploadFile 上传文件
func (ctrl *ClaudeCLIController) UploadFile(c *gin.Context) {
	ctrl.module.FileHandler.Upload(c)
}

// DownloadFile 下载文件
func (ctrl *ClaudeCLIController) DownloadFile(c *gin.Context) {
	ctrl.module.FileHandler.Download(c)
}

// ========== 会话管理 ==========

// ListSessions 获取会话列表
func (ctrl *ClaudeCLIController) ListSessions(c *gin.Context) {
	ctrl.module.RESTHandler.ListSessions(c)
}

// GetSessionHistory 获取会话历史消息（前端格式）
func (ctrl *ClaudeCLIController) GetSessionHistory(c *gin.Context) {
	ctrl.module.RESTHandler.GetSessionHistory(c)
}

// ========== 任务管理 ==========

// ListTasks 获取任务列表
func (ctrl *ClaudeCLIController) ListTasks(c *gin.Context) {
	ctrl.module.RESTHandler.ListTasks(c)
}

// CreateTask 创建任务
func (ctrl *ClaudeCLIController) CreateTask(c *gin.Context) {
	ctrl.module.RESTHandler.CreateTask(c)
}

// UpdateTask 更新任务
func (ctrl *ClaudeCLIController) UpdateTask(c *gin.Context) {
	ctrl.module.RESTHandler.UpdateTask(c)
}

// DeleteTask 删除任务
func (ctrl *ClaudeCLIController) DeleteTask(c *gin.Context) {
	ctrl.module.RESTHandler.DeleteTask(c)
}

// ReorderTasks 批量排序任务
func (ctrl *ClaudeCLIController) ReorderTasks(c *gin.Context) {
	ctrl.module.RESTHandler.ReorderTasks(c)
}

// ========== 文件管理 ==========

// ListFiles 获取文件列表
func (ctrl *ClaudeCLIController) ListFiles(c *gin.Context) {
	ctrl.module.RESTHandler.ListFiles(c)
}

// CreateFolder 创建文件夹
func (ctrl *ClaudeCLIController) CreateFolder(c *gin.Context) {
	ctrl.module.RESTHandler.CreateFolder(c)
}

// DeleteFile 删除文件
func (ctrl *ClaudeCLIController) DeleteFile(c *gin.Context) {
	ctrl.module.RESTHandler.DeleteFile(c)
}

// RenameFile 重命名文件
func (ctrl *ClaudeCLIController) RenameFile(c *gin.Context) {
	ctrl.module.RESTHandler.RenameFile(c)
}

// ========== 配置管理 ==========

// GetConfig 获取配置
func (ctrl *ClaudeCLIController) GetConfig(c *gin.Context) {
	ctrl.module.RESTHandler.GetConfig(c)
}

// SaveConfig 保存配置
func (ctrl *ClaudeCLIController) SaveConfig(c *gin.Context) {
	ctrl.module.RESTHandler.SaveConfig(c)
}
