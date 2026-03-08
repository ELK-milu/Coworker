package sandbox

import (
	"errors"
	"os"
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

// Mount 额外挂载点
type Mount struct {
	RealPath    string // 宿主机上的真实路径
	VirtualPath string // 沙箱内的虚拟路径（如 /.skill）
	ReadOnly    bool   // 是否只读
}

// Sandbox 沙箱，用于隔离用户工作空间
type Sandbox struct {
	userID      string  // 用户 ID
	realBase    string  // 真实路径基础: /app/userdata/{user_id}/workspace
	virtualBase string  // 虚拟路径基础: /workspace
	extraMounts []Mount // 额外挂载点
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

// AddMount 添加额外挂载点（如 /.skill → userdata/{uid}/.skill）
func (s *Sandbox) AddMount(virtualPath, realPath string, readOnly bool) {
	s.extraMounts = append(s.extraMounts, Mount{
		RealPath:    filepath.Clean(realPath),
		VirtualPath: virtualPath,
		ReadOnly:    readOnly,
	})
}

// ExtraMounts 返回额外挂载点列表
func (s *Sandbox) ExtraMounts() []Mount {
	return s.extraMounts
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
	} else if s.matchExtraMount(normalizedPath, &realPath) {
		// 额外挂载路径匹配成功，realPath 已被赋值
	} else if strings.HasPrefix(normalizedPath, "/") {
		// 其他绝对路径（如 /etc/passwd）- 拒绝
		return "", ErrPathOutsideSandbox
	} else {
		// 相对路径 -> 相对于真实工作目录
		realPath = filepath.Join(s.realBase, normalizedPath)
	}

	// 最终验证：确保路径在沙箱内（主工作区或额外挂载）
	if err := s.validateAll(realPath); err != nil {
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

	// 先检查额外挂载（优先级高于主工作区，避免子目录误匹配）
	for _, m := range s.extraMounts {
		cleanMount := filepath.Clean(m.RealPath)
		if strings.HasPrefix(cleanPath, cleanMount) {
			relPath := strings.TrimPrefix(cleanPath, cleanMount)
			relPath = strings.TrimPrefix(relPath, "/")
			relPath = strings.TrimPrefix(relPath, "\\")
			if relPath == "" {
				return m.VirtualPath
			}
			return filepath.ToSlash(filepath.Join(m.VirtualPath, relPath))
		}
	}

	cleanBase := filepath.Clean(s.realBase)

	// 检查是否在主工作区内
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

// validate 验证路径是否在沙箱内（仅检查主工作区）
func (s *Sandbox) validate(realPath string) error {
	cleanPath := filepath.Clean(realPath)
	cleanBase := filepath.Clean(s.realBase)

	// 解析符号链接
	if _, err := os.Lstat(cleanPath); err == nil {
		if r, err := filepath.EvalSymlinks(cleanPath); err == nil {
			cleanPath = r
		}
	}

	sep := string(filepath.Separator)
	if cleanPath == cleanBase || strings.HasPrefix(cleanPath, cleanBase+sep) {
		return nil
	}

	return ErrPathOutsideSandbox
}

// validateAll 验证路径是否在主工作区或任何额外挂载内
// 会解析符号链接以防止通过 symlink 逃逸沙箱
func (s *Sandbox) validateAll(realPath string) error {
	cleanPath := filepath.Clean(realPath)

	// 解析符号链接获取真实物理路径
	resolved := cleanPath
	if _, err := os.Lstat(cleanPath); err == nil {
		// 文件/目录存在时解析 symlink
		if r, err := filepath.EvalSymlinks(cleanPath); err == nil {
			resolved = r
		}
	} else {
		// 文件不存在时解析父目录的 symlink
		dir := filepath.Dir(cleanPath)
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			resolved = filepath.Join(r, filepath.Base(cleanPath))
		}
	}

	// 使用路径分隔符后缀防止 "workspace_evil" 匹配 "workspace"
	sep := string(filepath.Separator)

	// 检查主工作区
	base := filepath.Clean(s.realBase)
	if resolved == base || strings.HasPrefix(resolved, base+sep) {
		return nil
	}

	// 检查额外挂载
	for _, m := range s.extraMounts {
		mountBase := filepath.Clean(m.RealPath)
		if resolved == mountBase || strings.HasPrefix(resolved, mountBase+sep) {
			return nil
		}
	}

	return ErrPathOutsideSandbox
}

// matchExtraMount 匹配额外挂载路径，匹配成功时设置 realPath 并返回 true
func (s *Sandbox) matchExtraMount(normalizedPath string, realPath *string) bool {
	for _, m := range s.extraMounts {
		if strings.HasPrefix(normalizedPath, m.VirtualPath) {
			relPath := strings.TrimPrefix(normalizedPath, m.VirtualPath)
			relPath = strings.TrimPrefix(relPath, "/")
			if relPath == "" {
				*realPath = m.RealPath
			} else {
				*realPath = filepath.Join(m.RealPath, relPath)
			}
			return true
		}
	}
	return false
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

	// 替换额外挂载的真实路径为虚拟路径
	for _, m := range s.extraMounts {
		output = strings.ReplaceAll(output, m.RealPath, m.VirtualPath)
	}

	// 替换主工作区路径
	return strings.ReplaceAll(output, s.realBase, s.virtualBase)
}
