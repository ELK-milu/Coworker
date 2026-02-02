package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/claudecli/internal/client"
	"github.com/QuantumNous/new-api/claudecli/internal/loop"
	"github.com/QuantumNous/new-api/claudecli/internal/mcp"
	"github.com/QuantumNous/new-api/claudecli/internal/permissions"
	"github.com/QuantumNous/new-api/claudecli/internal/session"
	"github.com/QuantumNous/new-api/claudecli/internal/skills"
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
	client      *client.ClaudeClient
	sessions    *session.Manager
	tools       *tools.Registry
	workspace   *workspace.Manager
	tasks       *task.Manager
	permissions *permissions.Checker
	skills      *skills.Registry
	skillExec   *skills.Executor
	mcp         *mcp.Manager
	system      string
	mu          sync.Mutex
	cancelFunc  context.CancelFunc
	connMu      sync.Mutex
}

// NewWSHandler 创建 WebSocket 处理器
func NewWSHandler(
	c *client.ClaudeClient,
	sm *session.Manager,
	tr *tools.Registry,
	wm *workspace.Manager,
	tm *task.Manager,
	perm *permissions.Checker,
	sk *skills.Registry,
	mcpMgr *mcp.Manager,
	systemPrompt string,
) *WSHandler {
	return &WSHandler{
		client:      c,
		sessions:    sm,
		tools:       tr,
		workspace:   wm,
		tasks:       tm,
		permissions: perm,
		skills:      sk,
		skillExec:   skills.NewExecutor(sk),
		mcp:         mcpMgr,
		system:      systemPrompt,
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
		// Permission 相关消息
		case "set_permission_mode":
			log.Printf("[WS] Processing set_permission_mode message")
			h.handleSetPermissionMode(conn, wsMsg.Payload)
		case "get_permission_mode":
			log.Printf("[WS] Processing get_permission_mode message")
			h.handleGetPermissionMode(conn)
		// Skill 相关消息
		case "skill_call":
			log.Printf("[WS] Processing skill_call message")
			h.handleSkillCall(conn, wsMsg.Payload)
		case "list_skills":
			log.Printf("[WS] Processing list_skills message")
			h.handleListSkills(conn)
		// AskUser 响应消息
		case "ask_user_response":
			log.Printf("[WS] Processing ask_user_response message")
			h.handleAskUserResponse(conn, wsMsg.Payload)
		// MCP 相关消息
		case "mcp_connect":
			log.Printf("[WS] Processing mcp_connect message")
			h.handleMCPConnect(conn, wsMsg.Payload)
		case "mcp_disconnect":
			log.Printf("[WS] Processing mcp_disconnect message")
			h.handleMCPDisconnect(conn, wsMsg.Payload)
		case "mcp_list":
			log.Printf("[WS] Processing mcp_list message")
			h.handleMCPList(conn)
		case "mcp_call":
			log.Printf("[WS] Processing mcp_call message")
			h.handleMCPCall(conn, wsMsg.Payload)
		// Config 相关消息
		case "load_config":
			log.Printf("[WS] Processing load_config message")
			h.handleLoadConfig(conn, wsMsg.Payload)
		case "save_config":
			log.Printf("[WS] Processing save_config message")
			h.handleSaveConfig(conn, wsMsg.Payload)
		// Structured Output 相关消息
		case "set_output_schema":
			log.Printf("[WS] Processing set_output_schema message")
			h.handleSetOutputSchema(conn, wsMsg.Payload)
		case "clear_output_schema":
			log.Printf("[WS] Processing clear_output_schema message")
			h.handleClearOutputSchema(conn)
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
			payload["elapsed_ms"] = event.ElapsedMs
			payload["timeout_ms"] = event.TimeoutMs
			payload["timed_out"] = event.TimedOut
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

// ========== Permission 相关处理 ==========

// SetPermissionModePayload 设置权限模式载荷
type SetPermissionModePayload struct {
	Mode string `json:"mode"`
}

// handleSetPermissionMode 处理设置权限模式请求
func (h *WSHandler) handleSetPermissionMode(conn *websocket.Conn, payload json.RawMessage) {
	var req SetPermissionModePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid set_permission_mode payload")
		return
	}

	if req.Mode == "" {
		h.sendError(conn, "mode is required")
		return
	}

	mode := permissions.PermissionMode(req.Mode)
	h.permissions.SetMode(mode)

	log.Printf("[WS] Permission mode set to: %s", req.Mode)

	h.sendJSON(conn, map[string]interface{}{
		"type": "permission_mode_changed",
		"payload": map[string]interface{}{
			"mode":    req.Mode,
			"success": true,
		},
	})
}

// handleGetPermissionMode 处理获取权限模式请求
func (h *WSHandler) handleGetPermissionMode(conn *websocket.Conn) {
	mode := h.permissions.GetMode()

	h.sendJSON(conn, map[string]interface{}{
		"type": "permission_mode",
		"payload": map[string]interface{}{
			"mode": string(mode),
		},
	})
}

// ========== Skill 相关处理 ==========

// SkillCallPayload 技能调用载荷
type SkillCallPayload struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}

// handleSkillCall 处理技能调用请求
func (h *WSHandler) handleSkillCall(conn *websocket.Conn, payload json.RawMessage) {
	var req SkillCallPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid skill_call payload")
		return
	}

	// 移除前导斜杠
	name := strings.TrimPrefix(req.Name, "/")

	content, err := h.skillExec.Execute(name, req.Args)
	if err != nil {
		h.sendError(conn, err.Error())
		return
	}

	log.Printf("[WS] Skill expanded: %s", name)

	h.sendJSON(conn, map[string]interface{}{
		"type": "skill_expanded",
		"payload": map[string]interface{}{
			"name":    name,
			"content": content,
		},
	})
}

// handleListSkills 处理列出技能请求
func (h *WSHandler) handleListSkills(conn *websocket.Conn) {
	skillList := h.skills.GetAll()

	h.sendJSON(conn, map[string]interface{}{
		"type": "skills_list",
		"payload": map[string]interface{}{
			"skills": skillList,
		},
	})
}

// ========== AskUser 相关处理 ==========

// AskUserResponsePayload 用户响应载荷
type AskUserResponsePayload struct {
	RequestID string            `json:"request_id"`
	Answers   map[string]string `json:"answers"`
	Cancelled bool              `json:"cancelled"`
}

// handleAskUserResponse 处理用户响应
func (h *WSHandler) handleAskUserResponse(conn *websocket.Conn, payload json.RawMessage) {
	var req AskUserResponsePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid ask_user_response payload")
		return
	}

	// 获取 AskUserQuestionTool 实例
	tool, found := h.tools.Get("AskUserQuestion")
	if !found {
		h.sendError(conn, "AskUserQuestion tool not found")
		return
	}

	askTool, ok := tool.(*tools.AskUserQuestionTool)
	if !ok {
		h.sendError(conn, "invalid AskUserQuestion tool type")
		return
	}

	// 转发响应到工具
	resp := &tools.UserResponse{
		RequestID: req.RequestID,
		Answers:   req.Answers,
		Cancelled: req.Cancelled,
	}

	if askTool.HandleResponse(resp) {
		log.Printf("[WS] AskUser response handled: %s", req.RequestID)
	} else {
		log.Printf("[WS] AskUser response not found: %s", req.RequestID)
	}
}

// ========== MCP 相关处理 ==========

// MCPConnectPayload MCP 连接载荷
type MCPConnectPayload struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Env     []string `json:"env"`
	Cwd     string   `json:"cwd"`
}

// handleMCPConnect 处理 MCP 连接请求
func (h *WSHandler) handleMCPConnect(conn *websocket.Conn, payload json.RawMessage) {
	var req MCPConnectPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid mcp_connect payload")
		return
	}

	go func() {
		cfg := &mcp.TransportConfig{
			Command: req.Command,
			Args:    req.Args,
			Env:     req.Env,
			Cwd:     req.Cwd,
		}

		mcpConn, err := h.mcp.Connect(context.Background(), req.Name, cfg)
		if err != nil {
			h.sendError(conn, "mcp connect failed: "+err.Error())
			return
		}

		h.sendJSON(conn, map[string]interface{}{
			"type": "mcp_connected",
			"payload": map[string]interface{}{
				"id":     mcpConn.ID,
				"name":   mcpConn.Name,
				"server": mcpConn.Server,
				"tools":  mcpConn.Tools,
			},
		})
	}()
}

// MCPDisconnectPayload MCP 断开载荷
type MCPDisconnectPayload struct {
	ID string `json:"id"`
}

// handleMCPDisconnect 处理 MCP 断开请求
func (h *WSHandler) handleMCPDisconnect(conn *websocket.Conn, payload json.RawMessage) {
	var req MCPDisconnectPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid mcp_disconnect payload")
		return
	}

	if err := h.mcp.Disconnect(req.ID); err != nil {
		h.sendError(conn, err.Error())
		return
	}

	h.sendJSON(conn, map[string]interface{}{
		"type": "mcp_disconnected",
		"payload": map[string]interface{}{
			"id": req.ID,
		},
	})
}

// handleMCPList 处理 MCP 列表请求
func (h *WSHandler) handleMCPList(conn *websocket.Conn) {
	connections := h.mcp.List()

	list := make([]map[string]interface{}, 0, len(connections))
	for _, c := range connections {
		list = append(list, map[string]interface{}{
			"id":     c.ID,
			"name":   c.Name,
			"server": c.Server,
			"tools":  c.Tools,
		})
	}

	h.sendJSON(conn, map[string]interface{}{
		"type": "mcp_list",
		"payload": map[string]interface{}{
			"connections": list,
		},
	})
}

// MCPCallPayload MCP 工具调用载荷
type MCPCallPayload struct {
	ConnID   string          `json:"conn_id"`
	ToolName string          `json:"tool_name"`
	Args     json.RawMessage `json:"args"`
}

// handleMCPCall 处理 MCP 工具调用请求
func (h *WSHandler) handleMCPCall(conn *websocket.Conn, payload json.RawMessage) {
	var req MCPCallPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid mcp_call payload")
		return
	}

	go func() {
		result, err := h.mcp.CallTool(context.Background(), req.ConnID, req.ToolName, req.Args)
		if err != nil {
			h.sendError(conn, "mcp call failed: "+err.Error())
			return
		}

		h.sendJSON(conn, map[string]interface{}{
			"type": "mcp_result",
			"payload": map[string]interface{}{
				"conn_id":   req.ConnID,
				"tool_name": req.ToolName,
				"result":    result,
			},
		})
	}()
}

// ========== Config 相关处理 ==========

// ConfigPayload 配置载荷
type ConfigPayload struct {
	UserID  string `json:"user_id"`
	Content string `json:"content"`
}

// handleLoadConfig 处理加载配置请求
func (h *WSHandler) handleLoadConfig(conn *websocket.Conn, payload json.RawMessage) {
	var req ConfigPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid load_config payload")
		return
	}

	go func() {
		content, err := h.workspace.LoadConfig(req.UserID)
		if err != nil {
			h.sendJSON(conn, map[string]interface{}{
				"type": "config_loaded",
				"payload": map[string]interface{}{
					"success": true,
					"content": "",
				},
			})
			return
		}

		h.sendJSON(conn, map[string]interface{}{
			"type": "config_loaded",
			"payload": map[string]interface{}{
				"success": true,
				"content": content,
			},
		})
	}()
}

// handleSaveConfig 处理保存配置请求
func (h *WSHandler) handleSaveConfig(conn *websocket.Conn, payload json.RawMessage) {
	var req ConfigPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid save_config payload")
		return
	}

	go func() {
		err := h.workspace.SaveConfig(req.UserID, req.Content)
		if err != nil {
			h.sendError(conn, "save config failed: "+err.Error())
			return
		}

		h.sendJSON(conn, map[string]interface{}{
			"type": "config_saved",
			"payload": map[string]interface{}{
				"success": true,
			},
		})
	}()
}

// ========== Structured Output 相关处理 ==========

// SetOutputSchemaPayload 设置输出 schema 载荷
type SetOutputSchemaPayload struct {
	Schema map[string]interface{} `json:"schema"`
}

// handleSetOutputSchema 处理设置输出 schema 请求
func (h *WSHandler) handleSetOutputSchema(conn *websocket.Conn, payload json.RawMessage) {
	var req SetOutputSchemaPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid set_output_schema payload")
		return
	}

	if req.Schema == nil {
		h.sendError(conn, "schema is required")
		return
	}

	if err := h.tools.SetStructuredOutputSchema(req.Schema); err != nil {
		h.sendError(conn, "failed to set schema: "+err.Error())
		return
	}

	log.Printf("[WS] Output schema set successfully")

	h.sendJSON(conn, map[string]interface{}{
		"type": "output_schema_set",
		"payload": map[string]interface{}{
			"success": true,
		},
	})
}

// handleClearOutputSchema 处理清除输出 schema 请求
func (h *WSHandler) handleClearOutputSchema(conn *websocket.Conn) {
	h.tools.ClearStructuredOutputSchema()

	log.Printf("[WS] Output schema cleared")

	h.sendJSON(conn, map[string]interface{}{
		"type": "output_schema_cleared",
		"payload": map[string]interface{}{
			"success": true,
		},
	})
}
