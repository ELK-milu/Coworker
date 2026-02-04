package claudecli

// ClaudeCLI 模块 - 提供 Claude Code CLI 功能

import (
	"github.com/QuantumNous/new-api/claudecli/internal/api"
	"github.com/QuantumNous/new-api/claudecli/internal/client"
	"github.com/QuantumNous/new-api/claudecli/internal/config"
	"github.com/QuantumNous/new-api/claudecli/internal/container"
	"github.com/QuantumNous/new-api/claudecli/internal/mcp"
	"github.com/QuantumNous/new-api/claudecli/internal/permissions"
	"github.com/QuantumNous/new-api/claudecli/internal/session"
	"github.com/QuantumNous/new-api/claudecli/internal/skills"
	"github.com/QuantumNous/new-api/claudecli/internal/task"
	"github.com/QuantumNous/new-api/claudecli/internal/tools"
	"github.com/QuantumNous/new-api/claudecli/internal/workspace"
	"log"
)

// Module claudecli 模块实例
type Module struct {
	Config       *config.Config
	Client       *client.ClaudeClient
	Sessions     *session.Manager
	Tools        *tools.Registry
	Workspace    *workspace.Manager
	Tasks        *task.Manager
	Containers   *container.ContainerManager
	RESTHandler  *api.RESTHandler
	WSHandler    *api.WSHandler
	FileHandler  *api.FileHandler
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

	// 创建工作空间管理器
	workspaceManager := workspace.NewManager(cfg.Security.WorkingDir)

	// 设置用户会话存储基础目录
	session.SetUserBaseDir(cfg.Security.WorkingDir)

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

	// 创建任务管理器
	taskManager := task.NewManager(cfg.Security.WorkingDir)

	// 创建容器管理器（如果启用）
	var containerMgr *container.ContainerManager
	if cfg.Container.Enabled {
		var err error
		containerMgr, err = container.NewContainerManager(cfg.Security.WorkingDir, container.Config{
			Image:        cfg.Container.Image,
			Runtime:      cfg.Container.Runtime,
			MemoryMB:     cfg.Container.MemoryMB,
			CPUQuota:     cfg.Container.CPUQuota,
			PidLimit:     cfg.Container.PidLimit,
			DiskMB:       cfg.Container.DiskMB,
			IdleTimeout:  cfg.Container.IdleTimeout,
			HostBasePath: cfg.Container.HostBasePath,
		})
		if err != nil {
			log.Printf("[ClaudeCLI] WARNING: Container isolation disabled: %v", err)
			containerMgr = nil
		} else {
			log.Println("[ClaudeCLI] Container isolation enabled")
		}
	}

	// 注册所有工具
	registerTools(toolRegistry, cfg, taskManager, containerMgr)

	// 创建权限检查器
	permChecker := permissions.NewChecker()

	// 创建技能注册表
	skillRegistry := skills.NewRegistry()

	// 创建 MCP 管理器
	mcpManager := mcp.NewManager()

	// 创建 REST 处理器
	restHandler := api.NewRESTHandler(sessionManager)
	restHandler.SetTaskManager(taskManager)
	restHandler.SetWorkspaceManager(workspaceManager)

	// 创建 WebSocket 处理器（不再传递静态系统提示词，改为动态构建）
	wsHandler := api.NewWSHandler(claudeClient, sessionManager, toolRegistry, workspaceManager, taskManager, permChecker, skillRegistry, mcpManager, cfg)

	// 创建文件处理器
	fileHandler := api.NewFileHandler(workspaceManager)

	instance = &Module{
		Config:      cfg,
		Client:      claudeClient,
		Sessions:    sessionManager,
		Tools:       toolRegistry,
		Workspace:   workspaceManager,
		Tasks:       taskManager,
		Containers:  containerMgr,
		RESTHandler: restHandler,
		WSHandler:   wsHandler,
		FileHandler: fileHandler,
	}

	log.Println("[ClaudeCLI] Module initialized successfully")
	return instance
}

// registerTools 注册所有工具
func registerTools(registry *tools.Registry, cfg *config.Config, taskManager *task.Manager, containerMgr *container.ContainerManager) {
	workingDir := cfg.Security.WorkingDir
	blockedCommands := cfg.Security.BlockedCommands

	// 注册基础工具
	bashTool := tools.NewBashTool(workingDir, blockedCommands)
	if containerMgr != nil {
		bashTool.SetContainerManager(containerMgr)
	}
	registry.Register(bashTool)
	registry.Register(tools.NewReadTool(workingDir))
	registry.Register(tools.NewWriteTool(workingDir))
	registry.Register(tools.NewEditTool(workingDir))
	registry.Register(tools.NewGlobTool(workingDir))
	registry.Register(tools.NewGrepTool(workingDir))
	registry.Register(tools.NewWebFetchTool())
	registry.Register(tools.NewAskUserQuestionTool())
	registry.Register(tools.NewStructuredOutputTool())

	// 注册任务工具
	registry.Register(tools.NewTaskCreateTool(taskManager))
	registry.Register(tools.NewTaskUpdateTool(taskManager))
	registry.Register(tools.NewTaskListTool(taskManager))
	registry.Register(tools.NewTaskGetTool(taskManager))

	log.Printf("[ClaudeCLI] Registered %d tools (container isolation: %v)", 13, containerMgr != nil)
}

// Shutdown 优雅关闭模块
func (m *Module) Shutdown() {
	if m.Containers != nil {
		log.Println("[ClaudeCLI] Shutting down container manager...")
		m.Containers.StopAll()
	}
}

// GetInstance 获取模块实例
func GetInstance() *Module {
	if instance == nil {
		return Init()
	}
	return instance
}
