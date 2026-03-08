package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// SetCoworkerRouter 设置 Coworker 路由
func SetCoworkerRouter(router *gin.Engine) {
	// 创建 Coworker 控制器实例
	coworkerCtrl := controller.NewCoworkerController()

	// Coworker API 路由组
	coworkerGroup := router.Group("/coworker")
	coworkerGroup.Use(middleware.GlobalAPIRateLimit())
	{
		// 公开端点（无需认证）
		coworkerGroup.GET("/health", coworkerCtrl.Health)

		// 微信公众号回调（微信服务器直接调用，无需认证）
		coworkerGroup.GET("/wechat/callback", coworkerCtrl.WeChatVerify)
		coworkerGroup.POST("/wechat/callback", coworkerCtrl.WeChatCallback)
	}

	// 需要用户认证的端点
	userGroup := coworkerGroup.Group("")
	userGroup.Use(middleware.UserAuth())
	{
		// 会话管理
		userGroup.GET("/sessions", coworkerCtrl.ListSessions)
		userGroup.POST("/sessions", coworkerCtrl.CreateSession)
		userGroup.GET("/sessions/:id", coworkerCtrl.GetSession)
		userGroup.GET("/sessions/:id/history", coworkerCtrl.GetSessionHistory)
		userGroup.DELETE("/sessions/:id", coworkerCtrl.DeleteSession)

		// 任务管理
		userGroup.GET("/tasks", coworkerCtrl.ListTasks)
		userGroup.POST("/tasks", coworkerCtrl.CreateTask)
		userGroup.PUT("/tasks/reorder", coworkerCtrl.ReorderTasks)
		userGroup.PUT("/tasks/:id", coworkerCtrl.UpdateTask)
		userGroup.DELETE("/tasks/:id", coworkerCtrl.DeleteTask)

		// 文件管理
		userGroup.GET("/files", coworkerCtrl.ListFiles)
		userGroup.POST("/files/folder", coworkerCtrl.CreateFolder)
		userGroup.PUT("/files/rename", coworkerCtrl.RenameFile)
		userGroup.DELETE("/files", coworkerCtrl.DeleteFile)
		userGroup.POST("/files/upload", coworkerCtrl.UploadFile)
		userGroup.GET("/files/download", coworkerCtrl.DownloadFile)
		userGroup.GET("/files/preview", coworkerCtrl.PreviewFile)
		userGroup.GET("/files/stats", coworkerCtrl.GetWorkspaceStats)

		// 配置管理
		userGroup.GET("/config", coworkerCtrl.GetConfig)
		userGroup.PUT("/config", coworkerCtrl.SaveConfig)

		// 用户信息
		userGroup.GET("/userinfo", coworkerCtrl.GetUserInfo)
		userGroup.PUT("/userinfo", coworkerCtrl.SaveUserInfo)

		// 记忆管理
		userGroup.GET("/memories", coworkerCtrl.ListMemories)
		userGroup.GET("/memories/search", coworkerCtrl.SearchMemories)
		userGroup.POST("/memories", coworkerCtrl.CreateMemory)
		userGroup.GET("/memories/:id", coworkerCtrl.GetMemory)
		userGroup.PUT("/memories/:id", coworkerCtrl.UpdateMemory)
		userGroup.DELETE("/memories/:id", coworkerCtrl.DeleteMemory)

		// 定价配置
		userGroup.GET("/ratio_config", coworkerCtrl.GetRatioConfig)

		// Job 管理
		userGroup.GET("/jobs", coworkerCtrl.ListJobs)
		userGroup.POST("/jobs", coworkerCtrl.CreateJob)
		userGroup.PUT("/jobs/reorder", coworkerCtrl.ReorderJobs)
		userGroup.PUT("/jobs/:id", coworkerCtrl.UpdateJob)
		userGroup.DELETE("/jobs/:id", coworkerCtrl.DeleteJob)
		userGroup.POST("/jobs/:id/run", coworkerCtrl.RunJob)

		// 技能商店（用户操作）
		userGroup.GET("/store/items", coworkerCtrl.ListStoreItems)
		userGroup.GET("/store/items/:id", coworkerCtrl.GetStoreItem)
		userGroup.GET("/store/items/:id/download", coworkerCtrl.DownloadStoreItem)
		userGroup.GET("/store/user", coworkerCtrl.GetUserStore)
		userGroup.PUT("/store/user", coworkerCtrl.SaveUserStore)
		userGroup.POST("/store/user/install/:id", coworkerCtrl.InstallStoreItem)
		userGroup.DELETE("/store/user/uninstall/:id", coworkerCtrl.UninstallStoreItem)
		userGroup.GET("/store/user/:id/config", coworkerCtrl.GetUserMCPConfig)
		userGroup.PUT("/store/user/:id/config", coworkerCtrl.SaveUserMCPConfig)
		userGroup.POST("/store/user/favorite/:id", coworkerCtrl.FavoriteStoreItem)
		userGroup.GET("/store/user/favorites", coworkerCtrl.GetUserFavorites)

		// MCP 连接测试
		userGroup.POST("/mcp/test", coworkerCtrl.TestMCPConnection)

		// 内置模型设置（读取）
		userGroup.GET("/builtin-model", coworkerCtrl.GetBuiltinModel)

		// WebSocket 连接
		userGroup.GET("/ws", coworkerCtrl.HandleWebSocket)
	}

	// 需要管理员权限的端点
	adminGroup := coworkerGroup.Group("")
	adminGroup.Use(middleware.AdminAuth())
	{
		// 技能商店管理（仅管理员）
		adminGroup.POST("/store/items", coworkerCtrl.CreateStoreItem)
		adminGroup.PUT("/store/items/:id", coworkerCtrl.UpdateStoreItem)
		adminGroup.DELETE("/store/items/:id", coworkerCtrl.DeleteStoreItem)
		adminGroup.POST("/store/import", coworkerCtrl.ImportStoreItems)
		adminGroup.POST("/store/import-modelscope", coworkerCtrl.ImportFromModelScope)

		// AI 分类（仅管理员）
		adminGroup.POST("/store/items/:id/classify", coworkerCtrl.ClassifyStoreItem)
		adminGroup.POST("/store/classify-all", coworkerCtrl.ClassifyAllStoreItems)

		// 内置模型设置（修改，仅管理员）
		adminGroup.PUT("/builtin-model", coworkerCtrl.SaveBuiltinModel)

		// 微信推送（仅管理员）
		adminGroup.POST("/wechat/notify", coworkerCtrl.WeChatNotify)
	}
}
