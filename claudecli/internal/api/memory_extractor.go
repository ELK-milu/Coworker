package api

import (
	"log"

	"github.com/QuantumNous/new-api/claudecli/internal/memory"
	"github.com/QuantumNous/new-api/claudecli/internal/session"
)

// extractMemoriesFromSession 从会话中提取记忆
// 在以下场景调用：
// 1. WS 断开连接时
// 2. 上下文压缩时
// 3. 用户主动要求时
func (h *WSHandler) extractMemoriesFromSession(userID string, sess *session.Session) {
	if h.memories == nil {
		log.Printf("[Memory] Memory manager not initialized, skipping extraction")
		return
	}

	if sess == nil {
		log.Printf("[Memory] Session is nil, skipping extraction")
		return
	}

	messages := sess.GetMessages()
	if len(messages) < 2 {
		log.Printf("[Memory] Not enough messages to extract (%d), skipping", len(messages))
		return
	}

	log.Printf("[Memory] Starting memory extraction for user %s, session %s (%d messages)",
		userID, sess.ID, len(messages))

	// 使用 Extractor 提取记忆
	extractor := memory.NewExtractor(h.memories)
	extracted := extractor.ExtractFromConversation(userID, sess.ID, messages)

	if len(extracted) == 0 {
		log.Printf("[Memory] No memories extracted from session %s", sess.ID)
		return
	}

	// 保存提取的记忆
	savedCount := 0
	for _, mem := range extracted {
		if _, err := h.memories.Create(userID, mem); err != nil {
			log.Printf("[Memory] Failed to save memory: %v", err)
		} else {
			savedCount++
		}
	}

	log.Printf("[Memory] Extracted and saved %d memories from session %s", savedCount, sess.ID)
}

// extractMemoriesAsync 异步提取记忆
func (h *WSHandler) extractMemoriesAsync(userID string, sess *session.Session) {
	go h.extractMemoriesFromSession(userID, sess)
}
