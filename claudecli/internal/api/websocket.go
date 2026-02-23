package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/claudecli/internal/agent"
	"github.com/QuantumNous/new-api/claudecli/internal/client"
	"github.com/QuantumNous/new-api/claudecli/internal/config"
	"github.com/QuantumNous/new-api/claudecli/internal/embedding"
	"github.com/QuantumNous/new-api/claudecli/internal/eventbus"
	"github.com/QuantumNous/new-api/claudecli/internal/job"
	"github.com/QuantumNous/new-api/claudecli/internal/loop"
	"github.com/QuantumNous/new-api/claudecli/internal/mcp"
	"github.com/QuantumNous/new-api/claudecli/internal/memory"
	"github.com/QuantumNous/new-api/claudecli/internal/profile"
	"github.com/QuantumNous/new-api/claudecli/internal/prompt"
	"github.com/QuantumNous/new-api/claudecli/internal/sandbox"
	"github.com/QuantumNous/new-api/claudecli/internal/sanitize"
	"github.com/QuantumNous/new-api/claudecli/internal/session"
	"github.com/QuantumNous/new-api/claudecli/internal/store"
	"github.com/QuantumNous/new-api/claudecli/internal/task"
	"github.com/QuantumNous/new-api/claudecli/internal/tools"
	"github.com/QuantumNous/new-api/claudecli/internal/variable"
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
	jobs        *job.Manager
	variables   *variable.Manager
	memories    *memory.Manager
	profiles    *profile.Manager
	mcp         *mcp.Manager
	store       *store.Manager
	embedding   *embedding.Client      // Embedding 客户端
	milvus      *memory.MilvusClient   // Milvus 向量数据库客户端
	config      *config.Config         // 配置（用于动态构建系统提示词）
	mu          sync.Mutex
	cancelFunc  context.CancelFunc
	connMu      sync.Mutex
	// P0.4: 会话 Busy 锁 — 防止同一会话并发处理
	busySessions sync.Map // map[sessionID]bool
	// 当前连接的会话信息（用于断开时提取记忆）
	currentUserID    string
	currentSessionID string
	// P2.5: 文件修改时间追踪
	fileTime *tools.FileTime
	// 事件总线
	bus *eventbus.Bus
}

// NewWSHandler 创建 WebSocket 处理器
func NewWSHandler(
	c *client.ClaudeClient,
	sm *session.Manager,
	tr *tools.Registry,
	wm *workspace.Manager,
	tm *task.Manager,
	mcpMgr *mcp.Manager,
	cfg *config.Config,
) *WSHandler {
	return &WSHandler{
		client:    c,
		sessions:  sm,
		tools:     tr,
		workspace: wm,
		tasks:     tm,
		mcp:       mcpMgr,
		config:    cfg,
	}
}

// SetJobManager 设置 Job 管理器
func (h *WSHandler) SetJobManager(jm *job.Manager) {
	h.jobs = jm
}

// SetVariableManager 设置变量管理器
func (h *WSHandler) SetVariableManager(vm *variable.Manager) {
	h.variables = vm
}

// SetMemoryManager 设置记忆管理器
func (h *WSHandler) SetMemoryManager(mm *memory.Manager) {
	h.memories = mm
}

// SetProfileManager 设置用户画像管理器
func (h *WSHandler) SetProfileManager(pm *profile.Manager) {
	h.profiles = pm
}

// SetFileTime 设置文件修改时间追踪器
func (h *WSHandler) SetFileTime(ft *tools.FileTime) {
	h.fileTime = ft
}

// SetEmbeddingClient 设置 Embedding 客户端
func (h *WSHandler) SetEmbeddingClient(ec *embedding.Client) {
	h.embedding = ec
}

// SetMilvusClient 设置 Milvus 客户端
func (h *WSHandler) SetMilvusClient(mc *memory.MilvusClient) {
	h.milvus = mc
}

// SetEventBus 设置事件总线
func (h *WSHandler) SetEventBus(bus *eventbus.Bus) {
	h.bus = bus
}

// SetStoreManager 设置商店管理器
func (h *WSHandler) SetStoreManager(sm *store.Manager) {
	h.store = sm
}

// IsBusySession 检查会话是否正在被 WebSocket 使用
func (h *WSHandler) IsBusySession(sessionID string) (interface{}, bool) {
	return h.busySessions.Load(sessionID)
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
	AgentName   string `json:"agent,omitempty"` // P2.4: 可选的 Agent 名称（默认 build）
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
	// 连接断开时通过 EventBus 通知记忆系统
	defer func() {
		h.mu.Lock()
		userID := h.currentUserID
		sessionID := h.currentSessionID
		h.mu.Unlock()

		if userID != "" && sessionID != "" {
			if sess := h.sessions.Get(sessionID); sess != nil {
				log.Printf("[WS] Connection closed, emitting SessionEnd for user %s, session %s", userID, sessionID)
				if h.bus != nil {
					windowIndex := 0
					if sess.Context != nil {
						windowIndex = sess.Context.GetWindowIndex()
					}
					h.bus.Emit(eventbus.Event{
						Type:      eventbus.EventSessionEnd,
						UserID:    userID,
						SessionID: sessionID,
						Data: map[string]interface{}{
							"session":      sess,
							"window_index": windowIndex,
						},
					})
				}
			}
		}
	}()

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
		case "task_reorder":
			log.Printf("[WS] Processing task_reorder message")
			h.handleTaskReorder(conn, wsMsg.Payload)
		// Compact 相关消息
		case "compact":
			log.Printf("[WS] Processing compact message")
			h.handleCompact(conn, wsMsg.Payload)
		case "context_stats":
			log.Printf("[WS] Processing context_stats message")
			h.handleContextStats(conn, wsMsg.Payload)
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
		// Job 相关消息
		case "job_create":
			log.Printf("[WS] Processing job_create message")
			h.handleJobCreate(conn, wsMsg.Payload)
		case "job_update":
			log.Printf("[WS] Processing job_update message")
			h.handleJobUpdate(conn, wsMsg.Payload)
		case "job_delete":
			log.Printf("[WS] Processing job_delete message")
			h.handleJobDelete(conn, wsMsg.Payload)
		case "job_list":
			log.Printf("[WS] Processing job_list message")
			h.handleJobList(conn, wsMsg.Payload)
		case "job_run":
			log.Printf("[WS] Processing job_run message")
			h.handleJobRun(conn, wsMsg.Payload)
		case "job_reorder":
			log.Printf("[WS] Processing job_reorder message")
			h.handleJobReorder(conn, wsMsg.Payload)
		// Memory 相关消息
		case "memory_create":
			log.Printf("[WS] Processing memory_create message")
			h.handleMemoryCreate(conn, wsMsg.Payload)
		case "memory_update":
			log.Printf("[WS] Processing memory_update message")
			h.handleMemoryUpdate(conn, wsMsg.Payload)
		case "memory_delete":
			log.Printf("[WS] Processing memory_delete message")
			h.handleMemoryDelete(conn, wsMsg.Payload)
		case "memory_list":
			log.Printf("[WS] Processing memory_list message")
			h.handleMemoryList(conn, wsMsg.Payload)
		case "memory_search":
			log.Printf("[WS] Processing memory_search message")
			h.handleMemorySearch(conn, wsMsg.Payload)
		case "extract_memories":
			log.Printf("[WS] Processing extract_memories message")
			h.handleExtractMemories(conn, wsMsg.Payload)
		// Profile 相关消息
		case "profile_get":
			log.Printf("[WS] Processing profile_get message")
			h.handleProfileGet(conn, wsMsg.Payload)
		case "profile_update":
			log.Printf("[WS] Processing profile_update message")
			h.handleProfileUpdate(conn, wsMsg.Payload)
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

	// 防提示词注入：剥离用户输入中的系统标签
	chat.Message = sanitize.UserInput(chat.Message)

	// 获取或创建会话
	isNewSession := false
	sess := h.sessions.Get(chat.SessionID)
	if sess == nil {
		sess = h.sessions.Create(chat.UserID)
		isNewSession = true
	}

	// P0.4: 会话 Busy 锁 — 防止同一会话并发处理
	if _, busy := h.busySessions.LoadOrStore(sess.ID, true); busy {
		log.Printf("[WS] Session %s is busy, rejecting request", sess.ID)
		h.sendJSON(conn, map[string]interface{}{
			"type": "error",
			"payload": map[string]interface{}{
				"error": "Session is currently processing a request. Please wait for it to complete or abort the current request.",
			},
		})
		return
	}

	// 更新当前连接的会话信息（用于断开时提取记忆）
	h.mu.Lock()
	h.currentUserID = chat.UserID
	h.currentSessionID = sess.ID
	h.mu.Unlock()

	// 注入 EventBus 到会话的上下文管理器（用于 compaction 时自动触发记忆提取）
	if h.bus != nil && sess.Context != nil {
		sess.Context.SetEventBus(h.bus)
		sess.Context.SetEventContext(chat.UserID, sess.ID)
	}

	// 获取用户工作空间路径
	userWorkDir := h.workspace.GetUserWorkDir(chat.UserID)

	// 创建用户沙箱
	sb := sandbox.NewSandbox(chat.UserID, userWorkDir)

	// 如果前端指定了工作路径，更新会话的工作目录（仍使用真实路径）
	if chat.WorkingPath != "" {
		realWorkDir, err := sb.ToReal(chat.WorkingPath)
		if err == nil {
			sess.SetWorkingDir(realWorkDir)
			log.Printf("[WS] Updated working dir for session %s: %s (virtual: %s)", sess.ID, realWorkDir, chat.WorkingPath)
		}
	} else {
		sess.SetWorkingDir(userWorkDir)
	}

	// 渐进式披露：刷新 SkillsTool 的可用技能列表（动态 description）
	// 复制用户已安装的 skill 到 workspace/.skills/
	if h.store != nil {
		if err := h.store.CopySkillsToWorkspace(chat.UserID, userWorkDir); err != nil {
			log.Printf("[WS] Failed to copy skills to workspace: %v", err)
		}
	}
	if skillTool, ok := h.tools.Get("Skills"); ok {
		if inner, ok := tools.UnwrapAs[*tools.SkillsTool](skillTool); ok {
			inner.RefreshForUser(chat.UserID, userWorkDir)
		}
	}

	// 动态构建系统提示词（使用虚拟路径，传入用户消息用于记忆检索）
	systemPrompt := h.buildUserSystemPrompt(chat.UserID, sb, chat.Message)

	// 创建可取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	h.mu.Lock()
	h.cancelFunc = cancel
	h.mu.Unlock()

	// 创建事件通道
	eventCh := make(chan loop.LoopEvent, 100)

	// 如果是新会话，发送 session_created 事件
	if isNewSession {
		// 先发送 session_created 消息
		h.sendJSON(conn, map[string]interface{}{
			"type": "session_created",
			"payload": map[string]interface{}{
				"session_id": sess.ID,
				"title":      "新对话",
				"created_at": sess.CreatedAt.Unix(),
				"updated_at": sess.UpdatedAt.Unix(),
			},
		})
	}

	// P2.4: 根据 payload 选择 Agent
	selectedAgent := agent.DefaultRegistry.GetDefault()
	if chat.AgentName != "" {
		if a := agent.DefaultRegistry.Get(chat.AgentName); a != nil {
			selectedAgent = a
			log.Printf("[WS] Using agent: %s", chat.AgentName)
		} else {
			log.Printf("[WS] Unknown agent '%s', using default", chat.AgentName)
		}
	}

	// 启动对话循环（传递沙箱和 Agent）
	go h.runConversation(ctx, sess, chat.UserID, chat.Message, systemPrompt, sb, selectedAgent, eventCh, conn, isNewSession)

	// 异步转发事件到 WebSocket（不阻塞消息读取循环）
	go h.forwardEvents(conn, sess.ID, chat.UserID, eventCh)
}

// getClientForUser 根据用户配置创建 per-user 客户端（固定令牌 playground-default）
func (h *WSHandler) getClientForUser(userID string) *client.ClaudeClient {
	if h.workspace == nil {
		return h.client
	}
	info, err := h.workspace.LoadUserInfo(userID)
	if err != nil || info == nil {
		return h.client
	}
	// 使用用户选择的模型，未选则用全局默认
	model := h.config.Claude.Model
	if info.SelectedModel != "" {
		model = info.SelectedModel
	}
	// 令牌：优先用户配置的 key，否则用固定 "playground-default"
	tokenKey := info.ApiTokenKey
	if tokenKey == "" {
		tokenKey = "playground-default"
	}
	// 构建 Relay URL: http://127.0.0.1:{PORT}
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	relayBaseURL := "http://127.0.0.1:" + port
	log.Printf("[WS] User %s using Relay, token: %s, model: %s", userID, tokenKey, model)
	c := client.NewClaudeClient(
		tokenKey, "", relayBaseURL,
		model, int(h.config.Claude.MaxTokens),
	)
	c.SetSamplingParams(info.Temperature, info.TopP)
	return c
}

// runConversation 运行对话
func (h *WSHandler) runConversation(ctx context.Context, sess *session.Session, userID string, msg string, systemPrompt string, sb *sandbox.Sandbox, ag *agent.AgentType, eventCh chan loop.LoopEvent, conn *websocket.Conn, isNewSession bool) {
	defer close(eventCh)
	// P0.4: 对话结束时释放 Busy 锁
	defer h.busySessions.Delete(sess.ID)

	userClient := h.getClientForUser(userID)
	l := loop.NewConversationLoop(userClient, sess, h.tools, systemPrompt, userID, sb, h.fileTime, ag, eventCh)
	l.ProcessMessage(ctx, msg)

	// 对话结束后保存会话
	if err := h.sessions.Save(sess.ID); err != nil {
		log.Printf("[WS] Failed to save session %s: %v", sess.ID, err)
	} else {
		log.Printf("[WS] Session saved: %s", sess.ID)
	}

	// 每轮对话结束后通过 EventBus 通知记忆系统（增量提取）
	if h.bus != nil {
		windowIndex := 0
		if sess.Context != nil {
			windowIndex = sess.Context.GetWindowIndex()
		}
		h.bus.Emit(eventbus.Event{
			Type:      eventbus.EventTurnCompleted,
			UserID:    userID,
			SessionID: sess.ID,
			Data: map[string]interface{}{
				"session":      sess,
				"window_index": windowIndex,
			},
		})
	}

	// 如果是新会话且还没有标题，异步生成标题
	// 注意：使用新的 context，因为对话的 ctx 可能已被取消
	if isNewSession && sess.GetTitle() == "" {
		go h.generateSessionTitle(context.Background(), sess, msg, conn)
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
	frontendMessages := ConvertMessagesToFrontend(messages)

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

// ConvertMessagesToFrontend 将后端消息格式转换为前端格式
func ConvertMessagesToFrontend(messages []types.Message) []map[string]interface{} {
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
							// 添加执行时间信息
							if toolResult.ElapsedMs > 0 {
								result[i]["elapsedMs"] = toolResult.ElapsedMs
							}
							if toolResult.TimeoutMs > 0 {
								result[i]["timeoutMs"] = toolResult.TimeoutMs
							}
							if toolResult.TimedOut {
								result[i]["timedOut"] = toolResult.TimedOut
							}
							if toolResult.ExecEnv != "" {
								result[i]["execEnv"] = toolResult.ExecEnv
							}
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
				// 提取执行时间信息
				elapsedMs, _ := blockMap["elapsed_ms"].(float64)
				timeoutMs, _ := blockMap["timeout_ms"].(float64)
				timedOut, _ := blockMap["timed_out"].(bool)
				execEnv, _ := blockMap["exec_env"].(string)
				// 查找对应的工具调用并更新结果
				for i := len(result) - 1; i >= 0; i-- {
					if result[i]["toolId"] == toolUseID {
						result[i]["result"] = content
						result[i]["isError"] = isError
						// 添加执行时间信息
						if elapsedMs > 0 {
							result[i]["elapsedMs"] = int64(elapsedMs)
						}
						if timeoutMs > 0 {
							result[i]["timeoutMs"] = int64(timeoutMs)
						}
						if timedOut {
							result[i]["timedOut"] = timedOut
						}
						if execEnv != "" {
							result[i]["execEnv"] = execEnv
						}
						break
					}
				}
			}
		}
	}

	return result
}

// forwardEvents 转发事件到 WebSocket
func (h *WSHandler) forwardEvents(conn *websocket.Conn, sessID string, userID string, eventCh <-chan loop.LoopEvent) {
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
		case loop.EventTypeToolInput:
			log.Printf("[WS] Forwarding tool_input event: tool_id=%s, input_len=%d", event.ToolID, len(event.ToolInput))
			payload["name"] = event.ToolName
			payload["tool_id"] = event.ToolID
			payload["input"] = event.ToolInput
		case loop.EventTypeToolEnd:
			payload["name"] = event.ToolName
			payload["tool_id"] = event.ToolID
			payload["input"] = event.ToolInput
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
		case loop.EventTypeTaskChanged:
			// 任务变更事件
			payload["action"] = event.TaskAction
			payload["task"] = event.TaskData
			}

		h.sendJSON(conn, map[string]interface{}{
			"type":    event.Type,
			"payload": payload,
		})

		// 任务变更后，额外发送完整任务列表快照（用于前端对话流中嵌入历史进度）
		if event.Type == loop.EventTypeTaskChanged && h.tasks != nil {
			taskList := h.tasks.List(userID, "default")
			if len(taskList) > 0 {
				h.sendJSON(conn, map[string]interface{}{
					"type": "task_progress",
					"payload": map[string]interface{}{
						"session_id": sessID,
						"tasks":      taskList,
					},
				})
			}
		}
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

// TaskReorderPayload 任务排序载荷
type TaskReorderPayload struct {
	UserID  string   `json:"user_id"`
	ListID  string   `json:"list_id"`
	TaskIDs []string `json:"task_ids"`
}

// handleTaskReorder 处理任务排序请求
func (h *WSHandler) handleTaskReorder(conn *websocket.Conn, payload json.RawMessage) {
	var req TaskReorderPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid task_reorder payload")
		return
	}

	if req.UserID == "" || len(req.TaskIDs) == 0 {
		h.sendError(conn, "user_id and task_ids are required")
		return
	}

	if req.ListID == "" {
		req.ListID = "default"
	}

	if h.tasks == nil {
		h.sendError(conn, "task manager not initialized")
		return
	}

	go func() {
		if err := h.tasks.UpdateOrder(req.UserID, req.ListID, req.TaskIDs); err != nil {
			h.sendError(conn, "failed to reorder tasks: "+err.Error())
			return
		}

		log.Printf("[WS] Reordered tasks for user %s", req.UserID)

		h.sendJSON(conn, map[string]interface{}{
			"type": "tasks_reordered",
			"payload": map[string]interface{}{
				"user_id":  req.UserID,
				"list_id":  req.ListID,
				"task_ids": req.TaskIDs,
				"success":  true,
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
		// 注意：压缩前的记忆提取现在由 context.Manager 内部通过 EventBus 的
		// BeforeCompact 事件自动触发，无需在此显式调用

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


// ========== Skill 相关处理 ==========

// SkillCallPayload 技能调用载荷
type SkillCallPayload struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}

// handleSkillCall 处理技能调用请求（从 store 加载）
func (h *WSHandler) handleSkillCall(conn *websocket.Conn, payload json.RawMessage) {
	var req SkillCallPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid skill_call payload")
		return
	}

	name := strings.TrimPrefix(req.Name, "/")

	if h.store == nil {
		h.sendError(conn, "store not available")
		return
	}

	// 从 store 查找技能内容
	var content string
	for _, item := range h.store.List() {
		if item.Type == store.TypeSkill && item.Name == name && item.Content != "" {
			content = item.Content
			break
		}
	}
	if content == "" {
		h.sendError(conn, fmt.Sprintf("skill not found: %s", name))
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

// handleListSkills 处理列出技能请求（从 store 加载）
func (h *WSHandler) handleListSkills(conn *websocket.Conn) {
	var skillList []map[string]string
	if h.store != nil {
		for _, item := range h.store.List() {
			if item.Type == store.TypeSkill {
				skillList = append(skillList, map[string]string{
					"name":        item.Name,
					"description": item.Description,
				})
			}
		}
	}
	h.sendJSON(conn, map[string]interface{}{
		"type":    "skills_list",
		"payload": map[string]interface{}{"skills": skillList},
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

// buildUserSystemPrompt 为用户动态构建系统提示词（使用虚拟路径）
func (h *WSHandler) buildUserSystemPrompt(userID string, sb *sandbox.Sandbox, userMessage string) string {
	// 使用虚拟工作目录
	virtualWorkDir := sb.GetVirtualWorkingDir()
	realWorkDir := sb.GetRealWorkingDir()

	// 确定平台：如果启用 nsjail 沙箱，命令在隔离环境中执行
	platform := "linux"
	if h.config.Nsjail.Enabled {
		platform = "linux (nsjail sandbox)"
	}

	// 获取任务列表渲染
	tasksRender := ""
	if h.tasks != nil {
		tasksRender = h.tasks.RenderCompact(userID, "default", 10)
	}

	// 获取相关记忆（功能4）- 根据用户消息检索
	relevantMemories := ""
	if h.memories != nil {
		// 使用用户消息检索相关记忆
		mems := h.memories.Retrieve(userID, userMessage, 5)
		if len(mems) > 0 {
			relevantMemories = h.memories.FormatForPrompt(mems)
			log.Printf("[WS] Retrieved %d memories for user %s", len(mems), userID)
		}
	}

	// 加载用户信息（用户称呼、Coworker称呼、手机号、邮箱）
	var userName, coworkerName, userPhone, userEmail string
	if h.workspace != nil {
		if userInfo, err := h.workspace.LoadUserInfo(userID); err == nil && userInfo != nil {
			userName = userInfo.UserName
			coworkerName = userInfo.CoworkerName
			userPhone = userInfo.Phone
			userEmail = userInfo.Email
		}
	}

	// 加载用户自定义提示词 (COWORKER.md)
	customRules := ""
	if h.workspace != nil {
		content, err := h.workspace.LoadConfig(userID)
		if err != nil {
			log.Printf("[WS] Failed to load COWORKER.md for user %s: %v", userID, err)
		} else if content == "" {
			log.Printf("[WS] COWORKER.md not found or empty for user %s (path: %s/.claude/COWORKER.md)", userID, h.workspace.GetUserWorkspace(userID))
		} else {
			customRules = content
			log.Printf("[WS] Loaded COWORKER.md for user %s (%d chars)", userID, len(content))
		}
	} else {
		log.Printf("[WS] WARNING: workspace manager is nil, cannot load COWORKER.md")
	}

	// 加载用户已安装的 Agent 指令（Skill 已通过工具描述渐进式披露）
	installedAgents := ""
	if h.store != nil {
		installedAgents = h.buildInstalledAgentsPrompt(userID)
	}

	// 构建提示词上下文
	promptCtx := &prompt.PromptContext{
		WorkingDir:       virtualWorkDir, // 使用虚拟路径
		Model:            h.config.Claude.Model,
		PermissionMode:   "default",
		Platform:         platform,
		TasksRender:      tasksRender,
		RelevantMemories: relevantMemories,
		CustomRules:      customRules,
		InstalledAgents:  installedAgents,
		// 用户信息
		UserName:     userName,
		CoworkerName: coworkerName,
		UserPhone:    userPhone,
		UserEmail:    userEmail,
	}

	// nsjail 沙箱模式：用户工作空间是隔离的，不继承宿主机的 git 状态
	// 只检查用户工作空间内是否有 .git 目录
	if h.config.Nsjail.Enabled {
		// 检查用户工作空间内的 git 状态（而非宿主机）
		promptCtx.IsGitRepo = prompt.IsGitRepo(realWorkDir)
		if promptCtx.IsGitRepo {
			gitStatus := prompt.GetGitStatus(realWorkDir)
			if gitStatus != nil {
				gitStatus.Staged = sb.VirtualizePaths(gitStatus.Staged)
				gitStatus.Unstaged = sb.VirtualizePaths(gitStatus.Unstaged)
				gitStatus.Untracked = sb.VirtualizePaths(gitStatus.Untracked)
				promptCtx.GitStatus = gitStatus
			}
		}
		// 用户工作空间内查找 CLAUDE.md
		promptCtx.ClaudeMdPath = prompt.FindClaudeMd(realWorkDir)
	} else {
		// 非容器模式：使用真实路径检查 git 状态
		promptCtx.IsGitRepo = prompt.IsGitRepo(realWorkDir)
		promptCtx.ClaudeMdPath = prompt.FindClaudeMd(realWorkDir)
		if promptCtx.IsGitRepo {
			gitStatus := prompt.GetGitStatus(realWorkDir)
			if gitStatus != nil {
				gitStatus.Staged = sb.VirtualizePaths(gitStatus.Staged)
				gitStatus.Unstaged = sb.VirtualizePaths(gitStatus.Unstaged)
				gitStatus.Untracked = sb.VirtualizePaths(gitStatus.Untracked)
				promptCtx.GitStatus = gitStatus
			}
		}
	}

	systemPrompt := prompt.BuildSystemPrompt(promptCtx)
	log.Printf("[WS] Built system prompt for user %s, length: %d chars, isGitRepo: %v, hasCoworkerMd: %v",
		userID, len(systemPrompt), promptCtx.IsGitRepo, customRules != "")

	return systemPrompt
}

// buildInstalledAgentsPrompt 注入用户已安装的 Agent 指令到系统提示词
// 注意：Skill 已通过 SkillsTool 的动态 Description 实现渐进式披露，不再注入系统提示词
func (h *WSHandler) buildInstalledAgentsPrompt(userID string) string {
	ids := h.store.LoadUserInstalled(userID)
	if len(ids) == 0 {
		return ""
	}

	var agentPrompts []string
	for _, id := range ids {
		item := h.store.GetByID(id)
		if item != nil && item.Type == store.TypeAgent && item.Content != "" {
			agentPrompts = append(agentPrompts, fmt.Sprintf("## Agent: %s\n%s", item.Name, item.Content))
		}
	}

	if len(agentPrompts) == 0 {
		return ""
	}

	return "# Installed Agent Instructions\n\n" + strings.Join(agentPrompts, "\n\n")
}

// generateSessionTitle 异步生成会话标题
// 参考 OpenCode: ensureTitle() + agent/prompt/title.txt
func (h *WSHandler) generateSessionTitle(ctx context.Context, sess *session.Session, firstMessage string, conn *websocket.Conn) {
	// 使用 OpenCode 风格的 TitlePrompt 作为系统提示词
	// 用户消息作为输入，让 AI 生成标题
	userMsg := firstMessage
	if len(userMsg) > 500 {
		userMsg = userMsg[:500] + "..."
	}

	// 使用 TitlePrompt 作为系统提示词 + 用户消息作为输入
	// 参考 OpenCode: agent/prompt/title.txt
	titlePromptMsg := prompt.TitlePrompt + "\n\nUser message: " + userMsg

	// 使用轻量级模型生成标题
	title, err := h.client.CreateSimpleMessage(ctx, titlePromptMsg, 50)
	if err != nil {
		log.Printf("[WS] Failed to generate title for session %s: %v", sess.ID, err)
		// 使用消息前缀作为后备标题
		title = firstMessage
		if len(title) > 30 {
			title = title[:30] + "..."
		}
	}

	// 清理标题
	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'")
	if len(title) > 50 {
		title = title[:50] + "..."
	}

	// 更新会话标题
	sess.SetTitle(title)

	// 保存会话
	if err := h.sessions.Save(sess.ID); err != nil {
		log.Printf("[WS] Failed to save session after title update: %v", err)
	}

	// 发送 title_updated 消息
	h.sendJSON(conn, map[string]interface{}{
		"type": "title_updated",
		"payload": map[string]interface{}{
			"session_id": sess.ID,
			"title":      title,
		},
	})

	log.Printf("[WS] Generated title for session %s: %s", sess.ID, title)
}

// ========== Job 相关处理 ==========

// JobCreatePayload 创建 Job 载荷
type JobCreatePayload struct {
	UserID  string `json:"user_id"`
	Name    string `json:"name"`
	Command string `json:"command"`

	// 新的简化调度配置
	ScheduleType    string `json:"schedule_type"`              // once, daily, weekly, interval, cron
	Time            string `json:"time,omitempty"`             // HH:MM
	Weekdays        []int  `json:"weekdays,omitempty"`         // [0-6], 0=周日
	IntervalMinutes int    `json:"interval_minutes,omitempty"` // 间隔分钟
	RunAt           int64  `json:"run_at,omitempty"`           // 单次执行时间戳

	// 兼容旧 API
	CronExpr string `json:"cron_expr,omitempty"`
}

// handleJobCreate 处理创建 Job 请求
func (h *WSHandler) handleJobCreate(conn *websocket.Conn, payload json.RawMessage) {
	var req JobCreatePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid job_create payload")
		return
	}

	if req.UserID == "" || req.Name == "" || req.Command == "" {
		h.sendError(conn, "user_id, name and command are required")
		return
	}

	// 确定调度类型
	scheduleType := job.ScheduleType(req.ScheduleType)
	if scheduleType == "" {
		// 兼容旧 API：如果提供了 cron_expr，使用 cron 类型
		if req.CronExpr != "" {
			scheduleType = job.ScheduleCron
		} else {
			h.sendError(conn, "schedule_type or cron_expr is required")
			return
		}
	}

	if h.jobs == nil {
		h.sendError(conn, "job manager not initialized")
		return
	}

	go func() {
		j, err := h.jobs.CreateWithSchedule(
			req.UserID, req.Name, req.Command,
			scheduleType,
			req.CronExpr, req.Time,
			req.Weekdays, req.IntervalMinutes, req.RunAt,
		)
		if err != nil {
			h.sendError(conn, "failed to create job: "+err.Error())
			return
		}

		log.Printf("[WS] Created job for user %s: %s", req.UserID, j.ID)

		h.sendJSON(conn, map[string]interface{}{
			"type": "job_created",
			"payload": map[string]interface{}{
				"user_id": req.UserID,
				"job":     j,
				"success": true,
			},
		})
	}()
}

// JobUpdatePayload 更新 Job 载荷
type JobUpdatePayload struct {
	UserID   string `json:"user_id"`
	JobID    string `json:"job_id"`
	Name     string `json:"name,omitempty"`
	CronExpr string `json:"cron_expr,omitempty"`
	Command  string `json:"command,omitempty"`
	Enabled  *bool  `json:"enabled,omitempty"`
}

// handleJobUpdate 处理更新 Job 请求
func (h *WSHandler) handleJobUpdate(conn *websocket.Conn, payload json.RawMessage) {
	var req JobUpdatePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid job_update payload")
		return
	}

	if req.UserID == "" || req.JobID == "" {
		h.sendError(conn, "user_id and job_id are required")
		return
	}

	if h.jobs == nil {
		h.sendError(conn, "job manager not initialized")
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.CronExpr != "" {
		updates["cron_expr"] = req.CronExpr
	}
	if req.Command != "" {
		updates["command"] = req.Command
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	go func() {
		j, err := h.jobs.Update(req.UserID, req.JobID, updates)
		if err != nil {
			h.sendError(conn, "failed to update job: "+err.Error())
			return
		}

		log.Printf("[WS] Updated job for user %s: %s", req.UserID, req.JobID)

		h.sendJSON(conn, map[string]interface{}{
			"type": "job_updated",
			"payload": map[string]interface{}{
				"user_id": req.UserID,
				"job":     j,
				"success": true,
			},
		})
	}()
}

// JobDeletePayload 删除 Job 载荷
type JobDeletePayload struct {
	UserID string `json:"user_id"`
	JobID  string `json:"job_id"`
}

// handleJobDelete 处理删除 Job 请求
func (h *WSHandler) handleJobDelete(conn *websocket.Conn, payload json.RawMessage) {
	var req JobDeletePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid job_delete payload")
		return
	}

	if req.UserID == "" || req.JobID == "" {
		h.sendError(conn, "user_id and job_id are required")
		return
	}

	if h.jobs == nil {
		h.sendError(conn, "job manager not initialized")
		return
	}

	go func() {
		if err := h.jobs.Delete(req.UserID, req.JobID); err != nil {
			h.sendError(conn, "failed to delete job: "+err.Error())
			return
		}

		log.Printf("[WS] Deleted job for user %s: %s", req.UserID, req.JobID)

		h.sendJSON(conn, map[string]interface{}{
			"type": "job_deleted",
			"payload": map[string]interface{}{
				"user_id": req.UserID,
				"job_id":  req.JobID,
				"success": true,
			},
		})
	}()
}

// JobListPayload Job 列表载荷
type JobListPayload struct {
	UserID string `json:"user_id"`
}

// handleJobList 处理 Job 列表请求
func (h *WSHandler) handleJobList(conn *websocket.Conn, payload json.RawMessage) {
	var req JobListPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid job_list payload")
		return
	}

	if req.UserID == "" {
		h.sendError(conn, "user_id is required")
		return
	}

	if h.jobs == nil {
		h.sendError(conn, "job manager not initialized")
		return
	}

	go func() {
		jobs := h.jobs.List(req.UserID)
		log.Printf("[WS] Listed %d jobs for user %s", len(jobs), req.UserID)

		h.sendJSON(conn, map[string]interface{}{
			"type": "jobs_list",
			"payload": map[string]interface{}{
				"user_id": req.UserID,
				"jobs":    jobs,
			},
		})
	}()
}

// JobRunPayload Job 运行载荷
type JobRunPayload struct {
	UserID string `json:"user_id"`
	JobID  string `json:"job_id"`
}

// handleJobRun 处理手动触发 Job 请求
func (h *WSHandler) handleJobRun(conn *websocket.Conn, payload json.RawMessage) {
	var req JobRunPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid job_run payload")
		return
	}

	if req.UserID == "" || req.JobID == "" {
		h.sendError(conn, "user_id and job_id are required")
		return
	}

	if h.jobs == nil {
		h.sendError(conn, "job manager not initialized")
		return
	}

	j := h.jobs.Get(req.UserID, req.JobID)
	if j == nil {
		h.sendError(conn, "job not found")
		return
	}

	// 标记为运行中
	h.jobs.MarkRunning(req.UserID, req.JobID)

	log.Printf("[WS] Job manually triggered for user %s: %s", req.UserID, req.JobID)

	h.sendJSON(conn, map[string]interface{}{
		"type": "job_triggered",
		"payload": map[string]interface{}{
			"user_id": req.UserID,
			"job":     j,
			"success": true,
		},
	})
}

// JobReorderPayload Job 排序载荷
type JobReorderPayload struct {
	UserID string   `json:"user_id"`
	JobIDs []string `json:"job_ids"`
}

// handleJobReorder 处理 Job 排序请求
func (h *WSHandler) handleJobReorder(conn *websocket.Conn, payload json.RawMessage) {
	var req JobReorderPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid job_reorder payload")
		return
	}

	if req.UserID == "" || len(req.JobIDs) == 0 {
		h.sendError(conn, "user_id and job_ids are required")
		return
	}

	if h.jobs == nil {
		h.sendError(conn, "job manager not initialized")
		return
	}

	go func() {
		if err := h.jobs.UpdateOrder(req.UserID, req.JobIDs); err != nil {
			h.sendError(conn, "failed to reorder jobs: "+err.Error())
			return
		}

		log.Printf("[WS] Reordered jobs for user %s", req.UserID)

		h.sendJSON(conn, map[string]interface{}{
			"type": "jobs_reordered",
			"payload": map[string]interface{}{
				"user_id": req.UserID,
				"job_ids": req.JobIDs,
				"success": true,
			},
		})
	}()
}

// ========== Memory 相关处理 ==========

// MemoryCreatePayload 创建记忆载荷
type MemoryCreatePayload struct {
	UserID    string   `json:"user_id"`
	Tags      []string `json:"tags"`
	Content   string   `json:"content"`
	Summary   string   `json:"summary"`
	Weight    float64  `json:"weight"`
	SessionID string   `json:"session_id"`
}

// handleMemoryCreate 处理创建记忆请求
func (h *WSHandler) handleMemoryCreate(conn *websocket.Conn, payload json.RawMessage) {
	var req MemoryCreatePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid memory_create payload")
		return
	}

	if req.UserID == "" || req.Content == "" {
		h.sendError(conn, "user_id and content are required")
		return
	}

	if h.memories == nil {
		h.sendError(conn, "memory manager not initialized")
		return
	}

	go func() {
		mem := &memory.Memory{
			Tags:      req.Tags,
			Content:   req.Content,
			Summary:   req.Summary,
			Weight:    req.Weight,
			SessionID: req.SessionID,
			Source:    "manual",
		}

		created, err := h.memories.Create(req.UserID, mem)
		if err != nil {
			h.sendError(conn, "failed to create memory: "+err.Error())
			return
		}

		log.Printf("[WS] Created memory for user %s: %s", req.UserID, created.ID)

		h.sendJSON(conn, map[string]interface{}{
			"type": "memory_created",
			"payload": map[string]interface{}{
				"user_id": req.UserID,
				"memory":  created,
				"success": true,
			},
		})
	}()
}

// MemoryUpdatePayload 更新记忆载荷
type MemoryUpdatePayload struct {
	UserID   string   `json:"user_id"`
	MemoryID string   `json:"memory_id"`
	Tags     []string `json:"tags,omitempty"`
	Content  string   `json:"content,omitempty"`
	Summary  string   `json:"summary,omitempty"`
	Weight   float64  `json:"weight,omitempty"`
}

// handleMemoryUpdate 处理更新记忆请求
func (h *WSHandler) handleMemoryUpdate(conn *websocket.Conn, payload json.RawMessage) {
	var req MemoryUpdatePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid memory_update payload")
		return
	}

	if req.UserID == "" || req.MemoryID == "" {
		h.sendError(conn, "user_id and memory_id are required")
		return
	}

	if h.memories == nil {
		h.sendError(conn, "memory manager not initialized")
		return
	}

	go func() {
		updates := make(map[string]interface{})
		if req.Content != "" {
			updates["content"] = req.Content
		}
		if req.Summary != "" {
			updates["summary"] = req.Summary
		}
		if len(req.Tags) > 0 {
			updates["tags"] = req.Tags
		}
		if req.Weight > 0 {
			updates["weight"] = req.Weight
		}

		updated, err := h.memories.Update(req.UserID, req.MemoryID, updates)
		if err != nil {
			h.sendError(conn, "failed to update memory: "+err.Error())
			return
		}

		log.Printf("[WS] Updated memory for user %s: %s", req.UserID, req.MemoryID)

		h.sendJSON(conn, map[string]interface{}{
			"type": "memory_updated",
			"payload": map[string]interface{}{
				"user_id": req.UserID,
				"memory":  updated,
				"success": true,
			},
		})
	}()
}

// MemoryDeletePayload 删除记忆载荷
type MemoryDeletePayload struct {
	UserID   string `json:"user_id"`
	MemoryID string `json:"memory_id"`
}

// handleMemoryDelete 处理删除记忆请求
func (h *WSHandler) handleMemoryDelete(conn *websocket.Conn, payload json.RawMessage) {
	var req MemoryDeletePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid memory_delete payload")
		return
	}

	if req.UserID == "" || req.MemoryID == "" {
		h.sendError(conn, "user_id and memory_id are required")
		return
	}

	if h.memories == nil {
		h.sendError(conn, "memory manager not initialized")
		return
	}

	go func() {
		if err := h.memories.Delete(req.UserID, req.MemoryID); err != nil {
			h.sendError(conn, "failed to delete memory: "+err.Error())
			return
		}

		log.Printf("[WS] Deleted memory for user %s: %s", req.UserID, req.MemoryID)

		h.sendJSON(conn, map[string]interface{}{
			"type": "memory_deleted",
			"payload": map[string]interface{}{
				"user_id":   req.UserID,
				"memory_id": req.MemoryID,
				"success":   true,
			},
		})
	}()
}

// MemoryListPayload 列出记忆载荷
type MemoryListPayload struct {
	UserID string `json:"user_id"`
}

// handleMemoryList 处理列出记忆请求
func (h *WSHandler) handleMemoryList(conn *websocket.Conn, payload json.RawMessage) {
	var req MemoryListPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid memory_list payload")
		return
	}

	if req.UserID == "" {
		h.sendError(conn, "user_id is required")
		return
	}

	if h.memories == nil {
		h.sendError(conn, "memory manager not initialized")
		return
	}

	go func() {
		h.memories.LoadUserMemories(req.UserID)
		memories := h.memories.List(req.UserID)

		log.Printf("[WS] Listed %d memories for user %s", len(memories), req.UserID)

		h.sendJSON(conn, map[string]interface{}{
			"type": "memories_list",
			"payload": map[string]interface{}{
				"user_id":  req.UserID,
				"memories": memories,
			},
		})
	}()
}

// MemorySearchPayload 搜索记忆载荷
type MemorySearchPayload struct {
	UserID string `json:"user_id"`
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
}

// handleMemorySearch 处理搜索记忆请求
func (h *WSHandler) handleMemorySearch(conn *websocket.Conn, payload json.RawMessage) {
	var req MemorySearchPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid memory_search payload")
		return
	}

	if req.UserID == "" || req.Query == "" {
		h.sendError(conn, "user_id and query are required")
		return
	}

	if h.memories == nil {
		h.sendError(conn, "memory manager not initialized")
		return
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	go func() {
		h.memories.LoadUserMemories(req.UserID)
		retriever := memory.NewRetriever(h.memories, h.config.Security.WorkingDir)
		results := retriever.Retrieve(req.UserID, req.Query, limit)

		log.Printf("[WS] Found %d memories for query: %s", len(results), req.Query)

		h.sendJSON(conn, map[string]interface{}{
			"type": "memories_search_result",
			"payload": map[string]interface{}{
				"user_id":  req.UserID,
				"query":    req.Query,
				"memories": results,
			},
		})
	}()
}

// ExtractMemoriesPayload 提取记忆载荷
type ExtractMemoriesPayload struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
}

// handleExtractMemories 处理用户手动触发的记忆提取请求
func (h *WSHandler) handleExtractMemories(conn *websocket.Conn, payload json.RawMessage) {
	var req ExtractMemoriesPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid extract_memories payload")
		return
	}

	if req.UserID == "" || req.SessionID == "" {
		h.sendError(conn, "user_id and session_id are required")
		return
	}

	if h.memories == nil {
		h.sendError(conn, "memory manager not initialized")
		return
	}

	sess := h.sessions.Get(req.SessionID)
	if sess == nil {
		h.sendError(conn, "session not found")
		return
	}

	go func() {
		log.Printf("[WS] User requested memory extraction for session %s", req.SessionID)
		h.extractMemoriesFromSession(req.UserID, sess)

		h.sendJSON(conn, map[string]interface{}{
			"type": "memories_extracted",
			"payload": map[string]interface{}{
				"user_id":    req.UserID,
				"session_id": req.SessionID,
				"success":    true,
			},
		})
	}()
}

// ========== Profile 相关处理 ==========

// ProfileGetPayload 获取画像载荷
type ProfileGetPayload struct {
	UserID string `json:"user_id"`
}

// handleProfileGet 处理获取画像请求
func (h *WSHandler) handleProfileGet(conn *websocket.Conn, payload json.RawMessage) {
	var req ProfileGetPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid profile_get payload")
		return
	}

	if req.UserID == "" {
		h.sendError(conn, "user_id is required")
		return
	}

	if h.profiles == nil {
		h.sendError(conn, "profile manager not initialized")
		return
	}

	go func() {
		p := h.profiles.GetOrCreate(req.UserID)

		log.Printf("[WS] Got profile for user %s", req.UserID)

		h.sendJSON(conn, map[string]interface{}{
			"type": "profile_data",
			"payload": map[string]interface{}{
				"user_id": req.UserID,
				"profile": p,
			},
		})
	}()
}

// ProfileUpdatePayload 更新画像载荷
type ProfileUpdatePayload struct {
	UserID        string            `json:"user_id"`
	Languages     []string          `json:"languages,omitempty"`
	Frameworks    []string          `json:"frameworks,omitempty"`
	ResponseStyle string            `json:"response_style,omitempty"`
	Language      string            `json:"language,omitempty"`
	CodingStyle   map[string]string `json:"coding_style,omitempty"`
}

// handleProfileUpdate 处理更新画像请求
func (h *WSHandler) handleProfileUpdate(conn *websocket.Conn, payload json.RawMessage) {
	var req ProfileUpdatePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(conn, "invalid profile_update payload")
		return
	}

	if req.UserID == "" {
		h.sendError(conn, "user_id is required")
		return
	}

	if h.profiles == nil {
		h.sendError(conn, "profile manager not initialized")
		return
	}

	go func() {
		updates := make(map[string]interface{})
		if len(req.Languages) > 0 {
			updates["languages"] = req.Languages
		}
		if len(req.Frameworks) > 0 {
			updates["frameworks"] = req.Frameworks
		}
		if req.ResponseStyle != "" {
			updates["response_style"] = req.ResponseStyle
		}
		if req.Language != "" {
			updates["language"] = req.Language
		}
		if len(req.CodingStyle) > 0 {
			updates["coding_style"] = req.CodingStyle
		}

		if err := h.profiles.Update(req.UserID, updates); err != nil {
			h.sendError(conn, "failed to update profile: "+err.Error())
			return
		}

		p := h.profiles.GetOrCreate(req.UserID)

		log.Printf("[WS] Updated profile for user %s", req.UserID)

		h.sendJSON(conn, map[string]interface{}{
			"type": "profile_updated",
			"payload": map[string]interface{}{
				"user_id": req.UserID,
				"profile": p,
				"success": true,
			},
		})
	}()
}
