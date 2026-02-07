package memory

// MilvusConfig Milvus 配置
type MilvusConfig struct {
	Enabled    bool
	Host       string
	Port       int
	Collection string
	Dimension  int  // 向量维度，取决于 embedding 模型
	EnableBM25 bool // 启用 BM25 全文搜索
}

// VectorSearchResult 向量搜索结果
type VectorSearchResult struct {
	ID       string
	Score    float32
	Distance float32
}

// HybridSearchResult 混合搜索结果
type HybridSearchResult struct {
	ID          string
	Score       float32
	VectorScore float32
	BM25Score   float32
}
