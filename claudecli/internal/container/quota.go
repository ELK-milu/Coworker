package container

import (
	"fmt"
	"os"
	"path/filepath"
)

// CheckDiskQuota 检查用户工作空间是否超过磁盘配额
func (cm *ContainerManager) CheckDiskQuota(userID string) error {
	if cm.config.DiskMB <= 0 {
		return nil // 不限制
	}

	usage, err := cm.GetDiskUsage(userID)
	if err != nil {
		return fmt.Errorf("check disk usage: %w", err)
	}

	limitBytes := cm.config.DiskMB * 1024 * 1024
	if usage > limitBytes {
		usedMB := usage / (1024 * 1024)
		return fmt.Errorf("disk quota exceeded: %dMB used, %dMB allowed. Please delete some files to free up space", usedMB, cm.config.DiskMB)
	}

	return nil
}

// GetDiskUsage 计算用户工作空间的磁盘使用量 (bytes)
func (cm *ContainerManager) GetDiskUsage(userID string) (int64, error) {
	workDir := filepath.Join(cm.baseDir, userID, "workspace")

	var totalSize int64
	err := filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// 跳过无法访问的文件
			return nil
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	return totalSize, err
}

// GetDiskUsageMB 返回用户工作空间使用量（MB）
func (cm *ContainerManager) GetDiskUsageMB(userID string) (int64, error) {
	bytes, err := cm.GetDiskUsage(userID)
	if err != nil {
		return 0, err
	}
	return bytes / (1024 * 1024), nil
}

// GetDiskQuotaMB 返回磁盘配额（MB）
func (cm *ContainerManager) GetDiskQuotaMB() int64 {
	return cm.config.DiskMB
}
