package context

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/coworker/pkg/types"
)

// CreateSummary 创建对话摘要
func CreateSummary(turns []ConversationTurn) string {
	parts := []string{"=== Previous Conversation Summary ===\n"}

	for _, turn := range turns {
		userContent := extractMessageCore(turn.User)
		assistantContent := extractMessageCore(turn.Assistant)

		// 提取关键信息
		userSummary := truncateString(userContent, 300)
		assistantSummary := truncateString(assistantContent, 400)

		// 提取文件引用
		fileRefs := extractFileReferences(userContent + " " + assistantContent)

		timestamp := time.UnixMilli(turn.Timestamp).Format("15:04:05")

		parts = append(parts, fmt.Sprintf("[%s]", timestamp))
		parts = append(parts, fmt.Sprintf("User: %s", userSummary))
		parts = append(parts, fmt.Sprintf("Assistant: %s", assistantSummary))

		if len(fileRefs) > 0 {
			maxRefs := 5
			if len(fileRefs) < maxRefs {
				maxRefs = len(fileRefs)
			}
			parts = append(parts, fmt.Sprintf("Files: %s", strings.Join(fileRefs[:maxRefs], ", ")))
		}

		parts = append(parts, "") // 空行分隔
	}

	parts = append(parts, "=== End of Summary ===\n")
	return strings.Join(parts, "\n")
}

// extractMessageCore 提取消息的核心内容
func extractMessageCore(msg types.Message) string {
	if msg.Content == nil || len(msg.Content) == 0 {
		return ""
	}

	var parts []string
	for _, block := range msg.Content {
		switch b := block.(type) {
		case types.TextBlock:
			parts = append(parts, b.Text)
		case types.ToolUseBlock:
			inputBytes, _ := json.Marshal(b.Input)
			inputStr := string(inputBytes)
			if len(inputStr) > 200 {
				inputStr = inputStr[:200]
			}
			parts = append(parts, fmt.Sprintf("[Tool: %s]\nInput: %s", b.Name, inputStr))
		case types.ToolResultBlock:
			compressed := CompressToolOutput(b.Content, 300)
			parts = append(parts, fmt.Sprintf("[Result: %s]\n%s", b.ToolUseID, compressed))
		}
	}

	return strings.Join(parts, "\n\n")
}

// extractFileReferences 提取文件路径引用
func extractFileReferences(text string) []string {
	re := regexp.MustCompile(`(?:/[\w\-_.]+)+\.\w+`)
	matches := re.FindAllString(text, -1)

	if len(matches) == 0 {
		return nil
	}

	// 去重
	seen := make(map[string]bool)
	var refs []string
	for _, match := range matches {
		if !seen[match] {
			seen[match] = true
			refs = append(refs, match)
		}
	}

	return refs
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
