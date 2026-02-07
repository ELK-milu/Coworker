package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// RerankRequest Rerank 请求 (Jina/Cohere 兼容格式)
type RerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n,omitempty"`
}

// RerankResponse Rerank 响应
type RerankResponse struct {
	Results []RerankResult `json:"results"`
}

// RerankResult 单个重排结果
type RerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

// Rerank 对文档进行重排序
func (c *Client) Rerank(ctx context.Context, query string, documents []string, topN int) ([]RerankResult, error) {
	if len(documents) == 0 {
		return nil, nil
	}

	apiKey := c.config.GetActiveAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("rerank API key not configured")
	}

	// 获取 rerank 模型和 URL
	baseURL := c.config.GetActiveBaseURL()
	rerankModel := c.config.GetRerankModel()
	url := baseURL + "/rerank"

	reqBody := RerankRequest{
		Model:     rerankModel,
		Query:     query,
		Documents: documents,
		TopN:      topN,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rerank request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create rerank request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send rerank request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read rerank response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rerank API error (status %d): %s", resp.StatusCode, string(body))
	}

	var rerankResp RerankResponse
	if err := json.Unmarshal(body, &rerankResp); err != nil {
		return nil, fmt.Errorf("failed to parse rerank response: %w", err)
	}

	return rerankResp.Results, nil
}
