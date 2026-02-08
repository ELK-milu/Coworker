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

	// 流式调用（仅在连接建立失败时重试，流开始后不重试）
	var streamErr error
	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("[API] Stream connection retry %d/%d", attempt, MaxRetries)
		}

		stream := c.client.Beta.Messages.NewStreaming(ctx, params)
		eventStarted := false

		log.Printf("[API] Starting stream processing")
		for stream.Next() {
			event := stream.Current()
			if !eventStarted {
				eventStarted = true
			}
			if event.Type != "content_block_delta" {
				log.Printf("[API] Event type: %s", event.Type)
			}
			c.handleBetaStreamEvent(event, eventCh)
		}

		streamErr = stream.Err()
		if streamErr == nil {
			// 流正常完成
			break
		}

		// 流已经开始产出事件 → 不重试（避免重复内容）
		if eventStarted {
			log.Printf("[API] Stream error after events started, not retrying: %v", streamErr)
			break
		}

		// 流未开始就失败 → 检查是否可重试
		retryErr := isRetryableError(streamErr)
		if retryErr == nil || attempt >= MaxRetries {
			break
		}

		backoff := calculateBackoff(attempt, retryErr.RetryAfterMs)
		log.Printf("[API] Stream connection failed (status=%d), waiting %v before retry",
			retryErr.StatusCode, backoff)

		select {
		case <-ctx.Done():
			streamErr = ctx.Err()
			break
		case <-time.After(backoff):
			continue
		}
		break
	}

	if streamErr != nil {
		log.Printf("[API] Stream error: %v", streamErr)
		eventCh <- StreamEvent{Type: EventError, Error: streamErr.Error()}
	}
	log.Printf("[API] Stream completed")
}

// CreateSimpleMessage 创建简单消息（非流式，用于标题生成等轻量级任务）
func (c *ClaudeClient) CreateSimpleMessage(ctx context.Context, prompt string, maxTokens int64) (string, error) {
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
		Model:     anthropic.Model(c.model),
		MaxTokens: maxTokens,
		Messages:  messages,
	}

	response, err := c.client.Beta.Messages.New(ctx, params)
	if err != nil {
		return "", err
	}

	var text strings.Builder
	for _, block := range response.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	return text.String(), nil
}
