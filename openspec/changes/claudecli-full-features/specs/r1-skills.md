# R1: Skill 系统规范

## 概述

实现用户自定义技能系统，支持参数替换和技能来源管理。

---

## 数据结构

### Skill 定义

```go
type Skill struct {
    Name         string   `json:"name"`
    Description  string   `json:"description"`
    Content      string   `json:"content"`
    AllowedTools []string `json:"allowed_tools,omitempty"`
    Source       string   `json:"source"` // "project" | "user"
    FilePath     string   `json:"file_path"`
}
```

### Skill 文件格式

```markdown
---
name: commit
description: 智能 Git 提交
allowed_tools:
  - Bash
  - Read
---

分析当前 git 改动并生成提交信息。

参数: $1 = 额外说明
```

---

## 接口定义

### SkillRegistry

```go
type SkillRegistry struct {
    skills map[string]*Skill
    mu     sync.RWMutex
}

// Register 注册技能
func (r *SkillRegistry) Register(skill *Skill) error

// Get 获取技能
func (r *SkillRegistry) Get(name string) (*Skill, bool)

// GetAll 获取所有技能
func (r *SkillRegistry) GetAll() []*Skill

// LoadFromDir 从目录加载技能
func (r *SkillRegistry) LoadFromDir(dir, source string) error

// Unregister 注销技能
func (r *SkillRegistry) Unregister(name string)
```

### SkillParser

```go
// ParseSkillFile 解析技能文件
func ParseSkillFile(path string) (*Skill, error)

// ParseFrontmatter 解析 YAML frontmatter
func ParseFrontmatter(content string) (map[string]interface{}, string, error)
```

### SkillExecutor

```go
type SkillExecutor struct {
    registry *SkillRegistry
}

// Execute 执行技能
func (e *SkillExecutor) Execute(name string, args []string) (string, error)

// SubstituteParams 替换参数
func (e *SkillExecutor) SubstituteParams(content string, args []string) string
```

---

## 参数替换规则

| 语法 | 说明 | 示例输入 | 结果 |
|------|------|----------|------|
| `$0` | 完整参数 | `/skill a b c` | `a b c` |
| `$1` | 第1个参数 | `/skill foo bar` | `foo` |
| `$2` | 第2个参数 | `/skill foo bar` | `bar` |
| `$ARGUMENTS[0]` | 数组索引 | `/skill x y` | `x` |

---

## 来源优先级

1. 项目级: `{workspace}/.claude/skills/*.md`
2. 用户级: `{userdata}/{user_id}/skills/*.md`

同名时项目级优先。

---

## 验收标准

- [ ] 解析 Markdown frontmatter
- [ ] 支持 $0, $1, $ARGUMENTS[N] 参数
- [ ] 项目级优先于用户级
- [ ] 注册到工具列表供 Claude 调用
