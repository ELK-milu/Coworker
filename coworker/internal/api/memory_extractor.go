package api

import (
	"log"

	"github.com/QuantumNous/new-api/coworker/internal/memory"
	"github.com/QuantumNous/new-api/coworker/internal/session"
)

// extractMemoriesFromSession 从会话中提取记忆
// 在以下场景调用：
// 1. WS 断开连接时
// 2. 上下文压缩时
// 3. 用户主动要求时
// 4. 每轮对话结束后（增量提取）
func (h *WSHandler) extractMemoriesFromSession(userID string, sess *session.Session) {
	if h.memories == nil {
		log.Printf("[Memory] Memory manager not initialized, skipping extraction")
		return
	}

	if sess == nil {
		log.Printf("[Memory] Session is nil, skipping extraction")
		return
	}

	// 检查是否需要提取（避免重复提取）
	if !sess.NeedsMemoryExtraction() {
		log.Printf("[Memory] Session %s has no new content since last extraction, skipping", sess.ID)
		return
	}

	messages := sess.GetMessages()
	if len(messages) < 2 {
		log.Printf("[Memory] Not enough messages to extract (%d), skipping", len(messages))
		return
	}

	log.Printf("[Memory] Starting memory extraction for user %s, session %s (%d messages)",
		userID, sess.ID, len(messages))

	// 使用 Extractor 提取记忆（注入 AI 客户端）
	extractor := memory.NewExtractor(h.memories)
	if h.client != nil {
		extractor.SetAIClient(h.client)
	}
	extracted := extractor.ExtractFromConversation(userID, sess.ID, messages)

	if len(extracted) == 0 {
		log.Printf("[Memory] No memories extracted from session %s", sess.ID)
		// 即使没有提取到记忆，也标记为已提取，避免重复尝试
		sess.MarkMemoryExtracted()
		return
	}

	// 保存提取的记忆（使用 CreateOrMerge 避免重复）
	newCount := 0
	mergedCount := 0
	for _, mem := range extracted {
		_, isNew, err := h.memories.CreateOrMerge(userID, mem)
		if err != nil {
			log.Printf("[Memory] Failed to save memory: %v", err)
		} else if isNew {
			newCount++
		} else {
			mergedCount++
		}
	}

	// 标记会话已提取
	sess.MarkMemoryExtracted()

	log.Printf("[Memory] Extraction complete for session %s: %d new, %d merged",
		sess.ID, newCount, mergedCount)
}

// extractMemoriesAsync 异步提取记忆
func (h *WSHandler) extractMemoriesAsync(userID string, sess *session.Session) {
	go h.extractMemoriesFromSession(userID, sess)
}
