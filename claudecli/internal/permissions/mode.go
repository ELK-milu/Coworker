package permissions

// PermissionMode 权限模式
type PermissionMode string

const (
	// ModeDefault 默认模式 - 写入/执行需要用户确认
	ModeDefault PermissionMode = "default"
	// ModeAcceptEdits 自动编辑模式 - 写入自动允许，执行仍需确认
	ModeAcceptEdits PermissionMode = "acceptEdits"
	// ModePlan 规划模式 - 只读，禁止写入和执行
	ModePlan PermissionMode = "plan"
	// ModeBypassPermissions 绕过权限模式 - 跳过所有权限检查
	ModeBypassPermissions PermissionMode = "bypassPermissions"
)

// ToolCapability 工具能力标签
type ToolCapability string

const (
	CapabilityRead    ToolCapability = "read"
	CapabilityWrite   ToolCapability = "write"
	CapabilityExecute ToolCapability = "execute"
	CapabilityNetwork ToolCapability = "network"
)

// CheckBehavior 权限检查行为
type CheckBehavior string

const (
	BehaviorAllow CheckBehavior = "allow"
	BehaviorDeny  CheckBehavior = "deny"
	BehaviorAsk   CheckBehavior = "ask"
)
