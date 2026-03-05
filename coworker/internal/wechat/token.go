package wechat

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// tokenManager 微信 access_token 管理器
type tokenManager struct {
	appID     string
	appSecret string

	mu          sync.RWMutex
	accessToken string
	expiresAt   time.Time
}

// tokenResponse 微信 access_token 接口响应
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"` // 秒
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

func newTokenManager(appID, appSecret string) *tokenManager {
	return &tokenManager{
		appID:     appID,
		appSecret: appSecret,
	}
}

// GetAccessToken 获取有效的 access_token（自动刷新）
func (t *tokenManager) GetAccessToken() (string, error) {
	t.mu.RLock()
	if t.accessToken != "" && time.Now().Before(t.expiresAt) {
		token := t.accessToken
		t.mu.RUnlock()
		return token, nil
	}
	t.mu.RUnlock()

	return t.refresh()
}

// refresh 强制刷新 access_token
func (t *tokenManager) refresh() (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// double-check: 可能其他 goroutine 已刷新
	if t.accessToken != "" && time.Now().Before(t.expiresAt) {
		return t.accessToken, nil
	}

	url := fmt.Sprintf(
		"https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		t.appID, t.appSecret,
	)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("request access_token failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read access_token response failed: %w", err)
	}

	var result tokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse access_token response failed: %w", err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("wechat API error: code=%d, msg=%s", result.ErrCode, result.ErrMsg)
	}

	t.accessToken = result.AccessToken
	// 提前 5 分钟过期，避免边界情况
	t.expiresAt = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)

	log.Printf("[WeChat] Access token refreshed, expires in %d seconds", result.ExpiresIn)
	return t.accessToken, nil
}

// startAutoRefresh 启动自动刷新 goroutine
func (t *tokenManager) startAutoRefresh(stopCh <-chan struct{}) {
	go func() {
		// 启动时立即刷新一次
		if _, err := t.refresh(); err != nil {
			log.Printf("[WeChat] Initial token refresh failed: %v", err)
		}

		ticker := time.NewTicker(90 * time.Minute) // 每 90 分钟刷新（token 有效期 2 小时）
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if _, err := t.refresh(); err != nil {
					log.Printf("[WeChat] Auto refresh token failed: %v", err)
				}
			case <-stopCh:
				return
			}
		}
	}()
}
