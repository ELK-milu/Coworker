package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/QuantumNous/new-api/coworker/internal/mcp/transport"
	"github.com/QuantumNous/new-api/coworker/internal/store"
	"github.com/QuantumNous/new-api/coworker/pkg/types"
)

// MCPServerConfig 魔搭等平台提供的 MCP 服务器配置
type MCPServerConfig struct {
	Type    string            `json:"type"`              // "streamable_http"
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// MCPJsonWrapper 用户粘贴的 MCP 配置 JSON 外层结构
type MCPJsonWrapper struct {
	MCPServers map[string]*MCPServerConfig `json:"mcpServers"`
}

// ParseMCPJson 解析用户粘贴的 MCP 配置 JSON
// 返回 serverName 和 MCPServerConfig
func ParseMCPJson(raw string) (string, *MCPServerConfig, error) {
	if raw == "" {
		return "", nil, fmt.Errorf("MCP 配置 JSON 为空")
	}

	var wrapper MCPJsonWrapper
	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
		return "", nil, fmt.Errorf("JSON 解析失败: %w", err)
	}

	if len(wrapper.MCPServers) == 0 {
		return "", nil, fmt.Errorf("mcpServers 为空，请检查 JSON 格式")
	}

	// 提取第一个（也是唯一一个）server entry
	for name, cfg := range wrapper.MCPServers {
		if cfg == nil {
			return "", nil, fmt.Errorf("服务器 %s 配置为空", name)
		}
		if cfg.URL == "" {
			return "", nil, fmt.Errorf("服务器 %s 缺少 url 字段", name)
		}
		return name, cfg, nil
	}

	return "", nil, fmt.Errorf("mcpServers 为空")
}

// ValidateMCPJson 校验 JSON 中的服务名是否匹配 MCP 条目名
func ValidateMCPJson(raw string, expectedName string) error {
	name, _, err := ParseMCPJson(raw)
	if err != nil {
		return err
	}
	if name != expectedName {
		return fmt.Errorf("服务名不匹配: JSON 中为 %q，期望 %q", name, expectedName)
	}
	return nil
}

// UserMCPManager 每用户 MCP 连接管理器
type UserMCPManager struct {
	managers map[string]*Manager // userID → 该用户的 MCP Manager
	store    *store.Manager
	mu       sync.RWMutex
}

// NewUserMCPManager 创建每用户 MCP 管理器
func NewUserMCPManager(storeMgr *store.Manager) *UserMCPManager {
	return &UserMCPManager{
		managers: make(map[string]*Manager),
		store:    storeMgr,
	}
}

// getOrCreateManager 获取或创建用户的 MCP Manager
func (m *UserMCPManager) getOrCreateManager(userID string) *Manager {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mgr, ok := m.managers[userID]; ok {
		return mgr
	}
	mgr := NewManager()
	m.managers[userID] = mgr
	return mgr
}

// EnsureConnected 确保用户已安装的 TypeMCP 条目已连接
func (m *UserMCPManager) EnsureConnected(ctx context.Context, userID string) error {
	if m.store == nil || userID == "" {
		return nil
	}

	mgr := m.getOrCreateManager(userID)

	// 获取已连接的服务器名列表
	existingNames := make(map[string]bool)
	for _, conn := range mgr.List() {
		existingNames[conn.Name] = true
	}

	// 遍历用户已安装的 MCP 条目
	ids := m.store.LoadUserInstalled(userID)
	for _, id := range ids {
		item := m.store.GetByID(id)
		if item == nil || item.Type != store.TypeMCP {
			continue
		}
		// 跳过已连接的
		if existingNames[item.Name] {
			continue
		}

		// 读取用户配置的 MCP JSON
		mcpJson := m.store.GetUserMCPJson(userID, id)
		if mcpJson == "" {
			continue // 用户未配置
		}

		// 解析 MCP JSON
		_, serverCfg, err := ParseMCPJson(mcpJson)
		if err != nil {
			log.Printf("[UserMCP] Failed to parse MCP JSON for %s: %v", item.Name, err)
			continue
		}

		// 构建 transport.Config
		cfg := &transport.Config{
			URL:     serverCfg.URL,
			Headers: serverCfg.Headers,
			Timeout: 30,
		}

		// 尝试连接（忽略失败，不阻塞）
		if _, err := mgr.Connect(ctx, item.Name, cfg); err != nil {
			log.Printf("[UserMCP] Warning: failed to connect MCP %s: %v", item.Name, err)
		} else {
			log.Printf("[UserMCP] Connected MCP server: %s for user %s", item.Name, userID)
		}
	}

	return nil
}

// GetToolsForUser 获取用户所有已连接 MCP 的工具列表
func (m *UserMCPManager) GetToolsForUser(userID string) []types.Tool {
	m.mu.RLock()
	mgr, ok := m.managers[userID]
	m.mu.RUnlock()

	if !ok {
		return nil
	}

	var allTools []types.Tool
	for _, conn := range mgr.List() {
		tools := WrapConnectionTools(conn, mgr)
		allTools = append(allTools, tools...)
	}
	return allTools
}

// DisconnectUser 断开用户所有 MCP 连接
func (m *UserMCPManager) DisconnectUser(userID string) {
	m.mu.Lock()
	mgr, ok := m.managers[userID]
	if ok {
		delete(m.managers, userID)
	}
	m.mu.Unlock()

	if !ok {
		return
	}

	for _, conn := range mgr.List() {
		if err := mgr.Disconnect(conn.ID); err != nil {
			log.Printf("[UserMCP] Failed to disconnect %s: %v", conn.Name, err)
		}
	}
}
