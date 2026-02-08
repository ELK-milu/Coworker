package permissions

import (
	"regexp"
	"strings"
)

// Wildcard 通配符匹配工具
// 参考 OpenCode packages/opencode/src/util/wildcard.ts

// WildcardMatch 检查字符串是否匹配通配符模式
// 支持 * (匹配任意字符) 和 ? (匹配单个字符)
// 特殊规则: 如果模式以 " *" 结尾，尾部部分变为可选（如 "ls *" 匹配 "ls" 和 "ls -la"）
func WildcardMatch(str, pattern string) bool {
	// 转义正则特殊字符
	escaped := regexp.QuoteMeta(pattern)

	// 将通配符转换为正则
	// QuoteMeta 会将 * 转义为 \*，? 转义为 \?
	// 需要将 \* 替换为 .*，\? 替换为 .
	escaped = strings.ReplaceAll(escaped, `\*`, `.*`)
	escaped = strings.ReplaceAll(escaped, `\?`, `.`)

	// 特殊规则: 如果模式以 " *" 结尾（空格+通配符），使尾部可选
	// 这允许 "ls *" 同时匹配 "ls" 和 "ls -la"
	if strings.HasSuffix(escaped, " .*") {
		escaped = escaped[:len(escaped)-3] + "( .*)?"
	}

	re, err := regexp.Compile("^" + escaped + "$")
	if err != nil {
		return false
	}
	return re.MatchString(str)
}
