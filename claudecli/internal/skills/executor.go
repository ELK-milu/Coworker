package skills

import (
	"fmt"
	"regexp"
	"strings"
)

// Executor 技能执行器
type Executor struct {
	registry *Registry
}

// NewExecutor 创建技能执行器
func NewExecutor(registry *Registry) *Executor {
	return &Executor{registry: registry}
}

// Execute 执行技能
func (e *Executor) Execute(name string, args []string) (string, error) {
	skill, ok := e.registry.Get(name)
	if !ok {
		return "", fmt.Errorf("skill not found: %s", name)
	}
	return e.SubstituteParams(skill.Content, args), nil
}

// SubstituteParams 替换参数
func (e *Executor) SubstituteParams(content string, args []string) string {
	result := content

	// $0 -> 完整参数
	result = strings.ReplaceAll(result, "$0", strings.Join(args, " "))

	// $ARGUMENTS[N] -> 数组索引
	re := regexp.MustCompile(`\$ARGUMENTS\[(\d+)\]`)
	result = re.ReplaceAllStringFunc(result, func(m string) string {
		matches := re.FindStringSubmatch(m)
		if len(matches) < 2 {
			return m
		}
		idx := 0
		fmt.Sscanf(matches[1], "%d", &idx)
		if idx < len(args) {
			return args[idx]
		}
		return ""
	})

	// $1, $2, ... -> 位置参数 (从大到小替换避免 $1 和 $10 冲突)
	for i := len(args); i >= 1; i-- {
		placeholder := fmt.Sprintf("$%d", i)
		if i-1 < len(args) {
			result = strings.ReplaceAll(result, placeholder, args[i-1])
		}
	}

	return result
}
