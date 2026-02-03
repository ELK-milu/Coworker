package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/gin-gonic/gin"
)

// SetClaudeCLIRouter 设置 ClaudeCLI 路由
func SetClaudeCLIRouter(router *gin.Engine) {
	// 创建 ClaudeCLI 控制器实例
	claudeCLICtrl := controller.NewClaudeCLIController()

	// Coworker API 路由组 (新的 REST API)
	coworkerGroup := router.Group("/coworker")
	{
		// 健康检查
		coworkerGroup.GET("/health", claudeCLICtrl.Health)

		// 会话管理
		coworkerGroup.GET("/sessions", claudeCLICtrl.ListSessions)
		coworkerGroup.POST("/sessions", claudeCLICtrl.CreateSession)
		coworkerGroup.GET("/sessions/:id", claudeCLICtrl.GetSession)
		coworkerGroup.GET("/sessions/:id/history", claudeCLICtrl.GetSessionHistory)
		coworkerGroup.DELETE("/sessions/:id", claudeCLICtrl.DeleteSession)

		// 任务管理
		coworkerGroup.GET("/tasks", claudeCLICtrl.ListTasks)
		coworkerGroup.POST("/tasks", claudeCLICtrl.CreateTask)
		coworkerGroup.PUT("/tasks/reorder", claudeCLICtrl.ReorderTasks)
		coworkerGroup.PUT("/tasks/:id", claudeCLICtrl.UpdateTask)
		coworkerGroup.DELETE("/tasks/:id", claudeCLICtrl.DeleteTask)

		// 文件管理
		coworkerGroup.GET("/files", claudeCLICtrl.ListFiles)
		coworkerGroup.POST("/files/folder", claudeCLICtrl.CreateFolder)
		coworkerGroup.PUT("/files/rename", claudeCLICtrl.RenameFile)
		coworkerGroup.DELETE("/files", claudeCLICtrl.DeleteFile)
		coworkerGroup.POST("/files/upload", claudeCLICtrl.UploadFile)
		coworkerGroup.GET("/files/download", claudeCLICtrl.DownloadFile)

		// 配置管理
		coworkerGroup.GET("/config", claudeCLICtrl.GetConfig)
		coworkerGroup.PUT("/config", claudeCLICtrl.SaveConfig)

		// WebSocket 连接
		coworkerGroup.GET("/ws", claudeCLICtrl.HandleWebSocket)
	}

	// 保留旧的 /claudecli 路由以兼容
	claudecliGroup := router.Group("/claudecli")
	{
		claudecliGroup.GET("/health", claudeCLICtrl.Health)
		claudecliGroup.GET("/ws", claudeCLICtrl.HandleWebSocket)
	}
}
