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
// 增强版：同时支持 Mode 模式检查和 Ruleset 规则集评估
// 参考 OpenCode permission/next.ts 的双层权限架构
type Checker struct {
	mode           PermissionMode
	toolCapability map[string][]ToolCapability
	ruleset        Ruleset  // P2.6: 用户级权限规则集
	approved       Ruleset  // P2.6: 运行时批准的规则（"always allow"）
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

// SetRuleset 设置用户级权限规则集
// 参考 OpenCode: const user = PermissionNext.fromConfig(cfg.permission ?? {})
func (c *Checker) SetRuleset(rs Ruleset) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ruleset = rs
}

// GetRuleset 获取当前规则集
func (c *Checker) GetRuleset() Ruleset {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ruleset
}

// Approve 添加运行时批准规则（"always allow"）
// 参考 OpenCode: s.approved.push({permission, pattern, action: "allow"})
func (c *Checker) Approve(permission, pattern string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.approved = append(c.approved, Rule{
		Permission: permission,
		Pattern:    pattern,
		Action:     BehaviorAllow,
	})
}

// CheckWithRuleset 使用 Ruleset 引擎检查权限
// 参考 OpenCode: PermissionNext.evaluate(permission, pattern, ruleset, approved)
func (c *Checker) CheckWithRuleset(permission, pattern string, agentRuleset Ruleset) *CheckResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	rule := Evaluate(permission, pattern, agentRuleset, c.ruleset, c.approved)
	return &CheckResult{
		Behavior: rule.Action,
		Message:  "ruleset: " + rule.Permission + " " + rule.Pattern,
	}
}

// CheckTool 综合检查：先用 Mode 模式，再用 Ruleset 引擎
// 如果 agentRuleset 非空，优先使用 Ruleset 引擎
func (c *Checker) CheckTool(toolName string, pattern string, agentRuleset Ruleset) *CheckResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 如果有 Agent 规则集，使用 Ruleset 引擎
	if len(agentRuleset) > 0 {
		rule := Evaluate(toolName, pattern, agentRuleset, c.ruleset, c.approved)
		return &CheckResult{
			Behavior: rule.Action,
			Message:  "ruleset: " + rule.Permission + " " + rule.Pattern,
		}
	}

	// 回退到 Mode 模式检查
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
