package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// HTTPTransport Streamable HTTP 传输（支持 SSE 降级）
type HTTPTransport struct {
	config    *Config
	client    *http.Client
	sessionID string // Mcp-Session-Id 跟踪
	msgCh     chan *Message
	running   bool
	mu        sync.Mutex
	cancel    context.CancelFunc
}

// NewHTTPTransport 创建 HTTP 传输
func NewHTTPTransport(cfg *Config) *HTTPTransport {
	timeout := 30
	if cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}
	return &HTTPTransport{
		config: cfg,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		msgCh: make(chan *Message, 100),
	}
}

func (t *HTTPTransport) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.running {
		return fmt.Errorf("transport already running")
	}

	_, cancel := context.WithCancel(ctx)
	t.cancel = cancel
	t.running = true

	return nil
}

func (t *HTTPTransport) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return nil
	}

	t.running = false
	if t.cancel != nil {
		t.cancel()
	}

	close(t.msgCh)
	return nil
}

// Send 发送 JSON-RPC 请求并处理响应
// Streamable HTTP: POST JSON-RPC → 检查 Content-Type:
//   - application/json → 直接解析放入 msgCh
//   - text/event-stream → SSE 流解析
func (t *HTTPTransport) Send(msg *Message) error {
	t.mu.Lock()
	if !t.running {
		t.mu.Unlock()
		return fmt.Errorf("transport not running")
	}
	t.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", t.config.URL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	// 附加用户自定义 headers（如 Authorization）
	for k, v := range t.config.Headers {
		req.Header.Set(k, v)
	}

	// 附加 session ID
	if t.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", t.sessionID)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}

	// 通知类消息（无 ID）不需要响应
	if msg.ID == nil && msg.Method != "" {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return nil
	}

	// 保存 session ID
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		t.sessionID = sid
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
	}

	ct := resp.Header.Get("Content-Type")

	switch {
	case strings.Contains(ct, "text/event-stream"):
		// SSE 流响应 — 在 goroutine 中解析，避免阻塞
		go t.parseSSE(resp.Body)
	case strings.Contains(ct, "application/json"):
		// 直接 JSON 响应
		defer resp.Body.Close()
		var respMsg Message
		if err := json.NewDecoder(resp.Body).Decode(&respMsg); err != nil {
			return fmt.Errorf("decode json response: %w", err)
		}
		select {
		case t.msgCh <- &respMsg:
		default:
			log.Printf("[MCP-HTTP] msgCh full, dropping message id=%v", respMsg.ID)
		}
	default:
		// 尝试按 JSON 解析
		defer resp.Body.Close()
		var respMsg Message
		if err := json.NewDecoder(resp.Body).Decode(&respMsg); err != nil {
			return fmt.Errorf("unexpected content-type %q", ct)
		}
		select {
		case t.msgCh <- &respMsg:
		default:
		}
	}

	return nil
}

func (t *HTTPTransport) Receive() <-chan *Message {
	return t.msgCh
}

func (t *HTTPTransport) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}

// parseSSE 解析 SSE 流，将 event: message 的 data 解析为 JSON-RPC 消息
func (t *HTTPTransport) parseSSE(body io.ReadCloser) {
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var eventType string
	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// 空行 = 事件结束，分发
			if len(dataLines) > 0 {
				t.dispatchSSEEvent(eventType, strings.Join(dataLines, "\n"))
			}
			eventType = ""
			dataLines = nil
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data:"))
		} else if strings.HasPrefix(line, ":") {
			// 注释行（如 : PING），忽略
			continue
		}
	}

	// 处理最后一个事件（如果没有尾部空行）
	if len(dataLines) > 0 {
		t.dispatchSSEEvent(eventType, strings.Join(dataLines, "\n"))
	}
}

// dispatchSSEEvent 将 SSE 事件数据解析为 JSON-RPC 消息放入 msgCh
func (t *HTTPTransport) dispatchSSEEvent(eventType, data string) {
	// 默认 event type 为 "message"
	if eventType != "" && eventType != "message" {
		// 忽略非 message 事件
		return
	}

	data = strings.TrimSpace(data)
	if data == "" {
		return
	}

	var msg Message
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		log.Printf("[MCP-HTTP] SSE parse error: %v (data=%q)", err, data[:min(len(data), 100)])
		return
	}

	select {
	case t.msgCh <- &msg:
	default:
		log.Printf("[MCP-HTTP] msgCh full, dropping SSE message id=%v", msg.ID)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
