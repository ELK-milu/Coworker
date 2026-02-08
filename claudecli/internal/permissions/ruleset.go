package permissions

// Ruleset 评估引擎
// 参考 OpenCode packages/opencode/src/permission/next.ts

// Rule 权限规则
type Rule struct {
	Permission string        `json:"permission"` // 权限名称（如 "read", "edit", "bash", "*"）
	Pattern    string        `json:"pattern"`    // 文件/命令模式（支持通配符）
	Action     CheckBehavior `json:"action"`     // 操作：allow/deny/ask
}

// Ruleset 规则集合（有序，后定义的规则优先级更高）
type Ruleset []Rule

// Merge 合并多个规则集（后面的优先级更高）
// 参考 OpenCode: export function merge(...rulesets: Ruleset[]): Ruleset { return rulesets.flat() }
func Merge(rulesets ...Ruleset) Ruleset {
	var result Ruleset
	for _, rs := range rulesets {
		result = append(result, rs...)
	}
	return result
}

// Evaluate 评估权限
// 使用 findLast 语义：最后匹配的规则获胜
// 参考 OpenCode: merged.findLast(rule => Wildcard.match(permission, rule.permission) && Wildcard.match(pattern, rule.pattern))
func Evaluate(permission string, pattern string, rulesets ...Ruleset) Rule {
	merged := Merge(rulesets...)

	// 从后向前查找最后匹配的规则
	for i := len(merged) - 1; i >= 0; i-- {
		rule := merged[i]
		if WildcardMatch(permission, rule.Permission) && WildcardMatch(pattern, rule.Pattern) {
			return rule
		}
	}

	// 默认返回 "ask"
	return Rule{
		Permission: permission,
		Pattern:    "*",
		Action:     BehaviorAsk,
	}
}

// FromConfig 从配置映射创建规则集
// 支持两种格式：
//   简单格式: {"read": "allow", "edit": "deny"}
//   详细格式: {"read": {"*": "allow", "*.env": "ask"}}
// 参考 OpenCode: export function fromConfig(permission: Config.Permission)
func FromConfig(config map[string]interface{}) Ruleset {
	var ruleset Ruleset

	for key, value := range config {
		switch v := value.(type) {
		case string:
			// 简单格式: "read": "allow"
			action := parseAction(v)
			ruleset = append(ruleset, Rule{
				Permission: key,
				Pattern:    "*",
				Action:     action,
			})
		case map[string]interface{}:
			// 详细格式: "read": {"*": "allow", "*.env": "ask"}
			for pattern, actionStr := range v {
				if s, ok := actionStr.(string); ok {
					ruleset = append(ruleset, Rule{
						Permission: key,
						Pattern:    pattern,
						Action:     parseAction(s),
					})
				}
			}
		case map[string]string:
			// 详细格式（类型安全版本）
			for pattern, actionStr := range v {
				ruleset = append(ruleset, Rule{
					Permission: key,
					Pattern:    pattern,
					Action:     parseAction(actionStr),
				})
			}
		}
	}

	return ruleset
}

// parseAction 解析行为字符串
func parseAction(s string) CheckBehavior {
	switch s {
	case "allow":
		return BehaviorAllow
	case "deny":
		return BehaviorDeny
	case "ask":
		return BehaviorAsk
	default:
		return BehaviorAsk
	}
}

// EDIT_TOOLS 编辑类工具列表
// 参考 OpenCode: const EDIT_TOOLS = ["edit", "write", "patch", "multiedit"]
var editTools = map[string]bool{
	"Edit":  true,
	"Write": true,
}

// DisabledTools 获取被完全禁用的工具集合
// 参考 OpenCode: export function disabled(tools: string[], ruleset: Ruleset): Set<string>
func DisabledTools(toolNames []string, ruleset Ruleset) map[string]bool {
	result := make(map[string]bool)

	for _, toolName := range toolNames {
		// 编辑类工具使用 "edit" 权限
		permission := toolName
		if editTools[toolName] {
			permission = "edit"
		}

		// 从后向前查找最后匹配的规则
		for i := len(ruleset) - 1; i >= 0; i-- {
			r := ruleset[i]
			if WildcardMatch(permission, r.Permission) {
				if r.Pattern == "*" && r.Action == BehaviorDeny {
					result[toolName] = true
				}
				break
			}
		}
	}

	return result
}
