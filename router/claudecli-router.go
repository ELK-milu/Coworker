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
		coworkerGroup.GET("/files/preview", claudeCLICtrl.PreviewFile)

		// 配置管理
		coworkerGroup.GET("/config", claudeCLICtrl.GetConfig)
		coworkerGroup.PUT("/config", claudeCLICtrl.SaveConfig)

		// 用户信息
		coworkerGroup.GET("/userinfo", claudeCLICtrl.GetUserInfo)
		coworkerGroup.PUT("/userinfo", claudeCLICtrl.SaveUserInfo)

		// 记忆管理
		coworkerGroup.GET("/memories", claudeCLICtrl.ListMemories)
		coworkerGroup.GET("/memories/search", claudeCLICtrl.SearchMemories)
		coworkerGroup.POST("/memories", claudeCLICtrl.CreateMemory)
		coworkerGroup.GET("/memories/:id", claudeCLICtrl.GetMemory)
		coworkerGroup.PUT("/memories/:id", claudeCLICtrl.UpdateMemory)
		coworkerGroup.DELETE("/memories/:id", claudeCLICtrl.DeleteMemory)

		// 定价配置
		coworkerGroup.GET("/ratio_config", claudeCLICtrl.GetRatioConfig)

		// Job 管理
		coworkerGroup.GET("/jobs", claudeCLICtrl.ListJobs)
		coworkerGroup.POST("/jobs", claudeCLICtrl.CreateJob)
		coworkerGroup.PUT("/jobs/reorder", claudeCLICtrl.ReorderJobs)
		coworkerGroup.PUT("/jobs/:id", claudeCLICtrl.UpdateJob)
		coworkerGroup.DELETE("/jobs/:id", claudeCLICtrl.DeleteJob)
		coworkerGroup.POST("/jobs/:id/run", claudeCLICtrl.RunJob)

		// 技能商店
		coworkerGroup.GET("/store/items", claudeCLICtrl.ListStoreItems)
		coworkerGroup.POST("/store/items", claudeCLICtrl.CreateStoreItem)
		coworkerGroup.PUT("/store/items/:id", claudeCLICtrl.UpdateStoreItem)
		coworkerGroup.DELETE("/store/items/:id", claudeCLICtrl.DeleteStoreItem)
		coworkerGroup.POST("/store/import", claudeCLICtrl.ImportStoreItems)
		coworkerGroup.GET("/store/user", claudeCLICtrl.GetUserStore)
		coworkerGroup.PUT("/store/user", claudeCLICtrl.SaveUserStore)

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
