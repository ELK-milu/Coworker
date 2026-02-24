package variable

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Variable 变量定义
type Variable struct {
	Name        string `json:"name"`        // 变量名 (如 {{current_time}})
	Description string `json:"description"` // 描述
	Type        string `json:"type"`        // static/dynamic/computed
	Value       string `json:"value"`       // 静态值或默认值
}

// ResolveContext 变量解析上下文
type ResolveContext struct {
	UserID     string
	SessionID  string
	WorkDir    string
	ProjectDir string
	Metadata   map[string]interface{}
}

// Resolver 变量解析函数类型
type Resolver func(ctx *ResolveContext) string

// Manager 变量管理器
type Manager struct {
	baseDir   string
	resolvers map[string]Resolver
	userVars  map[string]map[string]*Variable // userID -> varName -> Variable
	mu        sync.RWMutex
}

// NewManager 创建变量管理器
func NewManager(baseDir string) *Manager {
	m := &Manager{
		baseDir:   baseDir,
		resolvers: make(map[string]Resolver),
		userVars:  make(map[string]map[string]*Variable),
	}
	// 注册内置变量
	m.registerBuiltinVariables()
	return m
}

// RegisterResolver 注册变量解析器
func (m *Manager) RegisterResolver(name string, resolver Resolver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resolvers[name] = resolver
}

// Resolve 解析模板中的所有变量
func (m *Manager) Resolve(template string, ctx *ResolveContext) string {
	if template == "" || ctx == nil {
		return template
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := template

	// 1. 解析内置变量
	for name, resolver := range m.resolvers {
		if strings.Contains(result, name) {
			value := resolver(ctx)
			result = strings.ReplaceAll(result, name, value)
		}
	}

	// 2. 解析用户自定义变量
	if userVars, ok := m.userVars[ctx.UserID]; ok {
		for name, v := range userVars {
			if strings.Contains(result, name) {
				result = strings.ReplaceAll(result, name, v.Value)
			}
		}
	}

	return result
}

// ResolveAll 解析所有已知变量并返回映射
func (m *Manager) ResolveAll(ctx *ResolveContext) map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string)

	// 内置变量
	for name, resolver := range m.resolvers {
		result[name] = resolver(ctx)
	}

	// 用户变量
	if userVars, ok := m.userVars[ctx.UserID]; ok {
		for name, v := range userVars {
			result[name] = v.Value
		}
	}

	return result
}

// SetUserVariable 设置用户自定义变量
func (m *Manager) SetUserVariable(userID string, v *Variable) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.userVars[userID] == nil {
		m.userVars[userID] = make(map[string]*Variable)
	}

	// 验证变量名格式
	if !isValidVariableName(v.Name) {
		return fmt.Errorf("invalid variable name: %s (must be {{name}})", v.Name)
	}

	m.userVars[userID][v.Name] = v

	// 持久化
	return m.saveUserVariables(userID)
}

// DeleteUserVariable 删除用户自定义变量
func (m *Manager) DeleteUserVariable(userID, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.userVars[userID] != nil {
		delete(m.userVars[userID], name)
		return m.saveUserVariables(userID)
	}
	return nil
}

// GetUserVariables 获取用户所有自定义变量
func (m *Manager) GetUserVariables(userID string) []*Variable {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Variable
	if userVars, ok := m.userVars[userID]; ok {
		for _, v := range userVars {
			result = append(result, v)
		}
	}
	return result
}

// ListBuiltinVariables 列出所有内置变量
func (m *Manager) ListBuiltinVariables() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var names []string
	for name := range m.resolvers {
		names = append(names, name)
	}
	return names
}

// LoadUserVariables 加载用户变量
func (m *Manager) LoadUserVariables(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := m.getUserVariablesPath(userID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var vars map[string]*Variable
	if err := json.Unmarshal(data, &vars); err != nil {
		return err
	}

	m.userVars[userID] = vars
	return nil
}

// saveUserVariables 保存用户变量
func (m *Manager) saveUserVariables(userID string) error {
	path := m.getUserVariablesPath(userID)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m.userVars[userID], "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// getUserVariablesPath 获取用户变量文件路径
func (m *Manager) getUserVariablesPath(userID string) string {
	return filepath.Join(m.baseDir, userID, ".coworker", "variables.json")
}

// isValidVariableName 验证变量名格式
func isValidVariableName(name string) bool {
	matched, _ := regexp.MatchString(`^\{\{[a-z_][a-z0-9_]*\}\}$`, name)
	return matched
}
