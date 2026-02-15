package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// AIClient AI 客户端接口（用于记忆提取）
type AIClient interface {
	CreateSimpleMessage(ctx context.Context, prompt string, maxTokens int64) (string, error)
}

// Extractor 记忆提取器
type Extractor struct {
	manager  *Manager
	aiClient AIClient // 可选的 AI 客户端
}

// NewExtractor 创建提取器
func NewExtractor(manager *Manager) *Extractor {
	return &Extractor{manager: manager}
}

// SetAIClient 设置 AI 客户端（启用 AI 提取）
func (e *Extractor) SetAIClient(client AIClient) {
	e.aiClient = client
}

// ExtractFromConversation 从对话中提取记忆
// 优先使用 AI 提取，降级为关键词提取
func (e *Extractor) ExtractFromConversation(userID, sessionID string, messages []types.Message) []*Memory {
	// 只提取有实质内容的消息
	if len(messages) < 2 {
		return nil
	}

	// 优先使用 AI 提取
	if e.aiClient != nil {
		extracted := e.extractWithAI(userID, sessionID, messages)
		if len(extracted) > 0 {
			return extracted
		}
		log.Printf("[MemoryExtractor] AI extraction returned empty, falling back to keyword extraction")
	}

	// 降级：关键词提取
	return e.extractWithKeywords(userID, sessionID, messages)
}

// extractWithAI 使用 AI 从对话中提取记忆
func (e *Extractor) extractWithAI(userID, sessionID string, messages []types.Message) []*Memory {
	// 构建对话摘要（限制 token 消耗）
	conversationText := e.buildConversationText(messages, 4000)
	if conversationText == "" {
		return nil
	}

	// 检测对话语言，指导 AI 用相同语言提取记忆
	langHint := detectLanguageHint(conversationText)

	prompt := fmt.Sprintf(`%s

%s

<conversation>
%s
</conversation>

Respond with ONLY a JSON array. No explanation, no markdown fences.`, GetExtractionPrompt(), langHint, conversationText)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := e.aiClient.CreateSimpleMessage(ctx, prompt, 2000)
	if err != nil {
		log.Printf("[MemoryExtractor] AI extraction failed: %v", err)
		return nil
	}

	// 解析 JSON 响应
	return e.parseAIResponse(response, sessionID)
}

// extractedItem AI 提取结果的 JSON 结构
type extractedItem struct {
	Tags    []string `json:"tags"`
	Content string   `json:"content"`
	Summary string   `json:"summary"`
	Weight  float64  `json:"weight"`
}

// parseAIResponse 解析 AI 返回的 JSON
func (e *Extractor) parseAIResponse(response, sessionID string) []*Memory {
	// 清理响应：去除 markdown 代码块标记
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```") {
		// 去除 ```json 和 ```
		lines := strings.Split(response, "\n")
		if len(lines) > 2 {
			response = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var items []extractedItem
	if err := json.Unmarshal([]byte(response), &items); err != nil {
		log.Printf("[MemoryExtractor] Failed to parse AI response: %v, response: %.200s", err, response)
		return nil
	}

	if len(items) == 0 {
		return nil
	}

	// 限制最多 10 条
	if len(items) > 10 {
		items = items[:10]
	}

	var memories []*Memory
	for _, item := range items {
		if item.Content == "" || len(item.Tags) == 0 {
			continue
		}

		weight := item.Weight
		if weight <= 0 || weight > 1 {
			weight = 0.5
		}

		memories = append(memories, &Memory{
			Tags:      normalizeTags(item.Tags),
			Content:   item.Content,
			Summary:   item.Summary,
			Source:    "ai_extracted",
			SessionID: sessionID,
			Weight:    weight,
		})
	}

	log.Printf("[MemoryExtractor] AI extracted %d memories", len(memories))
	return memories
}

// buildConversationText 构建对话文本（限制长度）
func (e *Extractor) buildConversationText(messages []types.Message, maxChars int) string {
	var sb strings.Builder

	// 收集所有消息文本
	texts := make([]string, 0, len(messages))
	for _, msg := range messages {
		text := getMessageText(msg)
		if text == "" {
			continue
		}

		role := "User"
		if msg.Role == "assistant" {
			role = "Assistant"
		}

		// 截断单条过长的消息
		if len(text) > 800 {
			text = text[:800] + "..."
		}

		texts = append(texts, fmt.Sprintf("[%s]: %s", role, text))
	}

	// 从最新的消息开始取，直到达到字符限制
	startIdx := len(texts)
	remaining := maxChars
	for i := len(texts) - 1; i >= 0; i-- {
		lineLen := len(texts[i]) + 1
		if remaining-lineLen < 0 {
			break
		}
		remaining -= lineLen
		startIdx = i
	}

	// 正序输出
	for i := startIdx; i < len(texts); i++ {
		sb.WriteString(texts[i])
		sb.WriteString("\n")
	}

	return sb.String()
}

// extractWithKeywords 关键词提取（降级方案）
func (e *Extractor) extractWithKeywords(userID, sessionID string, messages []types.Message) []*Memory {
	var extracted []*Memory

	// 扫描用户消息中的偏好表达
	for _, msg := range messages {
		if msg.Role != "user" {
			continue
		}
		text := getMessageText(msg)
		if text == "" {
			continue
		}

		// 检测偏好表达
		if pref := detectPreference(text); pref != "" {
			extracted = append(extracted, &Memory{
				Tags:      []string{"preference"},
				Content:   pref,
				Summary:   "User preference",
				Source:    "keyword_extracted",
				SessionID: sessionID,
				Weight:    0.7,
			})
		}
	}

	// 限制数量
	if len(extracted) > 5 {
		extracted = extracted[:5]
	}

	return extracted
}

// detectPreference 检测偏好表达
func detectPreference(text string) string {
	// 中文偏好表达
	cnPatterns := []string{
		"我习惯", "我喜欢用", "我偏好", "以后都用", "不要用", "别用",
		"我一般用", "我通常用", "请记住",
	}
	for _, p := range cnPatterns {
		if idx := strings.Index(text, p); idx >= 0 {
			// 提取偏好句子（从关键词开始，到句号或换行）
			rest := text[idx:]
			if end := strings.IndexAny(rest, "。\n"); end > 0 {
				return rest[:end]
			}
			if len(rest) > 200 {
				return rest[:200]
			}
			return rest
		}
	}

	// 英文偏好表达
	enPatterns := []string{
		"I prefer", "I always use", "Don't use", "Never use",
		"I like to", "Remember that", "Please remember",
	}
	lower := strings.ToLower(text)
	for _, p := range enPatterns {
		if idx := strings.Index(lower, strings.ToLower(p)); idx >= 0 {
			rest := text[idx:]
			if end := strings.IndexAny(rest, ".\n"); end > 0 {
				return rest[:end]
			}
			if len(rest) > 200 {
				return rest[:200]
			}
			return rest
		}
	}

	return ""
}

// normalizeTags 标准化标签（去重、小写、去空）
func normalizeTags(tags []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" && !seen[tag] {
			seen[tag] = true
			result = append(result, tag)
		}
	}
	return result
}

// getMessageText 获取消息文本内容
func getMessageText(msg types.Message) string {
	var parts []string
	for _, item := range msg.Content {
		if m, ok := item.(map[string]interface{}); ok {
			if text, ok := m["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		if tb, ok := item.(types.TextBlock); ok {
			parts = append(parts, tb.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// ExtractSessionSummary 提取上下文窗口级总结
// 将整个上下文窗口的对话总结为一条综合性记忆
// 触发时机：compaction 前、会话结束时
func (e *Extractor) ExtractSessionSummary(userID, sessionID string, messages []types.Message) *Memory {
	if e.aiClient == nil {
		log.Printf("[MemoryExtractor] AI client not available, skipping session summary")
		return nil
	}

	if len(messages) < 4 {
		log.Printf("[MemoryExtractor] Too few messages (%d) for session summary, skipping", len(messages))
		return nil
	}

	// 构建对话文本（窗口总结需要更多上下文，允许 8000 字符）
	conversationText := e.buildConversationText(messages, 8000)
	if conversationText == "" {
		return nil
	}

	// 检测对话主要语言
	langHint := detectLanguageHint(conversationText)

	prompt := fmt.Sprintf(`%s

%s

<conversation>
%s
</conversation>

Respond with ONLY a single paragraph. No JSON, no markdown, no explanation.`, GetSessionSummaryPrompt(), langHint, conversationText)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := e.aiClient.CreateSimpleMessage(ctx, prompt, 1000)
	if err != nil {
		log.Printf("[MemoryExtractor] Session summary extraction failed: %v", err)
		return nil
	}

	response = strings.TrimSpace(response)
	if response == "" || len(response) < 20 {
		log.Printf("[MemoryExtractor] Session summary too short, skipping")
		return nil
	}

	// 生成简短标题作为 summary
	summaryTitle := generateSummaryTitle(response)

	log.Printf("[MemoryExtractor] Session summary extracted (%d chars)", len(response))
	return &Memory{
		Tags:      []string{"session-summary"},
		Content:   response,
		Summary:   summaryTitle,
		Source:    "context_window_summary",
		SessionID: sessionID,
		Weight:    0.6,
	}
}

// detectLanguageHint 检测对话主要语言并返回语言提示
func detectLanguageHint(text string) string {
	// 统计中文字符占比
	cnCount := 0
	total := 0
	for _, r := range text {
		if r > 127 {
			cnCount++
		}
		total++
		if total > 500 {
			break // 只检测前 500 个字符
		}
	}

	if total > 0 && float64(cnCount)/float64(total) > 0.15 {
		return "IMPORTANT: The conversation is primarily in Chinese. Write the summary in Chinese (中文)."
	}
	return ""
}

// generateSummaryTitle 从总结内容生成简短标题
func generateSummaryTitle(content string) string {
	// 取第一句话作为标题
	for _, sep := range []string{"。", ". ", "，", ", "} {
		if idx := strings.Index(content, sep); idx > 0 && idx < 80 {
			return content[:idx]
		}
	}
	// 截断到 60 字符
	if len(content) > 60 {
		// 找到最近的空格或标点断开
		cutoff := 60
		for i := cutoff; i > 30; i-- {
			if content[i] == ' ' || content[i] == ',' {
				cutoff = i
				break
			}
		}
		return content[:cutoff] + "..."
	}
	return content
}

// GetSessionSummaryPrompt 获取上下文窗口总结提示词
func GetSessionSummaryPrompt() string {
	return `You are a session summarizer. Summarize the conversation below as a concise session log.

Focus on:
1. What the user was working on (project, feature, task)
2. What was accomplished (completed changes, files modified, problems solved)
3. Key decisions made and why
4. Problems encountered and their solutions
5. Any unfinished work or next steps mentioned

Rules:
- Write a single cohesive paragraph, 100-300 words
- Be specific: include file names, function names, technology choices
- Focus on OUTCOMES, not the back-and-forth process
- Skip greetings, small talk, and routine tool operations
- If the conversation was trivial (just greetings or simple questions), respond with just "trivial"`
}

// GetExtractionPrompt 获取 AI 提取提示词
func GetExtractionPrompt() string {
	return `You are a memory extraction assistant. Analyze the conversation below and extract information worth remembering long-term.

Extract ONLY these categories:
1. User preferences — coding style, tool choices, language preferences, workflow habits
2. Project context — project name, tech stack, architecture decisions, conventions
3. Important decisions — why a specific approach was chosen over alternatives
4. Error solutions — specific error + root cause + fix (only if clearly resolved)
5. User corrections — when user corrects AI behavior ("don't do X, do Y instead")

Rules:
- Be SPECIFIC and ACTIONABLE — "User prefers PostgreSQL for this project" not "User likes databases"
- One concept per item — don't bundle unrelated facts
- Skip routine code edits, temporary debugging, and generic knowledge
- Skip information that would be in project config files (package.json, go.mod, etc.)
- Return empty array [] if nothing significant was discussed

Output JSON array:
[
  {
    "tags": ["preference", "coding-style"],
    "content": "User prefers using functional components with hooks in React, avoids class components",
    "summary": "React: functional components + hooks preferred",
    "weight": 0.8
  }
]`
}
