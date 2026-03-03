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
	prompt := `你是一个 MCP 工具/技能分类专家。根据以下工具的名称和描述，从给定分类中选择最合适的一个。

## 分类定义

- 自动化：工作流自动化、CI/CD、定时任务、批量处理、浏览器自动化、RPA（如 n8n、Puppeteer、GitHub Actions、cron 相关）
- 工具：通用实用工具、文件处理、格式转换、计算器、日历、翻译、天气、系统管理（不属于其他明确分类的通用工具）
- 开发：编程辅助、代码生成/分析/重构、IDE 集成、调试、测试框架、Git 操作、包管理、终端命令（如 ESLint、Prettier、Jest、Docker）
- API：API 网关、REST/GraphQL 客户端、第三方服务集成、Webhook、OAuth、SDK 封装（如 Stripe、Twilio、Slack API、Postman）
- 文档：文档生成/管理、知识库、笔记、Wiki、Markdown 处理、PDF 操作、OCR（如 Notion、Confluence、Docusaurus）
- 数据：数据库操作、数据分析/可视化、ETL、爬虫、Excel/CSV 处理、BI 报表、向量数据库（如 PostgreSQL、Pandas、Tableau）
- 创作：AI 绘图、音视频生成/编辑、设计工具、写作辅助、内容生成、社交媒体管理（如 DALL-E、Midjourney、Figma、写作助手）
- 搜索：网页搜索、语义搜索、知识检索、RAG、搜索引擎集成、信息聚合（如 Google Search、Brave Search、Perplexity、Exa）
- 其他：不属于以上任何分类的工具

## 规则

1. 只回复分类名称（如"开发"），不要其他任何内容
2. 如果工具横跨多个分类，选择最核心的那个
3. 如果完全无法判断，回复"其他"

## 待分类工具

名称：` + name + "\n描述：" + description

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := c.CreateSimpleMessage(ctx, prompt, 64)
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
