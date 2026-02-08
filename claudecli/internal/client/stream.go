package client

import (
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
	"context"
	"log"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// CreateMessageStream 创建流式消息
func (c *ClaudeClient) CreateMessageStream(
	ctx context.Context,
	messages []types.Message,
	tools []types.ToolDefinition,
	systemPrompt string,
) (<-chan StreamEvent, error) {
	eventCh := make(chan StreamEvent, 100)

	go func() {
		defer close(eventCh)
		c.streamMessages(ctx, messages, tools, systemPrompt, eventCh)
	}()

	return eventCh, nil
}

func (c *ClaudeClient) streamMessages(
	ctx context.Context,
	messages []types.Message,
	tools []types.ToolDefinition,
	systemPrompt string,
	eventCh chan<- StreamEvent,
) {
	// 构建消息
	apiMessages := convertBetaMessages(messages)

	// 构建 betas
	betas := buildBetas(c.model, c.isOAuth)
	log.Printf("[API] Model: %s, OAuth: %v, Betas: %v", c.model, c.isOAuth, betas)
	log.Printf("[API] Messages count: %d", len(apiMessages))

	// 构建系统提示（OAuth 模式需要特殊格式）
	systemBlocks := formatSystemPrompt(systemPrompt, c.isOAuth)

	params := anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: c.maxTokens,
		System:    systemBlocks,
		Messages:  apiMessages,
		Tools:     buildBetaTools(tools),
		Betas:     betas,
	}

	// 带重试的流式调用
	err := retryWithBackoff(ctx, "streamMessages", func() error {
		stream := c.client.Beta.Messages.NewStreaming(ctx, params)

		log.Printf("[API] Starting stream processing")
		for stream.Next() {
			event := stream.Current()
			if event.Type != "content_block_delta" {
				log.Printf("[API] Event type: %s", event.Type)
			}
			c.handleBetaStreamEvent(event, eventCh)
		}

		if err := stream.Err(); err != nil {
			// 检查是否可重试（仅在未产生任何输出时重试）
			return err
		}
		return nil
	})

	if err != nil {
		log.Printf("[API] Stream error after retries: %v", err)
		eventCh <- StreamEvent{Type: EventError, Error: err.Error()}
	}
	log.Printf("[API] Stream completed")
}

// CreateSimpleMessage 创建简单消息（非流式，用于标题生成等轻量级任务）
func (c *ClaudeClient) CreateSimpleMessage(ctx context.Context, prompt string, maxTokens int64) (string, error) {
	// 使用 Haiku 模型降低成本
	model := "claude-3-5-haiku-latest"

	messages := []anthropic.BetaMessageParam{
		{
			Role: "user",
			Content: []anthropic.BetaContentBlockParamUnion{
				{
					OfText: &anthropic.BetaTextBlockParam{
						Type: "text",
						Text: prompt,
					},
				},
			},
		},
	}

	params := anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: maxTokens,
		Messages:  messages,
	}

	var result string
	err := retryWithBackoff(ctx, "CreateSimpleMessage", func() error {
		response, err := c.client.Beta.Messages.New(ctx, params)
		if err != nil {
			return err
		}

		var text strings.Builder
		for _, block := range response.Content {
			if block.Type == "text" {
				text.WriteString(block.Text)
			}
		}
		result = text.String()
		return nil
	})

	return result, err
}
