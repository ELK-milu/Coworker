package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/coworker/internal/mcp/transport"
	"github.com/QuantumNous/new-api/coworker/internal/store"
	"github.com/QuantumNous/new-api/coworker/pkg/types"
)

const (
	// smitheryAPIBase Smithery Connect API 基地址
	smitheryAPIBase = "https://api.smithery.ai"
	// smitheryNamespace Smithery 命名空间
	smitheryNamespace = "coworker"
)

// SmitheryApiKeyProvider 获取用户全局 Smithery API Key 的接口
// 避免直接依赖 workspace 包（解耦循环引用）
type SmitheryApiKeyProvider interface {
	GetSmitheryApiKey(userID string) string
}

// UserMCPManager 每用户 MCP 连接管理器
// 管理每个用户独立安装的 MCP 服务器连接
type UserMCPManager struct {
	managers    map[string]*Manager // userID → 该用户的 MCP Manager
	store       *store.Manager
	keyProvider SmitheryApiKeyProvider
	mu          sync.RWMutex
}

// NewUserMCPManager 创建每用户 MCP 管理器
func NewUserMCPManager(storeMgr *store.Manager) *UserMCPManager {
	return &UserMCPManager{
		managers: make(map[string]*Manager),
		store:    storeMgr,
	}
}

// SetSmitheryApiKeyProvider 设置全局 API Key 提供者
func (m *UserMCPManager) SetSmitheryApiKeyProvider(p SmitheryApiKeyProvider) {
	m.keyProvider = p
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

	// 获取用户全局 Smithery API Key
	var globalApiKey string
	if m.keyProvider != nil {
		globalApiKey = m.keyProvider.GetSmitheryApiKey(userID)
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

		// 构建传输配置
		cfg := buildMCPTransportConfig(ctx, item.ServerURL, item.Name, item.ConfigSchema, userID, globalApiKey)
		if cfg == nil {
			log.Printf("[UserMCP] Failed to build config for %s: %s", item.Name, item.ServerURL)
			continue
		}

		// 尝试连接（忽略失败，不阻塞）
		if _, err := mgr.Connect(ctx, item.Name, cfg); err != nil {
			log.Printf("[UserMCP] Warning: failed to connect MCP %s: %v", item.Name, err)
		} else {
			log.Printf("[UserMCP] Connected MCP server: %s for user %s (http=%v)", item.Name, userID, cfg.IsHTTP())
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

// buildMCPTransportConfig 根据 ServerURL 构建传输配置
// 如果 schema 包含 apikey 字段且 globalApiKey 非空 → 走 Smithery Connect 代理
// 否则 HTTP URL → 直连，非 HTTP → Stdio
func buildMCPTransportConfig(ctx context.Context, serverURL, itemName string, schema []store.ConfigField, userID, globalApiKey string) *transport.Config {
	serverURL = strings.TrimSpace(serverURL)
	if serverURL == "" {
		return nil
	}

	// Stdio 模式
	if !strings.HasPrefix(serverURL, "http://") && !strings.HasPrefix(serverURL, "https://") {
		parts := strings.Fields(serverURL)
		if len(parts) == 0 {
			return nil
		}
		return &transport.Config{
			Command: parts[0],
			Args:    parts[1:],
		}
	}

	// HTTP 模式 — 检查是否需要走 Smithery Connect
	useSmithery := false
	apiKeyRequired := false
	for _, field := range schema {
		if field.Type == "apikey" {
			useSmithery = true
			if field.Required {
				apiKeyRequired = true
			}
		}
	}

	// 需要 Smithery 但没有 API Key
	if apiKeyRequired && globalApiKey == "" {
		log.Printf("[UserMCP] Required API key not set for %s, skipping", itemName)
		return nil
	}

	// 走 Smithery Connect 代理
	if useSmithery && globalApiKey != "" {
		connID := sanitizeConnectionID(fmt.Sprintf("u%s-%s", userID, itemName))
		mcpEndpoint, err := CreateSmitheryConnection(ctx, globalApiKey, serverURL, connID)
		if err != nil {
			log.Printf("[UserMCP] Smithery Connect failed for %s: %v", itemName, err)
			return nil
		}

		log.Printf("[UserMCP] Smithery connection created: %s → %s", itemName, mcpEndpoint)
		return &transport.Config{
			URL:     mcpEndpoint,
			Headers: map[string]string{"Authorization": "Bearer " + globalApiKey},
			Timeout: 30,
		}
	}

	// 直连 HTTP 模式（无 Smithery）
	return &transport.Config{
		URL:     serverURL,
		Headers: make(map[string]string),
		Timeout: 30,
	}
}

// CreateSmitheryConnection 通过 Smithery Connect API 创建/确认连接
// 返回 MCP 代理端点 URL: https://api.smithery.ai/connect/{namespace}/{connectionId}/mcp
func CreateSmitheryConnection(ctx context.Context, apiKey, mcpURL, connectionID string) (string, error) {
	endpoint := fmt.Sprintf("%s/connect/%s", smitheryAPIBase, smitheryNamespace)

	body, _ := json.Marshal(map[string]interface{}{
		"mcpUrl":       mcpURL,
		"connectionId": connectionID,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("smithery API call: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return "", fmt.Errorf("smithery API %d: %s", resp.StatusCode, string(respBody))
	}

	// 解析响应检查状态
	var result struct {
		ConnectionID string `json:"connectionId"`
		Status       struct {
			State string `json:"state"`
		} `json:"status"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		log.Printf("[UserMCP] Smithery response parse warning: %v, body: %s", err, string(respBody))
	}

	if result.Status.State == "auth_required" {
		return "", fmt.Errorf("OAuth authorization required for this MCP server")
	}

	// 使用返回的 connectionId（可能与请求的不同）
	connID := connectionID
	if result.ConnectionID != "" {
		connID = result.ConnectionID
	}

	// 返回 MCP 代理端点
	return fmt.Sprintf("%s/connect/%s/%s/mcp", smitheryAPIBase, smitheryNamespace, connID), nil
}

// sanitizeConnectionID 清理 connectionId 中的非法字符
var connIDRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func sanitizeConnectionID(id string) string {
	return connIDRegex.ReplaceAllString(id, "-")
}
