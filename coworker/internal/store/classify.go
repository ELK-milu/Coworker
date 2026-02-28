package store

import (
	"context"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/coworker/internal/client"
)

// ValidCategories 有效分类列表
var ValidCategories = []string{"自动化", "工具", "开发", "API", "文档", "数据", "创作", "搜索", "其他"}

// ClassifyItem 使用 AI 对条目进行分类
func ClassifyItem(c *client.ClaudeClient, name, description string) string {
	prompt := "你是一个分类器。根据以下工具的名称和描述，从这些分类中选择最合适的一个，只回复分类名称，不要其他内容。\n" +
		"分类列表：自动化, 工具, 开发, API, 文档, 数据, 创作, 搜索, 其他\n\n" +
		"名称：" + name + "\n描述：" + description

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := c.CreateSimpleMessage(ctx, prompt, 32)
	if err != nil {
		return "其他"
	}

	cat := strings.TrimSpace(result)
	for _, v := range ValidCategories {
		if cat == v {
			return cat
		}
	}
	return "其他"
}
