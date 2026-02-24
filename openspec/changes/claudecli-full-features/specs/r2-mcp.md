# R2: MCP 集成规范

## 概述

实现 Model Context Protocol 集成，支持连接外部 MCP 服务器扩展工具能力。

---

## Transport 接口

```go
type Transport interface {
    Connect() error
    Disconnect() error
    Send(msg *Message) error
    Receive() (*Message, error)
    IsConnected() bool
}
```

---

## 消息格式

```go
type Message struct {
    JSONRPC string      `json:"jsonrpc"` // "2.0"
    ID      interface{} `json:"id,omitempty"`
    Method  string      `json:"method,omitempty"`
    Params  interface{} `json:"params,omitempty"`
    Result  interface{} `json:"result,omitempty"`
    Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}
```

---

## 服务器配置

```go
type ServerConfig struct {
    Name    string            `json:"name"`
    Type    string            `json:"type"` // stdio | websocket | http
    Command string            `json:"command,omitempty"`
    Args    []string          `json:"args,omitempty"`
    Env     map[string]string `json:"env,omitempty"`
    URL     string            `json:"url,omitempty"`
    Headers map[string]string `json:"headers,omitempty"`
}
```

---

## Stdio Transport

```go
type StdioTransport struct {
    cmd     *exec.Cmd
    stdin   io.WriteCloser
    stdout  io.ReadCloser
    scanner *bufio.Scanner
    mu      sync.Mutex
}

func (t *StdioTransport) Connect() error
func (t *StdioTransport) Send(msg *Message) error
func (t *StdioTransport) Receive() (*Message, error)
func (t *StdioTransport) Disconnect() error
```

---

## WebSocket Transport

```go
type WebSocketTransport struct {
    url       string
    conn      *websocket.Conn
    headers   http.Header
    mu        sync.Mutex
}

func (t *WebSocketTransport) Connect() error
func (t *WebSocketTransport) Send(msg *Message) error
func (t *WebSocketTransport) Receive() (*Message, error)
func (t *WebSocketTransport) Disconnect() error
```

---

## 连接管理器

```go
type Manager struct {
    connections map[string]*Connection
    tools       map[string]*McpTool
    mu          sync.RWMutex
}

func (m *Manager) Connect(config *ServerConfig) error
func (m *Manager) Disconnect(name string) error
func (m *Manager) ListTools(serverName string) ([]*McpTool, error)
func (m *Manager) CallTool(serverName, toolName string, args map[string]interface{}) (*ToolResult, error)
```

---

## 重连策略

- 指数退避: 1s → 2s → 4s (最大 30s)
- 最大重试: 3 次
- 心跳间隔: 30s

---

## 验收标准

- [ ] Stdio 传输实现
- [ ] WebSocket 传输实现
- [ ] 连接管理器
- [ ] 工具动态注册
- [ ] 重连机制
