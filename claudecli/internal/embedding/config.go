package embedding

import (
	"os"
	"strconv"
)

// Provider Embedding 服务提供商
type Provider string

const (
	ProviderSiliconFlow Provider = "siliconflow" // 硅基流动
	ProviderDashScope   Provider = "dashscope"   // 阿里云百炼
)

// Config Embedding 配置
type Config struct {
	Provider    Provider // 服务提供商
	Model       string   // Embedding 模型名称
	RerankModel string   // Rerank 模型名称
	Dimension   int      // 向量维度

	// 硅基流动配置
	SiliconFlowAPIKey  string
	SiliconFlowBaseURL string

	// 阿里云百炼配置
	DashScopeAPIKey  string
	DashScopeBaseURL string
}

// LoadConfigFromEnv 从环境变量加载配置
func LoadConfigFromEnv() *Config {
	dimension, _ := strconv.Atoi(os.Getenv("EMBEDDING_DIMENSION"))
	if dimension == 0 {
		dimension = 1024 // 默认维度
	}

	return &Config{
		Provider:    Provider(getEnvOrDefault("EMBEDDING_PROVIDER", "siliconflow")),
		Model:       getEnvOrDefault("EMBEDDING_MODEL", "BAAI/bge-large-zh-v1.5"),
		RerankModel: getEnvOrDefault("RERANK_MODEL", "BAAI/bge-reranker-v2-m3"),
		Dimension:   dimension,

		SiliconFlowAPIKey:  os.Getenv("SILICONFLOW_API_KEY"),
		SiliconFlowBaseURL: getEnvOrDefault("SILICONFLOW_BASE_URL", "https://api.siliconflow.cn/v1"),

		DashScopeAPIKey:  os.Getenv("DASHSCOPE_API_KEY"),
		DashScopeBaseURL: getEnvOrDefault("DASHSCOPE_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
	}
}

// GetActiveAPIKey 获取当前 Provider 的 API Key
func (c *Config) GetActiveAPIKey() string {
	switch c.Provider {
	case ProviderSiliconFlow:
		return c.SiliconFlowAPIKey
	case ProviderDashScope:
		return c.DashScopeAPIKey
	default:
		return c.SiliconFlowAPIKey
	}
}

// GetActiveBaseURL 获取当前 Provider 的 Base URL
func (c *Config) GetActiveBaseURL() string {
	switch c.Provider {
	case ProviderSiliconFlow:
		return c.SiliconFlowBaseURL
	case ProviderDashScope:
		return c.DashScopeBaseURL
	default:
		return c.SiliconFlowBaseURL
	}
}

// GetRerankModel 获取 Rerank 模型名称
func (c *Config) GetRerankModel() string {
	if c.RerankModel != "" {
		return c.RerankModel
	}
	// 默认 rerank 模型
	switch c.Provider {
	case ProviderSiliconFlow:
		return "BAAI/bge-reranker-v2-m3"
	case ProviderDashScope:
		return "gte-rerank"
	default:
		return "BAAI/bge-reranker-v2-m3"
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
