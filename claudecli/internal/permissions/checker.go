package permissions

import (
	"sync"
)

// CheckResult 权限检查结果
type CheckResult struct {
	Behavior  CheckBehavior `json:"behavior"`
	Message   string        `json:"message,omitempty"`
	RequestID string        `json:"request_id,omitempty"`
}

// Checker 权限检查器
type Checker struct {
	mode           PermissionMode
	toolCapability map[string][]ToolCapability
	mu             sync.RWMutex
}

// NewChecker 创建权限检查器
func NewChecker() *Checker {
	c := &Checker{
		mode:           ModeDefault,
		toolCapability: make(map[string][]ToolCapability),
	}
	c.registerDefaultCapabilities()
	return c
}

// registerDefaultCapabilities 注册默认工具能力
func (c *Checker) registerDefaultCapabilities() {
	c.toolCapability["Read"] = []ToolCapability{CapabilityRead}
	c.toolCapability["Glob"] = []ToolCapability{CapabilityRead}
	c.toolCapability["Grep"] = []ToolCapability{CapabilityRead}
	c.toolCapability["Write"] = []ToolCapability{CapabilityWrite}
	c.toolCapability["Edit"] = []ToolCapability{CapabilityWrite}
	c.toolCapability["Bash"] = []ToolCapability{CapabilityExecute}
	c.toolCapability["WebFetch"] = []ToolCapability{CapabilityNetwork}
	c.toolCapability["WebSearch"] = []ToolCapability{CapabilityNetwork}
}

// RegisterToolCapability 注册工具能力
func (c *Checker) RegisterToolCapability(toolName string, caps []ToolCapability) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.toolCapability[toolName] = caps
}

// SetMode 设置权限模式
func (c *Checker) SetMode(mode PermissionMode) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mode = mode
}

// GetMode 获取当前权限模式
func (c *Checker) GetMode() PermissionMode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.mode
}

// Check 检查工具权限
func (c *Checker) Check(toolName string, input interface{}) *CheckResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	caps, exists := c.toolCapability[toolName]
	if !exists {
		caps = []ToolCapability{CapabilityRead}
	}

	return c.checkWithMode(caps)
}

// checkWithMode 根据模式检查能力
func (c *Checker) checkWithMode(caps []ToolCapability) *CheckResult {
	switch c.mode {
	case ModeBypassPermissions:
		return &CheckResult{Behavior: BehaviorAllow}

	case ModePlan:
		for _, cap := range caps {
			if cap == CapabilityWrite || cap == CapabilityExecute {
				return &CheckResult{
					Behavior: BehaviorDeny,
					Message:  "规划模式下禁止写入和执行操作",
				}
			}
		}
		return &CheckResult{Behavior: BehaviorAllow}

	case ModeAcceptEdits:
		for _, cap := range caps {
			if cap == CapabilityExecute {
				return &CheckResult{
					Behavior: BehaviorAsk,
					Message:  "执行操作需要确认",
				}
			}
		}
		return &CheckResult{Behavior: BehaviorAllow}

	default: // ModeDefault
		for _, cap := range caps {
			if cap == CapabilityWrite || cap == CapabilityExecute || cap == CapabilityNetwork {
				return &CheckResult{
					Behavior: BehaviorAsk,
					Message:  "此操作需要确认",
				}
			}
		}
		return &CheckResult{Behavior: BehaviorAllow}
	}
}
