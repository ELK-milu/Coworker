package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/QuantumNous/new-api/claudecli/internal/client"
	"github.com/QuantumNous/new-api/claudecli/internal/loop"
	"github.com/QuantumNous/new-api/claudecli/internal/session"
	"github.com/QuantumNous/new-api/claudecli/internal/task"
	"github.com/QuantumNous/new-api/claudecli/internal/tools"
	"github.com/QuantumNous/new-api/claudecli/internal/workspace"
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WSHandler WebSocket 处理器
type WSHandler struct {
	client     *client.ClaudeClient
	sessions   *session.Manager
	tools      *tools.Registry
	workspace  *workspace.Manager
	tasks      *task.Manager
	system     string
	mu         sync.Mutex
	cancelFunc context.CancelFunc
	connMu     sync.Mutex // 保护 WebSocket 连接的并发写入
}

// NewWSHandler 创建 WebSocket 处理器
func NewWSHandler(
	c *client.ClaudeClient,
	sm *session.Manager,
	tr *tools.Registry,
	wm *workspace.Manager,
	tm *task.Manager,
	systemPrompt string,
) *WSHandler {
	return &WSHandler{
		client:    c,
		sessions:  sm,
		tools:     tr,
		workspace: wm,
		tasks:     tm,
		system:    systemPrompt,
	}
}

// WSMessage WebSocket 消息
type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// ChatPayload 聊天消息载荷
type ChatPayload struct {
	Message     string `json:"message"`
	SessionID   string `json:"session_id"`
	UserID      string `json:"user_id"`
	WorkingPath string `json:"working_path"` // 前端当前选择的工作路径（相对于 workspace）
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

		switch wsMsg.Type {
		case "chat":
			log.Printf("[WS] Processing chat message")
			h.handleChat(conn, wsMsg.Payload)
		case "abort":
			log.Printf("[WS] Processing abort message")
			h.handleAbort()
		case "load_history":
			log.Printf("[WS] Processing load_history message")
			h.handleLoadHistory(conn, wsMsg.Payload)
		case "list_sessions":
			log.Printf("[WS] Processing list_sessions message")
			h.handleListSessions(conn, wsMsg.Payload)
		case "delete_session":
			log.Printf("[WS] Processing delete_session message")
			h.handleDeleteSession(conn, wsMsg.Payload)
		case "list_files":
			log.Printf("[WS] Processing list_files message")
			h.handleListFiles(conn, wsMsg.Payload)
		case "workspace_stats":
			log.Printf("[WS] Processing workspace_stats message")
			h.handleWorkspaceStats(conn, wsMsg.Payload)
		case "create_folder":
			log.Printf("[WS] Processing create_folder message")
			h.handleCreateFolder(conn, wsMsg.Payload)
		case "delete_file":
			log.Printf("[WS] Processing delete_file message")
			h.handleDeleteFile(conn, wsMsg.Payload)
		case "rename_file":
			log.Printf("[WS] Processing rename_file message")
			h.handleRenameFile(conn, wsMsg.Payload)
		// Task 相关消息
		case "task_create":
			log.Printf("[WS] Processing task_create message")
			h.handleTaskCreate(conn, wsMsg.Payload)
		case "task_get":
			log.Printf("[WS] Processing task_get message")
			h.handleTaskGet(conn, wsMsg.Payload)
		case "task_update":
			log.Printf("[WS] Processing task_update message")
			h.handleTaskUpdate(conn, wsMsg.Payload)
		case "task_list":
			log.Printf("[WS] Processing task_list message")
			h.handleTaskList(conn, wsMsg.Payload)
		// Compact 相关消息
		case "compact":
			log.Printf("[WS] Processing compact message")
			h.handleCompact(conn, wsMsg.Payload)
		case "context_stats":
			log.Printf("[WS] Processing context_stats message")
			h.handleContextStats(conn, wsMsg.Payload)
		}
	}
}

// sendJSON 线程安全地发送 JSON 消息
func (h *WSHandler) sendJSON(conn *websocket.Conn, v interface{}) error {
	h.connMu.Lock()
	defer h.connMu.Unlock()
	return conn.WriteJSON(v)
}

// sendError 发送错误消息
func (h *WSHandler) sendError(conn *websocket.Conn, msg string) {
	h.sendJSON(conn, map[string]interface{}{
		"type":    "error",
		"payload": map[string]string{"error": msg},
	})
}

// handleAbort 处理中断请求
func (h *WSHandler) handleAbort() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancelFunc != nil {
		h.cancelFunc()
		h.cancelFunc = nil
		log.Printf("[WS] Conversation aborted")
	}
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

	// 如果前端指定了工作路径，更新会话的工作目录
	if chat.WorkingPath != "" {
		baseWorkDir := h.workspace.GetUserWorkDir(chat.UserID)
		newWorkDir := filepath.Join(baseWorkDir, chat.WorkingPath)
		sess.SetWorkingDir(newWorkDir)
		log.Printf("[WS] Updated working dir for session %s: %s", sess.ID, newWorkDir)
	}

	// 创建可取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	h.mu.Lock()
	h.cancelFunc = cancel
	h.mu.Unlock()

	// 创建事件通道
	eventCh := make(chan loop.LoopEvent, 100)

	// 启动对话循环
	go h.runConversation(ctx, sess, chat.Message, eventCh)

	// 异步转发事件到 WebSocket（不阻塞消息读取循环）
	go h.forwardEvents(conn, sess.ID, eventCh)
}

// runConversation 运行对话
func (h *WSHandler) runConversation(ctx context.Context, sess *session.Session, msg string, eventCh chan loop.LoopEvent) {
	defer close(eventCh)
	l := loop.NewConversationLoop(h.client, sess, h.tools, h.system, eventCh)
	l.ProcessMessage(ctx, msg)

	// 对话结束后保存会话
	if err := h.sessions.Save(sess.ID); err != nil {
		log.Printf("[WS] Failed to save session %s: %v", sess.ID, err)
	} else {
		log.Printf("[WS] Session saved: %s", sess.ID)
	}
}

// LoadHistoryPayload 加载历史消息载荷
type LoadHistoryPayload struct {
	SessionID string `json:"session_id"`
}

// handleLoadHistory 处理加载历史消息请求
func (h *WSHandler) handleLoadHistory(conn *websocket.Conn, payload json.RawMessage) {
	var req LoadHistoryPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid load_history payload")
		return
	}

	if req.SessionID == "" {
		// 没有 session_id，返回空历史
		h.sendJSON(conn, map[string]interface{}{
			"type": "history",
			"payload": map[string]interface{}{
				"session_id": "",
				"messages":   []interface{}{},
			},
		})
		return
	}

	// 获取会话
	sess := h.sessions.Get(req.SessionID)
	if sess == nil {
		// 会话不存在，返回空历史
		h.sendJSON(conn, map[string]interface{}{
			"type": "history",
			"payload": map[string]interface{}{
				"session_id": req.SessionID,
				"messages":   []interface{}{},
				"not_found":  true,
			},
		})
		return
	}

	// 获取会话消息并转换为前端格式
	messages := sess.GetMessages()
	frontendMessages := convertMessagesToFrontend(messages)

	log.Printf("[WS] Loaded history for session %s: %d messages", req.SessionID, len(frontendMessages))

	h.sendJSON(conn, map[string]interface{}{
		"type": "history",
		"payload": map[string]interface{}{
			"session_id": req.SessionID,
			"messages":   frontendMessages,
		},
	})
}

// ListSessionsPayload 获取会话列表载荷
type ListSessionsPayload struct {
	UserID string `json:"user_id"`
}

// handleListSessions 处理获取会话列表请求
func (h *WSHandler) handleListSessions(conn *websocket.Conn, payload json.RawMessage) {
	var req ListSessionsPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		// 兼容旧版本，不传 user_id
		req.UserID = ""
	}

	sessions := h.sessions.List(req.UserID)

	// 转换为前端格式
	var sessionList []map[string]interface{}
	for _, sess := range sessions {
		// 获取第一条用户消息作为标题
		title := "新对话"
		messages := sess.GetMessages()
		for _, msg := range messages {
			if msg.Role == "user" {
				for _, block := range msg.Content {
					if textBlock, ok := block.(types.TextBlock); ok {
						title = textBlock.Text
						if len(title) > 50 {
							title = title[:50] + "..."
						}
						break
					}
					if blockMap, ok := block.(map[string]interface{}); ok {
						if blockMap["type"] == "text" {
							if text, ok := blockMap["text"].(string); ok {
								title = text
								if len(title) > 50 {
									title = title[:50] + "..."
								}
								break
							}
						}
					}
				}
				break
			}
		}

		sessionList = append(sessionList, map[string]interface{}{
			"id":           sess.ID,
			"title":        title,
			"created_at":   sess.CreatedAt.Unix(),
			"updated_at":   sess.UpdatedAt.Unix(),
			"message_count": len(messages),
		})
	}

	log.Printf("[WS] Listed %d sessions", len(sessionList))

	h.sendJSON(conn, map[string]interface{}{
		"type": "sessions_list",
		"payload": map[string]interface{}{
			"sessions": sessionList,
		},
	})
}

// DeleteSessionPayload 删除会话载荷
type DeleteSessionPayload struct {
	SessionID string `json:"session_id"`
}

// handleDeleteSession 处理删除会话请求
func (h *WSHandler) handleDeleteSession(conn *websocket.Conn, payload json.RawMessage) {
	var req DeleteSessionPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid delete_session payload")
		return
	}

	if req.SessionID == "" {
		h.sendError(conn, "session_id is required")
		return
	}

	h.sessions.Delete(req.SessionID)
	log.Printf("[WS] Deleted session: %s", req.SessionID)

	h.sendJSON(conn, map[string]interface{}{
		"type": "session_deleted",
		"payload": map[string]interface{}{
			"session_id": req.SessionID,
			"success":    true,
		},
	})
}

// convertMessagesToFrontend 将后端消息格式转换为前端格式
func convertMessagesToFrontend(messages []types.Message) []map[string]interface{} {
	var result []map[string]interface{}

	for _, msg := range messages {
		for _, block := range msg.Content {
			blockMap, ok := block.(map[string]interface{})
			if !ok {
				// 尝试处理结构体类型
				if textBlock, ok := block.(types.TextBlock); ok {
					if msg.Role == "user" {
						result = append(result, map[string]interface{}{
							"type":    "user",
							"content": textBlock.Text,
						})
					} else if msg.Role == "assistant" {
						result = append(result, map[string]interface{}{
							"type":    "assistant",
							"content": textBlock.Text,
						})
					}
				} else if toolUse, ok := block.(types.ToolUseBlock); ok {
					inputStr := ""
					if inputBytes, err := json.Marshal(toolUse.Input); err == nil {
						inputStr = string(inputBytes)
					}
					result = append(result, map[string]interface{}{
						"type":     "tool",
						"toolName": toolUse.Name,
						"toolId":   toolUse.ID,
						"input":    inputStr,
						"status":   "completed",
					})
				} else if toolResult, ok := block.(types.ToolResultBlock); ok {
					// 查找对应的工具调用并更新结果
					for i := len(result) - 1; i >= 0; i-- {
						if result[i]["toolId"] == toolResult.ToolUseID {
							result[i]["result"] = toolResult.Content
							result[i]["isError"] = toolResult.IsError
							break
						}
					}
				}
				continue
			}

			blockType, _ := blockMap["type"].(string)

			switch blockType {
			case "text":
				text, _ := blockMap["text"].(string)
				if msg.Role == "user" {
					result = append(result, map[string]interface{}{
						"type":    "user",
						"content": text,
					})
				} else if msg.Role == "assistant" {
					result = append(result, map[string]interface{}{
						"type":    "assistant",
						"content": text,
					})
				}

			case "tool_use":
				name, _ := blockMap["name"].(string)
				id, _ := blockMap["id"].(string)
				input, _ := blockMap["input"]
				inputStr := ""
				if inputBytes, err := json.Marshal(input); err == nil {
					inputStr = string(inputBytes)
				}
				result = append(result, map[string]interface{}{
					"type":     "tool",
					"toolName": name,
					"toolId":   id,
					"input":    inputStr,
					"status":   "completed",
				})

			case "tool_result":
				toolUseID, _ := blockMap["tool_use_id"].(string)
				content, _ := blockMap["content"].(string)
				isError, _ := blockMap["is_error"].(bool)
				// 查找对应的工具调用并更新结果
				for i := len(result) - 1; i >= 0; i-- {
					if result[i]["toolId"] == toolUseID {
						result[i]["result"] = content
						result[i]["isError"] = isError
						break
					}
				}
			}
		}
	}

	return result
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
		case loop.EventTypeThinking:
			payload["content"] = event.Text
		case loop.EventTypeToolStart:
			payload["name"] = event.ToolName
			payload["tool_id"] = event.ToolID
			payload["input"] = event.ToolInput
		case loop.EventTypeToolEnd:
			payload["name"] = event.ToolName
			payload["tool_id"] = event.ToolID
			payload["result"] = event.ToolResult
			payload["is_error"] = event.IsError
		case loop.EventTypeError:
			payload["error"] = event.Error
		case loop.EventTypeStatus:
			if event.Status != nil {
				payload["model"] = event.Status.Model
				payload["input_tokens"] = event.Status.InputTokens
				payload["output_tokens"] = event.Status.OutputTokens
				payload["total_tokens"] = event.Status.TotalTokens
				payload["context_used"] = event.Status.ContextUsed
				payload["context_max"] = event.Status.ContextMax
				payload["context_percent"] = event.Status.ContextPercent
				payload["elapsed_ms"] = event.Status.ElapsedMs
				payload["mode"] = event.Status.Mode
			}
		}

		h.sendJSON(conn, map[string]interface{}{
			"type":    event.Type,
			"payload": payload,
		})
	}
}

// ListFilesPayload 文件列表载荷
type ListFilesPayload struct {
	UserID string `json:"user_id"`
	Path   string `json:"path"`
}

// handleListFiles 处理文件列表请求
func (h *WSHandler) handleListFiles(conn *websocket.Conn, payload json.RawMessage) {
	var req ListFilesPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid list_files payload")
		return
	}

	if req.UserID == "" {
		h.sendError(conn, "user_id is required")
		return
	}

	if h.workspace == nil {
		h.sendError(conn, "workspace manager not initialized")
		return
	}

	// 异步执行文件 I/O 操作
	go func() {
		// 确保用户工作空间存在
		if err := h.workspace.EnsureUserWorkspace(req.UserID); err != nil {
			h.sendError(conn, "failed to create workspace: "+err.Error())
			return
		}

		files, err := h.workspace.ListFiles(req.UserID, req.Path)
		if err != nil {
			h.sendError(conn, "failed to list files: "+err.Error())
			return
		}

		log.Printf("[WS] Listed %d files for user %s path %s", len(files), req.UserID, req.Path)

		h.sendJSON(conn, map[string]interface{}{
			"type": "files_list",
			"payload": map[string]interface{}{
				"user_id": req.UserID,
				"path":    req.Path,
				"files":   files,
			},
		})
	}()
}

// WorkspaceStatsPayload 工作空间统计载荷
type WorkspaceStatsPayload struct {
	UserID string `json:"user_id"`
}

// handleWorkspaceStats 处理工作空间统计请求
func (h *WSHandler) handleWorkspaceStats(conn *websocket.Conn, payload json.RawMessage) {
	var req WorkspaceStatsPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid workspace_stats payload")
		return
	}

	if req.UserID == "" {
		h.sendError(conn, "user_id is required")
		return
	}

	if h.workspace == nil {
		h.sendError(conn, "workspace manager not initialized")
		return
	}

	stats, err := h.workspace.GetWorkspaceStats(req.UserID)
	if err != nil {
		h.sendError(conn, "failed to get workspace stats: "+err.Error())
		return
	}

	log.Printf("[WS] Got workspace stats for user %s", req.UserID)

	h.sendJSON(conn, map[string]interface{}{
		"type": "workspace_stats",
		"payload": map[string]interface{}{
			"user_id": req.UserID,
			"stats":   stats,
		},
	})
}

// CreateFolderPayload 创建文件夹载荷
type CreateFolderPayload struct {
	UserID string `json:"user_id"`
	Path   string `json:"path"`
}

// handleCreateFolder 处理创建文件夹请求
func (h *WSHandler) handleCreateFolder(conn *websocket.Conn, payload json.RawMessage) {
	var req CreateFolderPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid create_folder payload")
		return
	}

	if req.UserID == "" || req.Path == "" {
		h.sendError(conn, "user_id and path are required")
		return
	}

	if h.workspace == nil {
		h.sendError(conn, "workspace manager not initialized")
		return
	}

	// 异步执行文件 I/O 操作
	go func() {
		if err := h.workspace.CreateFolder(req.UserID, req.Path); err != nil {
			h.sendError(conn, "failed to create folder: "+err.Error())
			return
		}

		log.Printf("[WS] Created folder for user %s: %s", req.UserID, req.Path)

		h.sendJSON(conn, map[string]interface{}{
			"type": "folder_created",
			"payload": map[string]interface{}{
				"user_id": req.UserID,
				"path":    req.Path,
				"success": true,
			},
		})
	}()
}

// DeleteFilePayload 删除文件载荷
type DeleteFilePayload struct {
	UserID string `json:"user_id"`
	Path   string `json:"path"`
}

// handleDeleteFile 处理删除文件请求
func (h *WSHandler) handleDeleteFile(conn *websocket.Conn, payload json.RawMessage) {
	var req DeleteFilePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid delete_file payload")
		return
	}

	if req.UserID == "" || req.Path == "" {
		h.sendError(conn, "user_id and path are required")
		return
	}

	if h.workspace == nil {
		h.sendError(conn, "workspace manager not initialized")
		return
	}

	// 异步执行文件 I/O 操作
	go func() {
		if err := h.workspace.DeleteFile(req.UserID, req.Path); err != nil {
			h.sendError(conn, "failed to delete file: "+err.Error())
			return
		}

		log.Printf("[WS] Deleted file for user %s: %s", req.UserID, req.Path)

		h.sendJSON(conn, map[string]interface{}{
			"type": "file_deleted",
			"payload": map[string]interface{}{
				"user_id": req.UserID,
				"path":    req.Path,
				"success": true,
			},
		})
	}()
}

// RenameFilePayload 重命名文件载荷
type RenameFilePayload struct {
	UserID  string `json:"user_id"`
	Path    string `json:"path"`
	NewName string `json:"new_name"`
}

// handleRenameFile 处理重命名文件请求
func (h *WSHandler) handleRenameFile(conn *websocket.Conn, payload json.RawMessage) {
	var req RenameFilePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid rename_file payload")
		return
	}

	if req.UserID == "" || req.Path == "" || req.NewName == "" {
		h.sendError(conn, "user_id, path and new_name are required")
		return
	}

	if h.workspace == nil {
		h.sendError(conn, "workspace manager not initialized")
		return
	}

	// 异步执行文件 I/O 操作
	go func() {
		if err := h.workspace.RenameFile(req.UserID, req.Path, req.NewName); err != nil {
			h.sendError(conn, "failed to rename file: "+err.Error())
			return
		}

		log.Printf("[WS] Renamed file for user %s: %s -> %s", req.UserID, req.Path, req.NewName)

		h.sendJSON(conn, map[string]interface{}{
			"type": "file_renamed",
			"payload": map[string]interface{}{
				"user_id":  req.UserID,
				"old_path": req.Path,
				"new_name": req.NewName,
				"success":  true,
			},
		})
	}()
}

// ========== Task 相关处理 ==========

// TaskCreatePayload 创建任务载荷
type TaskCreatePayload struct {
	UserID      string `json:"user_id"`
	ListID      string `json:"list_id"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
	ActiveForm  string `json:"activeForm"`
}

// handleTaskCreate 处理创建任务请求
func (h *WSHandler) handleTaskCreate(conn *websocket.Conn, payload json.RawMessage) {
	var req TaskCreatePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid task_create payload")
		return
	}

	if req.UserID == "" || req.Subject == "" {
		h.sendError(conn, "user_id and subject are required")
		return
	}

	if req.ListID == "" {
		req.ListID = "default"
	}

	if h.tasks == nil {
		h.sendError(conn, "task manager not initialized")
		return
	}

	// 异步执行文件 I/O 操作
	go func() {
		t, err := h.tasks.Create(req.UserID, req.ListID, req.Subject, req.Description, req.ActiveForm)
		if err != nil {
			h.sendError(conn, "failed to create task: "+err.Error())
			return
		}

		log.Printf("[WS] Created task for user %s: %s", req.UserID, t.ID)

		h.sendJSON(conn, map[string]interface{}{
			"type": "task_created",
			"payload": map[string]interface{}{
				"user_id": req.UserID,
				"list_id": req.ListID,
				"task":    t,
				"success": true,
			},
		})
	}()
}

// TaskGetPayload 获取任务载荷
type TaskGetPayload struct {
	UserID string `json:"user_id"`
	ListID string `json:"list_id"`
	TaskID string `json:"task_id"`
}

// handleTaskGet 处理获取任务请求
func (h *WSHandler) handleTaskGet(conn *websocket.Conn, payload json.RawMessage) {
	var req TaskGetPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid task_get payload")
		return
	}

	if req.UserID == "" || req.TaskID == "" {
		h.sendError(conn, "user_id and task_id are required")
		return
	}

	if req.ListID == "" {
		req.ListID = "default"
	}

	if h.tasks == nil {
		h.sendError(conn, "task manager not initialized")
		return
	}

	t := h.tasks.Get(req.UserID, req.ListID, req.TaskID)
	if t == nil {
		h.sendError(conn, "task not found")
		return
	}

	h.sendJSON(conn, map[string]interface{}{
		"type": "task_detail",
		"payload": map[string]interface{}{
			"user_id": req.UserID,
			"list_id": req.ListID,
			"task":    t,
		},
	})
}

// TaskUpdatePayload 更新任务载荷
type TaskUpdatePayload struct {
	UserID      string                 `json:"user_id"`
	ListID      string                 `json:"list_id"`
	TaskID      string                 `json:"task_id"`
	Subject     string                 `json:"subject,omitempty"`
	Description string                 `json:"description,omitempty"`
	ActiveForm  string                 `json:"activeForm,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Owner       string                 `json:"owner,omitempty"`
	AddBlocks   []string               `json:"addBlocks,omitempty"`
	AddBlockedBy []string              `json:"addBlockedBy,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// handleTaskUpdate 处理更新任务请求
func (h *WSHandler) handleTaskUpdate(conn *websocket.Conn, payload json.RawMessage) {
	var req TaskUpdatePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid task_update payload")
		return
	}

	if req.UserID == "" || req.TaskID == "" {
		h.sendError(conn, "user_id and task_id are required")
		return
	}

	if req.ListID == "" {
		req.ListID = "default"
	}

	if h.tasks == nil {
		h.sendError(conn, "task manager not initialized")
		return
	}

	updates := make(map[string]interface{})
	if req.Subject != "" {
		updates["subject"] = req.Subject
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.ActiveForm != "" {
		updates["activeForm"] = req.ActiveForm
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Owner != "" {
		updates["owner"] = req.Owner
	}
	if len(req.AddBlocks) > 0 {
		updates["addBlocks"] = req.AddBlocks
	}
	if len(req.AddBlockedBy) > 0 {
		updates["addBlockedBy"] = req.AddBlockedBy
	}

	// 异步执行文件 I/O 操作
	go func() {
		t, err := h.tasks.Update(req.UserID, req.ListID, req.TaskID, updates)
		if err != nil {
			h.sendError(conn, "failed to update task: "+err.Error())
			return
		}

		log.Printf("[WS] Updated task for user %s: %s", req.UserID, req.TaskID)

		h.sendJSON(conn, map[string]interface{}{
			"type": "task_updated",
			"payload": map[string]interface{}{
				"user_id": req.UserID,
				"list_id": req.ListID,
				"task":    t,
				"success": true,
			},
		})
	}()
}

// TaskListPayload 任务列表载荷
type TaskListPayload struct {
	UserID string `json:"user_id"`
	ListID string `json:"list_id"`
}

// handleTaskList 处理任务列表请求
func (h *WSHandler) handleTaskList(conn *websocket.Conn, payload json.RawMessage) {
	var req TaskListPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid task_list payload")
		return
	}

	if req.UserID == "" {
		h.sendError(conn, "user_id is required")
		return
	}

	if req.ListID == "" {
		req.ListID = "default"
	}

	if h.tasks == nil {
		h.sendError(conn, "task manager not initialized")
		return
	}

	// 异步执行目录遍历操作
	go func() {
		tasks := h.tasks.List(req.UserID, req.ListID)
		log.Printf("[WS] Listed %d tasks for user %s", len(tasks), req.UserID)

		h.sendJSON(conn, map[string]interface{}{
			"type": "tasks_list",
			"payload": map[string]interface{}{
				"user_id": req.UserID,
				"list_id": req.ListID,
				"tasks":   tasks,
			},
		})
	}()
}

// ========== Compact 相关处理 ==========

// CompactPayload 压缩上下文载荷
type CompactPayload struct {
	SessionID string `json:"session_id"`
}

// handleCompact 处理压缩上下文请求
func (h *WSHandler) handleCompact(conn *websocket.Conn, payload json.RawMessage) {
	var req CompactPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid compact payload")
		return
	}

	if req.SessionID == "" {
		h.sendError(conn, "session_id is required")
		return
	}

	sess := h.sessions.Get(req.SessionID)
	if sess == nil {
		h.sendError(conn, "session not found")
		return
	}

	// 异步执行压缩操作
	go func() {
		// 执行压缩
		sess.CompactContext()

		// 保存会话
		if err := h.sessions.Save(req.SessionID); err != nil {
			log.Printf("[WS] Failed to save session after compact: %v", err)
		}

		// 获取压缩后的统计信息
		stats := sess.GetContextStats()

		log.Printf("[WS] Compacted context for session %s", req.SessionID)

		h.sendJSON(conn, map[string]interface{}{
			"type": "compact_done",
			"payload": map[string]interface{}{
				"session_id": req.SessionID,
				"stats":      stats,
				"success":    true,
			},
		})
	}()
}

// ContextStatsPayload 上下文统计载荷
type ContextStatsPayload struct {
	SessionID string `json:"session_id"`
}

// handleContextStats 处理上下文统计请求
func (h *WSHandler) handleContextStats(conn *websocket.Conn, payload json.RawMessage) {
	var req ContextStatsPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid context_stats payload")
		return
	}

	if req.SessionID == "" {
		h.sendError(conn, "session_id is required")
		return
	}

	sess := h.sessions.Get(req.SessionID)
	if sess == nil {
		h.sendError(conn, "session not found")
		return
	}

	stats := sess.GetContextStats()

	h.sendJSON(conn, map[string]interface{}{
		"type": "context_stats",
		"payload": map[string]interface{}{
			"session_id": req.SessionID,
			"stats":      stats,
		},
	})
}
