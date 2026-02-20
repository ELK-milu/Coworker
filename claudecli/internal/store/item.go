package store

import "time"

type ItemType string

const (
	TypeSkill ItemType = "skill"
	TypeAgent ItemType = "agent"
	TypeMCP   ItemType = "mcp"
)

// ConfigField 用户配置字段定义
type ConfigField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"` // "string" | "password" | "url"
	Required    bool   `json:"required"`
	Placeholder string `json:"placeholder,omitempty"`
}

// StoreItem 技能商店条目
type StoreItem struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Type         ItemType      `json:"type"`
	Icon         string        `json:"icon,omitempty"`
	Author       string        `json:"author,omitempty"`
	GithubURL    string        `json:"github_url,omitempty"`
	Content      string        `json:"content,omitempty"`    // skill/agent: markdown 内容
	ServerURL    string        `json:"server_url,omitempty"` // mcp: 服务器 URL
	ConfigSchema []ConfigField `json:"config_schema,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// PluginJSON .claude-plugin/plugin.json 格式
type PluginJSON struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Author      string `json:"author"`
}

// MarketplaceJSON .claude-plugin/marketplace.json 格式
type MarketplaceJSON struct {
	Plugins []MarketplacePlugin `json:"plugins"`
}

// MarketplacePlugin marketplace 中的单个 plugin 条目
type MarketplacePlugin struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Version     string      `json:"version"`
	Source      interface{} `json:"source"` // string 或 { "source": "url", "url": "..." }
}
