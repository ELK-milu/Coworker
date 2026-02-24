package skills

// Skill 技能定义
type Skill struct {
	Name         string   `json:"name" yaml:"name"`
	Description  string   `json:"description" yaml:"description"`
	Content      string   `json:"content" yaml:"-"`
	AllowedTools []string `json:"allowed_tools,omitempty" yaml:"allowed_tools"`
	Source       string   `json:"source" yaml:"-"`
	FilePath     string   `json:"file_path" yaml:"-"`
}

// Frontmatter YAML frontmatter 结构
type Frontmatter struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	AllowedTools []string `yaml:"allowed_tools"`
}
