package memory

import (
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// Extractor 记忆提取器
type Extractor struct {
	manager *Manager
}

// NewExtractor 创建提取器
func NewExtractor(manager *Manager) *Extractor {
	return &Extractor{manager: manager}
}

// ExtractFromConversation 从对话中提取记忆
func (e *Extractor) ExtractFromConversation(userID, sessionID string, messages []types.Message) []*Memory {
	var extracted []*Memory

	// 提取技术偏好
	techPrefs := e.extractTechPreferences(messages)
	if len(techPrefs) > 0 {
		mem := &Memory{
			Tags:      []string{"preferences", "tech-stack"},
			Content:   strings.Join(techPrefs, "\n"),
			Summary:   "User's technology preferences",
			Source:    "extracted",
			SessionID: sessionID,
			Weight:    0.7,
		}
		extracted = append(extracted, mem)
	}

	// 提取项目信息
	projectInfo := e.extractProjectInfo(messages)
	if projectInfo != "" {
		mem := &Memory{
			Tags:      []string{"project", "context"},
			Content:   projectInfo,
			Summary:   "Project context information",
			Source:    "extracted",
			SessionID: sessionID,
			Weight:    0.6,
		}
		extracted = append(extracted, mem)
	}

	// 提取错误和解决方案
	errorSolutions := e.extractErrorSolutions(messages)
	for _, es := range errorSolutions {
		mem := &Memory{
			Tags:      []string{"error", "solution", "troubleshooting"},
			Content:   es,
			Summary:   "Error and solution",
			Source:    "extracted",
			SessionID: sessionID,
			Weight:    0.8,
		}
		extracted = append(extracted, mem)
	}

	return extracted
}

// extractTechPreferences 提取技术偏好
func (e *Extractor) extractTechPreferences(messages []types.Message) []string {
	var prefs []string
	seen := make(map[string]bool)

	// 常见技术关键词
	techKeywords := []string{
		"react", "vue", "angular", "svelte",
		"python", "golang", "rust", "typescript", "javascript",
		"postgresql", "mysql", "mongodb", "redis",
		"docker", "kubernetes", "aws", "gcp", "azure",
		"gin", "fastapi", "express", "nextjs",
	}

	for _, msg := range messages {
		content := getMessageText(msg)
		contentLower := strings.ToLower(content)

		for _, tech := range techKeywords {
			if strings.Contains(contentLower, tech) && !seen[tech] {
				seen[tech] = true
				prefs = append(prefs, tech)
			}
		}
	}

	return prefs
}

// extractProjectInfo 提取项目信息
func (e *Extractor) extractProjectInfo(messages []types.Message) string {
	var info strings.Builder

	// 查找项目相关的描述
	projectPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)project\s+(?:is|called|named)\s+["']?(\w+)["']?`),
		regexp.MustCompile(`(?i)working\s+on\s+["']?(\w+)["']?`),
		regexp.MustCompile(`(?i)building\s+(?:a|an)\s+(\w+(?:\s+\w+)?)`),
	}

	for _, msg := range messages {
		if msg.Role != "user" {
			continue
		}

		content := getMessageText(msg)
		for _, pattern := range projectPatterns {
			if matches := pattern.FindStringSubmatch(content); len(matches) > 1 {
				info.WriteString("- ")
				info.WriteString(matches[1])
				info.WriteString("\n")
			}
		}
	}

	return info.String()
}

// extractErrorSolutions 提取错误和解决方案
func (e *Extractor) extractErrorSolutions(messages []types.Message) []string {
	var solutions []string

	errorPattern := regexp.MustCompile(`(?i)(error|failed|exception|cannot|unable)[\s:]+(.{10,100})`)

	for i, msg := range messages {
		content := getMessageText(msg)

		// 查找错误
		if matches := errorPattern.FindStringSubmatch(content); len(matches) > 2 {
			errorDesc := strings.TrimSpace(matches[2])

			// 查找后续的解决方案
			if i+1 < len(messages) && messages[i+1].Role == "assistant" {
				nextContent := getMessageText(messages[i+1])
				if len(nextContent) > 50 {
					// 截取解决方案摘要
					solution := nextContent
					if len(solution) > 500 {
						solution = solution[:500] + "..."
					}

					solutions = append(solutions, "Error: "+errorDesc+"\nSolution: "+solution)
				}
			}
		}
	}

	// 限制数量
	if len(solutions) > 5 {
		solutions = solutions[:5]
	}

	return solutions
}

// getMessageText 获取消息文本内容
func getMessageText(msg types.Message) string {
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
	return strings.Join(parts, "\n")
}

// GetExtractionPrompt 获取 AI 提取提示词
func GetExtractionPrompt() string {
	return `Analyze the conversation and extract information worth remembering long-term:

1. User preferences (programming languages, frameworks, coding style)
2. Project information (name, structure, tech stack)
3. Important decisions (architecture choices, design patterns)
4. Common issues (errors, solutions)

Output JSON format:
[
  {
    "tags": ["tag1", "tag2"],
    "content": "detailed content",
    "summary": "brief summary",
    "weight": 0.8
  }
]

Only extract truly valuable information. Return empty array [] if nothing significant.`
}
