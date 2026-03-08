package wechat

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/coworker/internal/client"
	"github.com/QuantumNous/new-api/model"
)

// perUserRateLimiter 基于简单时间窗口的 per-user 速率限制
type perUserRateLimiter struct {
	mu       sync.Mutex
	records  map[string][]time.Time
	maxCount int
	window   time.Duration
}

var wechatRateLimiter = &perUserRateLimiter{
	records:  make(map[string][]time.Time),
	maxCount: 3,               // 每个窗口最多 3 条
	window:   10 * time.Second, // 10 秒窗口
}

// allow 检查是否允许该用户发送消息
func (r *perUserRateLimiter) allow(openID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	// 清理过期记录
	records := r.records[openID]
	valid := records[:0]
	for _, t := range records {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= r.maxCount {
		r.records[openID] = valid
		return false
	}

	r.records[openID] = append(valid, now)
	return true
}

// chatWithAI 异步调用 AI 对话，获取回复后通过客服消息发送给用户
func (s *Service) chatWithAI(openID string, userID int, userMessage string) {
	// 速率限制：每 10 秒最多 3 条消息
	if !wechatRateLimiter.allow(openID) {
		_ = s.SendTextToOpenID(openID, "消息频率过高，请稍后再试。")
		return
	}

	go func() {
		// 构建用户专属的 AI 客户端（通过 Relay 走计费）
		aiClient := s.getClientForUser(userID)
		if aiClient == nil {
			log.Printf("[WeChat] Failed to create AI client for user %d", userID)
			_ = s.SendTextToOpenID(openID, "抱歉，AI 服务暂时不可用，请稍后再试。")
			return
		}

		// 调用 AI
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		// 构建对话提示（简单单轮对话，后续可扩展为多轮）
		systemPrompt := "你是 Coworker AI 助手，正在通过微信公众号与用户对话。请简洁地回答用户的问题。回复内容不要使用 Markdown 格式（微信不支持渲染），使用纯文本。"
		prompt := fmt.Sprintf("%s\n\n用户消息：%s", systemPrompt, userMessage)

		reply, err := aiClient.CreateSimpleMessage(ctx, prompt, 2048)
		if err != nil {
			log.Printf("[WeChat] AI chat failed for user %d: %v", userID, err)
			_ = s.SendTextToOpenID(openID, "抱歉，AI 回复失败，请稍后再试。")
			return
		}

		reply = strings.TrimSpace(reply)
		if reply == "" {
			reply = "（AI 未生成回复内容）"
		}

		// 通过客服消息 API 发送回复
		if err := s.SendTextToOpenID(openID, reply); err != nil {
			log.Printf("[WeChat] Failed to send AI reply to %s: %v", openID, err)
		} else {
			log.Printf("[WeChat] AI reply sent to user %d (openID=%s), len=%d", userID, openID, len(reply))
		}
	}()
}

// getClientForUser 为用户创建 AI 客户端（通过 Relay 走计费）
func (s *Service) getClientForUser(userID int) *client.ClaudeClient {
	// 获取用户 profile 中的模型选择
	profile, err := model.GetCoworkerUserProfile(userID)
	selectedModel := ""
	if err == nil && profile != nil && profile.SelectedModel != "" {
		selectedModel = profile.SelectedModel
	}
	if selectedModel == "" {
		selectedModel = s.defaultModel
	}

	// 获取用户的 Coworker token
	tokenKey, err := model.GetOrCreateCoworkerToken(userID)
	if err != nil {
		log.Printf("[WeChat] Failed to get Coworker token for user %d: %v", userID, err)
		return nil
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	relayURL := "http://127.0.0.1:" + port

	c := client.NewClaudeClient(tokenKey, "", relayURL, selectedModel, 4096)

	// 应用用户的采样参数
	if profile != nil {
		c.SetSamplingParams(profile.Temperature, profile.TopP)
	}

	return c
}

// lookupUserByWechatID 通过微信号查找用户
// 在 CoworkerUserProfile 表中查找 wechat_id 匹配的用户
func lookupUserByWechatID(wechatID string) (int, error) {
	var profile model.CoworkerUserProfile
	err := model.DB.Where("wechat_id = ?", wechatID).First(&profile).Error
	if err != nil {
		return 0, fmt.Errorf("wechat_id '%s' not found: %w", wechatID, err)
	}
	return profile.UserID, nil
}

// getUserDisplayName 获取用户显示名称
func getUserDisplayName(userID int) string {
	profile, err := model.GetCoworkerUserProfile(userID)
	if err != nil || profile == nil {
		return strconv.Itoa(userID)
	}
	if profile.CoworkerName != "" {
		return profile.CoworkerName
	}
	if profile.UserName != "" {
		return profile.UserName
	}
	return strconv.Itoa(userID)
}
