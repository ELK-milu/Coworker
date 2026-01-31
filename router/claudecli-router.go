package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/gin-gonic/gin"
)

// SetClaudeCLIRouter 设置 ClaudeCLI 路由
func SetClaudeCLIRouter(router *gin.Engine) {
	// 创建 ClaudeCLI 控制器实例
	claudeCLICtrl := controller.NewClaudeCLIController()

	// ClaudeCLI API 路由组
	claudecliGroup := router.Group("/claudecli")
	{
		// 健康检查
		claudecliGroup.GET("/health", claudeCLICtrl.Health)

		// 会话管理
		claudecliGroup.POST("/sessions", claudeCLICtrl.CreateSession)
		claudecliGroup.GET("/sessions/:id", claudeCLICtrl.GetSession)
		claudecliGroup.DELETE("/sessions/:id", claudeCLICtrl.DeleteSession)

		// WebSocket 连接
		claudecliGroup.GET("/ws", claudeCLICtrl.HandleWebSocket)
	}
}
