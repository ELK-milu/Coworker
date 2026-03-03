package store

import (
	"context"
	"log"
	"strings"
	"time"
	"unicode"

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

// isMostlyChinese 检测文本是否主要为中文（中文字符占比 > 30%）
func isMostlyChinese(text string) bool {
	if text == "" {
		return false
	}
	total := 0
	chinese := 0
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.Is(unicode.Han, r) {
			total++
			if unicode.Is(unicode.Han, r) {
				chinese++
			}
		}
	}
	if total == 0 {
		return false
	}
	return float64(chinese)/float64(total) > 0.3
}

// TranslateText 使用 AI 将文本翻译为简洁的中文
// 如果文本已经是中文则原样返回
func TranslateText(c *client.ClaudeClient, text string) string {
	if text == "" || isMostlyChinese(text) {
		return text
	}

	prompt := "将以下英文文本翻译为简洁的中文，只输出翻译结果，不要其他内容：\n" + text

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := c.CreateSimpleMessage(ctx, prompt, 128)
	if err != nil {
		return text
	}

	translated := strings.TrimSpace(result)
	if translated == "" {
		return text
	}
	return translated
}

// PostImportEnhance 异步增强导入的条目（翻译 name→display_name, description→display_desc + AI 分类）
// 在 goroutine 中执行，不阻塞调用者
func PostImportEnhance(c *client.ClaudeClient, store *Manager, items []StoreItem) {
	if c == nil || store == nil || len(items) == 0 {
		return
	}

	go func() {
		for _, item := range items {
			// 翻译 name → display_name
			if item.DisplayName == "" && item.Name != "" {
				translated := TranslateText(c, item.Name)
				if translated != item.Name {
					item.DisplayName = translated
				}
			}

			// 翻译 description → display_desc
			if item.DisplayDesc == "" && item.Description != "" {
				translated := TranslateText(c, item.Description)
				if translated != item.Description {
					item.DisplayDesc = translated
				}
			}

			// AI 分类
			if item.Category == "" {
				desc := item.Description
				if item.DisplayDesc != "" {
					desc = item.DisplayDesc
				}
				name := item.Name
				if item.DisplayName != "" {
					name = item.DisplayName
				}
				item.Category = ClassifyItem(c, name, desc)
			}

			// 持久化更新
			if item.DisplayName != "" || item.DisplayDesc != "" || item.Category != "" {
				if _, err := store.Update(item.ID, item); err != nil {
					log.Printf("[Store] PostImportEnhance: update %s failed: %v", item.Name, err)
				} else {
					log.Printf("[Store] PostImportEnhance: enhanced %s → display_name=%q, category=%s",
						item.Name, item.DisplayName, item.Category)
				}
			}
		}
		log.Printf("[Store] PostImportEnhance: completed %d items", len(items))
	}()
}
