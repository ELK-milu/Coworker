package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

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

	// 并发请求支持
	idCounter uint64
	pending   sync.Map // map[requestID]chan *transport.Message
	isHTTP    bool     // HTTP 模式不需要 dispatchLoop
}

// nextID 生成唯一请求 ID
func (c *Connection) nextID() uint64 {
	return atomic.AddUint64(&c.idCounter, 1)
}

// sendAndWait 发送请求并等待匹配 ID 的响应
func (c *Connection) sendAndWait(ctx context.Context, method string, params json.RawMessage, timeout time.Duration) (*transport.Message, error) {
	id := c.nextID()
	ch := make(chan *transport.Message, 1)
	c.pending.Store(id, ch)
	defer c.pending.Delete(id)

	msg := &transport.Message{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	if err := c.Transport.Send(msg); err != nil {
		return nil, fmt.Errorf("send %s: %w", method, err)
	}

	select {
	case resp := <-ch:
		if resp == nil {
			return nil, fmt.Errorf("%s: connection closed", method)
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("%s error: %s (code=%d)", method, resp.Error.Message, resp.Error.Code)
		}
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(timeout):
		return nil, fmt.Errorf("%s: timeout after %v", method, timeout)
	}
}

// sendNotification 发送不需要响应的通知
func (c *Connection) sendNotification(method string, params json.RawMessage) error {
	msg := &transport.Message{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return c.Transport.Send(msg)
}

// dispatchLoop 从 Transport.Receive 读消息，按 msg.ID 分发到 pending channel
// 仅 Stdio 模式需要（HTTP 模式每次请求独立获取响应）
func (c *Connection) dispatchLoop() {
	ch := c.Transport.Receive()
	for msg := range ch {
		if msg.ID == nil {
			// 通知消息，忽略
			continue
		}

		// ID 可能是 float64（JSON 反序列化）或 uint64
		var idKey uint64
		switch v := msg.ID.(type) {
		case float64:
			idKey = uint64(v)
		case uint64:
			idKey = v
		case int:
			idKey = uint64(v)
		case json.Number:
			if n, err := v.Int64(); err == nil {
				idKey = uint64(n)
			}
		default:
			log.Printf("[MCP] Unknown ID type %T: %v", msg.ID, msg.ID)
			continue
		}

		if pendingCh, ok := c.pending.Load(idKey); ok {
			pendingCh.(chan *transport.Message) <- msg
		} else {
			log.Printf("[MCP] No pending request for ID %d", idKey)
		}
	}

	// 连接关闭，通知所有 pending
	c.pending.Range(func(key, value interface{}) bool {
		close(value.(chan *transport.Message))
		return true
	})
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

	// 根据配置选择传输层
	var t transport.Transport
	if cfg.IsHTTP() {
		t = transport.NewHTTPTransport(cfg)
	} else {
		t = transport.NewStdioTransport(cfg)
	}

	if err := t.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start transport: %w", err)
	}

	conn := &Connection{
		ID:        id,
		Name:      name,
		Transport: t,
		running:   true,
		isHTTP:    cfg.IsHTTP(),
	}

	// Stdio 模式需要 dispatchLoop 来从 Receive channel 分发消息
	// HTTP 模式的 Send() 内部直接将响应放入 msgCh，也需要 dispatchLoop
	go conn.dispatchLoop()

	// 初始化连接
	if err := m.initialize(ctx, conn); err != nil {
		t.Stop()
		return nil, err
	}

	m.mu.Lock()
	m.connections[id] = conn
	m.mu.Unlock()

	log.Printf("[MCP] Connected: %s (%s, http=%v)", name, id, conn.isHTTP)
	return conn, nil
}

// initialize 初始化 MCP 连接
func (m *Manager) initialize(ctx context.Context, conn *Connection) error {
	params := json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"coworker","version":"1.0.0"}}`)
	resp, err := conn.sendAndWait(ctx, "initialize", params, 30*time.Second)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	// 解析服务器信息
	var result struct {
		ServerInfo ServerInfo `json:"serverInfo"`
	}
	if err := json.Unmarshal(resp.Result, &result); err == nil {
		conn.Server = &result.ServerInfo
	}

	// 发送 initialized 通知
	conn.sendNotification("notifications/initialized", nil)

	// 获取工具列表
	if err := m.listTools(ctx, conn); err != nil {
		log.Printf("[MCP] Warning: failed to list tools: %v", err)
	}

	return nil
}

// listTools 获取工具列表
func (m *Manager) listTools(ctx context.Context, conn *Connection) error {
	resp, err := conn.sendAndWait(ctx, "tools/list", nil, 30*time.Second)
	if err != nil {
		return err
	}

	var result struct {
		Tools []ToolInfo `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err == nil {
		conn.Tools = result.Tools
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

	// 使用 json.Marshal 安全构建 JSON，防止注入
	paramObj := struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}{
		Name:      toolName,
		Arguments: args,
	}
	params, err := json.Marshal(paramObj)
	if err != nil {
		return nil, fmt.Errorf("marshal tool params: %w", err)
	}

	resp, err := conn.sendAndWait(ctx, "tools/call", params, 120*time.Second)
	if err != nil {
		return nil, err
	}

	return resp.Result, nil
}
