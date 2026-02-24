package tools

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Truncation 工具输出截断器
// 参考 OpenCode tool/truncation.ts 实现
// 所有工具输出自动经过截断处理，防止大输出消耗过多 token
type Truncation struct {
	maxLines     int
	maxBytes     int
	outputDir    string
	retentionMs  int64
	mu           sync.Mutex
	cleanupOnce  sync.Once
	cleanupDone  chan struct{}
}

const (
	DefaultMaxLines   = 2000
	DefaultMaxBytes   = 50 * 1024 // 50KB
	RetentionDuration = 7 * 24 * time.Hour
	CleanupInterval   = 1 * time.Hour
)

// TruncateResult 截断结果
type TruncateResult struct {
	Content    string
	Truncated  bool
	OutputPath string // 完整输出保存路径（仅截断时有值）
}

// NewTruncation 创建截断器
func NewTruncation(dataDir string) *Truncation {
	outputDir := filepath.Join(dataDir, "tool-output")
	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("[Truncation] Failed to create output dir: %v", err)
	}

	t := &Truncation{
		maxLines:    DefaultMaxLines,
		maxBytes:    DefaultMaxBytes,
		outputDir:   outputDir,
		retentionMs: int64(RetentionDuration / time.Millisecond),
		cleanupDone: make(chan struct{}),
	}

	return t
}

// StartCleanup 启动定期清理（后台 goroutine）
func (t *Truncation) StartCleanup() {
	t.cleanupOnce.Do(func() {
		go t.cleanupLoop()
	})
}

// StopCleanup 停止清理
func (t *Truncation) StopCleanup() {
	select {
	case t.cleanupDone <- struct{}{}:
	default:
	}
}

// cleanupLoop 定期清理过期截断文件
func (t *Truncation) cleanupLoop() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.cleanup()
		case <-t.cleanupDone:
			return
		}
	}
}

// cleanup 清理过期的截断输出文件
func (t *Truncation) cleanup() {
	cutoff := time.Now().Add(-RetentionDuration)

	entries, err := os.ReadDir(t.outputDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(t.outputDir, entry.Name())
			if err := os.Remove(path); err == nil {
				log.Printf("[Truncation] Cleaned up expired file: %s", entry.Name())
			}
		}
	}
}

// TruncateOutput 截断工具输出
// direction: "head" 保留头部（默认），"tail" 保留尾部
func (t *Truncation) TruncateOutput(text string, direction string) *TruncateResult {
	if direction == "" {
		direction = "head"
	}

	lines := strings.Split(text, "\n")
	totalBytes := len(text)

	// 未超限，直接返回
	if len(lines) <= t.maxLines && totalBytes <= t.maxBytes {
		return &TruncateResult{
			Content:   text,
			Truncated: false,
		}
	}

	// 需要截断
	var out []string
	bytes := 0
	hitBytes := false

	if direction == "head" {
		for i := 0; i < len(lines) && i < t.maxLines; i++ {
			lineSize := len(lines[i])
			if i > 0 {
				lineSize++ // +1 for newline
			}
			if bytes+lineSize > t.maxBytes {
				hitBytes = true
				break
			}
			out = append(out, lines[i])
			bytes += lineSize
		}
	} else {
		// tail: 从后向前收集
		for i := len(lines) - 1; i >= 0 && len(out) < t.maxLines; i-- {
			lineSize := len(lines[i])
			if len(out) > 0 {
				lineSize++ // +1 for newline
			}
			if bytes+lineSize > t.maxBytes {
				hitBytes = true
				break
			}
			out = append([]string{lines[i]}, out...)
			bytes += lineSize
		}
	}

	// 计算被移除的量
	var removed int
	var unit string
	if hitBytes {
		removed = totalBytes - bytes
		unit = "bytes"
	} else {
		removed = len(lines) - len(out)
		unit = "lines"
	}

	preview := strings.Join(out, "\n")

	// 保存完整输出到文件
	outputPath := t.saveFullOutput(text)

	// 构建截断提示
	hint := fmt.Sprintf(
		"The tool output was truncated. Full output saved to: %s\n"+
			"Use Grep to search the full content or Read with offset/limit to view specific sections.",
		outputPath,
	)

	var message string
	if direction == "head" {
		message = fmt.Sprintf("%s\n\n...%d %s truncated...\n\n%s", preview, removed, unit, hint)
	} else {
		message = fmt.Sprintf("...%d %s truncated...\n\n%s\n\n%s", removed, unit, hint, preview)
	}

	return &TruncateResult{
		Content:    message,
		Truncated:  true,
		OutputPath: outputPath,
	}
}

// saveFullOutput 保存完整输出到文件
func (t *Truncation) saveFullOutput(text string) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	filename := fmt.Sprintf("tool_%d.txt", time.Now().UnixNano())
	outputPath := filepath.Join(t.outputDir, filename)

	if err := os.WriteFile(outputPath, []byte(text), 0644); err != nil {
		log.Printf("[Truncation] Failed to save full output: %v", err)
		return "(failed to save)"
	}

	return outputPath
}
