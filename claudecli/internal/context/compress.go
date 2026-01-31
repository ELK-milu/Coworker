package context

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

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
