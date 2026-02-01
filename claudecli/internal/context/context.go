package context

import (
	"sync"
	"time"

	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// Token 估算常量
const (
	CharsPerToken      = 3.5    // 英文约4，中文约2
	MaxContextTokens   = 180000 // Claude 3.5 的上下文窗口
	ReserveTokens      = 32000  // 保留给输出
	CodeBlockMaxLines  = 50     // 代码块最大保留行数
	ToolOutputMaxChars = 2000   // 工具输出最大字符数
	FileContentMaxChars = 1500  // 文件内容最大字符数
	SummaryTargetRatio = 0.3    // 摘要目标压缩比
)

// Config 上下文配置
type Config struct {
	MaxTokens                    int     `json:"max_tokens"`
	ReserveTokens                int     `json:"reserve_tokens"`
	SummarizeThreshold           float64 `json:"summarize_threshold"`
	KeepRecentMessages           int     `json:"keep_recent_messages"`
	EnableAISummary              bool    `json:"enable_ai_summary"`
	CodeBlockMaxLines            int     `json:"code_block_max_lines"`
	ToolOutputMaxChars           int     `json:"tool_output_max_chars"`
	EnableIncrementalCompression bool    `json:"enable_incremental_compression"`
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		MaxTokens:                    MaxContextTokens,
		ReserveTokens:                ReserveTokens,
		SummarizeThreshold:           0.7,
		KeepRecentMessages:           10,
		EnableAISummary:              false,
		CodeBlockMaxLines:            CodeBlockMaxLines,
		ToolOutputMaxChars:           ToolOutputMaxChars,
		EnableIncrementalCompression: true,
	}
}

// TokenUsage API token 使用统计
type TokenUsage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheReadTokens     int `json:"cache_read_tokens,omitempty"`
	CacheCreationTokens int `json:"cache_creation_tokens,omitempty"`
	ThinkingTokens      int `json:"thinking_tokens,omitempty"`
}

// Total 返回总 token 数
func (u TokenUsage) Total() int {
	return u.InputTokens + u.OutputTokens + u.CacheReadTokens + u.CacheCreationTokens + u.ThinkingTokens
}

// ConversationTurn 对话轮次
type ConversationTurn struct {
	User           types.Message `json:"user"`
	Assistant      types.Message `json:"assistant"`
	Timestamp      int64         `json:"timestamp"`
	TokenEstimate  int           `json:"token_estimate"`
	OriginalTokens int           `json:"original_tokens"`
	Summarized     bool          `json:"summarized,omitempty"`
	Summary        string        `json:"summary,omitempty"`
	Compressed     bool          `json:"compressed,omitempty"`
	APIUsage       *TokenUsage   `json:"api_usage,omitempty"`
}

// Stats 上下文统计
type Stats struct {
	TotalMessages      int     `json:"total_messages"`
	EstimatedTokens    int     `json:"estimated_tokens"`
	SummarizedMessages int     `json:"summarized_messages"`
	CompressionRatio   float64 `json:"compression_ratio"`
	SavedTokens        int     `json:"saved_tokens"`
	CompressionCount   int     `json:"compression_count"`
	// 新增字段用于状态栏显示
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
	ContextUsed  int `json:"context_used"`
	ContextMax   int `json:"context_max"`
}

// Manager 上下文管理器
type Manager struct {
	config           Config
	turns            []ConversationTurn
	systemPrompt     string
	compressionCount int
	savedTokens      int
	mu               sync.RWMutex
}

// NewManager 创建上下文管理器
func NewManager(config *Config) *Manager {
	cfg := DefaultConfig()
	if config != nil {
		cfg = *config
	}
	return &Manager{
		config: cfg,
		turns:  make([]ConversationTurn, 0),
	}
}

// SetSystemPrompt 设置系统提示
func (m *Manager) SetSystemPrompt(prompt string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.systemPrompt = prompt
}

// AddTurn 添加对话轮次
func (m *Manager) AddTurn(user, assistant types.Message, usage *TokenUsage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	originalUserTokens := EstimateMessageTokens(user)
	originalAssistantTokens := EstimateMessageTokens(assistant)
	originalTokens := originalUserTokens + originalAssistantTokens

	// 应用增量压缩
	processedUser := user
	processedAssistant := assistant
	compressed := false

	if m.config.EnableIncrementalCompression {
		processedUser = CompressMessage(user, m.config.ToolOutputMaxChars)
		processedAssistant = CompressMessage(assistant, m.config.ToolOutputMaxChars)

		compressedTokens := EstimateMessageTokens(processedUser) + EstimateMessageTokens(processedAssistant)
		if compressedTokens < originalTokens {
			compressed = true
			m.savedTokens += originalTokens - compressedTokens
		}
	}

	tokenEstimate := EstimateMessageTokens(processedUser) + EstimateMessageTokens(processedAssistant)

	m.turns = append(m.turns, ConversationTurn{
		User:           processedUser,
		Assistant:      processedAssistant,
		Timestamp:      time.Now().UnixMilli(),
		TokenEstimate:  tokenEstimate,
		OriginalTokens: originalTokens,
		Compressed:     compressed,
		APIUsage:       usage,
	})

	// 检查是否需要压缩
	m.maybeCompress()
}

// GetMessages 获取当前上下文的消息
func (m *Manager) GetMessages() []types.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	messages := make([]types.Message, 0)

	// 添加摘要消息（如果有）
	var summarizedTurns []ConversationTurn
	for _, t := range m.turns {
		if t.Summarized {
			summarizedTurns = append(summarizedTurns, t)
		}
	}

	if len(summarizedTurns) > 0 {
		summary := CreateSummary(summarizedTurns)
		messages = append(messages, types.Message{
			Role:    "user",
			Content: []interface{}{types.TextBlock{Type: "text", Text: summary}},
		})
		messages = append(messages, types.Message{
			Role:    "assistant",
			Content: []interface{}{types.TextBlock{Type: "text", Text: "I understand. I'll keep this context in mind."}},
		})
	}

	// 添加非摘要的消息
	for _, turn := range m.turns {
		if !turn.Summarized {
			messages = append(messages, turn.User)
			messages = append(messages, turn.Assistant)
		}
	}

	return messages
}

// GetUsedTokens 获取已使用的 token 数
func (m *Manager) GetUsedTokens() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := EstimateTokens(m.systemPrompt)

	for _, turn := range m.turns {
		if turn.Summarized && turn.Summary != "" {
			total += EstimateTokens(turn.Summary)
		} else if turn.APIUsage != nil {
			total += turn.APIUsage.Total()
		} else {
			total += turn.TokenEstimate
		}
	}

	return total
}

// GetAvailableTokens 获取可用的 token 数
func (m *Manager) GetAvailableTokens() int {
	used := m.GetUsedTokens()
	return m.config.MaxTokens - m.config.ReserveTokens - used
}

// IsNearLimit 检查是否接近上下文限制
func (m *Manager) IsNearLimit() bool {
	used := m.GetUsedTokens()
	total := m.config.MaxTokens - m.config.ReserveTokens
	return float64(used)/float64(total) >= m.config.SummarizeThreshold
}

// maybeCompress 检查并执行压缩
func (m *Manager) maybeCompress() {
	threshold := float64(m.config.MaxTokens) * m.config.SummarizeThreshold
	used := float64(m.getUsedTokensUnsafe())

	if used < threshold {
		return
	}

	// 标记旧消息为需要摘要
	recentCount := m.config.KeepRecentMessages
	if len(m.turns) <= recentCount {
		return
	}

	toSummarize := m.turns[:len(m.turns)-recentCount]
	if len(toSummarize) == 0 {
		return
	}

	beforeTokens := 0
	for _, t := range toSummarize {
		beforeTokens += t.TokenEstimate
	}

	// 生成摘要
	summary := CreateSummary(toSummarize)

	// 标记为已摘要
	for i := range toSummarize {
		if !m.turns[i].Summarized {
			m.turns[i].Summarized = true
			m.turns[i].Summary = summary
		}
	}

	afterTokens := EstimateTokens(summary)
	m.savedTokens += beforeTokens - afterTokens
	m.compressionCount++
}

// getUsedTokensUnsafe 获取已使用的 token 数（不加锁）
func (m *Manager) getUsedTokensUnsafe() int {
	total := EstimateTokens(m.systemPrompt)
	for _, turn := range m.turns {
		if turn.Summarized && turn.Summary != "" {
			total += EstimateTokens(turn.Summary)
		} else if turn.APIUsage != nil {
			total += turn.APIUsage.Total()
		} else {
			total += turn.TokenEstimate
		}
	}
	return total
}

// Compact 强制压缩
func (m *Manager) Compact() {
	m.mu.Lock()
	defer m.mu.Unlock()

	recentCount := m.config.KeepRecentMessages
	if len(m.turns) <= recentCount {
		return
	}

	toSummarize := m.turns[:len(m.turns)-recentCount]
	if len(toSummarize) == 0 {
		return
	}

	beforeTokens := 0
	for _, t := range toSummarize {
		beforeTokens += t.TokenEstimate
	}

	summary := CreateSummary(toSummarize)

	for i := range toSummarize {
		if !m.turns[i].Summarized {
			m.turns[i].Summarized = true
			m.turns[i].Summary = summary
		}
	}

	afterTokens := EstimateTokens(summary)
	m.savedTokens += beforeTokens - afterTokens
	m.compressionCount++
}

// GetStats 获取统计信息
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summarized := 0
	originalTokens := 0
	inputTokens := 0
	outputTokens := 0

	for _, t := range m.turns {
		if t.Summarized {
			summarized++
		}
		originalTokens += t.OriginalTokens

		// 统计输入输出 token
		if t.APIUsage != nil {
			inputTokens += t.APIUsage.InputTokens
			outputTokens += t.APIUsage.OutputTokens
		}
	}

	currentTokens := m.getUsedTokensUnsafe()
	ratio := 1.0
	if originalTokens > 0 {
		ratio = float64(currentTokens) / float64(originalTokens)
	}

	contextMax := m.config.MaxTokens - m.config.ReserveTokens

	return Stats{
		TotalMessages:      len(m.turns) * 2,
		EstimatedTokens:    currentTokens,
		SummarizedMessages: summarized * 2,
		CompressionRatio:   ratio,
		SavedTokens:        m.savedTokens,
		CompressionCount:   m.compressionCount,
		InputTokens:        inputTokens,
		OutputTokens:       outputTokens,
		TotalTokens:        inputTokens + outputTokens,
		ContextUsed:        currentTokens,
		ContextMax:         contextMax,
	}
}

// Clear 清除所有历史
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.turns = make([]ConversationTurn, 0)
	m.compressionCount = 0
	m.savedTokens = 0
}

// Export 导出为可序列化格式
func (m *Manager) Export() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"system_prompt":     m.systemPrompt,
		"turns":             m.turns,
		"config":            m.config,
		"compression_count": m.compressionCount,
		"saved_tokens":      m.savedTokens,
	}
}
