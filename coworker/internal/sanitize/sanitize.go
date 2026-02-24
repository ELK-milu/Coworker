package sanitize

import (
	"regexp"
	"strings"
)

// 需要从用户输入中剥离的系统标签模式
// 这些标签仅应由系统内部通过 SystemBlock 注入，用户不应伪造
var systemTagPatterns = []*regexp.Regexp{
	// <system-reminder>...</system-reminder> 完整标签对（含内容）
	regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>`),
	// 单独的开闭标签（防止不完整注入）
	regexp.MustCompile(`</?system-reminder>`),
}

// UserInput 清理用户输入，剥离系统级标签
// 防止用户通过伪造 <system-reminder> 等标签进行提示词注入
// 注意：不影响普通 HTML 标签（如 <div>, <p> 等）
func UserInput(text string) string {
	if text == "" {
		return text
	}

	// 快速检查：如果不包含任何系统标签关键字，直接返回
	if !strings.Contains(text, "system-reminder") {
		return text
	}

	result := text
	for _, pattern := range systemTagPatterns {
		result = pattern.ReplaceAllString(result, "")
	}

	return result
}
