package context

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// Microcompact 配置常量
const (
	// KeepRecentToolResults 保留最近的工具结果数量
	KeepRecentToolResults = 3
	// MicrocompactPlaceholder 工具结果被清理后的占位符
	MicrocompactPlaceholder = "[Tool output cleared to save context]"
)

// MicrocompactWhitelist 不清理的工具白名单
var MicrocompactWhitelist = map[string]bool{
	"Read":      true,
	"Bash":      true,
	"Grep":      true,
	"Glob":      true,
	"WebSearch": true,
	"WebFetch":  true,
}

// CompressMessage 压缩消息内容
func CompressMessage(msg types.Message, maxChars int) types.Message {
	if msg.Content == nil || len(msg.Content) == 0 {
		return msg
	}

	compressedBlocks := make([]interface{}, 0, len(msg.Content))

	for _, block := range msg.Content {
		switch b := block.(type) {
		case types.ToolResultBlock:
			if len(b.Content) > maxChars {
				b.Content = CompressToolOutput(b.Content, maxChars)
			}
			compressedBlocks = append(compressedBlocks, b)
		case types.TextBlock:
			compressed := compressTextBlock(b.Text, maxChars)
			compressedBlocks = append(compressedBlocks, types.TextBlock{
				Type: "text",
				Text: compressed,
			})
		default:
			compressedBlocks = append(compressedBlocks, block)
		}
	}

	return types.Message{
		Role:    msg.Role,
		Content: compressedBlocks,
	}
}

// compressTextBlock 压缩文本块中的代码
func compressTextBlock(text string, maxChars int) string {
	codeBlocks := extractCodeBlocks(text)
	if len(codeBlocks) == 0 {
		return text
	}

	result := text
	for _, cb := range codeBlocks {
		compressed := CompressCodeBlock(cb.code, CodeBlockMaxLines)
		marker := "```"
		if cb.language != "" {
			marker = "```" + cb.language
		}
		old := fmt.Sprintf("%s\n%s```", marker, cb.code)
		new := fmt.Sprintf("%s\n%s```", marker, compressed)
		result = strings.Replace(result, old, new, 1)
	}

	return result
}

// codeBlock 代码块信息
type codeBlock struct {
	code     string
	language string
	start    int
	end      int
}

// extractCodeBlocks 提取代码块
func extractCodeBlocks(text string) []codeBlock {
	re := regexp.MustCompile("```(\\w+)?\\n([\\s\\S]*?)```")
	matches := re.FindAllStringSubmatchIndex(text, -1)

	blocks := make([]codeBlock, 0)
	for _, match := range matches {
		if len(match) >= 6 {
			lang := ""
			if match[2] != -1 && match[3] != -1 {
				lang = text[match[2]:match[3]]
			}
			code := text[match[4]:match[5]]
			blocks = append(blocks, codeBlock{
				code:     code,
				language: lang,
				start:    match[0],
				end:      match[1],
			})
		}
	}

	return blocks
}

// CompressCodeBlock 压缩代码块
func CompressCodeBlock(code string, maxLines int) string {
	lines := strings.Split(code, "\n")
	if len(lines) <= maxLines {
		return code
	}

	keepHead := int(float64(maxLines) * 0.6)
	keepTail := int(float64(maxLines) * 0.4)

	head := strings.Join(lines[:keepHead], "\n")
	tail := strings.Join(lines[len(lines)-keepTail:], "\n")
	omitted := len(lines) - maxLines

	return fmt.Sprintf("%s\n\n... [%d lines omitted] ...\n\n%s", head, omitted, tail)
}

// CompressToolOutput 压缩工具输出
func CompressToolOutput(content string, maxChars int) string {
	if len(content) <= maxChars {
		return content
	}

	// 检测是否包含代码块
	codeBlocks := extractCodeBlocks(content)
	if len(codeBlocks) > 0 {
		result := content
		for _, block := range codeBlocks {
			compressed := CompressCodeBlock(block.code, CodeBlockMaxLines)
			marker := "```"
			if block.language != "" {
				marker = "```" + block.language
			}
			old := fmt.Sprintf("%s\n%s```", marker, block.code)
			new := fmt.Sprintf("%s\n%s```", marker, compressed)
			result = strings.Replace(result, old, new, 1)
		}
		if len(result) <= maxChars {
			return result
		}
	}

	// 检测是否是文件内容
	if strings.Contains(content, "→") || regexp.MustCompile(`^\s*\d+\s*[│|]`).MatchString(content) {
		lines := strings.Split(content, "\n")
		keepHead := 20
		keepTail := 10

		if len(lines) > keepHead+keepTail {
			head := strings.Join(lines[:keepHead], "\n")
			tail := strings.Join(lines[len(lines)-keepTail:], "\n")
			omitted := len(lines) - keepHead - keepTail
			return fmt.Sprintf("%s\n... [%d lines omitted] ...\n%s", head, omitted, tail)
		}
	}

	// 默认：简单截断
	keepHead := int(float64(maxChars) * 0.7)
	keepTail := int(float64(maxChars) * 0.3)
	head := content[:keepHead]
	tail := content[len(content)-keepTail:]
	omitted := len(content) - maxChars

	return fmt.Sprintf("%s\n\n... [~%d chars omitted] ...\n\n%s", head, omitted, tail)
}

// ToolCallInfo 工具调用信息
type ToolCallInfo struct {
	ToolID   string
	ToolName string
	Index    int // 在消息列表中的位置
}

// Microcompact 轻量级压缩 - 清理旧的工具调用结果
// 保留最近 KeepRecentToolResults 个工具结果，清理其他的
func Microcompact(messages []types.Message) []types.Message {
	// 1. 收集所有工具调用信息
	toolCalls := collectToolCalls(messages)
	if len(toolCalls) <= KeepRecentToolResults {
		return messages
	}

	// 2. 确定需要清理的工具调用（保留最近的）
	toClean := toolCalls[:len(toolCalls)-KeepRecentToolResults]
	cleanSet := make(map[string]bool)
	for _, tc := range toClean {
		cleanSet[tc.ToolID] = true
	}

	// 3. 清理旧的工具结果
	result := make([]types.Message, len(messages))
	for i, msg := range messages {
		result[i] = cleanToolResults(msg, cleanSet)
	}

	return result
}

// collectToolCalls 收集所有工具调用信息
func collectToolCalls(messages []types.Message) []ToolCallInfo {
	var calls []ToolCallInfo

	for i, msg := range messages {
		if msg.Role != "user" {
			continue
		}
		for _, block := range msg.Content {
			if tr, ok := block.(types.ToolResultBlock); ok {
				calls = append(calls, ToolCallInfo{
					ToolID:   tr.ToolUseID,
					ToolName: "", // 工具名称需要从 assistant 消息中获取
					Index:    i,
				})
			}
		}
	}

	return calls
}

// cleanToolResults 清理指定的工具结果
func cleanToolResults(msg types.Message, cleanSet map[string]bool) types.Message {
	if msg.Role != "user" || len(msg.Content) == 0 {
		return msg
	}

	newContent := make([]interface{}, 0, len(msg.Content))
	for _, block := range msg.Content {
		if tr, ok := block.(types.ToolResultBlock); ok {
			if cleanSet[tr.ToolUseID] {
				// 清理工具结果，替换为占位符
				tr.Content = MicrocompactPlaceholder
			}
			newContent = append(newContent, tr)
		} else {
			newContent = append(newContent, block)
		}
	}

	return types.Message{
		Role:    msg.Role,
		Content: newContent,
	}
}
