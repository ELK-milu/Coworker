package config

import (
	"os"
	"strconv"
	"sync"
	"time"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig
	Claude   ClaudeConfig
	Security SecurityConfig
	Nsjail   NsjailConfig
	Milvus   MilvusConfig
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host string
	Port int
}

// ClaudeConfig Claude API 配置
type ClaudeConfig struct {
	APIKey    string
	AuthToken string
	BaseURL   string
	Model     string
	MaxTokens int
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	WorkingDir      string
	AllowedDirs     []string
	BlockedCommands []string
}

// NsjailConfig nsjail 进程沙箱配置
type NsjailConfig struct {
	Enabled       bool          // 是否启用 nsjail
	ContainerName string        // nsjail 容器名称
	MaxConcurrent int           // 最大并发数
	MemoryMB      int           // 内存限制 (MB)
	ExecTimeout   time.Duration // 命令执行超时
}

// MilvusConfig Milvus 向量数据库配置
type MilvusConfig struct {
	Enabled    bool   // 是否启用
	Host       string // Milvus 主机
	Port       int    // Milvus 端口
	Collection string // Collection 名称
	Dimension  int    // 向量维度
	EnableBM25 bool   // 启用 BM25 混合检索
}

var (
	cfg  *Config
	once sync.Once
)

// Load 加载配置
func Load() *Config {
	once.Do(func() {
		cfg = &Config{
			Server: ServerConfig{
				Host: getEnv("SERVER_HOST", "0.0.0.0"),
				Port: 8080,
			},
			Claude: ClaudeConfig{
				APIKey:    "", // 不再从环境变量读取，用户必须在配置面板选择自己的令牌
				AuthToken: os.Getenv("ANTHROPIC_AUTH_TOKEN"),
				BaseURL:   getEnv("ANTHROPIC_BASE_URL", getEnv("ANTHROPIC_API_BASE_URL", "")),
				Model:     getEnv("CLAUDE_MODEL", "claude-sonnet-4-20250514"),
				MaxTokens: 16000,
			},
			Security: SecurityConfig{
				WorkingDir:  getEnv("WORKSPACE_BASE_PATH", getEnv("WORKING_DIR", "./userdata")),
				AllowedDirs: []string{"."},
				BlockedCommands: []string{
					"rm -rf /", "mkfs", "dd if=",
					":(){ :|:& };:", "chmod -R 777 /",
				},
			},
			Nsjail: NsjailConfig{
				Enabled:       getEnv("NSJAIL_ENABLED", "true") == "true",
				ContainerName: getEnv("NSJAIL_CONTAINER_NAME", "nsjail-sandbox"),
				MaxConcurrent: int(getEnvInt("NSJAIL_MAX_CONCURRENT", 50)),
				MemoryMB:      int(getEnvInt("NSJAIL_MEMORY_MB", 512)),
				ExecTimeout:   time.Duration(getEnvInt("NSJAIL_EXEC_TIMEOUT", 120)) * time.Second,
			},
			Milvus: MilvusConfig{
				Enabled:    getEnv("MILVUS_ENABLED", "false") == "true",
				Host:       getEnv("MILVUS_HOST", "localhost"),
				Port:       int(getEnvInt("MILVUS_PORT", 19530)),
				Collection: getEnv("MILVUS_COLLECTION", "claude_memories"),
				Dimension:  int(getEnvInt("EMBEDDING_DIMENSION", 1024)),
				EnableBM25: getEnv("MILVUS_ENABLE_BM25", "true") == "true",
			},
		}
	})
	return cfg
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int64) int64 {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

// Get 获取配置
func Get() *Config {
	return Load()
}
