package client

import (
	"github.com/QuantumNous/new-api/claudecli/pkg/types"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Beta headers
const (
	ClaudeCodeBeta = "claude-code-20250219"
	OAuthBeta      = "oauth-2025-04-20"
	ThinkingBeta   = "interleaved-thinking-2025-05-14"
	ClaudeIdentity = "You are Claude Code, Anthropic's official CLI for Claude."
	VersionBase    = "2.1.25"
)

// ClaudeClient API 客户端
type ClaudeClient struct {
	client    anthropic.Client
	model     string
	maxTokens int64
	isOAuth   bool
}

// NewClaudeClient 创建客户端
func NewClaudeClient(apiKey, authToken, baseURL, model string, maxTokens int) *ClaudeClient {
	var client anthropic.Client
	var isOAuth bool

	// 构建默认 headers（与官方 Claude Code 一致）
	defaultHeaders := map[string]string{
		"x-app":      "cli",
		"User-Agent": "claude-cli/" + VersionBase + " (external, claude-code)",
		"anthropic-dangerous-direct-browser-access": "true",
	}

	if authToken != "" {
		// OAuth 模式
		opts := []option.RequestOption{
			option.WithAuthToken(authToken),
		}
		if baseURL != "" {
			opts = append(opts, option.WithBaseURL(baseURL))
		}
		for k, v := range defaultHeaders {
			opts = append(opts, option.WithHeader(k, v))
		}
		client = anthropic.NewClient(opts...)
		isOAuth = true
	} else {
		// API Key 模式
		opts := []option.RequestOption{
			option.WithAPIKey(apiKey),
		}
		if baseURL != "" {
			opts = append(opts, option.WithBaseURL(baseURL))
		}
		for k, v := range defaultHeaders {
			opts = append(opts, option.WithHeader(k, v))
		}
		client = anthropic.NewClient(opts...)
	}

	return &ClaudeClient{
		client:    client,
		model:     model,
		maxTokens: int64(maxTokens),
		isOAuth:   isOAuth,
	}
}

// GetModel 获取当前模型名称
func (c *ClaudeClient) GetModel() string {
	return c.model
}

// buildBetas 构建 beta 头
func buildBetas(model string, isOAuth bool) []anthropic.AnthropicBeta {
	betas := []anthropic.AnthropicBeta{}
	if !strings.Contains(strings.ToLower(model), "haiku") {
		betas = append(betas, anthropic.AnthropicBeta(ClaudeCodeBeta))
	}
	betas = append(betas, anthropic.AnthropicBeta(ThinkingBeta))
	if isOAuth {
		betas = append(betas, anthropic.AnthropicBeta(OAuthBeta))
	}
	return betas
}

// buildBetaString 构建 beta header 字符串
func buildBetaString(model string, isOAuth bool) string {
	var betas []string
	if !strings.Contains(strings.ToLower(model), "haiku") {
		betas = append(betas, ClaudeCodeBeta)
	}
	betas = append(betas, ThinkingBeta)
	if isOAuth {
		betas = append(betas, OAuthBeta)
	}
	return strings.Join(betas, ",")
}

// formatSystemPrompt 格式化系统提示（OAuth 模式需要特殊格式）
func formatSystemPrompt(systemPrompt string, isOAuth bool) []anthropic.BetaTextBlockParam {
	// OAuth 模式需要特殊格式：第一个 block 必须以 ClaudeIdentity 开头
	if isOAuth {
		return []anthropic.BetaTextBlockParam{
			{
				Type: "text",
				Text: ClaudeIdentity,
				CacheControl: anthropic.BetaCacheControlEphemeralParam{
					Type: "ephemeral",
				},
			},
			{
				Type: "text",
				Text: systemPrompt,
				CacheControl: anthropic.BetaCacheControlEphemeralParam{
					Type: "ephemeral",
				},
			},
		}
	}
	// 非 OAuth 模式
	return []anthropic.BetaTextBlockParam{
		{
			Type: "text",
			Text: ClaudeIdentity + "\n\n" + systemPrompt,
		},
	}
}

// buildTools 构建工具列表
func buildTools(tools []types.ToolDefinition) []anthropic.ToolUnionParam {
	result := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		// 提取 properties 和 required
		props, _ := t.InputSchema["properties"]
		req, _ := t.InputSchema["required"].([]string)

		result = append(result, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(t.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: props,
					Required:   req,
				},
			},
		})
	}
	return result
}

// buildBetaTools 构建工具列表（Beta API）
func buildBetaTools(tools []types.ToolDefinition) []anthropic.BetaToolUnionParam {
	result := make([]anthropic.BetaToolUnionParam, 0, len(tools))
	for _, t := range tools {
		props, _ := t.InputSchema["properties"]
		req, _ := t.InputSchema["required"].([]string)

		result = append(result, anthropic.BetaToolUnionParam{
			OfTool: &anthropic.BetaToolParam{
				Name:        t.Name,
				Description: anthropic.String(t.Description),
				InputSchema: anthropic.BetaToolInputSchemaParam{
					Properties: props,
					Required:   req,
				},
			},
		})
	}
	return result
}
