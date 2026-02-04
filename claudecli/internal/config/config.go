package config

import (
	"os"
	"strconv"
	"sync"
	"time"
)

// Config 应用配置
type Config struct {
	Server    ServerConfig
	Claude    ClaudeConfig
	Security  SecurityConfig
	Container ContainerConfig
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

// ContainerConfig 容器隔离配置
type ContainerConfig struct {
	Enabled      bool          // 是否启用容器隔离
	Runtime      string        // 容器运行时 ("runsc" for gVisor, "" for default)
	Image        string        // 沙箱镜像名称
	MemoryMB     int64         // 内存限制 (MB)
	CPUQuota     float64       // CPU 限制 (0.5 = half core)
	PidLimit     int64         // 最大进程数
	DiskMB       int64         // 工作空间磁盘配额 (MB)
	IdleTimeout  time.Duration // 空闲超时
	HostBasePath string        // 宿主机上的 userdata 基础路径 (Docker-in-Docker 场景必需)
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
			Container: ContainerConfig{
				Enabled:      getEnv("CONTAINER_ENABLED", "") == "true",
				Runtime:      getEnv("CONTAINER_RUNTIME", ""), // 留空使用默认 Docker runtime，Windows 不支持 runsc
				Image:        getEnv("CONTAINER_IMAGE", "coworker-sandbox:latest"),
				MemoryMB:     getEnvInt("CONTAINER_MEMORY_MB", 256),
				CPUQuota:     getEnvFloat("CONTAINER_CPU_QUOTA", 0.5),
				PidLimit:     getEnvInt("CONTAINER_PID_LIMIT", 100),
				DiskMB:       getEnvInt("CONTAINER_DISK_MB", 250),
				IdleTimeout:  time.Duration(getEnvInt("CONTAINER_IDLE_TIMEOUT", 30)) * time.Minute,
				HostBasePath: getEnv("CONTAINER_HOST_BASE_PATH", ""), // 宿主机路径，留空则使用 WorkingDir
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
