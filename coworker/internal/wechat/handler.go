package wechat

import (
	"crypto/sha1"
	"crypto/subtle"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// maxCallbackBodySize 微信回调请求体最大尺寸 (1MB)
const maxCallbackBodySize = 1024 * 1024

// VerifyCallback 微信服务器验证回调（GET）
// 微信服务器配置 URL 时会发送 GET 请求验证
func (s *Service) VerifyCallback(c *gin.Context) {
	signature := c.Query("signature")
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")
	echostr := c.Query("echostr")

	if s.checkSignature(signature, timestamp, nonce) {
		c.String(http.StatusOK, echostr)
	} else {
		log.Printf("[WeChat] Signature verification failed: signature=%s", signature)
		c.String(http.StatusForbidden, "signature mismatch")
	}
}

// HandleCallback 处理微信推送的消息（POST）
func (s *Service) HandleCallback(c *gin.Context) {
	// C1: POST 回调也需要验证签名
	signature := c.Query("signature")
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")
	if !s.checkSignature(signature, timestamp, nonce) {
		log.Printf("[WeChat] POST callback signature verification failed")
		c.String(http.StatusForbidden, "invalid signature")
		return
	}

	// 时间戳时效检查（5 分钟窗口，防止重放攻击）
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || abs64(time.Now().Unix()-ts) > 300 {
		log.Printf("[WeChat] POST callback timestamp expired or invalid: %s", timestamp)
		c.String(http.StatusForbidden, "timestamp expired")
		return
	}

	// C3: 请求体大小限制
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxCallbackBodySize))
	if err != nil {
		log.Printf("[WeChat] Read request body failed: %v", err)
		c.String(http.StatusBadRequest, "")
		return
	}

	var msg InMessage
	if err := xml.Unmarshal(body, &msg); err != nil {
		log.Printf("[WeChat] Parse XML message failed: %v", err)
		c.String(http.StatusBadRequest, "")
		return
	}

	log.Printf("[WeChat] Received message: type=%s, from=%s", msg.MsgType, msg.FromUserName)

	switch msg.MsgType {
	case MsgTypeEvent:
		s.handleEvent(c, &msg)
	case MsgTypeText:
		s.handleText(c, &msg)
	case MsgTypeVoice:
		// 语音消息：如果有识别结果，当作文本处理
		if msg.Recognition != "" {
			msg.Content = msg.Recognition
			s.handleText(c, &msg)
		} else {
			s.replyText(c, &msg, "暂不支持语音消息，请发送文字。")
		}
	default:
		s.replyText(c, &msg, "暂不支持该消息类型，请发送文字消息。")
	}
}

// handleEvent 处理事件消息
func (s *Service) handleEvent(c *gin.Context, msg *InMessage) {
	switch msg.Event {
	case EventSubscribe:
		log.Printf("[WeChat] User subscribed: %s", msg.FromUserName)
		s.replyText(c, msg, "欢迎关注！\n\n发送「绑定 你的微信号」来绑定 Coworker 账号。\n绑定后可直接在此对话。\n\n发送「解绑」可解除绑定。\n发送「状态」查看绑定状态。")
	case EventUnsubscribe:
		log.Printf("[WeChat] User unsubscribed: %s", msg.FromUserName)
		// 取消关注时删除绑定
		_ = model.DeleteWechatBindingByOpenID(msg.FromUserName)
		c.String(http.StatusOK, "success")
	default:
		c.String(http.StatusOK, "success")
	}
}

// handleText 处理文本消息
func (s *Service) handleText(c *gin.Context, msg *InMessage) {
	openID := msg.FromUserName
	content := strings.TrimSpace(msg.Content)

	// 指令处理
	switch {
	case strings.HasPrefix(content, "绑定 ") || strings.HasPrefix(content, "绑定\n"):
		s.handleBind(c, msg, strings.TrimPrefix(strings.TrimPrefix(content, "绑定 "), "绑定\n"))
		return
	case content == "解绑":
		s.handleUnbind(c, msg)
		return
	case content == "状态":
		s.handleStatus(c, msg)
		return
	case content == "帮助" || content == "help":
		s.replyText(c, msg, "可用指令：\n\n「绑定 你的微信号」- 绑定 Coworker 账号\n「解绑」- 解除绑定\n「状态」- 查看绑定状态\n「帮助」- 显示此帮助\n\n绑定后直接发送消息即可与 AI 对话。")
		return
	}

	// 检查是否已绑定
	binding, err := model.GetWechatBindingByOpenID(openID)
	if err != nil {
		s.replyText(c, msg, "你还未绑定 Coworker 账号。\n请发送「绑定 你的微信号」进行绑定。\n\n（微信号需要先在 Coworker 用户设置中填写）")
		return
	}

	// 已绑定，转发给 AI（异步处理，先回复空字符串确认收到）
	// 微信要求 5 秒内回复，AI 处理较慢，使用客服消息 API 异步发送回复
	s.chatWithAI(openID, binding.UserID, content)

	// 回复空字符串表示收到（不会显示给用户）
	c.String(http.StatusOK, "success")
}

// handleBind 处理绑定指令
func (s *Service) handleBind(c *gin.Context, msg *InMessage, wechatID string) {
	openID := msg.FromUserName
	wechatID = strings.TrimSpace(wechatID)

	if wechatID == "" {
		s.replyText(c, msg, "请提供你的微信号。\n格式：「绑定 你的微信号」\n\n（微信号需要先在 Coworker 用户设置中填写）")
		return
	}

	// 检查是否已绑定
	existing, _ := model.GetWechatBindingByOpenID(openID)
	if existing != nil {
		name := getUserDisplayName(existing.UserID)
		s.replyText(c, msg, fmt.Sprintf("你已绑定用户「%s」。\n如需重新绑定，请先发送「解绑」。", name))
		return
	}

	// 通过微信号查找用户
	userID, err := lookupUserByWechatID(wechatID)
	if err != nil {
		s.replyText(c, msg, "未找到对应的 Coworker 用户。\n请确认：\n1. 微信号输入正确\n2. 已在 Coworker 设置中填写微信号")
		return
	}

	// 检查该用户是否已被其他 OpenID 绑定
	existingBinding, _ := model.GetWechatBindingByUserID(userID)
	if existingBinding != nil {
		s.replyText(c, msg, "该 Coworker 账号已被其他微信绑定。\n请先在另一个微信上发送「解绑」。")
		return
	}

	// 创建绑定
	binding := &model.CoworkerWechatBinding{
		OpenID: openID,
		UserID: userID,
	}
	if err := model.CreateWechatBinding(binding); err != nil {
		log.Printf("[WeChat] Create binding failed: %v", err)
		s.replyText(c, msg, "绑定失败，请稍后再试。")
		return
	}

	name := getUserDisplayName(userID)
	log.Printf("[WeChat] Binding created: openID=%s → userID=%d (%s)", openID, userID, name)
	s.replyText(c, msg, fmt.Sprintf("绑定成功！你已关联到用户「%s」。\n\n现在可以直接发送消息与 AI 对话了。", name))
}

// handleUnbind 处理解绑指令
func (s *Service) handleUnbind(c *gin.Context, msg *InMessage) {
	openID := msg.FromUserName

	existing, _ := model.GetWechatBindingByOpenID(openID)
	if existing == nil {
		s.replyText(c, msg, "你当前没有绑定任何 Coworker 账号。")
		return
	}

	name := getUserDisplayName(existing.UserID)
	if err := model.DeleteWechatBindingByOpenID(openID); err != nil {
		log.Printf("[WeChat] Delete binding failed: %v", err)
		s.replyText(c, msg, "解绑失败，请稍后再试。")
		return
	}

	log.Printf("[WeChat] Binding deleted: openID=%s (was user %d)", openID, existing.UserID)
	s.replyText(c, msg, fmt.Sprintf("已解除与用户「%s」的绑定。\n发送「绑定 微信号」可重新绑定。", name))
}

// handleStatus 处理状态查询指令
func (s *Service) handleStatus(c *gin.Context, msg *InMessage) {
	openID := msg.FromUserName

	binding, err := model.GetWechatBindingByOpenID(openID)
	if err != nil {
		s.replyText(c, msg, "当前状态：未绑定\n\n发送「绑定 你的微信号」进行绑定。")
		return
	}

	name := getUserDisplayName(binding.UserID)
	s.replyText(c, msg, fmt.Sprintf("当前状态：已绑定\n关联用户：%s\n\n发送消息即可与 AI 对话。", name))
}

// replyText 被动回复文本消息
func (s *Service) replyText(c *gin.Context, msg *InMessage, content string) {
	reply := ReplyTextMessage{
		ToUserName:   msg.FromUserName,
		FromUserName: msg.ToUserName,
		CreateTime:   time.Now().Unix(),
		MsgType:      "text",
		Content:      content,
	}

	data, err := xml.Marshal(reply)
	if err != nil {
		log.Printf("[WeChat] Marshal reply failed: %v", err)
		c.String(http.StatusOK, "success")
		return
	}

	c.Data(http.StatusOK, "application/xml", data)
}

// checkSignature 验证微信签名（constant-time 比较防止时序侧信道）
func (s *Service) checkSignature(signature, timestamp, nonce string) bool {
	arr := []string{s.verifyToken, timestamp, nonce}
	sort.Strings(arr)
	str := strings.Join(arr, "")

	h := sha1.New()
	h.Write([]byte(str))
	computed := fmt.Sprintf("%x", h.Sum(nil))

	return subtle.ConstantTimeCompare([]byte(computed), []byte(signature)) == 1
}

// abs64 返回 int64 绝对值
func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
