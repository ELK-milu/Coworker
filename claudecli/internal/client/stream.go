package client

import (
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
	"context"
	"log"

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

	// 创建流（使用 Beta API）
	stream := c.client.Beta.Messages.NewStreaming(ctx, anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: c.maxTokens,
		System:    systemBlocks,
		Messages:  apiMessages,
		Tools:     buildBetaTools(tools),
		Betas:     betas,
	})

	// 处理流事件
	log.Printf("[API] Starting stream processing")
	for stream.Next() {
		event := stream.Current()
		if event.Type != "content_block_delta" {
			log.Printf("[API] Event type: %s", event.Type)
		}
		c.handleBetaStreamEvent(event, eventCh)
	}

	if err := stream.Err(); err != nil {
		log.Printf("[API] Stream error: %v", err)
		eventCh <- StreamEvent{Type: EventError, Error: err.Error()}
	}
	log.Printf("[API] Stream completed")
}
