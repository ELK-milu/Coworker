package tools

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// FileTime 文件修改时间追踪器
// 用于检测文件是否被外部修改，防止覆盖用户手动编辑的内容
type FileTime struct {
	mu    sync.RWMutex
	times map[string]int64 // path -> modTime (UnixNano)
}

// NewFileTime 创建文件时间追踪器
func NewFileTime() *FileTime {
	return &FileTime{
		times: make(map[string]int64),
	}
}

// Record 记录文件的当前修改时间
func (ft *FileTime) Record(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，记录为 0（新文件）
			ft.mu.Lock()
			ft.times[path] = 0
			ft.mu.Unlock()
			return nil
		}
		return err
	}

	ft.mu.Lock()
	ft.times[path] = info.ModTime().UnixNano()
	ft.mu.Unlock()
	return nil
}

// Update 更新文件的记录时间（写入/编辑后调用）
func (ft *FileTime) Update(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	ft.mu.Lock()
	ft.times[path] = info.ModTime().UnixNano()
	ft.mu.Unlock()
	return nil
}

// Check 检查文件是否被外部修改
// 返回 true 表示文件未被修改（安全），false 表示被外部修改
func (ft *FileTime) Check(path string) (bool, error) {
	ft.mu.RLock()
	recorded, exists := ft.times[path]
	ft.mu.RUnlock()

	if !exists {
		// 未记录过，视为安全（首次操作）
		return true, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件被删除
			if recorded == 0 {
				return true, nil // 之前也不存在
			}
			return false, nil // 之前存在，现在被删除
		}
		return false, err
	}

	currentModTime := info.ModTime().UnixNano()
	return currentModTime == recorded, nil
}

// AssertNotModified 断言文件未被外部修改
// 如果被修改则返回错误
func (ft *FileTime) AssertNotModified(path string) error {
	safe, err := ft.Check(path)
	if err != nil {
		return fmt.Errorf("failed to check file time: %w", err)
	}
	if !safe {
		return fmt.Errorf(
			"file %s has been modified externally since last read/write. "+
				"Please read the file again to see the current content before editing",
			path,
		)
	}
	return nil
}

// Remove 移除文件的时间记录
func (ft *FileTime) Remove(path string) {
	ft.mu.Lock()
	delete(ft.times, path)
	ft.mu.Unlock()
}

// WithLock 在文件锁保护下执行操作
// 1. 检查文件是否被外部修改
// 2. 执行操作
// 3. 更新文件时间记录
func (ft *FileTime) WithLock(path string, fn func() error) error {
	// 检查文件是否被外部修改
	if err := ft.AssertNotModified(path); err != nil {
		return err
	}

	// 执行操作
	if err := fn(); err != nil {
		return err
	}

	// 更新时间记录
	return ft.Update(path)
}

// RecordAfterRead 读取文件后记录时间（供 Read 工具调用）
func (ft *FileTime) RecordAfterRead(path string) {
	_ = ft.Record(path)
}

// GetRecordedTime 获取记录的修改时间（调试用）
func (ft *FileTime) GetRecordedTime(path string) (time.Time, bool) {
	ft.mu.RLock()
	defer ft.mu.RUnlock()

	t, exists := ft.times[path]
	if !exists || t == 0 {
		return time.Time{}, exists
	}
	return time.Unix(0, t), true
}
