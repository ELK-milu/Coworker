# OpenCode 学习改进计划 - 实施文档

> 基于 OpenCode 项目分析，为 Coworker ClaudeCLI 制定的改进计划
> 创建日期: 2026-02-04

## 项目背景

- **分析对象**: OpenCode (TypeScript/Bun) - 开源 AI 编码代理 CLI
- **改进目标**: Coworker ClaudeCLI (Go) - `claudecli/`
- **详细分析**: `docs/opencode-analysis.md`

---

## 优先级说明

| 优先级 | 说明 | 时间范围 |
|--------|------|----------|
| P0 | 最高优先级，直接影响 AI 成功率 | 1-2周 |
| P1 | 短期改进 | 2-4周 |
| P2 | 中期改进 | 1-2月 |

---

## P0 - 立即开始

### 1. Edit 工具多层 Replacer (最高优先级)

**问题**: 当前仅支持精确匹配，AI 必须提供 100% 精确的 old_string

**当前实现** (`claudecli/internal/tools/edit.go`):
```go
// 仅使用 strings.Replace，精确匹配
newContent = strings.Replace(oldContent, in.OldString, in.NewString, 1)
```

**OpenCode 方案**: 多层 Replacer 链
1. SimpleReplacer - 精确匹配
2. LineTrimmedReplacer - 行尾空格容忍
3. LeadingWhitespaceReplacer - 前导空格/缩进容忍
4. BlockAnchorReplacer - 首尾行锚定 + Levenshtein 模糊匹配
5. WhitespaceNormalizedReplacer - 空格归一化
6. IndentNormalizedReplacer - 缩进归一化

**新增文件**:
- `claudecli/internal/tools/edit_replacer.go` - Replacer 接口和实现

**修改文件**:
- `claudecli/internal/tools/edit.go` - 集成 Replacer 链

**实现细节**:

```go
// edit_replacer.go

// Match 表示一个匹配结果
type Match struct {
    Start      int     // 匹配开始位置
    End        int     // 匹配结束位置
    Similarity float64 // 相似度 (0.0-1.0)
}

// Replacer 替换器接口
type Replacer interface {
    Name() string
    FindMatch(content, find string) *Match
}

// ReplacerChain 替换器链
type ReplacerChain struct {
    replacers []Replacer
}

// FindBestMatch 在链中查找最佳匹配
func (c *ReplacerChain) FindBestMatch(content, find string) (*Match, string)
```

**核心算法 - BlockAnchorReplacer**:
1. 提取 find 字符串的首行和尾行作为锚点
2. 在 content 中查找匹配的首行
3. 对中间内容计算 Levenshtein 距离
4. 相似度阈值: 单候选 0.0，多候选 0.3

**收益**: 显著提升 AI 编辑成功率，减少因空格/缩进差异导致的失败

---

### 2. Read 工具增强

**问题**:
- 无二进制文件检测，读取二进制文件会产生乱码
- 文件不存在时无模糊建议

**当前实现** (`claudecli/internal/tools/read.go`):
```go
// 直接读取，无任何检测
content, err := os.ReadFile(path)
```

**改进方案**:

```go
// 1. 二进制检测
func isBinaryFile(path string) bool {
    data, _ := os.ReadFile(path)
    if len(data) > 8000 { data = data[:8000] }
    for _, b := range data {
        if b == 0 { return true }
    }
    return false
}

// 2. 模糊文件建议
func suggestFiles(dir, base string) []string {
    entries, _ := os.ReadDir(dir)
    var suggestions []string
    for _, e := range entries {
        if strings.Contains(strings.ToLower(e.Name()), strings.ToLower(base)) {
            suggestions = append(suggestions, e.Name())
        }
    }
    return suggestions[:min(3, len(suggestions))]
}
```

**修改文件**:
- `claudecli/internal/tools/read.go`

**收益**:
- 避免读取二进制文件产生无意义输出
- 帮助 AI 找到正确的文件

---

## P1 - 短期 (2-4周)

### 3. 会话压缩 Prune 层

**问题**: 当前压缩仅保留最近 3 个工具结果，不够智能

**当前实现** (`claudecli/internal/context/compress.go`):
```go
// Microcompact 保留最近 3 个工具结果
const keepRecentToolResults = 3
```

**OpenCode 方案**: 两层压缩
- Prune 层: 基于 token 数保护最近的工具调用
- Compaction 层: AI 摘要生成

**新增文件**:
- `claudecli/internal/context/prune.go`

**实现细节**:
```go
const (
    PRUNE_MINIMUM = 20000  // 最小修剪量
    PRUNE_PROTECT = 40000  // 保护最近的 token 数
)

func Prune(messages []Message, currentTokens int) []Message {
    // 1. 计算需要修剪的 token 数
    // 2. 从旧到新遍历，清理工具输出
    // 3. 保护最近 40K tokens
}
```

**修改文件**:
- `claudecli/internal/context/context.go` - 集成 Prune

---

### 4. Glob 排除模式

**问题**: 无法排除 node_modules、.git 等目录

**当前实现** (`claudecli/internal/tools/glob.go`):
```go
type GlobInput struct {
    Pattern string `json:"pattern"`
    Path    string `json:"path,omitempty"`
}
// 无排除功能
```

**改进方案**:
```go
type GlobInput struct {
    Pattern string   `json:"pattern"`
    Path    string   `json:"path,omitempty"`
    Exclude []string `json:"exclude,omitempty"`  // 新增
}

// 默认排除
var defaultExclude = []string{
    "node_modules/**",
    ".git/**",
    "vendor/**",
    "__pycache__/**",
    ".venv/**",
}
```

**修改文件**:
- `claudecli/internal/tools/glob.go`

---

### 5. 权限记忆系统

**问题**: 每次危险操作都需要确认，无 "always" 选项

**新增文件**:
- `claudecli/internal/permission/memory.go`

**实现细节**:
```go
type PermissionMemory struct {
    rules map[string]string  // pattern -> "allow" | "deny"
}

func (m *PermissionMemory) Check(permission, pattern string) string {
    // 支持通配符匹配: "git *", "npm *"
}

func (m *PermissionMemory) Remember(permission, pattern, action string) {
    // 持久化到 userdata/{user_id}/permissions.json
}
```

---

## P2 - 中期 (1-2月)

### 6. LSP 集成

**功能**: 编辑后自动触发 LSP 诊断，返回错误信息给 AI

**新增目录**:
- `claudecli/internal/lsp/`

**实现路径**:
1. 集成 gopls (Go LSP)
2. 集成 typescript-language-server
3. Edit 工具调用后触发诊断
4. 返回诊断结果给 AI 自动修复

---

### 7. 简化快照系统

**功能**: 文件变更追踪和回滚

**新增文件**:
- `claudecli/internal/snapshot/snapshot.go`

**实现细节**:
```go
type Snapshot struct {
    ID        string
    Timestamp time.Time
    Files     map[string][]byte  // path -> content
}

func (s *SnapshotManager) Track(paths []string) string {
    // 保存文件快照
}

func (s *SnapshotManager) Restore(snapshotID string) error {
    // 恢复到快照
}
```

**存储**: `userdata/{user_id}/snapshots/`

---

## 文件清单

| 文件 | 操作 | 优先级 | 说明 |
|------|------|--------|------|
| `claudecli/internal/tools/edit_replacer.go` | 新增 | P0 | Replacer 链实现 |
| `claudecli/internal/tools/edit.go` | 修改 | P0 | 集成 Replacer 链 |
| `claudecli/internal/tools/read.go` | 修改 | P0 | 二进制检测 + 模糊建议 |
| `claudecli/internal/context/prune.go` | 新增 | P1 | Prune 层实现 |
| `claudecli/internal/context/context.go` | 修改 | P1 | 集成 Prune |
| `claudecli/internal/tools/glob.go` | 修改 | P1 | 排除模式 |
| `claudecli/internal/permission/memory.go` | 新增 | P1 | 权限记忆 |
| `claudecli/internal/lsp/` | 新增 | P2 | LSP 集成 |
| `claudecli/internal/snapshot/snapshot.go` | 新增 | P2 | 快照系统 |

---

## 验证方案

### 单元测试
```bash
go test ./claudecli/internal/tools/... -v
go test ./claudecli/internal/context/... -v
```

### 集成测试
1. 启动开发环境: `docker-compose -f docker-compose-dev.yml up`
2. 访问 Coworker 页面: http://localhost:3000/console/coworker
3. 测试 Edit 工具模糊匹配
4. 测试 Read 工具二进制检测
5. 测试上下文压缩

### 验收标准
- [x] Edit 工具支持空格/缩进容忍
- [x] Edit 工具支持 Levenshtein 模糊匹配
- [x] Read 工具检测并拒绝二进制文件
- [x] Read 工具提供模糊文件建议
- [x] Prune 层保护最近 40K tokens
- [x] Glob 工具支持排除模式
- [x] 权限记忆系统支持 "always allow/deny"

---

*文档创建日期: 2026-02-04*
*最后更新: 2026-02-04 - P0/P1 任务全部完成*
