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
				APIKey:    os.Getenv("ANTHROPIC_API_KEY"),
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
