package wechat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/QuantumNous/new-api/model"
)

// SendTextToUser 通过客服消息 API 向指定用户发送文本消息
// userID: Coworker 系统用户 ID
// text: 消息内容
func (s *Service) SendTextToUser(userID int, text string) error {
	// 查找用户的 OpenID 绑定
	binding, err := model.GetWechatBindingByUserID(userID)
	if err != nil {
		return fmt.Errorf("user %d has no wechat binding: %w", userID, err)
	}

	return s.SendTextToOpenID(binding.OpenID, text)
}

// SendTextToOpenID 通过客服消息 API 向指定 OpenID 发送文本消息
func (s *Service) SendTextToOpenID(openID string, text string) error {
	token, err := s.token.GetAccessToken()
	if err != nil {
		return fmt.Errorf("get access_token failed: %w", err)
	}

	// 微信文本消息有长度限制（2048字符），需要分段发送
	messages := splitMessage(text, 2000)
	for _, msg := range messages {
		if err := s.sendCustomerServiceMessage(token, openID, msg); err != nil {
			return err
		}
	}
	return nil
}

// sendCustomerServiceMessage 调用微信客服消息 API
func (s *Service) sendCustomerServiceMessage(token, openID, text string) error {
	msg := CustomerServiceMessage{
		ToUser:  openID,
		MsgType: "text",
		Text:    &CSTextContent{Content: text},
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message failed: %w", err)
	}

	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/message/custom/send?access_token=%s", token)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("send customer service message failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response failed: %w", err)
	}

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("parse response failed: %w", err)
	}

	if result.ErrCode != 0 {
		log.Printf("[WeChat] Send message to %s failed: code=%d, msg=%s", openID, result.ErrCode, result.ErrMsg)
		return fmt.Errorf("wechat API error: code=%d, msg=%s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

// splitMessage 将长消息分割为多段
func splitMessage(text string, maxLen int) []string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return []string{text}
	}

	var parts []string
	for len(runes) > 0 {
		end := maxLen
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, string(runes[:end]))
		runes = runes[end:]
	}
	return parts
}
