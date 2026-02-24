package mcp

import (
	"context"
	"log"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/coworker/internal/mcp/transport"
	"github.com/QuantumNous/new-api/coworker/internal/store"
	"github.com/QuantumNous/new-api/coworker/pkg/types"
)

// UserMCPManager 每用户 MCP 连接管理器
// 管理每个用户独立安装的 MCP 服务器连接
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
		if item.ServerURL == "" {
			continue
		}
		// 跳过已连接的
		if existingNames[item.Name] {
			continue
		}

		// 解析 ServerURL 为命令 + 参数
		cfg := parseMCPServerURL(item.ServerURL)
		if cfg == nil {
			log.Printf("[UserMCP] Failed to parse server URL for %s: %s", item.Name, item.ServerURL)
			continue
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

// parseMCPServerURL 解析 MCP 服务器 URL 为传输配置
// 支持格式: "command arg1 arg2..."（stdio 模式）
func parseMCPServerURL(serverURL string) *transport.Config {
	parts := strings.Fields(strings.TrimSpace(serverURL))
	if len(parts) == 0 {
		return nil
	}

	return &transport.Config{
		Command: parts[0],
		Args:    parts[1:],
	}
}
