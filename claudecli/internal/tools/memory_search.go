package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/claudecli/internal/memory"
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
)

// MemorySearchTool 记忆搜索工具
// 让 AI 可以主动搜索用户的长期记忆
type MemorySearchTool struct {
	manager *memory.Manager
}

// NewMemorySearchTool 创建记忆搜索工具
func NewMemorySearchTool(manager *memory.Manager) *MemorySearchTool {
	return &MemorySearchTool{manager: manager}
}

func (t *MemorySearchTool) Name() string {
	return "MemorySearch"
}

func (t *MemorySearchTool) Description() string {
	return `Search user's long-term memories for relevant information.

Use this tool when:
- You need to recall user's preferences, past decisions, or project context
- The user asks about something you discussed before
- You want to provide personalized responses based on user history

The search uses hybrid retrieval (BM25 + semantic) for best results.`
}

func (t *MemorySearchTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Natural language query to search memories. Be specific about what you're looking for.",
			},
			"tags": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Optional tags to filter memories (e.g., ['project', 'preferences', 'error'])",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of memories to return (default: 5, max: 10)",
				"default":     5,
			},
		},
		"required": []string{"query"},
	}
}

type memorySearchInput struct {
	Query string   `json:"query"`
	Tags  []string `json:"tags"`
	Limit int      `json:"limit"`
}

func (t *MemorySearchTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	var in memorySearchInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   "Invalid input: " + err.Error(),
		}, nil
	}

	if in.Query == "" {
		return &types.ToolResult{
			Success: false,
			Error:   "Query is required",
		}, nil
	}

	// 获取 userID
	userID, _ := ctx.Value(types.UserIDKey).(string)
	if userID == "" {
		return &types.ToolResult{
			Success: false,
			Error:   "User ID not found in context",
		}, nil
	}

	// 设置默认值
	limit := in.Limit
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}

	// 加载用户记忆（如果尚未加载）
	if err := t.manager.LoadUserMemories(userID); err != nil {
		// 忽略加载错误，可能是首次使用
	}

	// 执行检索
	memories := t.manager.Retrieve(userID, in.Query, limit)

	// 如果指定了标签，进行过滤
	if len(in.Tags) > 0 {
		memories = filterByTags(memories, in.Tags)
	}

	if len(memories) == 0 {
		return &types.ToolResult{
			Success: true,
			Output:  "No relevant memories found for: " + in.Query,
		}, nil
	}

	// 格式化输出
	output := formatMemoriesForTool(memories, in.Query)

	return &types.ToolResult{
		Success: true,
		Output:  output,
	}, nil
}

// filterByTags 按标签过滤记忆
func filterByTags(memories []*memory.Memory, tags []string) []*memory.Memory {
	if len(tags) == 0 {
		return memories
	}

	tagSet := make(map[string]bool)
	for _, tag := range tags {
		tagSet[strings.ToLower(tag)] = true
	}

	var filtered []*memory.Memory
	for _, mem := range memories {
		for _, memTag := range mem.Tags {
			if tagSet[strings.ToLower(memTag)] {
				filtered = append(filtered, mem)
				break
			}
		}
	}
	return filtered
}

// formatMemoriesForTool 格式化记忆用于工具输出
func formatMemoriesForTool(memories []*memory.Memory, query string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("[Memory Search Results for: \"%s\"]\n", query))
	sb.WriteString(fmt.Sprintf("Found %d relevant memories:\n\n", len(memories)))

	for i, mem := range memories {
		sb.WriteString(fmt.Sprintf("--- Memory %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("ID: %s\n", mem.ID))

		if len(mem.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(mem.Tags, ", ")))
		}

		if mem.Summary != "" {
			sb.WriteString(fmt.Sprintf("Summary: %s\n", mem.Summary))
		}

		// 内容截断
		content := mem.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		sb.WriteString(fmt.Sprintf("Content: %s\n", content))

		// 显示动态权重
		weight := mem.CalculateWeight()
		sb.WriteString(fmt.Sprintf("Relevance: %.1f%%\n\n", weight*100))
	}

	return sb.String()
}
