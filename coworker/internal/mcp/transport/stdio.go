package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// allowedStdioCommands MCP stdio 模式允许执行的命令白名单
var allowedStdioCommands = map[string]bool{
	"npx":     true,
	"node":    true,
	"python":  true,
	"python3": true,
	"uvx":     true,
	"uv":      true,
	"deno":    true,
	"bun":     true,
	"bunx":    true,
}

// dangerousEnvPrefixes 禁止在子进程中设置的危险环境变量前缀
var dangerousEnvPrefixes = []string{
	"LD_PRELOAD", "LD_LIBRARY_PATH", "DYLD_",
	"NODE_OPTIONS", "PYTHONSTARTUP",
}

// shellMetaChars 在参数中禁止出现的 shell 元字符
const shellMetaChars = "|;&$`\\\"'<>(){}!"

// validateStdioCommand 校验 stdio 命令安全性
func validateStdioCommand(cfg *Config) error {
	// 校验命令是否在白名单中
	base := cfg.Command
	// 提取命令基础名（去除路径）
	if idx := strings.LastIndexAny(base, "/\\"); idx >= 0 {
		base = base[idx+1:]
	}
	if !allowedStdioCommands[base] {
		return fmt.Errorf("command %q is not in the allowed list; allowed: npx, node, python, python3, uvx, uv, deno, bun, bunx", cfg.Command)
	}

	// 检查参数中是否含有 shell 元字符
	for _, arg := range cfg.Args {
		if strings.ContainsAny(arg, shellMetaChars) {
			return fmt.Errorf("argument %q contains forbidden shell metacharacters", arg)
		}
	}

	// 过滤危险环境变量
	if len(cfg.Env) > 0 {
		filtered := make([]string, 0, len(cfg.Env))
		for _, env := range cfg.Env {
			key := env
			if idx := strings.Index(env, "="); idx >= 0 {
				key = env[:idx]
			}
			dangerous := false
			for _, prefix := range dangerousEnvPrefixes {
				if strings.HasPrefix(strings.ToUpper(key), prefix) {
					dangerous = true
					break
				}
			}
			if !dangerous {
				filtered = append(filtered, env)
			}
		}
		cfg.Env = filtered
	}

	return nil
}

// StdioTransport 标准输入输出传输
type StdioTransport struct {
	config  *Config
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	msgCh   chan *Message
	running bool
	mu      sync.Mutex
	cancel  context.CancelFunc
}

// NewStdioTransport 创建 Stdio 传输
func NewStdioTransport(cfg *Config) *StdioTransport {
	return &StdioTransport{
		config: cfg,
		msgCh:  make(chan *Message, 100),
	}
}

func (t *StdioTransport) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.running {
		return fmt.Errorf("transport already running")
	}

	// 校验命令安全性
	if err := validateStdioCommand(t.config); err != nil {
		return fmt.Errorf("command validation failed: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	// 创建命令
	t.cmd = exec.CommandContext(ctx, t.config.Command, t.config.Args...)
	if t.config.Cwd != "" {
		t.cmd.Dir = t.config.Cwd
	}
	if len(t.config.Env) > 0 {
		t.cmd.Env = t.config.Env
	}

	var err error
	t.stdin, err = t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin: %w", err)
	}

	t.stdout, err = t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout: %w", err)
	}

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}

	t.running = true
	go t.readLoop()

	return nil
}

func (t *StdioTransport) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return nil
	}

	t.running = false
	if t.cancel != nil {
		t.cancel()
	}

	if t.stdin != nil {
		t.stdin.Close()
	}

	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
	}

	close(t.msgCh)
	return nil
}

func (t *StdioTransport) Send(msg *Message) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return fmt.Errorf("transport not running")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(t.stdin, "%s\n", data)
	return err
}

func (t *StdioTransport) Receive() <-chan *Message {
	return t.msgCh
}

func (t *StdioTransport) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}

func (t *StdioTransport) readLoop() {
	scanner := bufio.NewScanner(t.stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		select {
		case t.msgCh <- &msg:
		default:
		}
	}
}
