package profile

import (
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/coworker/pkg/types"
)

// Learner 用户画像学习器
type Learner struct {
	manager *Manager
}

// NewLearner 创建学习器
func NewLearner(manager *Manager) *Learner {
	return &Learner{manager: manager}
}

// LearnFromConversation 从对话中学习用户偏好
func (l *Learner) LearnFromConversation(userID string, messages []types.Message) {
	p := l.manager.GetOrCreate(userID)

	// 提取技术偏好
	langs, frameworks := l.extractTechPreferences(messages)

	// 合并到现有偏好
	p.Languages = mergeUnique(p.Languages, langs)
	p.Frameworks = mergeUnique(p.Frameworks, frameworks)

	// 限制数量
	if len(p.Languages) > 20 {
		p.Languages = p.Languages[:20]
	}
	if len(p.Frameworks) > 20 {
		p.Frameworks = p.Frameworks[:20]
	}

	l.manager.Save(userID, p)
}

// LearnFromToolUsage 从工具使用中学习
func (l *Learner) LearnFromToolUsage(userID, toolName string) {
	l.manager.RecordToolUsage(userID, toolName)
}

// LearnFromProject 从项目结构中学习
func (l *Learner) LearnFromProject(userID, workDir string) {
	// 检测项目类型
	techStack := detectTechStack(workDir)

	if len(techStack) > 0 {
		project := ProjectContext{
			Name:      extractProjectName(workDir),
			Path:      workDir,
			TechStack: techStack,
		}
		l.manager.AddProject(userID, project)
	}
}

// extractTechPreferences 提取技术偏好
func (l *Learner) extractTechPreferences(messages []types.Message) ([]string, []string) {
	var langs, frameworks []string
	seen := make(map[string]bool)

	// 语言关键词
	langKeywords := map[string]string{
		"python":     "Python",
		"golang":     "Go",
		"go ":        "Go",
		"javascript": "JavaScript",
		"typescript": "TypeScript",
		"rust":       "Rust",
		"java":       "Java",
		"c++":        "C++",
		"c#":         "C#",
		"ruby":       "Ruby",
		"php":        "PHP",
		"swift":      "Swift",
		"kotlin":     "Kotlin",
	}

	// 框架关键词
	frameworkKeywords := map[string]string{
		"react":      "React",
		"vue":        "Vue.js",
		"angular":    "Angular",
		"nextjs":     "Next.js",
		"next.js":    "Next.js",
		"express":    "Express",
		"fastapi":    "FastAPI",
		"django":     "Django",
		"flask":      "Flask",
		"gin":        "Gin",
		"spring":     "Spring",
		"rails":      "Rails",
		"laravel":    "Laravel",
		"svelte":     "Svelte",
		"tailwind":   "Tailwind CSS",
		"bootstrap":  "Bootstrap",
	}

	for _, msg := range messages {
		content := getMessageText(msg)
		contentLower := strings.ToLower(content)

		// 检测语言
		for kw, name := range langKeywords {
			if strings.Contains(contentLower, kw) && !seen[name] {
				seen[name] = true
				langs = append(langs, name)
			}
		}

		// 检测框架
		for kw, name := range frameworkKeywords {
			if strings.Contains(contentLower, kw) && !seen[name] {
				seen[name] = true
				frameworks = append(frameworks, name)
			}
		}
	}

	return langs, frameworks
}

// getMessageText 获取消息文本
func getMessageText(msg types.Message) string {
	var parts []string
	for _, item := range msg.Content {
		// 处理 map[string]interface 类型 (JSON 反序列化后的格式)
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

// mergeUnique 合并去重
func mergeUnique(existing, new []string) []string {
	seen := make(map[string]bool)
	for _, s := range existing {
		seen[s] = true
	}

	result := append([]string{}, existing...)
	for _, s := range new {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// detectTechStack 检测项目技术栈
func detectTechStack(workDir string) []string {
	var stack []string

	// 文件检测规则
	fileRules := map[string]string{
		"package.json":    "Node.js",
		"go.mod":          "Go",
		"requirements.txt": "Python",
		"Cargo.toml":      "Rust",
		"pom.xml":         "Java/Maven",
		"build.gradle":    "Java/Gradle",
		"Gemfile":         "Ruby",
		"composer.json":   "PHP",
	}

	for file, tech := range fileRules {
		if fileExists(workDir, file) {
			stack = append(stack, tech)
		}
	}

	return stack
}

// fileExists 检查文件是否存在
func fileExists(dir, file string) bool {
	// 简单实现，实际应该检查文件系统
	return false
}

// extractProjectName 提取项目名称
func extractProjectName(workDir string) string {
	// 从路径中提取项目名
	re := regexp.MustCompile(`[/\\]([^/\\]+)$`)
	if matches := re.FindStringSubmatch(workDir); len(matches) > 1 {
		return matches[1]
	}
	return "unknown"
}
