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
