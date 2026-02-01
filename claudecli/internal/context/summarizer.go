package context

import (
	"fmt"

	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// SummarySystemPrompt 摘要系统提示词
const SummarySystemPrompt = `Summarize this coding conversation in under 50 characters.
Capture the main task, key files, problems addressed, and current status.`

// CompactTrigger 压缩触发方式
type CompactTrigger string

const (
	CompactTriggerAuto   CompactTrigger = "auto"
	CompactTriggerManual CompactTrigger = "manual"
)

// GenerateSummaryPrompt 生成对话摘要的提示词
func GenerateSummaryPrompt(customInstructions string) string {
	basePrompt := `Your task is to create a detailed summary of the conversation so far.
This summary should capture technical details, code patterns, and architectural decisions.

Before providing your final summary, analyze each message to identify:
1. The user's explicit requests and intents
2. Your approach to addressing the requests
3. Key decisions, technical concepts and code patterns
4. Specific details: file names, code snippets, function signatures
5. Errors encountered and how they were fixed

Your summary should include:
1. Primary Request and Intent: User's explicit requests in detail
2. Key Technical Concepts: Technologies and frameworks discussed
3. Files and Code Sections: Files examined, modified, or created
4. Errors and Fixes: Errors encountered and solutions
5. Problem Solving: Problems solved and ongoing efforts
6. Current State: Current state and next steps`

	if customInstructions != "" {
		return basePrompt + "\n\nAdditional instructions:\n" + customInstructions
	}
	return basePrompt
}

// CreateCompactBoundaryMarker 创建压缩边界标记
func CreateCompactBoundaryMarker(trigger CompactTrigger, preTokens int) types.Message {
	text := fmt.Sprintf("--- Conversation Compacted (%s) ---\nPrevious messages were summarized to save %d tokens.", trigger, preTokens)
	return types.Message{
		Role: "user",
		Content: []interface{}{
			types.TextBlock{Type: "text", Text: text},
		},
	}
}

// FormatSummaryMessage 格式化摘要消息
func FormatSummaryMessage(summary string, microcompact bool) string {
	if microcompact {
		return summary
	}
	return fmt.Sprintf("<conversation-summary>\n%s\n</conversation-summary>", summary)
}
