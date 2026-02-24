package transport

import (
	"context"
	"encoding/json"
)

// Message MCP JSON-RPC 消息
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError JSON-RPC 错误
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Transport MCP 传输层接口
type Transport interface {
	// Start 启动传输
	Start(ctx context.Context) error
	// Stop 停止传输
	Stop() error
	// Send 发送消息
	Send(msg *Message) error
	// Receive 接收消息通道
	Receive() <-chan *Message
	// IsRunning 是否运行中
	IsRunning() bool
}

// Config 传输配置
type Config struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Env     []string `json:"env"`
	Cwd     string   `json:"cwd"`
}
