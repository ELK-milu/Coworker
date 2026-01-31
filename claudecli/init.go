package claudecli

// ClaudeCLI 模块 - 提供 Claude Code CLI 功能

import (
	"github.com/QuantumNous/new-api/claudecli/internal/api"
	"github.com/QuantumNous/new-api/claudecli/internal/client"
	"github.com/QuantumNous/new-api/claudecli/internal/config"
	"github.com/QuantumNous/new-api/claudecli/internal/session"
	"github.com/QuantumNous/new-api/claudecli/internal/tools"
	"log"
)

// Module claudecli 模块实例
type Module struct {
	Config      *config.Config
	Client      *client.ClaudeClient
	Sessions    *session.Manager
	Tools       *tools.Registry
	RESTHandler *api.RESTHandler
	WSHandler   *api.WSHandler
}

var (
	instance *Module
)

// Init 初始化 claudecli 模块
func Init() *Module {
	if instance != nil {
		return instance
	}

	log.Println("[ClaudeCLI] Initializing module...")

	// 加载配置
	cfg := config.Load()

	// 创建 Claude 客户端
	claudeClient := client.NewClaudeClient(
		cfg.Claude.APIKey,
		cfg.Claude.AuthToken,
		cfg.Claude.BaseURL,
		cfg.Claude.Model,
		cfg.Claude.MaxTokens,
	)

	// 创建会话管理器
	sessionManager := session.NewManager(cfg.Security.WorkingDir)

	// 创建工具注册表
	toolRegistry := tools.NewRegistry()

	// 注册所有工具
	registerTools(toolRegistry, cfg)

	// 系统提示词
	systemPrompt := "You are a helpful AI assistant with access to various tools."

	// 创建 REST 处理器
	restHandler := api.NewRESTHandler(sessionManager)

	// 创建 WebSocket 处理器
	wsHandler := api.NewWSHandler(claudeClient, sessionManager, toolRegistry, systemPrompt)

	instance = &Module{
		Config:      cfg,
		Client:      claudeClient,
		Sessions:    sessionManager,
		Tools:       toolRegistry,
		RESTHandler: restHandler,
		WSHandler:   wsHandler,
	}

	log.Println("[ClaudeCLI] Module initialized successfully")
	return instance
}

// registerTools 注册所有工具
func registerTools(registry *tools.Registry, cfg *config.Config) {
	workingDir := cfg.Security.WorkingDir
	blockedCommands := cfg.Security.BlockedCommands

	// 注册基础工具
	registry.Register(tools.NewBashTool(workingDir, blockedCommands))
	registry.Register(tools.NewReadTool(workingDir))
	registry.Register(tools.NewWriteTool(workingDir))
	registry.Register(tools.NewEditTool(workingDir))
	registry.Register(tools.NewGlobTool(workingDir))
	registry.Register(tools.NewGrepTool(workingDir))

	log.Printf("[ClaudeCLI] Registered %d tools", 6)
}

// GetInstance 获取模块实例
func GetInstance() *Module {
	if instance == nil {
		return Init()
	}
	return instance
}
