package api

import (
	"github.com/QuantumNous/new-api/claudecli/internal/client"
	"github.com/QuantumNous/new-api/claudecli/internal/loop"
	"github.com/QuantumNous/new-api/claudecli/internal/session"
	"github.com/QuantumNous/new-api/claudecli/internal/tools"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WSHandler WebSocket 处理器
type WSHandler struct {
	client   *client.ClaudeClient
	sessions *session.Manager
	tools    *tools.Registry
	system   string
	mu       sync.Mutex
}

// NewWSHandler 创建 WebSocket 处理器
func NewWSHandler(
	c *client.ClaudeClient,
	sm *session.Manager,
	tr *tools.Registry,
	systemPrompt string,
) *WSHandler {
	return &WSHandler{
		client:   c,
		sessions: sm,
		tools:    tr,
		system:   systemPrompt,
	}
}

// WSMessage WebSocket 消息
type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// ChatPayload 聊天消息载荷
type ChatPayload struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
}

// Handle 处理 WebSocket 连接
func (h *WSHandler) Handle(c *gin.Context) {
	log.Printf("[WS] Attempting to upgrade connection from %s", c.Request.RemoteAddr)
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("[WS] Connection established successfully")
	h.handleConnection(conn)
}

// handleConnection 处理连接
func (h *WSHandler) handleConnection(conn *websocket.Conn) {
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[WS] Read error: %v", err)
			break
		}

		log.Printf("[WS] Received: %s", string(msg))

		var wsMsg WSMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			h.sendError(conn, "invalid message format")
			continue
		}

		if wsMsg.Type == "chat" {
			log.Printf("[WS] Processing chat message")
			h.handleChat(conn, wsMsg.Payload)
		}
	}
}

// sendError 发送错误消息
func (h *WSHandler) sendError(conn *websocket.Conn, msg string) {
	conn.WriteJSON(map[string]interface{}{
		"type":    "error",
		"payload": map[string]string{"error": msg},
	})
}

// handleChat 处理聊天消息
func (h *WSHandler) handleChat(conn *websocket.Conn, payload json.RawMessage) {
	var chat ChatPayload
	if err := json.Unmarshal(payload, &chat); err != nil {
		h.sendError(conn, "invalid chat payload")
		return
	}

	// 获取或创建会话
	sess := h.sessions.Get(chat.SessionID)
	if sess == nil {
		sess = h.sessions.Create(chat.UserID)
	}

	// 创建事件通道
	eventCh := make(chan loop.LoopEvent, 100)

	// 启动对话循环
	go h.runConversation(sess, chat.Message, eventCh)

	// 转发事件到 WebSocket
	h.forwardEvents(conn, sess.ID, eventCh)
}

// runConversation 运行对话
func (h *WSHandler) runConversation(sess *session.Session, msg string, eventCh chan loop.LoopEvent) {
	defer close(eventCh)
	ctx := context.Background()
	l := loop.NewConversationLoop(h.client, sess, h.tools, h.system, eventCh)
	l.ProcessMessage(ctx, msg)
}

// forwardEvents 转发事件到 WebSocket
func (h *WSHandler) forwardEvents(conn *websocket.Conn, sessID string, eventCh <-chan loop.LoopEvent) {
	for event := range eventCh {
		payload := map[string]interface{}{
			"session_id": sessID,
		}

		switch event.Type {
		case loop.EventTypeText:
			payload["content"] = event.Text
		case loop.EventTypeToolStart:
			payload["name"] = event.ToolName
			payload["tool_id"] = event.ToolID
		case loop.EventTypeToolEnd:
			payload["name"] = event.ToolName
			payload["result"] = event.ToolResult
		case loop.EventTypeError:
			payload["error"] = event.Error
		}

		conn.WriteJSON(map[string]interface{}{
			"type":    event.Type,
			"payload": payload,
		})
	}
}
