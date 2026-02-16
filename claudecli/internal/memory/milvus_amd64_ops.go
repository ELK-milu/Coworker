//go:build linux && (amd64 || arm64)

package memory

import (
	"context"
	"fmt"
	"log"

	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// Insert 插入向量和文本
func (m *MilvusClient) Insert(ctx context.Context, id, userID, content string, denseVector []float32, sparseVector map[uint32]float32) error {
	if !m.config.Enabled || !m.connected {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// 构建列数据
	idCol := column.NewColumnVarChar("id", []string{id})
	userIDCol := column.NewColumnVarChar("user_id", []string{userID})
	contentCol := column.NewColumnVarChar("content", []string{content})
	denseCol := column.NewColumnFloatVector("dense_vector", m.config.Dimension, [][]float32{denseVector})

	// 构建 sparse vector
	positions := make([]uint32, 0, len(sparseVector))
	values := make([]float32, 0, len(sparseVector))
	for pos, val := range sparseVector {
		positions = append(positions, pos)
		values = append(values, val)
	}
	sparseEmb, err := entity.NewSliceSparseEmbedding(positions, values)
	if err != nil {
		return fmt.Errorf("failed to create sparse embedding: %w", err)
	}
	sparseCol := column.NewColumnSparseVectors("sparse_vector", []entity.SparseEmbedding{sparseEmb})

	// 使用新 SDK 的 Insert 方法
	insertOpt := milvusclient.NewColumnBasedInsertOption(m.config.Collection, idCol, userIDCol, contentCol, denseCol, sparseCol)

	_, err = m.client.Insert(ctx, insertOpt)
	if err != nil {
		return fmt.Errorf("insert failed: %w", err)
	}
	return nil
}

// Search 搜索相似向量 (仅 dense vector)
func (m *MilvusClient) Search(ctx context.Context, userID string, vector []float32, topK int) ([]VectorSearchResult, error) {
	if !m.config.Enabled || !m.connected {
		return nil, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	expr := fmt.Sprintf("user_id == \"%s\"", userID)

	// 使用新 SDK 的 Search 方法
	searchOpt := milvusclient.NewSearchOption(m.config.Collection, topK, []entity.Vector{entity.FloatVector(vector)}).
		WithFilter(expr).
		WithOutputFields("id").
		WithANNSField("dense_vector")

	results, err := m.client.Search(ctx, searchOpt)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	var searchResults []VectorSearchResult
	for _, result := range results {
		for i := 0; i < result.ResultCount; i++ {
			id, _ := result.IDs.GetAsString(i)
			searchResults = append(searchResults, VectorSearchResult{
				ID:       id,
				Score:    result.Scores[i],
				Distance: result.Scores[i],
			})
		}
	}

	return searchResults, nil
}

// HybridSearch 混合搜索 (dense vector + sparse vector + RRF)
func (m *MilvusClient) HybridSearch(ctx context.Context, userID string, denseVector []float32, sparseVector map[uint32]float32, topK int) ([]HybridSearchResult, error) {
	if !m.config.Enabled || !m.connected {
		return nil, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	expr := fmt.Sprintf("user_id == \"%s\"", userID)

	// Dense vector 搜索请求
	denseReq := milvusclient.NewAnnRequest("dense_vector", topK*2, entity.FloatVector(denseVector)).
		WithFilter(expr).
		WithSearchParam("nprobe", "16")

	// 构建 sparse embedding
	positions := make([]uint32, 0, len(sparseVector))
	values := make([]float32, 0, len(sparseVector))
	for pos, val := range sparseVector {
		positions = append(positions, pos)
		values = append(values, val)
	}
	sparseEmb, err := entity.NewSliceSparseEmbedding(positions, values)
	if err != nil {
		return nil, fmt.Errorf("failed to create sparse embedding: %w", err)
	}

	// Sparse vector 搜索请求 (BM25)
	sparseReq := milvusclient.NewAnnRequest("sparse_vector", topK*2, sparseEmb).
		WithFilter(expr)

	// RRF Ranker
	rrfRanker := milvusclient.NewRRFReranker().WithK(60)

	// 使用新 SDK 的 HybridSearch 方法
	hybridOpt := milvusclient.NewHybridSearchOption(m.config.Collection, topK, denseReq, sparseReq).
		WithReranker(rrfRanker).
		WithOutputFields("id")

	results, err := m.client.HybridSearch(ctx, hybridOpt)
	if err != nil {
		return nil, fmt.Errorf("hybrid search failed: %w", err)
	}

	var searchResults []HybridSearchResult
	for _, result := range results {
		for i := 0; i < result.ResultCount; i++ {
			id, _ := result.IDs.GetAsString(i)
			searchResults = append(searchResults, HybridSearchResult{
				ID:    id,
				Score: result.Scores[i],
			})
		}
	}

	return searchResults, nil
}

// Delete 删除向量
func (m *MilvusClient) Delete(ctx context.Context, id string) error {
	if !m.config.Enabled || !m.connected {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	expr := fmt.Sprintf("id == \"%s\"", id)

	// 使用新 SDK 的 Delete 方法
	deleteOpt := milvusclient.NewDeleteOption(m.config.Collection).
		WithExpr(expr)

	_, err := m.client.Delete(ctx, deleteOpt)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}
	return nil
}

// Close 关闭连接
func (m *MilvusClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client != nil && m.connected {
		m.connected = false
		err := m.client.Close(context.Background())
		if err != nil {
			log.Printf("[Milvus] Close error: %v", err)
		}
		return err
	}
	return nil
}

// IsEnabled 检查是否启用
func (m *MilvusClient) IsEnabled() bool {
	return m.config.Enabled && m.connected
}
