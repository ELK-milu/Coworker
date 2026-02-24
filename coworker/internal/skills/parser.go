package skills

import (
	"bufio"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseSkillFile 解析技能文件
func ParseSkillFile(path string) (*Skill, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return parseSkillContent(file, path)
}

// parseSkillContent 解析技能内容
func parseSkillContent(r io.Reader, path string) (*Skill, error) {
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	fm, content, err := parseFrontmatter(lines)
	if err != nil {
		return nil, err
	}

	return &Skill{
		Name:         fm.Name,
		Description:  fm.Description,
		Content:      content,
		AllowedTools: fm.AllowedTools,
		FilePath:     path,
	}, nil
}

// parseFrontmatter 解析 YAML frontmatter
func parseFrontmatter(lines []string) (*Frontmatter, string, error) {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return &Frontmatter{}, strings.Join(lines, "\n"), nil
	}

	// 查找结束标记
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		return &Frontmatter{}, strings.Join(lines, "\n"), nil
	}

	// 解析 YAML
	yamlContent := strings.Join(lines[1:endIdx], "\n")
	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		return nil, "", err
	}

	// 剩余内容
	content := strings.TrimSpace(strings.Join(lines[endIdx+1:], "\n"))
	return &fm, content, nil
}
