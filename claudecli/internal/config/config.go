package config

import (
	"os"
	"sync"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig
	Claude   ClaudeConfig
	Security SecurityConfig
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
				WorkingDir:  getEnv("WORKING_DIR", "."),
				AllowedDirs: []string{"."},
				BlockedCommands: []string{
					"rm -rf /", "mkfs", "dd if=",
					":(){ :|:& };:", "chmod -R 777 /",
				},
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

// Get 获取配置
func Get() *Config {
	return Load()
}
