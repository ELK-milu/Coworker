package memory

import (
	"context"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/claudecli/internal/embedding"
	"github.com/QuantumNous/new-api/model"
)

// Retriever 记忆检索器
type Retriever struct {
	manager   *Manager
	tokenizer *Tokenizer
	bm25      *BM25Ranker
	expander  *SemanticExpander
	milvus    *MilvusClient       // 可选的向量搜索客户端
	embedding *embedding.Client   // Embedding 客户端
}

// HybridWeights 混合评分权重
type HybridWeights struct {
	BM25   float64 // BM25 权重，默认 0.6
	Vector float64 // 向量相似度权重，默认 0.4
}

// DefaultHybridWeights 默认混合权重
var DefaultHybridWeights = HybridWeights{
	BM25:   0.6,
	Vector: 0.4,
}

// NewRetriever 创建检索器
func NewRetriever(manager *Manager, baseDir string) *Retriever {
	return &Retriever{
		manager:   manager,
		tokenizer: NewTokenizer(),
		bm25:      NewBM25Ranker(),
		expander:  NewSemanticExpander(baseDir),
	}
}

// SetMilvusClient 设置 Milvus 客户端（可选）
func (r *Retriever) SetMilvusClient(client *MilvusClient) {
	r.milvus = client
}

// SetEmbeddingClient 设置 Embedding 客户端（可选）
func (r *Retriever) SetEmbeddingClient(client *embedding.Client) {
	r.embedding = client
}

// ScoredMemory 带分数的记忆
type ScoredMemory struct {
	Memory      *Memory
	BM25Score   float64 // BM25 分数 (0-1)
	VectorScore float64 // 向量相似度分数 (0-1)
	RerankScore float64 // Rerank 分数 (0-1)
	HybridScore float64 // 混合分数
}

// Retrieve 混合检索
// 优先使用 Milvus 原生 HybridSearch (BM25 + Vector + RRF)
// 降级方案: 应用层 BM25 + Vector + Rerank
// DB 路径: 使用 SQL 全文搜索作为补充
func (r *Retriever) Retrieve(userID, query string, limit int) []*Memory {
	ctx := context.Background()

	// 优先使用 Milvus 原生混合检索
	if r.milvus != nil && r.milvus.IsEnabled() && r.embedding != nil {
		results := r.retrieveWithMilvusHybrid(ctx, userID, query, limit)
		if len(results) > 0 {
			return results
		}
	}

	// 应用层混合检索
	results := r.retrieveWithAppLevel(ctx, userID, query, limit)
	if len(results) > 0 {
		return results
	}

	// DB 全文搜索降级（当内存缓存为空时）
	if r.manager.useDB {
		if dbUserID, ok := parseUserID(userID); ok {
			return r.retrieveFromDB(dbUserID, query, limit)
		}
	}

	return nil
}

// retrieveFromDB 从数据库全文搜索
func (r *Retriever) retrieveFromDB(dbUserID int, query string, limit int) []*Memory {
	dbMems, err := model.SearchCoworkerMemories(dbUserID, query, limit)
	if err != nil || len(dbMems) == 0 {
		return nil
	}
	result := make([]*Memory, 0, len(dbMems))
	for _, dbMem := range dbMems {
		result = append(result, dbModelToMemory(dbMem))
	}
	return result
}

// candidate 候选记忆
type candidate struct {
	memory *Memory
	tokens []string
}

// retrieveWithMilvusHybrid 使用 Milvus 原生混合检索
func (r *Retriever) retrieveWithMilvusHybrid(ctx context.Context, userID, query string, limit int) []*Memory {
	// 生成 dense vector
	denseVector, err := r.embedding.Embed(ctx, query)
	if err != nil {
		return nil
	}

	// 生成 sparse vector (BM25)
	sparseVector := r.textToSparseVector(query)

	// 执行 Milvus HybridSearch
	results, err := r.milvus.HybridSearch(ctx, userID, denseVector, sparseVector, limit)
	if err != nil {
		return nil
	}

	// 转换结果
	memories := make([]*Memory, 0, len(results))
	for _, result := range results {
		mem := r.manager.GetByID(userID, result.ID)
		if mem != nil {
			memories = append(memories, mem)
			r.manager.RecordAccess(userID, mem.ID)
		}
	}

	return memories
}

// textToSparseVector 将文本转换为 sparse vector (用于 BM25)
func (r *Retriever) textToSparseVector(text string) map[uint32]float32 {
	tokens := r.tokenizer.Tokenize(text)
	if len(tokens) == 0 {
		return nil
	}

	// 计算词频
	termFreq := make(map[string]int)
	for _, token := range tokens {
		termFreq[token]++
	}

	// 转换为 sparse vector
	sparse := make(map[uint32]float32)
	for term, freq := range termFreq {
		// 使用简单的哈希函数将词映射到索引
		idx := hashString(term)
		sparse[idx] = float32(freq)
	}

	return sparse
}

// hashString 简单的字符串哈希函数
func hashString(s string) uint32 {
	var h uint32 = 0
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return h
}

// retrieveWithAppLevel 应用层混合检索（降级方案）
func (r *Retriever) retrieveWithAppLevel(ctx context.Context, userID, query string, limit int) []*Memory {
	// 分词
	queryTokens := r.tokenizer.Tokenize(query)
	if len(queryTokens) == 0 {
		return nil
	}

	// 语义扩展
	expandedTokens := r.expander.Expand(queryTokens)
	allQueryTokens := uniqueStrings(append(queryTokens, expandedTokens...))

	// 获取所有记忆并分词
	candidates := r.gatherCandidates(userID)
	if len(candidates) == 0 {
		return nil
	}

	// BM25 评分
	scored := r.bm25Score(candidates, allQueryTokens)
	if len(scored) == 0 {
		return nil
	}

	// 向量搜索 (可选)
	if r.milvus != nil && r.milvus.IsEnabled() && r.embedding != nil {
		r.addVectorScores(ctx, userID, query, scored)
	}

	// Rerank 重排序 (可选)
	if r.embedding != nil && len(scored) > 1 {
		r.rerankScores(ctx, query, scored)
	}

	// 混合评分
	r.calculateHybridScores(scored)

	// 按混合分数排序
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].HybridScore > scored[j].HybridScore
	})

	// 取 top K
	if len(scored) > limit {
		scored = scored[:limit]
	}

	// 记录访问并返回
	result := make([]*Memory, len(scored))
	for i, s := range scored {
		result[i] = s.Memory
		r.manager.RecordAccess(s.Memory.UserID, s.Memory.ID)
	}

	return result
}

// gatherCandidates 收集候选记忆
func (r *Retriever) gatherCandidates(userID string) []*candidate {
	r.manager.mu.RLock()
	defer r.manager.mu.RUnlock()

	userMems := r.manager.memories[userID]
	if userMems == nil {
		return nil
	}

	candidates := make([]*candidate, 0, len(userMems))
	for _, mem := range userMems {
		// 合并内容和标签进行分词
		text := mem.Content + " " + mem.Summary + " " + strings.Join(mem.Tags, " ")
		tokens := r.tokenizer.Tokenize(text)
		candidates = append(candidates, &candidate{
			memory: mem,
			tokens: tokens,
		})
	}

	return candidates
}

// bm25Score BM25 评分
func (r *Retriever) bm25Score(candidates []*candidate, queryTokens []string) []*ScoredMemory {
	// 计算 IDF
	allDocs := make([][]string, len(candidates))
	for i, c := range candidates {
		allDocs[i] = c.tokens
	}
	idfScores := r.bm25.CalculateIDF(allDocs)

	// 计算平均文档长度
	var totalLen int
	for _, doc := range allDocs {
		totalLen += len(doc)
	}
	avgDocLength := float64(totalLen) / float64(len(allDocs))

	// BM25 评分
	scored := make([]*ScoredMemory, 0, len(candidates))
	for _, c := range candidates {
		score := r.bm25.Score(queryTokens, c.tokens, avgDocLength, idfScores)
		if score > 0 {
			scored = append(scored, &ScoredMemory{
				Memory:    c.memory,
				BM25Score: score,
			})
		}
	}

	// 归一化
	r.normalizeBM25Scores(scored)
	return scored
}

// addVectorScores 添加向量相似度分数
func (r *Retriever) addVectorScores(ctx context.Context, userID, query string, scored []*ScoredMemory) {
	// 生成查询向量
	queryVector, err := r.embedding.Embed(ctx, query)
	if err != nil {
		return // 向量搜索失败，继续使用 BM25
	}

	// 从 Milvus 搜索
	results, err := r.milvus.Search(ctx, userID, queryVector, len(scored)*2)
	if err != nil {
		return
	}

	// 构建 ID -> 分数映射
	vectorScores := make(map[string]float64)
	for _, r := range results {
		// Milvus L2 距离转相似度 (距离越小越相似)
		vectorScores[r.ID] = 1.0 / (1.0 + float64(r.Distance))
	}

	// 更新分数
	for _, s := range scored {
		if score, ok := vectorScores[s.Memory.ID]; ok {
			s.VectorScore = score
		}
	}
}

// rerankScores 使用 Rerank API 重排序
func (r *Retriever) rerankScores(ctx context.Context, query string, scored []*ScoredMemory) {
	// 准备文档列表
	documents := make([]string, len(scored))
	for i, s := range scored {
		documents[i] = s.Memory.Content
		if s.Memory.Summary != "" {
			documents[i] = s.Memory.Summary + " " + s.Memory.Content
		}
	}

	// 调用 Rerank API
	results, err := r.embedding.Rerank(ctx, query, documents, len(documents))
	if err != nil {
		return // Rerank 失败，继续使用现有分数
	}

	// 更新 Rerank 分数
	for _, result := range results {
		if result.Index < len(scored) {
			scored[result.Index].RerankScore = result.RelevanceScore
		}
	}
}

// calculateHybridScores 计算混合分数
// 权重分配: BM25 30% + Vector 30% + Rerank 40%
func (r *Retriever) calculateHybridScores(scored []*ScoredMemory) {
	for _, s := range scored {
		hasVector := s.VectorScore > 0
		hasRerank := s.RerankScore > 0

		switch {
		case hasVector && hasRerank:
			// 三路混合: BM25 30% + Vector 30% + Rerank 40%
			s.HybridScore = s.BM25Score*0.3 + s.VectorScore*0.3 + s.RerankScore*0.4
		case hasRerank:
			// 双路混合: BM25 40% + Rerank 60%
			s.HybridScore = s.BM25Score*0.4 + s.RerankScore*0.6
		case hasVector:
			// 双路混合: BM25 60% + Vector 40%
			s.HybridScore = s.BM25Score*0.6 + s.VectorScore*0.4
		default:
			// 仅 BM25
			s.HybridScore = s.BM25Score
		}
	}
}

// uniqueStrings 去重字符串
func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(strs))
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// normalizeBM25Scores 归一化 BM25 分数到 0-1 范围
func (r *Retriever) normalizeBM25Scores(scored []*ScoredMemory) {
	if len(scored) == 0 {
		return
	}

	// 找最大分数
	maxScore := scored[0].BM25Score
	for _, s := range scored {
		if s.BM25Score > maxScore {
			maxScore = s.BM25Score
		}
	}

	// 归一化
	if maxScore > 0 {
		for _, s := range scored {
			s.BM25Score = s.BM25Score / maxScore
		}
	}
}


// FormatForPrompt 格式化记忆用于系统提示词
func FormatForPrompt(memories []*Memory) string {
	if len(memories) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, mem := range memories {
		sb.WriteString("## Memory ")
		sb.WriteString(string(rune('A' + i)))
		sb.WriteString("\n")

		if len(mem.Tags) > 0 {
			sb.WriteString("Tags: ")
			sb.WriteString(strings.Join(mem.Tags, ", "))
			sb.WriteString("\n")
		}

		if mem.Summary != "" {
			sb.WriteString(mem.Summary)
		} else {
			// 截取内容
			content := mem.Content
			if len(content) > 300 {
				content = content[:300] + "..."
			}
			sb.WriteString(content)
		}
		sb.WriteString("\n\n")
	}

	return sb.String()
}
