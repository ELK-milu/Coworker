package context

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SessionMemoryTemplate 默认的 session memory 模板
const SessionMemoryTemplate = `# Session Title
_A short and distinctive 5-10 word descriptive title for the session_

# Current State
_What is actively being worked on right now? Pending tasks not yet completed. Immediate next steps._

# Task Specification
_What did the user ask to build? Any design decisions or other explanatory context_

# Files and Functions
_What are the important files? In short, what do they contain and why are they relevant?_

# Workflow
_What bash commands are usually run and in what order? How to interpret their output if not obvious?_

# Errors & Corrections
_Errors encountered and how they were fixed. What did the user correct? What approaches failed?_

# Codebase and System Documentation
_What are the important system components? How do they work/fit together?_

# Learnings
_What has worked well? What has not? What to avoid?_

# Key Results
_If the user asked a specific output such as an answer to a question, a table, or other document, repeat the exact result here_

# Worklog
_Step by step, what was attempted, done? Very terse summary for each step_
`

// MaxSectionTokens 每个章节的最大 token 数
const MaxSectionTokens = 2000

// SessionMemory Session Memory 管理器
type SessionMemory struct {
	baseDir   string
	projectID string
	sessionID string
}

// NewSessionMemory 创建 Session Memory 管理器
func NewSessionMemory(baseDir, projectID, sessionID string) *SessionMemory {
	return &SessionMemory{
		baseDir:   baseDir,
		projectID: sanitizeProjectID(projectID),
		sessionID: sessionID,
	}
}

// sanitizeProjectID 清理项目 ID
func sanitizeProjectID(projectID string) string {
	// 移除驱动器号（Windows）
	sanitized := regexp.MustCompile(`^[a-zA-Z]:`).ReplaceAllString(projectID, "")
	// 替换路径分隔符和特殊字符
	sanitized = regexp.MustCompile(`[\\/:*?"<>|]`).ReplaceAllString(sanitized, "-")
	// 移除开头和结尾的连字符
	sanitized = strings.Trim(sanitized, "-")
	// 限制长度
	if len(sanitized) > 100 {
		sanitized = sanitized[:100]
	}
	if sanitized == "" {
		sanitized = "default"
	}
	return sanitized
}

// GetDir 获取 session memory 目录
func (sm *SessionMemory) GetDir() string {
	return filepath.Join(sm.baseDir, ".coworker", "projects", sm.projectID, sm.sessionID, "session-memory")
}

// GetSummaryPath 获取 summary.md 文件路径
func (sm *SessionMemory) GetSummaryPath() string {
	return filepath.Join(sm.GetDir(), "summary.md")
}

// Init 初始化 session memory 文件
func (sm *SessionMemory) Init() error {
	summaryPath := sm.GetSummaryPath()

	// 如果文件已存在，不需要初始化
	if _, err := os.Stat(summaryPath); err == nil {
		return nil
	}

	// 确保目录存在
	dir := filepath.Dir(summaryPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create session memory dir: %w", err)
	}

	// 写入模板
	if err := os.WriteFile(summaryPath, []byte(SessionMemoryTemplate), 0600); err != nil {
		return fmt.Errorf("failed to write session memory template: %w", err)
	}

	return nil
}

// Read 读取 session memory 内容
func (sm *SessionMemory) Read() (string, error) {
	summaryPath := sm.GetSummaryPath()
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// Write 写入 session memory 内容
func (sm *SessionMemory) Write(content string) error {
	summaryPath := sm.GetSummaryPath()
	dir := filepath.Dir(summaryPath)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	return os.WriteFile(summaryPath, []byte(content), 0600)
}

// IsEmpty 检查是否为空模板
func (sm *SessionMemory) IsEmpty() bool {
	content, err := sm.Read()
	if err != nil || content == "" {
		return true
	}
	return strings.TrimSpace(content) == strings.TrimSpace(SessionMemoryTemplate)
}

// GetUpdatePrompt 获取 session memory 更新提示词
func (sm *SessionMemory) GetUpdatePrompt() string {
	currentNotes, _ := sm.Read()
	if currentNotes == "" {
		currentNotes = SessionMemoryTemplate
	}

	sectionWarnings := getSectionWarnings(currentNotes)
	notesPath := sm.GetSummaryPath()

	return fmt.Sprintf(`Based on the conversation above, update the session notes file.

The file %s has current contents:
<current_notes_content>
%s
</current_notes_content>

CRITICAL RULES FOR EDITING:
- Maintain exact structure with all sections, headers, and italic descriptions intact
- NEVER modify or delete section headers (lines starting with '#')
- NEVER modify the italic _section description_ lines
- ONLY update content that appears BELOW the italic descriptions
- Write DETAILED, INFO-DENSE content - include file paths, function names, error messages
- Keep each section under ~%d tokens
- Focus on actionable, specific information%s`, notesPath, currentNotes, MaxSectionTokens, sectionWarnings)
}

// getSectionWarnings 获取章节长度警告
func getSectionWarnings(content string) string {
	sectionTokens := estimateSectionTokens(content)
	var warnings []string

	for section, tokens := range sectionTokens {
		if tokens > MaxSectionTokens {
			warnings = append(warnings, fmt.Sprintf(
				"- The \"%s\" section is ~%d tokens. Consider condensing.", section, tokens))
		}
	}

	if len(warnings) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(warnings, "\n")
}

// estimateSectionTokens 估算每个章节的 token 数
func estimateSectionTokens(content string) map[string]int {
	sections := make(map[string]int)
	lines := strings.Split(content, "\n")
	currentSection := ""
	var sectionContent []string

	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			if currentSection != "" && len(sectionContent) > 0 {
				text := strings.TrimSpace(strings.Join(sectionContent, "\n"))
				sections[currentSection] = EstimateTokens(text)
			}
			currentSection = line
			sectionContent = nil
		} else {
			sectionContent = append(sectionContent, line)
		}
	}

	// 处理最后一个章节
	if currentSection != "" && len(sectionContent) > 0 {
		text := strings.TrimSpace(strings.Join(sectionContent, "\n"))
		sections[currentSection] = EstimateTokens(text)
	}

	return sections
}

// FormatForSystemPrompt 格式化用于系统提示
func (sm *SessionMemory) FormatForSystemPrompt(isCompact bool) string {
	content, err := sm.Read()
	if err != nil || content == "" {
		return ""
	}

	lines := []string{
		"<session-notes>",
		content,
		"</session-notes>",
	}

	if isCompact {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("To read full session notes: %s", sm.GetSummaryPath()))
	}

	return strings.Join(lines, "\n")
}

// GetTitle 从 session memory 中提取标题
func (sm *SessionMemory) GetTitle() string {
	content, err := sm.Read()
	if err != nil || content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	inTitleSection := false

	for _, line := range lines {
		if strings.HasPrefix(line, "# Session Title") {
			inTitleSection = true
			continue
		}
		if inTitleSection {
			// 跳过斜体描述行
			if strings.HasPrefix(line, "_") {
				continue
			}
			// 跳过空行
			if strings.TrimSpace(line) == "" {
				continue
			}
			// 遇到下一个章节，停止
			if strings.HasPrefix(line, "#") {
				break
			}
			// 返回标题内容
			return strings.TrimSpace(line)
		}
	}

	return ""
}

// UpdateSection 更新指定章节的内容
func (sm *SessionMemory) UpdateSection(sectionName, newContent string) error {
	content, err := sm.Read()
	if err != nil {
		content = SessionMemoryTemplate
	}

	lines := strings.Split(content, "\n")
	var result []string
	inTargetSection := false
	foundSection := false
	skipUntilNextSection := false

	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			if strings.Contains(line, sectionName) {
				inTargetSection = true
				foundSection = true
				result = append(result, line)
				continue
			} else if inTargetSection {
				// 在目标章节结束前插入新内容
				result = append(result, newContent)
				result = append(result, "")
				inTargetSection = false
				skipUntilNextSection = false
			}
		}

		if inTargetSection {
			// 保留斜体描述行
			if strings.HasPrefix(line, "_") && strings.HasSuffix(line, "_") {
				result = append(result, line)
				result = append(result, "")
				skipUntilNextSection = true
				continue
			}
			// 跳过旧内容
			if skipUntilNextSection {
				continue
			}
		}

		result = append(result, line)
	}

	// 如果目标章节是最后一个章节
	if inTargetSection {
		result = append(result, newContent)
	}

	if !foundSection {
		return fmt.Errorf("section not found: %s", sectionName)
	}

	return sm.Write(strings.Join(result, "\n"))
}

// SessionMemoryManager 管理多个会话的 Session Memory
type SessionMemoryManager struct {
	baseDir string
}

// NewSessionMemoryManager 创建管理器
func NewSessionMemoryManager(baseDir string) *SessionMemoryManager {
	return &SessionMemoryManager{baseDir: baseDir}
}

// GetForSession 获取指定会话的 Session Memory
func (m *SessionMemoryManager) GetForSession(userID, projectID, sessionID string) *SessionMemory {
	userDir := filepath.Join(m.baseDir, userID)
	return NewSessionMemory(userDir, projectID, sessionID)
}

// LoadRelevantMemories 加载相关的 Session Memory（跨会话继承）
func (m *SessionMemoryManager) LoadRelevantMemories(userID, projectID string, limit int) []*SessionMemorySummary {
	userDir := filepath.Join(m.baseDir, userID)
	projectDir := filepath.Join(userDir, ".coworker", "projects", sanitizeProjectID(projectID))

	var summaries []*SessionMemorySummary

	// 遍历项目目录下的所有会话
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return summaries
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		sm := NewSessionMemory(userDir, projectID, sessionID)

		content, err := sm.Read()
		if err != nil || content == "" || sm.IsEmpty() {
			continue
		}

		title := sm.GetTitle()
		if title == "" {
			title = sessionID
		}

		summaries = append(summaries, &SessionMemorySummary{
			SessionID: sessionID,
			Title:     title,
			Content:   content,
		})

		if len(summaries) >= limit {
			break
		}
	}

	return summaries
}

// SessionMemorySummary Session Memory 摘要
type SessionMemorySummary struct {
	SessionID string `json:"session_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
}
