package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/coworker/internal/memory"
	"github.com/QuantumNous/new-api/coworker/pkg/types"
)

// MemorySaveTool 记忆保存工具
// 让 AI 可以主动保存重要信息到用户的长期记忆
type MemorySaveTool struct {
	manager *memory.Manager
}

// NewMemorySaveTool 创建记忆保存工具
func NewMemorySaveTool(manager *memory.Manager) *MemorySaveTool {
	return &MemorySaveTool{manager: manager}
}

func (t *MemorySaveTool) Name() string {
	return "MemorySave"
}

func (t *MemorySaveTool) Description() string {
	return `Save or update information in user's long-term memory.

Use this tool when:
- User explicitly asks you to remember something
- You discover important user preferences or decisions
- User shares project context that should be remembered
- You solve a problem that might be useful to recall later
- You need to update an existing memory with new information

If you provide an existing memory ID, the memory will be updated (overwritten).
If no ID is provided, a new memory will be created.
Be selective - only save truly valuable information.`
}

func (t *MemorySaveTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Optional. If provided, update the existing memory with this ID. If the ID doesn't exist, create a new memory with this ID.",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The information to remember. Be concise but complete.",
			},
			"summary": map[string]interface{}{
				"type":        "string",
				"description": "A brief one-line summary of the memory.",
			},
			"tags": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Tags to categorize this memory (e.g., 'preferences', 'project', 'error', 'decision')",
			},
			"weight": map[string]interface{}{
				"type":        "number",
				"description": "Importance weight from 0.1 to 1.0 (default: 0.5). Higher = more important.",
				"default":     0.5,
			},
		},
		"required": []string{"content", "tags"},
	}
}

type memorySaveInput struct {
	ID      string   `json:"id"`
	Content string   `json:"content"`
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
	Weight  float64  `json:"weight"`
}

func (t *MemorySaveTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	var in memorySaveInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   "Invalid input: " + err.Error(),
		}, nil
	}

	// 验证输入
	if in.Content == "" {
		return &types.ToolResult{
			Success: false,
			Error:   "Content is required",
		}, nil
	}

	if len(in.Tags) == 0 {
		return &types.ToolResult{
			Success: false,
			Error:   "At least one tag is required",
		}, nil
	}

	// 获取 userID 和 sessionID
	userID, _ := ctx.Value(types.UserIDKey).(string)
	if userID == "" {
		return &types.ToolResult{
			Success: false,
			Error:   "User ID not found in context",
		}, nil
	}

	sessionID, _ := ctx.Value(types.SessionIDKey).(string)

	// 设置默认权重
	weight := in.Weight
	if weight <= 0 || weight > 1 {
		weight = 0.5
	}

	// 构建记忆对象
	mem := &memory.Memory{
		Content:   in.Content,
		Summary:   in.Summary,
		Tags:      normalizeTags(in.Tags),
		Weight:    weight,
		Source:    "ai_saved",
		SessionID: sessionID,
	}

	var savedMem *memory.Memory
	var isNew bool

	if in.ID != "" {
		// 有 ID：upsert（存在则覆盖，不存在则以该 ID 创建）
		mem.ID = in.ID
		var err error
		savedMem, isNew, err = t.manager.UpsertByID(userID, mem)
		if err != nil {
			return &types.ToolResult{
				Success: false,
				Error:   "Failed to save memory: " + err.Error(),
			}, nil
		}
	} else {
		// 无 ID：创建新记忆
		var err error
		savedMem, err = t.manager.Create(userID, mem)
		if err != nil {
			return &types.ToolResult{
				Success: false,
				Error:   "Failed to save memory: " + err.Error(),
			}, nil
		}
		isNew = true
	}

	// 格式化输出
	action := "created"
	if !isNew {
		action = "updated"
	}
	output := fmt.Sprintf(
		"Memory %s successfully!\nID: %s\nTags: %s\nSummary: %s",
		action,
		savedMem.ID,
		strings.Join(savedMem.Tags, ", "),
		savedMem.Summary,
	)

	return &types.ToolResult{
		Success: true,
		Output:  output,
	}, nil
}

// normalizeTags 标准化标签
func normalizeTags(tags []string) []string {
	normalized := make([]string, 0, len(tags))
	seen := make(map[string]bool)

	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" && !seen[tag] {
			seen[tag] = true
			normalized = append(normalized, tag)
		}
	}

	return normalized
}
