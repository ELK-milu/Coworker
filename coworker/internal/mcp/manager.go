package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/QuantumNous/new-api/coworker/internal/mcp/transport"
)

// TransportConfig 传输配置别名
type TransportConfig = transport.Config

// ServerInfo MCP 服务器信息
type ServerInfo struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
}

// ToolInfo MCP 工具信息
type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Connection MCP 连接
type Connection struct {
	ID        string
	Name      string
	Transport transport.Transport
	Server    *ServerInfo
	Tools     []ToolInfo
	running   bool
	mu        sync.Mutex
}

// Manager MCP 连接管理器
type Manager struct {
	connections map[string]*Connection
	mu          sync.RWMutex
	idCounter   uint64
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		connections: make(map[string]*Connection),
	}
}

// Connect 连接到 MCP 服务器
func (m *Manager) Connect(ctx context.Context, name string, cfg *transport.Config) (*Connection, error) {
	id := fmt.Sprintf("mcp_%d", atomic.AddUint64(&m.idCounter, 1))

	t := transport.NewStdioTransport(cfg)
	if err := t.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start transport: %w", err)
	}

	conn := &Connection{
		ID:        id,
		Name:      name,
		Transport: t,
		running:   true,
	}

	// 初始化连接
	if err := m.initialize(ctx, conn); err != nil {
		t.Stop()
		return nil, err
	}

	m.mu.Lock()
	m.connections[id] = conn
	m.mu.Unlock()

	log.Printf("[MCP] Connected: %s (%s)", name, id)
	return conn, nil
}

// initialize 初始化 MCP 连接
func (m *Manager) initialize(ctx context.Context, conn *Connection) error {
	// 发送 initialize 请求
	initMsg := &transport.Message{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"coworker","version":"1.0.0"}}`),
	}

	if err := conn.Transport.Send(initMsg); err != nil {
		return fmt.Errorf("failed to send initialize: %w", err)
	}

	// 等待响应
	select {
	case msg := <-conn.Transport.Receive():
		if msg.Error != nil {
			return fmt.Errorf("initialize error: %s", msg.Error.Message)
		}
		// 解析服务器信息
		var result struct {
			ServerInfo ServerInfo `json:"serverInfo"`
		}
		if err := json.Unmarshal(msg.Result, &result); err == nil {
			conn.Server = &result.ServerInfo
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	// 发送 initialized 通知
	notifyMsg := &transport.Message{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	conn.Transport.Send(notifyMsg)

	// 获取工具列表
	if err := m.listTools(ctx, conn); err != nil {
		log.Printf("[MCP] Warning: failed to list tools: %v", err)
	}

	return nil
}

// listTools 获取工具列表
func (m *Manager) listTools(ctx context.Context, conn *Connection) error {
	msg := &transport.Message{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	if err := conn.Transport.Send(msg); err != nil {
		return err
	}

	select {
	case resp := <-conn.Transport.Receive():
		if resp.Error != nil {
			return fmt.Errorf("list tools error: %s", resp.Error.Message)
		}
		var result struct {
			Tools []ToolInfo `json:"tools"`
		}
		if err := json.Unmarshal(resp.Result, &result); err == nil {
			conn.Tools = result.Tools
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// Disconnect 断开连接
func (m *Manager) Disconnect(id string) error {
	m.mu.Lock()
	conn, ok := m.connections[id]
	if ok {
		delete(m.connections, id)
	}
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("connection not found: %s", id)
	}

	conn.Transport.Stop()
	log.Printf("[MCP] Disconnected: %s", id)
	return nil
}

// Get 获取连接
func (m *Manager) Get(id string) *Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connections[id]
}

// List 列出所有连接
func (m *Manager) List() []*Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*Connection, 0, len(m.connections))
	for _, conn := range m.connections {
		list = append(list, conn)
	}
	return list
}

// CallTool 调用 MCP 工具
func (m *Manager) CallTool(ctx context.Context, connID, toolName string, args json.RawMessage) (json.RawMessage, error) {
	conn := m.Get(connID)
	if conn == nil {
		return nil, fmt.Errorf("connection not found: %s", connID)
	}

	msg := &transport.Message{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  json.RawMessage(fmt.Sprintf(`{"name":"%s","arguments":%s}`, toolName, args)),
	}

	if err := conn.Transport.Send(msg); err != nil {
		return nil, err
	}

	select {
	case resp := <-conn.Transport.Receive():
		if resp.Error != nil {
			return nil, fmt.Errorf("tool error: %s", resp.Error.Message)
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
