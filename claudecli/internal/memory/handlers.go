package memory

import (
	"log"

	"github.com/QuantumNous/new-api/claudecli/internal/eventbus"
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// SessionAccessor 会话访问接口（避免直接依赖 session 包）
type SessionAccessor interface {
	GetMessages() []types.Message
	NeedsMemoryExtraction() bool
	MarkMemoryExtracted()
}

// MemoryHandlers 记忆系统事件处理器
// 注册到 EventBus，统一处理所有记忆提取场景
type MemoryHandlers struct {
	manager  *Manager
	aiClient AIClient
}

// NewMemoryHandlers 创建记忆事件处理器
func NewMemoryHandlers(manager *Manager, aiClient AIClient) *MemoryHandlers {
	return &MemoryHandlers{
		manager:  manager,
		aiClient: aiClient,
	}
}

// Register 注册所有事件处理器到 EventBus
func (h *MemoryHandlers) Register(bus *eventbus.Bus) {
	bus.OnSync(eventbus.EventBeforeCompact, h.HandleBeforeCompact)
	bus.OnAsync(eventbus.EventTurnCompleted, h.HandleTurnCompleted)
	bus.OnAsync(eventbus.EventSessionEnd, h.HandleSessionEnd)
}

// HandleBeforeCompact 压缩前提取上下文窗口总结（同步）
// 必须在压缩前完成，否则详细对话内容会丢失
func (h *MemoryHandlers) HandleBeforeCompact(event eventbus.Event) {
	if h.manager == nil {
		return
	}

	userID := event.UserID
	sessionID := event.SessionID
	if userID == "" || sessionID == "" {
		return
	}

	// 从事件数据中获取即将被压缩的消息
	messages, ok := event.Data["messages"].([]types.Message)
	if !ok || len(messages) < 4 {
		log.Printf("[MemoryHandlers] BeforeCompact: not enough messages for summary, skipping")
		return
	}

	log.Printf("[MemoryHandlers] BeforeCompact: extracting window summary for session %s (%d messages)",
		sessionID, len(messages))

	extractor := NewExtractor(h.manager)
	if h.aiClient != nil {
		extractor.SetAIClient(h.aiClient)
	}

	summary := extractor.ExtractSessionSummary(userID, sessionID, messages)
	if summary == nil {
		log.Printf("[MemoryHandlers] BeforeCompact: no summary extracted")
		return
	}

	// 检查是否为 trivial 对话
	if summary.Content == "trivial" {
		log.Printf("[MemoryHandlers] BeforeCompact: trivial conversation, skipping")
		return
	}

	_, isNew, err := h.manager.CreateOrMerge(userID, summary)
	if err != nil {
		log.Printf("[MemoryHandlers] BeforeCompact: failed to save summary: %v", err)
	} else if isNew {
		log.Printf("[MemoryHandlers] BeforeCompact: window summary saved for session %s", sessionID)
	} else {
		log.Printf("[MemoryHandlers] BeforeCompact: window summary merged for session %s", sessionID)
	}
}

// HandleTurnCompleted 对话轮次结束后提取离散事实（异步）
func (h *MemoryHandlers) HandleTurnCompleted(event eventbus.Event) {
	if h.manager == nil {
		return
	}

	userID := event.UserID
	sessionID := event.SessionID
	if userID == "" || sessionID == "" {
		return
	}

	// 从事件数据中获取 session accessor
	sess, ok := event.Data["session"].(SessionAccessor)
	if !ok || sess == nil {
		log.Printf("[MemoryHandlers] TurnCompleted: session not available")
		return
	}

	// 检查是否需要提取（避免重复）
	if !sess.NeedsMemoryExtraction() {
		return
	}

	messages := sess.GetMessages()
	if len(messages) < 2 {
		return
	}

	log.Printf("[MemoryHandlers] TurnCompleted: extracting facts for session %s (%d messages)",
		sessionID, len(messages))

	extractor := NewExtractor(h.manager)
	if h.aiClient != nil {
		extractor.SetAIClient(h.aiClient)
	}

	extracted := extractor.ExtractFromConversation(userID, sessionID, messages)

	newCount := 0
	mergedCount := 0
	for _, mem := range extracted {
		_, isNew, err := h.manager.CreateOrMerge(userID, mem)
		if err != nil {
			log.Printf("[MemoryHandlers] TurnCompleted: failed to save: %v", err)
		} else if isNew {
			newCount++
		} else {
			mergedCount++
		}
	}

	sess.MarkMemoryExtracted()

	if newCount > 0 || mergedCount > 0 {
		log.Printf("[MemoryHandlers] TurnCompleted: session %s — %d new, %d merged",
			sessionID, newCount, mergedCount)
	}
}

// HandleSessionEnd 会话结束时提取最后一个窗口的总结（异步）
func (h *MemoryHandlers) HandleSessionEnd(event eventbus.Event) {
	if h.manager == nil {
		return
	}

	userID := event.UserID
	sessionID := event.SessionID
	if userID == "" || sessionID == "" {
		return
	}

	sess, ok := event.Data["session"].(SessionAccessor)
	if !ok || sess == nil {
		return
	}

	messages := sess.GetMessages()
	if len(messages) < 4 {
		return
	}

	log.Printf("[MemoryHandlers] SessionEnd: extracting final summary for session %s", sessionID)

	extractor := NewExtractor(h.manager)
	if h.aiClient != nil {
		extractor.SetAIClient(h.aiClient)
	}

	// 1. 提取窗口总结
	summary := extractor.ExtractSessionSummary(userID, sessionID, messages)
	if summary != nil && summary.Content != "trivial" {
		if _, _, err := h.manager.CreateOrMerge(userID, summary); err != nil {
			log.Printf("[MemoryHandlers] SessionEnd: failed to save summary: %v", err)
		}
	}

	// 2. 提取离散事实（如果还有未提取的）
	if sess.NeedsMemoryExtraction() {
		extracted := extractor.ExtractFromConversation(userID, sessionID, messages)
		for _, mem := range extracted {
			h.manager.CreateOrMerge(userID, mem)
		}
		sess.MarkMemoryExtracted()
	}
}
