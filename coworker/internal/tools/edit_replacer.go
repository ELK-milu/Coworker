package tools

import (
	"strings"
	"unicode"
)

// Match 表示一个匹配结果
type Match struct {
	Start      int     // 匹配开始位置
	End        int     // 匹配结束位置
	Similarity float64 // 相似度 (0.0-1.0)
}

// Replacer 替换器接口
type Replacer interface {
	Name() string
	FindMatch(content, find string) *Match
}

// ReplacerChain 替换器链
type ReplacerChain struct {
	replacers []Replacer
}

// NewReplacerChain 创建替换器链
func NewReplacerChain() *ReplacerChain {
	return &ReplacerChain{
		replacers: []Replacer{
			&SimpleReplacer{},
			&LineTrimmedReplacer{},
			&LeadingWhitespaceReplacer{},
			&BlockAnchorReplacer{threshold: 0.3},
			&WhitespaceNormalizedReplacer{},
			&IndentNormalizedReplacer{},
			// P1.1: 新增 3 层 Replacer（参考 OpenCode）
			&EscapeNormalizedReplacer{},
			&TrimmedBoundaryReplacer{},
			&ContextAwareReplacer{contextLines: 3},
		},
	}
}

// FindBestMatch 在链中查找最佳匹配
func (c *ReplacerChain) FindBestMatch(content, find string) (*Match, string) {
	for _, r := range c.replacers {
		if match := r.FindMatch(content, find); match != nil {
			return match, r.Name()
		}
	}
	return nil, ""
}

// SimpleReplacer 精确匹配替换器
type SimpleReplacer struct{}

func (r *SimpleReplacer) Name() string { return "SimpleReplacer" }

func (r *SimpleReplacer) FindMatch(content, find string) *Match {
	idx := strings.Index(content, find)
	if idx == -1 {
		return nil
	}
	return &Match{
		Start:      idx,
		End:        idx + len(find),
		Similarity: 1.0,
	}
}

// LineTrimmedReplacer 行尾空格容忍替换器
type LineTrimmedReplacer struct{}

func (r *LineTrimmedReplacer) Name() string { return "LineTrimmedReplacer" }

func (r *LineTrimmedReplacer) FindMatch(content, find string) *Match {
	// 将内容和查找字符串都按行处理，去除行尾空格
	contentLines := strings.Split(content, "\n")
	findLines := strings.Split(find, "\n")

	if len(findLines) == 0 {
		return nil
	}

	// 对查找字符串的每行去除行尾空格
	trimmedFind := make([]string, len(findLines))
	for i, line := range findLines {
		trimmedFind[i] = strings.TrimRightFunc(line, unicode.IsSpace)
	}

	// 在内容中查找匹配
	for i := 0; i <= len(contentLines)-len(findLines); i++ {
		matched := true
		for j, findLine := range trimmedFind {
			contentLine := strings.TrimRightFunc(contentLines[i+j], unicode.IsSpace)
			if contentLine != findLine {
				matched = false
				break
			}
		}
		if matched {
			// 计算原始内容中的位置
			start := 0
			for k := 0; k < i; k++ {
				start += len(contentLines[k]) + 1 // +1 for newline
			}
			end := start
			for k := 0; k < len(findLines); k++ {
				end += len(contentLines[i+k])
				if i+k < len(contentLines)-1 {
					end++ // +1 for newline
				}
			}
			return &Match{
				Start:      start,
				End:        end,
				Similarity: 0.95,
			}
		}
	}
	return nil
}

// LeadingWhitespaceReplacer 前导空格容忍替换器
type LeadingWhitespaceReplacer struct{}

func (r *LeadingWhitespaceReplacer) Name() string { return "LeadingWhitespaceReplacer" }

func (r *LeadingWhitespaceReplacer) FindMatch(content, find string) *Match {
	contentLines := strings.Split(content, "\n")
	findLines := strings.Split(find, "\n")

	if len(findLines) == 0 {
		return nil
	}

	// 计算查找字符串的最小缩进
	minIndent := -1
	for _, line := range findLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}

	// 对查找字符串去除公共缩进并去除行尾空格
	normalizedFind := make([]string, len(findLines))
	for i, line := range findLines {
		if strings.TrimSpace(line) == "" {
			normalizedFind[i] = ""
		} else if minIndent > 0 && len(line) >= minIndent {
			normalizedFind[i] = strings.TrimRightFunc(line[minIndent:], unicode.IsSpace)
		} else {
			normalizedFind[i] = strings.TrimRightFunc(line, unicode.IsSpace)
		}
	}

	// 在内容中查找匹配
	for i := 0; i <= len(contentLines)-len(findLines); i++ {
		// 计算当前位置的缩进
		contentMinIndent := -1
		for j := 0; j < len(findLines); j++ {
			line := contentLines[i+j]
			if strings.TrimSpace(line) == "" {
				continue
			}
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			if contentMinIndent == -1 || indent < contentMinIndent {
				contentMinIndent = indent
			}
		}

		matched := true
		for j, findLine := range normalizedFind {
			contentLine := contentLines[i+j]
			var normalizedContent string
			if strings.TrimSpace(contentLine) == "" {
				normalizedContent = ""
			} else if contentMinIndent > 0 && len(contentLine) >= contentMinIndent {
				normalizedContent = strings.TrimRightFunc(contentLine[contentMinIndent:], unicode.IsSpace)
			} else {
				normalizedContent = strings.TrimRightFunc(contentLine, unicode.IsSpace)
			}

			if normalizedContent != findLine {
				matched = false
				break
			}
		}

		if matched {
			start := 0
			for k := 0; k < i; k++ {
				start += len(contentLines[k]) + 1
			}
			end := start
			for k := 0; k < len(findLines); k++ {
				end += len(contentLines[i+k])
				if i+k < len(contentLines)-1 {
					end++
				}
			}
			return &Match{
				Start:      start,
				End:        end,
				Similarity: 0.9,
			}
		}
	}
	return nil
}

// BlockAnchorReplacer 首尾行锚定 + Levenshtein 模糊匹配替换器
type BlockAnchorReplacer struct {
	threshold float64 // 相似度阈值
}

func (r *BlockAnchorReplacer) Name() string { return "BlockAnchorReplacer" }

func (r *BlockAnchorReplacer) FindMatch(content, find string) *Match {
	contentLines := strings.Split(content, "\n")
	findLines := strings.Split(find, "\n")

	if len(findLines) < 2 {
		return nil
	}

	// 获取首行和尾行作为锚点
	firstLine := strings.TrimSpace(findLines[0])
	lastLine := strings.TrimSpace(findLines[len(findLines)-1])

	if firstLine == "" || lastLine == "" {
		return nil
	}

	// 查找所有可能的匹配位置
	var candidates []struct {
		startIdx   int
		endIdx     int
		similarity float64
	}

	for i := 0; i <= len(contentLines)-len(findLines); i++ {
		// 检查首行是否匹配
		if strings.TrimSpace(contentLines[i]) != firstLine {
			continue
		}

		// 检查尾行是否匹配
		endIdx := i + len(findLines) - 1
		if endIdx >= len(contentLines) {
			continue
		}
		if strings.TrimSpace(contentLines[endIdx]) != lastLine {
			continue
		}

		// 计算中间内容的相似度
		similarity := r.calculateSimilarity(contentLines[i:endIdx+1], findLines)
		if similarity >= r.threshold {
			candidates = append(candidates, struct {
				startIdx   int
				endIdx     int
				similarity float64
			}{i, endIdx, similarity})
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// 选择相似度最高的候选
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.similarity > best.similarity {
			best = c
		}
	}

	// 计算字节位置
	start := 0
	for k := 0; k < best.startIdx; k++ {
		start += len(contentLines[k]) + 1
	}
	end := start
	for k := best.startIdx; k <= best.endIdx; k++ {
		end += len(contentLines[k])
		if k < len(contentLines)-1 {
			end++
		}
	}

	return &Match{
		Start:      start,
		End:        end,
		Similarity: best.similarity,
	}
}

// calculateSimilarity 计算两组行的相似度
func (r *BlockAnchorReplacer) calculateSimilarity(contentLines, findLines []string) float64 {
	if len(contentLines) != len(findLines) {
		return 0
	}

	totalSimilarity := 0.0
	for i := range contentLines {
		contentLine := strings.TrimSpace(contentLines[i])
		findLine := strings.TrimSpace(findLines[i])
		totalSimilarity += levenshteinSimilarity(contentLine, findLine)
	}

	return totalSimilarity / float64(len(contentLines))
}

// levenshteinSimilarity 计算两个字符串的 Levenshtein 相似度
func levenshteinSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	distance := levenshteinDistance(a, b)
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}

	return 1.0 - float64(distance)/float64(maxLen)
}

// levenshteinDistance 计算 Levenshtein 编辑距离
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// 使用两行数组优化空间
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)

	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = minInt(
				prev[j]+1,      // 删除
				curr[j-1]+1,    // 插入
				prev[j-1]+cost, // 替换
			)
		}
		prev, curr = curr, prev
	}

	return prev[len(b)]
}

// minInt 返回最小值
func minInt(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}

// WhitespaceNormalizedReplacer 空格归一化替换器
type WhitespaceNormalizedReplacer struct{}

func (r *WhitespaceNormalizedReplacer) Name() string { return "WhitespaceNormalizedReplacer" }

func (r *WhitespaceNormalizedReplacer) FindMatch(content, find string) *Match {
	// 将连续空格归一化为单个空格
	normalizedContent := normalizeWhitespace(content)
	normalizedFind := normalizeWhitespace(find)

	idx := strings.Index(normalizedContent, normalizedFind)
	if idx == -1 {
		return nil
	}

	// 需要映射回原始位置
	start, end := mapNormalizedPosition(content, normalizedContent, idx, idx+len(normalizedFind))

	return &Match{
		Start:      start,
		End:        end,
		Similarity: 0.85,
	}
}

// normalizeWhitespace 将连续空格归一化为单个空格
func normalizeWhitespace(s string) string {
	var result strings.Builder
	inSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) && r != '\n' {
			if !inSpace {
				result.WriteRune(' ')
				inSpace = true
			}
		} else {
			result.WriteRune(r)
			inSpace = false
		}
	}
	return result.String()
}

// mapNormalizedPosition 将归一化后的位置映射回原始位置
func mapNormalizedPosition(original, normalized string, normStart, normEnd int) (int, int) {
	origIdx := 0
	normIdx := 0
	start := 0
	end := 0

	inSpace := false
	for origIdx < len(original) && normIdx < normEnd {
		r := rune(original[origIdx])
		if unicode.IsSpace(r) && r != '\n' {
			if !inSpace {
				if normIdx == normStart {
					start = origIdx
				}
				normIdx++
				inSpace = true
			}
			origIdx++
		} else {
			if normIdx == normStart {
				start = origIdx
			}
			normIdx++
			origIdx++
			inSpace = false
		}
	}
	end = origIdx

	return start, end
}

// IndentNormalizedReplacer 缩进归一化替换器（Tab/空格互换）
type IndentNormalizedReplacer struct{}

func (r *IndentNormalizedReplacer) Name() string { return "IndentNormalizedReplacer" }

func (r *IndentNormalizedReplacer) FindMatch(content, find string) *Match {
	contentLines := strings.Split(content, "\n")
	findLines := strings.Split(find, "\n")

	if len(findLines) == 0 {
		return nil
	}

	// 归一化缩进：将 tab 转换为 4 空格
	normalizedFind := make([]string, len(findLines))
	for i, line := range findLines {
		normalizedFind[i] = normalizeIndent(line)
	}

	for i := 0; i <= len(contentLines)-len(findLines); i++ {
		matched := true
		for j, findLine := range normalizedFind {
			contentLine := normalizeIndent(contentLines[i+j])
			if contentLine != findLine {
				matched = false
				break
			}
		}

		if matched {
			start := 0
			for k := 0; k < i; k++ {
				start += len(contentLines[k]) + 1
			}
			end := start
			for k := 0; k < len(findLines); k++ {
				end += len(contentLines[i+k])
				if i+k < len(contentLines)-1 {
					end++
				}
			}
			return &Match{
				Start:      start,
				End:        end,
				Similarity: 0.8,
			}
		}
	}
	return nil
}

// normalizeIndent 归一化缩进（tab -> 4空格）
func normalizeIndent(s string) string {
	return strings.ReplaceAll(s, "\t", "    ")
}

// EscapeNormalizedReplacer 转义字符归一化替换器
// 参考 OpenCode EscapeNormalizedReplacer
// 处理 AI 生成的代码中转义字符不一致的情况
type EscapeNormalizedReplacer struct{}

func (r *EscapeNormalizedReplacer) Name() string { return "EscapeNormalizedReplacer" }

func (r *EscapeNormalizedReplacer) FindMatch(content, find string) *Match {
	normalizedContent := normalizeEscapes(content)
	normalizedFind := normalizeEscapes(find)

	idx := strings.Index(normalizedContent, normalizedFind)
	if idx == -1 {
		return nil
	}

	// 映射回原始位置
	start, end := mapEscapePosition(content, normalizedContent, idx, idx+len(normalizedFind))

	return &Match{
		Start:      start,
		End:        end,
		Similarity: 0.75,
	}
}

// normalizeEscapes 归一化转义字符
func normalizeEscapes(s string) string {
	// 统一常见转义序列
	replacer := strings.NewReplacer(
		"\\n", "\n",
		"\\t", "\t",
		"\\r", "\r",
		"\\\"", "\"",
		"\\'", "'",
		"\\\\", "\\",
	)
	return replacer.Replace(s)
}

// mapEscapePosition 映射转义归一化后的位置到原始位置
func mapEscapePosition(original, normalized string, normStart, normEnd int) (int, int) {
	// 简化实现：通过字符计数映射
	origIdx := 0
	normIdx := 0
	start := 0

	for origIdx < len(original) && normIdx < normEnd {
		if normIdx == normStart {
			start = origIdx
		}

		// 检查原始字符串中是否有转义序列
		if origIdx+1 < len(original) && original[origIdx] == '\\' {
			switch original[origIdx+1] {
			case 'n', 't', 'r', '"', '\'', '\\':
				origIdx += 2
				normIdx++
				continue
			}
		}

		origIdx++
		normIdx++
	}

	return start, origIdx
}

// TrimmedBoundaryReplacer 边界空白容忍替换器
// 参考 OpenCode TrimmedBoundaryReplacer
// 处理 AI 在代码块首尾添加/遗漏空行的情况
type TrimmedBoundaryReplacer struct{}

func (r *TrimmedBoundaryReplacer) Name() string { return "TrimmedBoundaryReplacer" }

func (r *TrimmedBoundaryReplacer) FindMatch(content, find string) *Match {
	// 去除查找字符串首尾的空行
	trimmedFind := trimBoundaryLines(find)
	if trimmedFind == "" {
		return nil
	}

	// 先尝试直接匹配去除边界后的字符串
	idx := strings.Index(content, trimmedFind)
	if idx != -1 {
		return &Match{
			Start:      idx,
			End:        idx + len(trimmedFind),
			Similarity: 0.7,
		}
	}

	// 尝试在内容中也去除边界空行后匹配
	contentLines := strings.Split(content, "\n")
	findLines := strings.Split(trimmedFind, "\n")

	if len(findLines) == 0 {
		return nil
	}

	for i := 0; i <= len(contentLines)-len(findLines); i++ {
		matched := true
		for j, findLine := range findLines {
			cl := strings.TrimSpace(contentLines[i+j])
			fl := strings.TrimSpace(findLine)
			if cl != fl {
				matched = false
				break
			}
		}

		if matched {
			start := 0
			for k := 0; k < i; k++ {
				start += len(contentLines[k]) + 1
			}
			end := start
			for k := 0; k < len(findLines); k++ {
				end += len(contentLines[i+k])
				if i+k < len(contentLines)-1 {
					end++
				}
			}
			return &Match{
				Start:      start,
				End:        end,
				Similarity: 0.7,
			}
		}
	}

	return nil
}

// trimBoundaryLines 去除字符串首尾的空行
func trimBoundaryLines(s string) string {
	lines := strings.Split(s, "\n")

	// 去除首部空行
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}

	// 去除尾部空行
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}

	if start >= end {
		return ""
	}

	return strings.Join(lines[start:end], "\n")
}

// ContextAwareReplacer 上下文感知匹配替换器
// 参考 OpenCode ContextAwareReplacer
// 使用查找字符串的前 N 行和后 N 行作为上下文锚点，
// 在内容中定位匹配区域，容忍中间内容的微小差异
type ContextAwareReplacer struct {
	contextLines int // 上下文行数（默认 3）
}

func (r *ContextAwareReplacer) Name() string { return "ContextAwareReplacer" }

func (r *ContextAwareReplacer) FindMatch(content, find string) *Match {
	contextSize := r.contextLines
	if contextSize <= 0 {
		contextSize = 3
	}

	contentLines := strings.Split(content, "\n")
	findLines := strings.Split(find, "\n")

	// 至少需要 2*contextSize+1 行才能使用上下文匹配
	if len(findLines) < 2*contextSize+1 {
		return nil
	}

	// 提取上下文锚点（前 N 行和后 N 行）
	headContext := findLines[:contextSize]
	tailContext := findLines[len(findLines)-contextSize:]
	expectedMiddleLen := len(findLines) - 2*contextSize

	// 在内容中查找头部上下文匹配
	for i := 0; i <= len(contentLines)-len(findLines); i++ {
		// 检查头部上下文
		headMatch := true
		for j := 0; j < contextSize; j++ {
			if strings.TrimSpace(contentLines[i+j]) != strings.TrimSpace(headContext[j]) {
				headMatch = false
				break
			}
		}
		if !headMatch {
			continue
		}

		// 检查尾部上下文（允许中间行数有 ±2 的偏差）
		for delta := -2; delta <= 2; delta++ {
			tailStart := i + contextSize + expectedMiddleLen + delta
			if tailStart < i+contextSize || tailStart+contextSize > len(contentLines) {
				continue
			}

			tailMatch := true
			for j := 0; j < contextSize; j++ {
				if strings.TrimSpace(contentLines[tailStart+j]) != strings.TrimSpace(tailContext[j]) {
					tailMatch = false
					break
				}
			}

			if tailMatch {
				endIdx := tailStart + contextSize - 1
				start := 0
				for k := 0; k < i; k++ {
					start += len(contentLines[k]) + 1
				}
				end := start
				for k := i; k <= endIdx; k++ {
					end += len(contentLines[k])
					if k < len(contentLines)-1 {
						end++
					}
				}
				return &Match{
					Start:      start,
					End:        end,
					Similarity: 0.65,
				}
			}
		}
	}

	return nil
}
