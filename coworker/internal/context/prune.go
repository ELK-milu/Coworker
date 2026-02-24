package context

import (
	"github.com/QuantumNous/new-api/coworker/pkg/types"
)

// Prune 层常量
const (
	PRUNE_MINIMUM = 20000 // 最小修剪量（tokens）
	PRUNE_PROTECT = 40000 // 保护最近的 token 数
)

// PruneResult 修剪结果
type PruneResult struct {
	PrunedMessages int // 被修剪的消息数
	SavedTokens    int // 节省的 token 数
}

// Prune 基于 token 数修剪旧的工具输出
// 保护最近 PRUNE_PROTECT tokens 的消息
func Prune(messages []types.Message, currentTokens int) ([]types.Message, *PruneResult) {
	if currentTokens < PRUNE_MINIMUM {
		return messages, &PruneResult{}
	}

	result := &PruneResult{}

	// 计算每条消息的 token 数和累计 token 数（从后往前）
	messageTokens := make([]int, len(messages))
	cumulativeTokens := make([]int, len(messages))

	totalTokens := 0
	for i := len(messages) - 1; i >= 0; i-- {
		tokens := EstimateMessageTokens(messages[i])
		messageTokens[i] = tokens
		totalTokens += tokens
		cumulativeTokens[i] = totalTokens
	}

	// 从旧到新遍历，修剪工具输出
	prunedMessages := make([]types.Message, len(messages))
	copy(prunedMessages, messages)

	for i := 0; i < len(prunedMessages); i++ {
		// 检查是否在保护范围内
		tokensFromEnd := 0
		if i < len(cumulativeTokens) {
			tokensFromEnd = cumulativeTokens[i]
		}

		if tokensFromEnd <= PRUNE_PROTECT {
			// 在保护范围内，不修剪
			break
		}

		// 修剪工具输出
		msg := &prunedMessages[i]
		if pruned := pruneToolResults(msg); pruned {
			result.PrunedMessages++
			newTokens := EstimateMessageTokens(*msg)
			result.SavedTokens += messageTokens[i] - newTokens
		}
	}

	return prunedMessages, result
}

// pruneToolResults 修剪消息中的工具结果
func pruneToolResults(msg *types.Message) bool {
	if len(msg.Content) == 0 {
		return false
	}

	pruned := false
	for i, block := range msg.Content {
		// 使用类型断言检查是否为 ToolResultBlock
		if toolResult, ok := block.(types.ToolResultBlock); ok {
			if len(toolResult.Content) > 500 {
				toolResult.Content = "[pruned: tool output truncated]"
				msg.Content[i] = toolResult
				pruned = true
			}
		}
		// 也检查 map[string]interface{} 类型（JSON 反序列化后的格式）
		if blockMap, ok := block.(map[string]interface{}); ok {
			if blockType, ok := blockMap["type"].(string); ok && blockType == "tool_result" {
				if content, ok := blockMap["content"].(string); ok && len(content) > 500 {
					blockMap["content"] = "[pruned: tool output truncated]"
					pruned = true
				}
			}
		}
	}

	return pruned
}
