package wechat

import (
	"log"
	"os"
)

// Service 微信公众号服务（全局单例，所有用户共用同一个公众号）
type Service struct {
	appID        string
	appSecret    string
	verifyToken  string
	defaultModel string
	token        *tokenManager
	stopCh       chan struct{}
}

// NewService 创建微信公众号服务
// 从 .env 读取配置：WECHAT_APP_ID, WECHAT_APP_SECRET, WECHAT_TOKEN
func NewService() *Service {
	appID := os.Getenv("WECHAT_APP_ID")
	appSecret := os.Getenv("WECHAT_APP_SECRET")
	verifyToken := os.Getenv("WECHAT_TOKEN")

	if appID == "" || appSecret == "" || verifyToken == "" {
		log.Println("[WeChat] Service disabled: missing WECHAT_APP_ID, WECHAT_APP_SECRET, or WECHAT_TOKEN in .env")
		return nil
	}

	defaultModel := os.Getenv("WECHAT_DEFAULT_MODEL")
	if defaultModel == "" {
		defaultModel = "gpt-4o-mini"
	}

	s := &Service{
		appID:        appID,
		appSecret:    appSecret,
		verifyToken:  verifyToken,
		defaultModel: defaultModel,
		token:        newTokenManager(appID, appSecret),
		stopCh:       make(chan struct{}),
	}

	// 启动 access_token 自动刷新
	s.token.startAutoRefresh(s.stopCh)

	log.Printf("[WeChat] Service initialized (appID=%s, model=%s)", appID, defaultModel)
	return s
}

// Stop 停止服务
func (s *Service) Stop() {
	close(s.stopCh)
	log.Println("[WeChat] Service stopped")
}

// IsEnabled 检查服务是否可用
func (s *Service) IsEnabled() bool {
	return s != nil
}
