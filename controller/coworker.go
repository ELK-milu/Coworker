package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/coworker"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// CoworkerController coworker 模块控制器
type CoworkerController struct {
	module *coworker.Module
}

// NewCoworkerController 创建控制器
func NewCoworkerController() *CoworkerController {
	return &CoworkerController{
		module: coworker.Init(),
	}
}

// Health 健康检查
func (ctrl *CoworkerController) Health(c *gin.Context) {
	ctrl.module.RESTHandler.Health(c)
}

// CreateSession 创建会话
func (ctrl *CoworkerController) CreateSession(c *gin.Context) {
	ctrl.module.RESTHandler.CreateSession(c)
}

// GetSession 获取会话
func (ctrl *CoworkerController) GetSession(c *gin.Context) {
	ctrl.module.RESTHandler.GetSession(c)
}

// DeleteSession 删除会话
func (ctrl *CoworkerController) DeleteSession(c *gin.Context) {
	ctrl.module.RESTHandler.DeleteSession(c)
}

// HandleWebSocket 处理 WebSocket 连接
func (ctrl *CoworkerController) HandleWebSocket(c *gin.Context) {
	ctrl.module.WSHandler.Handle(c)
}

// UploadFile 上传文件
func (ctrl *CoworkerController) UploadFile(c *gin.Context) {
	ctrl.module.FileHandler.Upload(c)
}

// DownloadFile 下载文件
func (ctrl *CoworkerController) DownloadFile(c *gin.Context) {
	ctrl.module.FileHandler.Download(c)
}

// PreviewFile 预览文件
func (ctrl *CoworkerController) PreviewFile(c *gin.Context) {
	ctrl.module.FileHandler.Preview(c)
}

// ========== 会话管理 ==========

// ListSessions 获取会话列表
func (ctrl *CoworkerController) ListSessions(c *gin.Context) {
	ctrl.module.RESTHandler.ListSessions(c)
}

// GetSessionHistory 获取会话历史消息（前端格式）
func (ctrl *CoworkerController) GetSessionHistory(c *gin.Context) {
	ctrl.module.RESTHandler.GetSessionHistory(c)
}

// ========== 任务管理 ==========

// ListTasks 获取任务列表
func (ctrl *CoworkerController) ListTasks(c *gin.Context) {
	ctrl.module.RESTHandler.ListTasks(c)
}

// CreateTask 创建任务
func (ctrl *CoworkerController) CreateTask(c *gin.Context) {
	ctrl.module.RESTHandler.CreateTask(c)
}

// UpdateTask 更新任务
func (ctrl *CoworkerController) UpdateTask(c *gin.Context) {
	ctrl.module.RESTHandler.UpdateTask(c)
}

// DeleteTask 删除任务
func (ctrl *CoworkerController) DeleteTask(c *gin.Context) {
	ctrl.module.RESTHandler.DeleteTask(c)
}

// ReorderTasks 批量排序任务
func (ctrl *CoworkerController) ReorderTasks(c *gin.Context) {
	ctrl.module.RESTHandler.ReorderTasks(c)
}

// ========== 文件管理 ==========

// ListFiles 获取文件列表
func (ctrl *CoworkerController) ListFiles(c *gin.Context) {
	ctrl.module.RESTHandler.ListFiles(c)
}

// CreateFolder 创建文件夹
func (ctrl *CoworkerController) CreateFolder(c *gin.Context) {
	ctrl.module.RESTHandler.CreateFolder(c)
}

// DeleteFile 删除文件
func (ctrl *CoworkerController) DeleteFile(c *gin.Context) {
	ctrl.module.RESTHandler.DeleteFile(c)
}

// RenameFile 重命名文件
func (ctrl *CoworkerController) RenameFile(c *gin.Context) {
	ctrl.module.RESTHandler.RenameFile(c)
}

// GetWorkspaceStats 获取工作空间使用统计
func (ctrl *CoworkerController) GetWorkspaceStats(c *gin.Context) {
	ctrl.module.RESTHandler.GetWorkspaceStats(c)
}

// ========== 配置管理 ==========

// GetConfig 获取配置
func (ctrl *CoworkerController) GetConfig(c *gin.Context) {
	ctrl.module.RESTHandler.GetConfig(c)
}

// SaveConfig 保存配置
func (ctrl *CoworkerController) SaveConfig(c *gin.Context) {
	ctrl.module.RESTHandler.SaveConfig(c)
}

// ========== 用户信息 ==========

// GetUserInfo 获取用户信息
func (ctrl *CoworkerController) GetUserInfo(c *gin.Context) {
	ctrl.module.RESTHandler.GetUserInfo(c)
}

// SaveUserInfo 保存用户信息
func (ctrl *CoworkerController) SaveUserInfo(c *gin.Context) {
	ctrl.module.RESTHandler.SaveUserInfo(c)
}

// ========== Job 管理 ==========

// ListJobs 获取 Job 列表
func (ctrl *CoworkerController) ListJobs(c *gin.Context) {
	ctrl.module.RESTHandler.ListJobs(c)
}

// CreateJob 创建 Job
func (ctrl *CoworkerController) CreateJob(c *gin.Context) {
	ctrl.module.RESTHandler.CreateJob(c)
}

// UpdateJob 更新 Job
func (ctrl *CoworkerController) UpdateJob(c *gin.Context) {
	ctrl.module.RESTHandler.UpdateJob(c)
}

// DeleteJob 删除 Job
func (ctrl *CoworkerController) DeleteJob(c *gin.Context) {
	ctrl.module.RESTHandler.DeleteJob(c)
}

// RunJob 手动触发 Job
func (ctrl *CoworkerController) RunJob(c *gin.Context) {
	ctrl.module.RESTHandler.RunJob(c)
}

// ReorderJobs 批量排序 Jobs
func (ctrl *CoworkerController) ReorderJobs(c *gin.Context) {
	ctrl.module.RESTHandler.ReorderJobs(c)
}

// ========== 记忆管理 ==========

// ListMemories 获取记忆列表
func (ctrl *CoworkerController) ListMemories(c *gin.Context) {
	ctrl.module.RESTHandler.ListMemories(c)
}

// GetMemory 获取单条记忆
func (ctrl *CoworkerController) GetMemory(c *gin.Context) {
	ctrl.module.RESTHandler.GetMemory(c)
}

// CreateMemory 创建记忆
func (ctrl *CoworkerController) CreateMemory(c *gin.Context) {
	ctrl.module.RESTHandler.CreateMemory(c)
}

// UpdateMemory 更新记忆
func (ctrl *CoworkerController) UpdateMemory(c *gin.Context) {
	ctrl.module.RESTHandler.UpdateMemory(c)
}

// DeleteMemory 删除记忆
func (ctrl *CoworkerController) DeleteMemory(c *gin.Context) {
	ctrl.module.RESTHandler.DeleteMemory(c)
}

// SearchMemories 搜索记忆
func (ctrl *CoworkerController) SearchMemories(c *gin.Context) {
	ctrl.module.RESTHandler.SearchMemories(c)
}

// ========== 定价配置 ==========

// GetRatioConfig 获取模型定价配置（内部接口，不受 ExposeRatioEnabled 开关限制）
func (ctrl *CoworkerController) GetRatioConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    ratio_setting.GetExposedData(),
	})
}

// ========== 技能商店 ==========

func (ctrl *CoworkerController) ListStoreItems(c *gin.Context) {
	ctrl.module.RESTHandler.ListStoreItems(c)
}
func (ctrl *CoworkerController) CreateStoreItem(c *gin.Context) {
	ctrl.module.RESTHandler.CreateStoreItem(c)
}
func (ctrl *CoworkerController) UpdateStoreItem(c *gin.Context) {
	ctrl.module.RESTHandler.UpdateStoreItem(c)
}
func (ctrl *CoworkerController) DeleteStoreItem(c *gin.Context) {
	ctrl.module.RESTHandler.DeleteStoreItem(c)
}
func (ctrl *CoworkerController) ImportStoreItems(c *gin.Context) {
	ctrl.module.RESTHandler.ImportStoreItems(c)
}
func (ctrl *CoworkerController) GetUserStore(c *gin.Context) {
	ctrl.module.RESTHandler.GetUserStore(c)
}
func (ctrl *CoworkerController) SaveUserStore(c *gin.Context) {
	ctrl.module.RESTHandler.SaveUserStore(c)
}
