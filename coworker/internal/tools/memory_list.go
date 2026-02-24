package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/coworker/internal/memory"
	"github.com/QuantumNous/new-api/coworker/pkg/types"
)

// MemoryListTool 记忆列表工具
type MemoryListTool struct {
	manager *memory.Manager
}

// NewMemoryListTool 创建记忆列表工具
func NewMemoryListTool(manager *memory.Manager) *MemoryListTool {
	return &MemoryListTool{manager: manager}
}

func (t *MemoryListTool) Name() string {
	return "MemoryList"
}

func (t *MemoryListTool) Description() string {
	return `List user's saved memories, optionally filtered by tags.

Use this tool to:
- See what memories are stored for the user
- Browse memories by category/tag
- Get an overview of user's stored preferences and context`
}

func (t *MemoryListTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"tags": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Filter by tags (optional)",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Max memories to return (default: 10)",
				"default":     10,
			},
		},
	}
}

type memoryListInput struct {
	Tags  []string `json:"tags"`
	Limit int      `json:"limit"`
}

func (t *MemoryListTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	var in memoryListInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   "Invalid input: " + err.Error(),
		}, nil
	}

	userID, _ := ctx.Value(types.UserIDKey).(string)
	if userID == "" {
		return &types.ToolResult{
			Success: false,
			Error:   "User ID not found",
		}, nil
	}

	// 加载记忆
	t.manager.LoadUserMemories(userID)

	// 获取所有记忆
	memories := t.manager.List(userID)

	// 按标签过滤
	if len(in.Tags) > 0 {
		memories = filterByTags(memories, in.Tags)
	}

	// 按权重排序
	sort.Slice(memories, func(i, j int) bool {
		return memories[i].CalculateWeight() > memories[j].CalculateWeight()
	})

	// 限制数量
	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	if len(memories) > limit {
		memories = memories[:limit]
	}

	if len(memories) == 0 {
		return &types.ToolResult{
			Success: true,
			Output:  "No memories found.",
		}, nil
	}

	// 格式化输出
	output := formatMemoryList(memories)

	return &types.ToolResult{
		Success: true,
		Output:  output,
	}, nil
}

func formatMemoryList(memories []*memory.Memory) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[User Memories: %d items]\n\n", len(memories)))

	for i, mem := range memories {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, mem.ID, mem.Summary))
		sb.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(mem.Tags, ", ")))
	}

	return sb.String()
}
