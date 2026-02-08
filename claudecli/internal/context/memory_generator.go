package context

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// SessionMemoryGenerator Session Memory AI 生成器
type SessionMemoryGenerator struct {
	baseDir string
}

// NewSessionMemoryGenerator 创建生成器
func NewSessionMemoryGenerator(baseDir string) *SessionMemoryGenerator {
	return &SessionMemoryGenerator{baseDir: baseDir}
}

// GeneratePrompt 生成 Session Memory 更新提示词
func (g *SessionMemoryGenerator) GeneratePrompt(messages []types.Message, currentMemory string) string {
	if currentMemory == "" {
		currentMemory = SessionMemoryTemplate
	}

	// 构建对话摘要
	conversationSummary := g.summarizeConversation(messages)

	return fmt.Sprintf(`Based on the conversation, update the session memory.

## Conversation Summary
%s

## Current Session Memory
%s

## Instructions
1. Update ONLY the content below each section's italic description
2. Keep all section headers and italic descriptions intact
3. Be specific: include file paths, function names, error messages
4. Focus on actionable information
5. Each section should be under 500 words

Output the complete updated session memory in markdown format.`, conversationSummary, currentMemory)
}

// summarizeConversation 生成对话摘要
func (g *SessionMemoryGenerator) summarizeConversation(messages []types.Message) string {
	var summary strings.Builder

	for i, msg := range messages {
		if i >= 20 { // 限制最近20条消息
			summary.WriteString("\n... (earlier messages omitted)")
			break
		}

		role := msg.Role
		if role == "user" {
			role = "User"
		} else if role == "assistant" {
			role = "Assistant"
		}

		// 截取内容
		content := getMessageContent(msg)
		if len(content) > 500 {
			content = content[:500] + "..."
		}

		summary.WriteString(fmt.Sprintf("\n### %s\n%s\n", role, content))
	}

	return summary.String()
}

// getMessageContent 获取消息内容
func getMessageContent(msg types.Message) string {
	var parts []string
	for _, item := range msg.Content {
		// 处理 map[string]interface{} 类型 (JSON 反序列化后的格式)
		if m, ok := item.(map[string]interface{}); ok {
			if text, ok := m["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		// 处理 types.TextBlock 类型
		if tb, ok := item.(types.TextBlock); ok {
			parts = append(parts, tb.Text)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

// ExtractStructuredMemory 从对话中提取结构化记忆
func (g *SessionMemoryGenerator) ExtractStructuredMemory(messages []types.Message) *StructuredMemory {
	sm := &StructuredMemory{
		KeyFiles:  make([]string, 0),
		Decisions: make([]string, 0),
		Errors:    make([]string, 0),
		Solutions: make([]string, 0),
		Learnings: make([]string, 0),
	}

	for _, msg := range messages {
		content := getMessageContent(msg)

		// 提取文件路径
		files := extractFilePaths(content)
		for _, f := range files {
			if !contains(sm.KeyFiles, f) && len(sm.KeyFiles) < 20 {
				sm.KeyFiles = append(sm.KeyFiles, f)
			}
		}

		// 提取错误信息
		if strings.Contains(strings.ToLower(content), "error") ||
			strings.Contains(strings.ToLower(content), "failed") {
			if len(sm.Errors) < 10 {
				errorLine := extractErrorLine(content)
				if errorLine != "" && !contains(sm.Errors, errorLine) {
					sm.Errors = append(sm.Errors, errorLine)
				}
			}
		}
	}

	return sm
}

// StructuredMemory 结构化记忆
type StructuredMemory struct {
	TaskSpec  string   `json:"task_spec"`
	KeyFiles  []string `json:"key_files"`
	Decisions []string `json:"decisions"`
	Errors    []string `json:"errors"`
	Solutions []string `json:"solutions"`
	Learnings []string `json:"learnings"`
	NextSteps []string `json:"next_steps"`
}

// extractFilePaths 提取文件路径
func extractFilePaths(content string) []string {
	var paths []string

	// 简单的文件路径匹配
	words := strings.Fields(content)
	for _, word := range words {
		// 检查是否像文件路径
		if (strings.Contains(word, "/") || strings.Contains(word, "\\")) &&
			(strings.Contains(word, ".") || strings.HasSuffix(word, "/")) {
			// 清理路径
			path := strings.Trim(word, "\"'`()[]{}:,")
			if len(path) > 3 && len(path) < 200 {
				paths = append(paths, path)
			}
		}
	}

	return paths
}

// extractErrorLine 提取错误行
func extractErrorLine(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "failed") {
			line = strings.TrimSpace(line)
			if len(line) > 10 && len(line) < 200 {
				return line
			}
		}
	}
	return ""
}

// contains 检查切片是否包含元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
