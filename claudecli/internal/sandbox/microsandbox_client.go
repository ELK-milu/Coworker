package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MicrosandboxClient 封装 Microsandbox HTTP API
type MicrosandboxClient struct {
	serverURL  string
	apiKey     string
	namespace  string
	httpClient *http.Client
}

// NewMicrosandboxClient 创建新的 Microsandbox 客户端
func NewMicrosandboxClient(serverURL, apiKey, namespace string) *MicrosandboxClient {
	return &MicrosandboxClient{
		serverURL: serverURL,
		apiKey:    apiKey,
		namespace: namespace,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: true,
			},
		},
	}
}

// JSON-RPC 请求/响应结构
type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
	ID      string `json:"id,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
	ID      string          `json:"id"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// StartSandbox 启动沙箱
func (c *MicrosandboxClient) StartSandbox(ctx context.Context, name, image string, memoryMB, cpus int) error {
	params := map[string]any{
		"namespace": c.namespace,
		"sandbox":   name,
		"config": map[string]any{
			"image":  image,
			"memory": memoryMB,
			"cpus":   cpus,
		},
	}
	_, err := c.call(ctx, "sandbox.start", params)
	return err
}

// StopSandbox 停止沙箱
func (c *MicrosandboxClient) StopSandbox(ctx context.Context, name string) error {
	params := map[string]any{
		"namespace": c.namespace,
		"sandbox":   name,
	}
	_, err := c.call(ctx, "sandbox.stop", params)
	return err
}

// CommandResult 命令执行结果
type CommandResult struct {
	Output   string
	Error    string
	ExitCode int
	Success  bool
}

// outputLine 输出行结构
type outputLine struct {
	Stream string `json:"stream"` // "stdout" or "stderr"
	Text   string `json:"text"`
}

// commandResponse 命令响应结构
type commandResponse struct {
	Output   []outputLine `json:"output"`
	Command  string       `json:"command"`
	Args     []string     `json:"args"`
	ExitCode int          `json:"exit_code"`
	Success  bool         `json:"success"`
}

// RunCommand 在沙箱中执行命令
func (c *MicrosandboxClient) RunCommand(ctx context.Context, name, command string, args []string, timeout int) (*CommandResult, error) {
	params := map[string]any{
		"namespace": c.namespace,
		"sandbox":   name,
		"command":   command,
		"args":      args,
	}
	if timeout > 0 {
		params["timeout"] = timeout
	}

	result, err := c.call(ctx, "sandbox.command.run", params)
	if err != nil {
		return nil, err
	}

	var resp commandResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse command result: %w", err)
	}

	// 分离 stdout 和 stderr
	var stdout, stderr bytes.Buffer
	for _, line := range resp.Output {
		if line.Stream == "stdout" {
			stdout.WriteString(line.Text)
			stdout.WriteString("\n")
		} else if line.Stream == "stderr" {
			stderr.WriteString(line.Text)
			stderr.WriteString("\n")
		}
	}

	return &CommandResult{
		Output:   trimLastNewline(stdout.String()),
		Error:    trimLastNewline(stderr.String()),
		ExitCode: resp.ExitCode,
		Success:  resp.Success,
	}, nil
}

func trimLastNewline(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\n' {
		return s[:len(s)-1]
	}
	return s
}

// SandboxMetrics 沙箱指标
type SandboxMetrics struct {
	Name        string
	Namespace   string
	Running     bool
	CPUUsage    float64
	MemoryUsage int
	DiskUsage   int
}

// GetMetrics 获取沙箱指标
func (c *MicrosandboxClient) GetMetrics(ctx context.Context, name string) (*SandboxMetrics, error) {
	params := map[string]any{
		"namespace": c.namespace,
		"sandbox":   name,
	}

	result, err := c.call(ctx, "sandbox.metrics.get", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Sandboxes []SandboxMetrics `json:"sandboxes"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse metrics: %w", err)
	}

	if len(resp.Sandboxes) == 0 {
		return &SandboxMetrics{Name: name, Namespace: c.namespace}, nil
	}
	return &resp.Sandboxes[0], nil
}

// call 执行 JSON-RPC 调用
func (c *MicrosandboxClient) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	req := &jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      fmt.Sprintf("%d", time.Now().UnixNano()),
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.serverURL + "/api/v1/rpc"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("request failed: status %d: %s", httpResp.StatusCode, string(body))
	}

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var jsonResp jsonRPCResponse
	if err := json.Unmarshal(respBytes, &jsonResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if jsonResp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s (code: %d)", jsonResp.Error.Message, jsonResp.Error.Code)
	}

	return jsonResp.Result, nil
}

// Ping 检查服务器连接
func (c *MicrosandboxClient) Ping(ctx context.Context) error {
	url := c.serverURL + "/api/v1/rpc"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 只要能连接就算成功
	return nil
}

// 错误定义
var (
	ErrMicrosandboxNotAvailable = errors.New("microsandbox server not available")
	ErrSandboxStartFailed       = errors.New("failed to start sandbox")
	ErrSandboxStopFailed        = errors.New("failed to stop sandbox")
	ErrCommandFailed            = errors.New("command execution failed")
)
