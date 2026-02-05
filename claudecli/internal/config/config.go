package config

import (
	"os"
	"strconv"
	"sync"
	"time"
)

// Config 应用配置
type Config struct {
	Server       ServerConfig
	Claude       ClaudeConfig
	Security     SecurityConfig
	Microsandbox MicrosandboxConfig
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

// MicrosandboxConfig Microsandbox MicroVM 沙箱配置
type MicrosandboxConfig struct {
	Enabled     bool          // 是否启用 Microsandbox
	ServerURL   string        // Microsandbox server URL
	APIKey      string        // API Key (生产环境必需)
	Namespace   string        // 命名空间
	PoolSize    int           // 沙箱池大小
	MaxWaitTime time.Duration // 获取沙箱最大等待时间
	MemoryMB    int           // 每个沙箱内存限制 (MB)
	CPUs        int           // 每个沙箱 CPU 核数
	ExecTimeout time.Duration // 命令执行超时
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
			Microsandbox: MicrosandboxConfig{
				Enabled:     getEnv("MICROSANDBOX_ENABLED", "") == "true",
				ServerURL:   getEnv("MSB_SERVER_URL", "http://127.0.0.1:5555"),
				APIKey:      getEnv("MSB_API_KEY", ""),
				Namespace:   getEnv("MSB_NAMESPACE", "default"),
				PoolSize:    int(getEnvInt("MSB_POOL_SIZE", 5)),
				MaxWaitTime: time.Duration(getEnvInt("MSB_MAX_WAIT_TIME", 30)) * time.Second,
				MemoryMB:    int(getEnvInt("MSB_MEMORY_MB", 512)),
				CPUs:        int(getEnvInt("MSB_CPUS", 1)),
				ExecTimeout: time.Duration(getEnvInt("MSB_EXEC_TIMEOUT", 120)) * time.Second,
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

// Get 获取配置
func Get() *Config {
	return Load()
}
