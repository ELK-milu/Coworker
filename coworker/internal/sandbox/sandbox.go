package sandbox

import (
	"errors"
	"path/filepath"
	"strings"
)

// 虚拟工作空间根路径
const VirtualWorkspaceRoot = "/workspace"

// 错误定义
var (
	ErrPathOutsideSandbox = errors.New("path is outside sandbox")
	ErrPathTraversal      = errors.New("path traversal detected")
	ErrSandboxNotInit     = errors.New("sandbox not initialized")
)

// Sandbox 沙箱，用于隔离用户工作空间
type Sandbox struct {
	userID      string // 用户 ID
	realBase    string // 真实路径基础: /app/userdata/{user_id}/workspace
	virtualBase string // 虚拟路径基础: /workspace
}

// NewSandbox 创建新的沙箱实例
func NewSandbox(userID, realWorkDir string) *Sandbox {
	return &Sandbox{
		userID:      userID,
		realBase:    filepath.Clean(realWorkDir),
		virtualBase: VirtualWorkspaceRoot,
	}
}

// GetUserID 获取用户 ID
func (s *Sandbox) GetUserID() string {
	return s.userID
}

// GetRealWorkingDir 获取真实工作目录
func (s *Sandbox) GetRealWorkingDir() string {
	return s.realBase
}

// GetVirtualWorkingDir 获取虚拟工作目录
func (s *Sandbox) GetVirtualWorkingDir() string {
	return s.virtualBase
}

// ToReal 将虚拟路径转换为真实路径（带安全验证）
func (s *Sandbox) ToReal(virtualPath string) (string, error) {
	if s == nil {
		return "", ErrSandboxNotInit
	}

	// 统一使用正斜杠处理
	normalizedPath := filepath.ToSlash(virtualPath)

	// 检查路径遍历攻击
	if containsTraversal(normalizedPath) {
		return "", ErrPathTraversal
	}

	var realPath string

	// 处理不同类型的路径
	if strings.HasPrefix(normalizedPath, s.virtualBase) {
		// 虚拟路径 /workspace/... -> 真实路径
		relPath := strings.TrimPrefix(normalizedPath, s.virtualBase)
		relPath = strings.TrimPrefix(relPath, "/")
		if relPath == "" {
			realPath = s.realBase
		} else {
			realPath = filepath.Join(s.realBase, relPath)
		}
	} else if strings.HasPrefix(normalizedPath, "/") {
		// 其他绝对路径（如 /etc/passwd）- 拒绝
		return "", ErrPathOutsideSandbox
	} else {
		// 相对路径 -> 相对于真实工作目录
		realPath = filepath.Join(s.realBase, normalizedPath)
	}

	// 最终验证：确保路径在沙箱内
	if err := s.validate(realPath); err != nil {
		return "", err
	}

	return realPath, nil
}

// ToVirtual 将真实路径转换为虚拟路径
func (s *Sandbox) ToVirtual(realPath string) string {
	if s == nil {
		return realPath
	}

	cleanPath := filepath.Clean(realPath)
	cleanBase := filepath.Clean(s.realBase)

	// 检查是否在沙箱内
	if strings.HasPrefix(cleanPath, cleanBase) {
		relPath := strings.TrimPrefix(cleanPath, cleanBase)
		relPath = strings.TrimPrefix(relPath, "/")
		relPath = strings.TrimPrefix(relPath, "\\")
		if relPath == "" {
			return s.virtualBase
		}
		return filepath.ToSlash(filepath.Join(s.virtualBase, relPath))
	}

	// 不在沙箱内，返回原路径（不应该发生）
	return realPath
}

// validate 验证路径是否在沙箱内
func (s *Sandbox) validate(realPath string) error {
	cleanPath := filepath.Clean(realPath)
	cleanBase := filepath.Clean(s.realBase)

	// 确保路径在沙箱基础目录内
	if !strings.HasPrefix(cleanPath, cleanBase) {
		return ErrPathOutsideSandbox
	}

	// 额外检查：确保不是通过符号链接逃逸
	// 注意：这里只做基本检查，生产环境可能需要更严格的检查
	return nil
}

// containsTraversal 检查路径是否包含遍历攻击
func containsTraversal(path string) bool {
	// 分割路径
	parts := strings.Split(filepath.ToSlash(path), "/")
	depth := 0

	for _, part := range parts {
		switch part {
		case "..":
			depth--
			if depth < 0 {
				return true
			}
		case "", ".":
			// 忽略空和当前目录
		default:
			depth++
		}
	}

	return false
}

// VirtualizePaths 批量虚拟化路径列表
func (s *Sandbox) VirtualizePaths(realPaths []string) []string {
	if s == nil {
		return realPaths
	}

	result := make([]string, len(realPaths))
	for i, p := range realPaths {
		result[i] = s.ToVirtual(p)
	}
	return result
}

// VirtualizeOutput 虚拟化输出字符串中的路径
func (s *Sandbox) VirtualizeOutput(output string) string {
	if s == nil || output == "" {
		return output
	}

	// 替换真实路径为虚拟路径
	return strings.ReplaceAll(output, s.realBase, s.virtualBase)
}
