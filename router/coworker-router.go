package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/gin-gonic/gin"
)

// SetCoworkerRouter 设置 Coworker 路由
func SetCoworkerRouter(router *gin.Engine) {
	// 创建 Coworker 控制器实例
	coworkerCtrl := controller.NewCoworkerController()

	// Coworker API 路由组 (新的 REST API)
	coworkerGroup := router.Group("/coworker")
	{
		// 健康检查
		coworkerGroup.GET("/health", coworkerCtrl.Health)

		// 会话管理
		coworkerGroup.GET("/sessions", coworkerCtrl.ListSessions)
		coworkerGroup.POST("/sessions", coworkerCtrl.CreateSession)
		coworkerGroup.GET("/sessions/:id", coworkerCtrl.GetSession)
		coworkerGroup.GET("/sessions/:id/history", coworkerCtrl.GetSessionHistory)
		coworkerGroup.DELETE("/sessions/:id", coworkerCtrl.DeleteSession)

		// 任务管理
		coworkerGroup.GET("/tasks", coworkerCtrl.ListTasks)
		coworkerGroup.POST("/tasks", coworkerCtrl.CreateTask)
		coworkerGroup.PUT("/tasks/reorder", coworkerCtrl.ReorderTasks)
		coworkerGroup.PUT("/tasks/:id", coworkerCtrl.UpdateTask)
		coworkerGroup.DELETE("/tasks/:id", coworkerCtrl.DeleteTask)

		// 文件管理
		coworkerGroup.GET("/files", coworkerCtrl.ListFiles)
		coworkerGroup.POST("/files/folder", coworkerCtrl.CreateFolder)
		coworkerGroup.PUT("/files/rename", coworkerCtrl.RenameFile)
		coworkerGroup.DELETE("/files", coworkerCtrl.DeleteFile)
		coworkerGroup.POST("/files/upload", coworkerCtrl.UploadFile)
		coworkerGroup.GET("/files/download", coworkerCtrl.DownloadFile)
		coworkerGroup.GET("/files/preview", coworkerCtrl.PreviewFile)
		coworkerGroup.GET("/files/stats", coworkerCtrl.GetWorkspaceStats)

		// 配置管理
		coworkerGroup.GET("/config", coworkerCtrl.GetConfig)
		coworkerGroup.PUT("/config", coworkerCtrl.SaveConfig)

		// 用户信息
		coworkerGroup.GET("/userinfo", coworkerCtrl.GetUserInfo)
		coworkerGroup.PUT("/userinfo", coworkerCtrl.SaveUserInfo)

		// 记忆管理
		coworkerGroup.GET("/memories", coworkerCtrl.ListMemories)
		coworkerGroup.GET("/memories/search", coworkerCtrl.SearchMemories)
		coworkerGroup.POST("/memories", coworkerCtrl.CreateMemory)
		coworkerGroup.GET("/memories/:id", coworkerCtrl.GetMemory)
		coworkerGroup.PUT("/memories/:id", coworkerCtrl.UpdateMemory)
		coworkerGroup.DELETE("/memories/:id", coworkerCtrl.DeleteMemory)

		// 定价配置
		coworkerGroup.GET("/ratio_config", coworkerCtrl.GetRatioConfig)

		// Job 管理
		coworkerGroup.GET("/jobs", coworkerCtrl.ListJobs)
		coworkerGroup.POST("/jobs", coworkerCtrl.CreateJob)
		coworkerGroup.PUT("/jobs/reorder", coworkerCtrl.ReorderJobs)
		coworkerGroup.PUT("/jobs/:id", coworkerCtrl.UpdateJob)
		coworkerGroup.DELETE("/jobs/:id", coworkerCtrl.DeleteJob)
		coworkerGroup.POST("/jobs/:id/run", coworkerCtrl.RunJob)

		// 技能商店
		coworkerGroup.GET("/store/items", coworkerCtrl.ListStoreItems)
		coworkerGroup.POST("/store/items", coworkerCtrl.CreateStoreItem)
		coworkerGroup.PUT("/store/items/:id", coworkerCtrl.UpdateStoreItem)
		coworkerGroup.DELETE("/store/items/:id", coworkerCtrl.DeleteStoreItem)
		coworkerGroup.POST("/store/import", coworkerCtrl.ImportStoreItems)
		coworkerGroup.POST("/store/import-modelscope", coworkerCtrl.ImportFromModelScope)
		coworkerGroup.GET("/store/user", coworkerCtrl.GetUserStore)
		coworkerGroup.PUT("/store/user", coworkerCtrl.SaveUserStore)
		coworkerGroup.POST("/store/user/install/:id", coworkerCtrl.InstallStoreItem)
		coworkerGroup.DELETE("/store/user/uninstall/:id", coworkerCtrl.UninstallStoreItem)
		// MCP 用户配置
		coworkerGroup.GET("/store/user/:id/config", coworkerCtrl.GetUserMCPConfig)
		coworkerGroup.PUT("/store/user/:id/config", coworkerCtrl.SaveUserMCPConfig)
		// 收藏
		coworkerGroup.POST("/store/user/favorite/:id", coworkerCtrl.FavoriteStoreItem)
		coworkerGroup.GET("/store/user/favorites", coworkerCtrl.GetUserFavorites)
		// AI 分类
		coworkerGroup.POST("/store/items/:id/classify", coworkerCtrl.ClassifyStoreItem)
		coworkerGroup.POST("/store/classify-all", coworkerCtrl.ClassifyAllStoreItems)
		// MCP 连接测试
		coworkerGroup.POST("/mcp/test", coworkerCtrl.TestMCPConnection)

		// WebSocket 连接
		coworkerGroup.GET("/ws", coworkerCtrl.HandleWebSocket)
	}

}
