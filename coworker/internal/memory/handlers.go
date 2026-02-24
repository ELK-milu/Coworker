package memory

import (
	"fmt"
	"log"

	"github.com/QuantumNous/new-api/coworker/internal/eventbus"
	"github.com/QuantumNous/new-api/coworker/pkg/types"
)

// SessionAccessor 会话访问接口（避免直接依赖 session 包）
type SessionAccessor interface {
	GetMessages() []types.Message
	NeedsMemoryExtraction() bool
	MarkMemoryExtracted()
}

// MemoryHandlers 记忆系统事件处理器
// 注册到 EventBus，统一处理所有记忆提取场景
// 核心策略：每个上下文窗口只维护一条记忆，通过 WindowID 做精确匹配
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

// makeWindowID 生成窗口 ID
func makeWindowID(sessionID string, windowIndex int) string {
	return fmt.Sprintf("%s-w%d", sessionID, windowIndex)
}

// getWindowIndex 从事件数据中提取窗口索引
func getWindowIndex(event eventbus.Event) int {
	if idx, ok := event.Data["window_index"].(int); ok {
		return idx
	}
	return 0
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

	messages, ok := event.Data["messages"].([]types.Message)
	if !ok || len(messages) < 4 {
		log.Printf("[MemoryHandlers] BeforeCompact: not enough messages for summary, skipping")
		return
	}

	windowID := makeWindowID(sessionID, getWindowIndex(event))
	log.Printf("[MemoryHandlers] BeforeCompact: extracting window summary for %s (%d messages)",
		windowID, len(messages))

	extractor := NewExtractor(h.manager)
	if h.aiClient != nil {
		extractor.SetAIClient(h.aiClient)
	}

	summary := extractor.ExtractSessionSummary(userID, sessionID, messages)
	if summary == nil || summary.Content == "trivial" {
		log.Printf("[MemoryHandlers] BeforeCompact: trivial or empty, skipping")
		return
	}

	summary.WindowID = windowID
	_, isNew, err := h.manager.UpsertByWindowID(userID, summary)
	if err != nil {
		log.Printf("[MemoryHandlers] BeforeCompact: failed to save: %v", err)
	} else if isNew {
		log.Printf("[MemoryHandlers] BeforeCompact: window memory created for %s", windowID)
	} else {
		log.Printf("[MemoryHandlers] BeforeCompact: window memory updated for %s", windowID)
	}
}

// HandleTurnCompleted 对话轮次结束后更新当前窗口记忆（异步）
// 每个窗口只维护一条记忆，每轮对话后替换其内容
func (h *MemoryHandlers) HandleTurnCompleted(event eventbus.Event) {
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
		log.Printf("[MemoryHandlers] TurnCompleted: session not available")
		return
	}

	if !sess.NeedsMemoryExtraction() {
		return
	}

	messages := sess.GetMessages()
	if len(messages) < 4 {
		return
	}

	windowID := makeWindowID(sessionID, getWindowIndex(event))
	log.Printf("[MemoryHandlers] TurnCompleted: updating window memory %s (%d messages)",
		windowID, len(messages))

	extractor := NewExtractor(h.manager)
	if h.aiClient != nil {
		extractor.SetAIClient(h.aiClient)
	}

	summary := extractor.ExtractSessionSummary(userID, sessionID, messages)
	if summary == nil || summary.Content == "trivial" {
		sess.MarkMemoryExtracted()
		return
	}

	summary.WindowID = windowID
	_, isNew, err := h.manager.UpsertByWindowID(userID, summary)
	if err != nil {
		log.Printf("[MemoryHandlers] TurnCompleted: failed to save: %v", err)
	} else if isNew {
		log.Printf("[MemoryHandlers] TurnCompleted: window memory created for %s", windowID)
	} else {
		log.Printf("[MemoryHandlers] TurnCompleted: window memory updated for %s", windowID)
	}

	sess.MarkMemoryExtracted()
}

// HandleSessionEnd 会话结束时最终更新窗口记忆（异步）
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

	windowID := makeWindowID(sessionID, getWindowIndex(event))
	log.Printf("[MemoryHandlers] SessionEnd: finalizing window memory %s", windowID)

	extractor := NewExtractor(h.manager)
	if h.aiClient != nil {
		extractor.SetAIClient(h.aiClient)
	}

	summary := extractor.ExtractSessionSummary(userID, sessionID, messages)
	if summary == nil || summary.Content == "trivial" {
		return
	}

	summary.WindowID = windowID
	if _, _, err := h.manager.UpsertByWindowID(userID, summary); err != nil {
		log.Printf("[MemoryHandlers] SessionEnd: failed to save: %v", err)
	}

	sess.MarkMemoryExtracted()
}
