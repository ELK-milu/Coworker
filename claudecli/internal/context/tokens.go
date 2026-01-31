package context

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// EstimateTokens 估算文本 token 数
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	// 检测文本类型
	hasAsian := regexp.MustCompile(`[\x{4e00}-\x{9fa5}\x{3040}-\x{309f}\x{30a0}-\x{30ff}]`).MatchString(text)
	hasCode := regexp.MustCompile(`^` + "```" + `|function |class |const |let |var |import |export `).MatchString(text)

	// 根据内容类型调整估算
	charsPerToken := CharsPerToken
	if hasAsian {
		charsPerToken = 2.0
	} else if hasCode {
		charsPerToken = 3.0
	}

	// 计算基础 token
	tokens := float64(len(text)) / charsPerToken

	// 为特殊字符添加权重
	specialChars := regexp.MustCompile(`[{}[\]().,;:!?<>]`).FindAllString(text, -1)
	tokens += float64(len(specialChars)) * 0.1

	// 换行符也会占用 token
	newlines := strings.Count(text, "\n")
	tokens += float64(newlines) * 0.5

	return int(tokens + 0.5)
}

// EstimateMessageTokens 估算消息 token 数
func EstimateMessageTokens(msg types.Message) int {
	if msg.Content == nil || len(msg.Content) == 0 {
		return 10
	}

	total := 10 // 消息开销

	for _, block := range msg.Content {
		switch b := block.(type) {
		case types.TextBlock:
			total += EstimateTokens(b.Text)
		case types.ToolUseBlock:
			total += EstimateTokens(b.Name)
			if b.Input != nil {
				inputBytes, _ := json.Marshal(b.Input)
				total += EstimateTokens(string(inputBytes))
			}
		case types.ToolResultBlock:
			total += EstimateTokens(b.Content)
		case map[string]interface{}:
			// 处理 map 类型
			if t, ok := b["type"].(string); ok {
				switch t {
				case "text":
					if text, ok := b["text"].(string); ok {
						total += EstimateTokens(text)
					}
				case "tool_use":
					if name, ok := b["name"].(string); ok {
						total += EstimateTokens(name)
					}
					if input, ok := b["input"]; ok {
						inputBytes, _ := json.Marshal(input)
						total += EstimateTokens(string(inputBytes))
					}
				case "tool_result":
					if content, ok := b["content"].(string); ok {
						total += EstimateTokens(content)
					}
				}
			}
		}
	}

	return total
}

// EstimateTotalTokens 估算消息数组的总 token 数
func EstimateTotalTokens(messages []types.Message) int {
	total := 0
	for _, msg := range messages {
		total += EstimateMessageTokens(msg)
	}
	return total
}
