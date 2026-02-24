package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

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
