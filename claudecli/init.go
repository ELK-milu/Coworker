package claudecli

// ClaudeCLI 模块 - 提供 Claude Code CLI 功能

import (
	"context"
	"log"
	"time"

	"github.com/QuantumNous/new-api/claudecli/internal/api"
	"github.com/QuantumNous/new-api/claudecli/internal/client"
	"github.com/QuantumNous/new-api/claudecli/internal/config"
	"github.com/QuantumNous/new-api/claudecli/internal/embedding"
	"github.com/QuantumNous/new-api/claudecli/internal/eventbus"
	"github.com/QuantumNous/new-api/claudecli/internal/job"
	"github.com/QuantumNous/new-api/claudecli/internal/mcp"
	"github.com/QuantumNous/new-api/claudecli/internal/memory"
	"github.com/QuantumNous/new-api/claudecli/internal/permissions"
	"github.com/QuantumNous/new-api/claudecli/internal/profile"
	"github.com/QuantumNous/new-api/claudecli/internal/sandbox"
	"github.com/QuantumNous/new-api/claudecli/internal/session"
	"github.com/QuantumNous/new-api/claudecli/internal/skills"
	"github.com/QuantumNous/new-api/claudecli/internal/task"
	"github.com/QuantumNous/new-api/claudecli/internal/tools"
	"github.com/QuantumNous/new-api/claudecli/internal/variable"
	"github.com/QuantumNous/new-api/claudecli/internal/workspace"
)

// Module claudecli 模块实例
type Module struct {
	Config      *config.Config
	Client      *client.ClaudeClient
	Sessions    *session.Manager
	Tools       *tools.Registry
	Workspace   *workspace.Manager
	Tasks       *task.Manager
	Jobs        *job.Manager
	Variables   *variable.Manager
	Memories    *memory.Manager
	Profiles    *profile.Manager
	SandboxPool *sandbox.SandboxPool
	RESTHandler *api.RESTHandler
	WSHandler   *api.WSHandler
	FileHandler *api.FileHandler
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

	// 创建 Job 管理器
	jobManager := job.NewManager(cfg.Security.WorkingDir)

	// 创建变量管理器
	variableManager := variable.NewManager(cfg.Security.WorkingDir)

	// 创建记忆管理器
	memoryManager := memory.NewManager(cfg.Security.WorkingDir)

	// 创建 Embedding 客户端
	embeddingCfg := embedding.LoadConfigFromEnv()
	var embeddingClient *embedding.Client
	if embeddingCfg.GetActiveAPIKey() != "" {
		embeddingClient = embedding.NewClient(embeddingCfg)
		log.Printf("[ClaudeCLI] Embedding client initialized (provider: %s, model: %s)", embeddingCfg.Provider, embeddingCfg.Model)
	} else {
		log.Println("[ClaudeCLI] Embedding client disabled (no API key configured)")
	}

	// 创建 Milvus 客户端
	var milvusClient *memory.MilvusClient
	if cfg.Milvus.Enabled {
		milvusCfg := memory.MilvusConfig{
			Enabled:    true,
			Host:       cfg.Milvus.Host,
			Port:       cfg.Milvus.Port,
			Collection: cfg.Milvus.Collection,
			Dimension:  cfg.Milvus.Dimension,
			EnableBM25: cfg.Milvus.EnableBM25,
		}
		milvusClient = memory.NewMilvusClient(milvusCfg)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := milvusClient.Connect(ctx); err != nil {
			log.Printf("[ClaudeCLI] WARNING: Milvus connection failed: %v", err)
			milvusClient = nil
		} else {
			log.Printf("[ClaudeCLI] Milvus connected (host: %s:%d)", cfg.Milvus.Host, cfg.Milvus.Port)
		}
		cancel()
	}

	// 创建用户画像管理器
	profileManager := profile.NewManager(cfg.Security.WorkingDir)

	// 创建 nsjail 沙箱池（如果启用）
	var sandboxPool *sandbox.SandboxPool
	if cfg.Nsjail.Enabled {
		poolCfg := sandbox.PoolConfig{
			MaxConcurrent: cfg.Nsjail.MaxConcurrent,
			MemoryMB:      cfg.Nsjail.MemoryMB,
			ExecTimeout:   cfg.Nsjail.ExecTimeout,
			ContainerName: cfg.Nsjail.ContainerName,
		}
		var err error
		sandboxPool, err = sandbox.NewSandboxPool(poolCfg)
		if err != nil {
			log.Printf("[ClaudeCLI] WARNING: nsjail pool disabled: %v", err)
			sandboxPool = nil
		} else {
			// 启动池
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := sandboxPool.Start(ctx); err != nil {
				log.Printf("[ClaudeCLI] WARNING: Failed to start sandbox pool: %v", err)
				sandboxPool = nil
			} else {
				log.Println("[ClaudeCLI] nsjail sandbox pool enabled")
			}
			cancel()
		}
	}

	// 注册所有工具
	registerTools(toolRegistry, cfg, taskManager, memoryManager, sandboxPool)

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
	restHandler.SetJobManager(jobManager)
	restHandler.SetMemoryManager(memoryManager)

	// 创建 WebSocket 处理器（不再传递静态系统提示词，改为动态构建）
	wsHandler := api.NewWSHandler(claudeClient, sessionManager, toolRegistry, workspaceManager, taskManager, permChecker, skillRegistry, mcpManager, cfg)
	wsHandler.SetJobManager(jobManager)
	wsHandler.SetVariableManager(variableManager)
	wsHandler.SetMemoryManager(memoryManager)
	wsHandler.SetProfileManager(profileManager)
	wsHandler.SetEmbeddingClient(embeddingClient)
	wsHandler.SetMilvusClient(milvusClient)

	// P2.5: 创建文件修改时间追踪器
	fileTime := tools.NewFileTime()
	wsHandler.SetFileTime(fileTime)

	// 创建 EventBus 并注册记忆事件处理器
	bus := eventbus.New()
	memoryHandlers := memory.NewMemoryHandlers(memoryManager, claudeClient)
	memoryHandlers.Register(bus)
	wsHandler.SetEventBus(bus)
	log.Println("[ClaudeCLI] EventBus initialized with memory handlers")

	// 创建文件处理器
	fileHandler := api.NewFileHandler(workspaceManager)

	instance = &Module{
		Config:      cfg,
		Client:      claudeClient,
		Sessions:    sessionManager,
		Tools:       toolRegistry,
		Workspace:   workspaceManager,
		Tasks:       taskManager,
		Jobs:        jobManager,
		Variables:   variableManager,
		Memories:    memoryManager,
		Profiles:    profileManager,
		SandboxPool: sandboxPool,
		RESTHandler: restHandler,
		WSHandler:   wsHandler,
		FileHandler: fileHandler,
	}

	// 创建 Job AI 执行器
	busyChecker := job.NewBusySessionChecker(func(sessionID string) bool {
		_, busy := wsHandler.IsBusySession(sessionID)
		return busy
	})
	jobExecutor := job.NewAIExecutor(&job.JobExecutorDeps{
		Client:       claudeClient,
		Sessions:     sessionManager,
		Tools:        toolRegistry,
		Workspace:    workspaceManager,
		Tasks:        taskManager,
		Memories:     memoryManager,
		Config:       cfg,
		Bus:          bus,
		FileTime:     fileTime,
		BusySessions: busyChecker,
	})
	jobManager.SetExecutor(jobExecutor)

	// 启动 Job 调度器
	jobManager.Start()

	log.Println("[ClaudeCLI] Module initialized successfully")
	return instance
}

// registerTools 注册所有工具（使用工厂模式）
func registerTools(registry *tools.Registry, cfg *config.Config, taskManager *task.Manager, memoryManager *memory.Manager, sandboxPool *sandbox.SandboxPool) {
	workingDir := cfg.Security.WorkingDir
	blockedCommands := cfg.Security.BlockedCommands

	// 创建截断器
	truncation := tools.NewTruncation(workingDir)
	truncation.StartCleanup()
	registry.SetTruncation(truncation)

	// 创建工具工厂（自动验证输入 + 截断输出）
	factory := tools.NewToolFactory(truncation)

	// 创建 Bash 工具（需要特殊配置）
	bashTool := tools.NewBashTool(workingDir, blockedCommands)
	if sandboxPool != nil {
		bashTool.SetSandboxPool(sandboxPool)
	}

	// 使用工厂模式注册基础工具
	tools.RegisterWithFactory(registry, factory, bashTool)
	tools.RegisterWithFactory(registry, factory, tools.NewReadTool(workingDir))
	tools.RegisterWithFactory(registry, factory, tools.NewWriteTool(workingDir))
	tools.RegisterWithFactory(registry, factory, tools.NewEditTool(workingDir))
	tools.RegisterWithFactory(registry, factory, tools.NewGlobTool(workingDir))
	tools.RegisterWithFactory(registry, factory, tools.NewGrepTool(workingDir))
	tools.RegisterWithFactory(registry, factory, tools.NewWebFetchTool())
	tools.RegisterWithFactory(registry, factory, tools.NewAskUserQuestionTool())
	tools.RegisterWithFactory(registry, factory, tools.NewStructuredOutputTool())

	// 使用工厂模式注册任务工具
	tools.RegisterWithFactory(registry, factory, tools.NewTaskCreateTool(taskManager))
	tools.RegisterWithFactory(registry, factory, tools.NewTaskUpdateTool(taskManager))
	tools.RegisterWithFactory(registry, factory, tools.NewTaskListTool(taskManager))
	tools.RegisterWithFactory(registry, factory, tools.NewTaskGetTool(taskManager))

	// 使用工厂模式注册记忆工具
	tools.RegisterWithFactory(registry, factory, tools.NewMemorySearchTool(memoryManager))
	tools.RegisterWithFactory(registry, factory, tools.NewMemorySaveTool(memoryManager))
	tools.RegisterWithFactory(registry, factory, tools.NewMemoryListTool(memoryManager))

	log.Printf("[ClaudeCLI] Registered %d tools with factory pattern (sandbox pool: %v)", 16, sandboxPool != nil)
}

// Shutdown 优雅关闭模块
func (m *Module) Shutdown() {
	if m.Jobs != nil {
		log.Println("[ClaudeCLI] Stopping job scheduler...")
		m.Jobs.Stop()
	}
	if m.SandboxPool != nil {
		log.Println("[ClaudeCLI] Shutting down sandbox pool...")
		m.SandboxPool.Stop()
	}
}

// GetInstance 获取模块实例
func GetInstance() *Module {
	if instance == nil {
		return Init()
	}
	return instance
}
