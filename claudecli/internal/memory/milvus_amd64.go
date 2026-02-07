//go:build amd64 || arm64

package memory

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// MilvusClient Milvus 客户端封装 (64位系统实现)
type MilvusClient struct {
	config    MilvusConfig
	client    *milvusclient.Client
	mu        sync.RWMutex
	connected bool
}

// NewMilvusClient 创建 Milvus 客户端
func NewMilvusClient(cfg MilvusConfig) *MilvusClient {
	return &MilvusClient{
		config: cfg,
	}
}

// Connect 连接到 Milvus
func (m *MilvusClient) Connect(ctx context.Context) error {
	if !m.config.Enabled {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.connected {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", m.config.Host, m.config.Port)
	cli, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address: addr,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to Milvus: %w", err)
	}

	m.client = cli
	m.connected = true
	log.Printf("[Milvus] Connected to %s", addr)

	return m.ensureCollection(ctx)
}

// ensureCollection 确保 collection 存在
func (m *MilvusClient) ensureCollection(ctx context.Context) error {
	has, err := m.client.HasCollection(ctx, milvusclient.NewHasCollectionOption(m.config.Collection))
	if err != nil {
		return err
	}

	if has {
		_, err := m.client.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(m.config.Collection))
		return err
	}

	// 创建 Schema
	schema := entity.NewSchema().
		WithName(m.config.Collection).
		WithDescription("Memory vectors for Claude CLI with BM25 support").
		WithField(entity.NewField().
			WithName("id").
			WithDataType(entity.FieldTypeVarChar).
			WithIsPrimaryKey(true).
			WithMaxLength(64)).
		WithField(entity.NewField().
			WithName("user_id").
			WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(64)).
		WithField(entity.NewField().
			WithName("content").
			WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(65535)).
		WithField(entity.NewField().
			WithName("dense_vector").
			WithDataType(entity.FieldTypeFloatVector).
			WithDim(int64(m.config.Dimension))).
		WithField(entity.NewField().
			WithName("sparse_vector").
			WithDataType(entity.FieldTypeSparseVector))

	err = m.client.CreateCollection(ctx, milvusclient.NewCreateCollectionOption(m.config.Collection, schema))
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	// 创建 dense vector 索引
	denseIdx := index.NewGenericIndex("IVF_FLAT", map[string]string{
		"metric_type": "L2",
		"nlist":       "128",
	})
	_, err = m.client.CreateIndex(ctx, milvusclient.NewCreateIndexOption(m.config.Collection, "dense_vector", denseIdx))
	if err != nil {
		return fmt.Errorf("failed to create dense index: %w", err)
	}

	// 创建 sparse vector 索引
	sparseIdx := index.NewGenericIndex("SPARSE_INVERTED_INDEX", map[string]string{
		"drop_ratio_build": "0.2",
	})
	_, err = m.client.CreateIndex(ctx, milvusclient.NewCreateIndexOption(m.config.Collection, "sparse_vector", sparseIdx))
	if err != nil {
		log.Printf("[Milvus] Warning: failed to create sparse index: %v", err)
	}

	_, err = m.client.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(m.config.Collection))
	return err
}
