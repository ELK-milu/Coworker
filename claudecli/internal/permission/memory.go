package permission

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Action 权限动作
type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
)

// Rule 权限规则
type Rule struct {
	Pattern string `json:"pattern"`
	Action  Action `json:"action"`
}

// Memory 权限记忆管理器
type Memory struct {
	rules    map[string][]Rule // permission type -> rules
	filePath string
	mu       sync.RWMutex
}

// NewMemory 创建权限记忆管理器
func NewMemory(userDataDir, userID string) *Memory {
	filePath := filepath.Join(userDataDir, userID, "permissions.json")
	m := &Memory{
		rules:    make(map[string][]Rule),
		filePath: filePath,
	}
	m.load()
	return m
}

// Check 检查权限
// 返回 "allow", "deny" 或 "" (未记忆)
func (m *Memory) Check(permType, pattern string) Action {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules, ok := m.rules[permType]
	if !ok {
		return ""
	}

	for _, rule := range rules {
		if matchPattern(rule.Pattern, pattern) {
			return rule.Action
		}
	}
	return ""
}

// Remember 记忆权限规则
func (m *Memory) Remember(permType, pattern string, action Action) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在相同规则
	rules := m.rules[permType]
	for i, rule := range rules {
		if rule.Pattern == pattern {
			rules[i].Action = action
			return m.save()
		}
	}

	// 添加新规则
	m.rules[permType] = append(m.rules[permType], Rule{
		Pattern: pattern,
		Action:  action,
	})
	return m.save()
}

// Forget 删除权限规则
func (m *Memory) Forget(permType, pattern string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rules := m.rules[permType]
	for i, rule := range rules {
		if rule.Pattern == pattern {
			m.rules[permType] = append(rules[:i], rules[i+1:]...)
			return m.save()
		}
	}
	return nil
}

// Clear 清除所有规则
func (m *Memory) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = make(map[string][]Rule)
	return m.save()
}

// load 从文件加载规则
func (m *Memory) load() {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &m.rules)
}

// save 保存规则到文件
func (m *Memory) save() error {
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.rules, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath, data, 0644)
}

// matchPattern 匹配模式（支持通配符）
func matchPattern(pattern, target string) bool {
	if pattern == target {
		return true
	}
	// 支持 "git *" 匹配 "git status"
	if strings.HasSuffix(pattern, " *") {
		prefix := strings.TrimSuffix(pattern, " *")
		return strings.HasPrefix(target, prefix+" ")
	}
	// 支持 "*" 匹配所有
	if pattern == "*" {
		return true
	}
	return false
}
