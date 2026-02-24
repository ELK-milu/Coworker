//go:build !linux || (!amd64 && !arm64)

package memory

import (
	"context"
	"log"
	"sync"
)

// MilvusClient Milvus 客户端封装 (32位系统 stub 实现)
// 32位系统不支持 Milvus SDK，提供空实现
type MilvusClient struct {
	config    MilvusConfig
	mu        sync.RWMutex
	connected bool
}

// NewMilvusClient 创建 Milvus 客户端 (stub)
func NewMilvusClient(cfg MilvusConfig) *MilvusClient {
	if cfg.Enabled {
		log.Printf("[Milvus] Warning: Milvus is not supported on 32-bit systems, vector search disabled")
	}
	return &MilvusClient{
		config: cfg,
	}
}

// Connect 连接到 Milvus (stub - 不执行任何操作)
func (m *MilvusClient) Connect(ctx context.Context) error {
	return nil
}

// Insert 插入向量 (stub - 不执行任何操作)
func (m *MilvusClient) Insert(ctx context.Context, id, userID, content string, denseVector []float32, sparseVector map[uint32]float32) error {
	return nil
}

// Search 搜索相似向量 (stub - 返回空结果)
func (m *MilvusClient) Search(ctx context.Context, userID string, vector []float32, topK int) ([]VectorSearchResult, error) {
	return nil, nil
}

// HybridSearch 混合搜索 (stub - 返回空结果)
func (m *MilvusClient) HybridSearch(ctx context.Context, userID string, denseVector []float32, sparseVector map[uint32]float32, topK int) ([]HybridSearchResult, error) {
	return nil, nil
}

// Delete 删除向量 (stub - 不执行任何操作)
func (m *MilvusClient) Delete(ctx context.Context, id string) error {
	return nil
}

// Close 关闭连接 (stub - 不执行任何操作)
func (m *MilvusClient) Close() error {
	return nil
}

// IsEnabled 检查是否启用 (stub - 始终返回 false)
func (m *MilvusClient) IsEnabled() bool {
	return false
}
