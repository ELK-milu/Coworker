package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// SemanticGroup 语义组
type SemanticGroup struct {
	Words  []string `json:"words"`
	Weight float64  `json:"weight"`
}

// SemanticGroups 语义组配置
type SemanticGroups struct {
	Groups map[string]*SemanticGroup `json:"groups"`
}

// SemanticExpander 语义扩展器
type SemanticExpander struct {
	groups       *SemanticGroups
	wordToGroup  map[string][]string // word -> 同组的所有词
	mu           sync.RWMutex
	configPath   string
}

// NewSemanticExpander 创建语义扩展器
func NewSemanticExpander(baseDir string) *SemanticExpander {
	se := &SemanticExpander{
		groups:      &SemanticGroups{Groups: make(map[string]*SemanticGroup)},
		wordToGroup: make(map[string][]string),
		configPath:  filepath.Join(baseDir, "semantic_groups.json"),
	}
	se.loadConfig()
	return se
}

// loadConfig 加载语义组配置
func (se *SemanticExpander) loadConfig() {
	se.mu.Lock()
	defer se.mu.Unlock()

	data, err := os.ReadFile(se.configPath)
	if err != nil {
		// 配置文件不存在，使用默认语义组
		se.initDefaultGroups()
		return
	}

	var groups SemanticGroups
	if err := json.Unmarshal(data, &groups); err != nil {
		se.initDefaultGroups()
		return
	}

	se.groups = &groups
	se.buildWordToGroupMap()
}

// initDefaultGroups 初始化默认语义组
func (se *SemanticExpander) initDefaultGroups() {
	se.groups = &SemanticGroups{
		Groups: map[string]*SemanticGroup{
			"编程语言": {
				Words:  []string{"python", "go", "golang", "javascript", "typescript", "java", "c++", "rust", "编程", "代码", "开发"},
				Weight: 1.0,
			},
			"前端开发": {
				Words:  []string{"react", "vue", "angular", "html", "css", "前端", "组件", "ui", "界面", "样式"},
				Weight: 1.0,
			},
			"后端开发": {
				Words:  []string{"api", "服务器", "数据库", "后端", "接口", "微服务", "rest", "grpc"},
				Weight: 1.0,
			},
			"数据库": {
				Words:  []string{"mysql", "postgresql", "mongodb", "redis", "sqlite", "数据库", "sql", "查询", "索引"},
				Weight: 1.0,
			},
			"AI机器学习": {
				Words:  []string{"ai", "机器学习", "深度学习", "神经网络", "模型", "训练", "推理", "llm", "gpt", "claude"},
				Weight: 1.0,
			},
			"DevOps": {
				Words:  []string{"docker", "kubernetes", "k8s", "ci", "cd", "部署", "容器", "运维", "监控"},
				Weight: 1.0,
			},
		},
	}
	se.buildWordToGroupMap()
}

// buildWordToGroupMap 构建词到组的映射
func (se *SemanticExpander) buildWordToGroupMap() {
	se.wordToGroup = make(map[string][]string)

	for _, group := range se.groups.Groups {
		lowercasedWords := make([]string, len(group.Words))
		for i, w := range group.Words {
			lowercasedWords[i] = strings.ToLower(w)
		}

		for _, word := range lowercasedWords {
			se.wordToGroup[word] = lowercasedWords
		}
	}
}

// Expand 扩展查询词
func (se *SemanticExpander) Expand(queryTokens []string) []string {
	se.mu.RLock()
	defer se.mu.RUnlock()

	expandedTokens := make(map[string]bool)
	activatedGroups := make(map[string]bool)

	for _, token := range queryTokens {
		groupWords, ok := se.wordToGroup[strings.ToLower(token)]
		if !ok {
			continue
		}

		// 使用组词列表作为 key 避免重复激活
		groupKey := strings.Join(groupWords, ",")
		if activatedGroups[groupKey] {
			continue
		}
		activatedGroups[groupKey] = true

		// 添加组内所有词（排除原查询词）
		for _, word := range groupWords {
			found := false
			for _, qt := range queryTokens {
				if strings.ToLower(qt) == word {
					found = true
					break
				}
			}
			if !found {
				expandedTokens[word] = true
			}
		}
	}

	result := make([]string, 0, len(expandedTokens))
	for token := range expandedTokens {
		result = append(result, token)
	}
	return result
}

// SaveConfig 保存语义组配置
func (se *SemanticExpander) SaveConfig() error {
	se.mu.RLock()
	defer se.mu.RUnlock()

	data, err := json.MarshalIndent(se.groups, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(se.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(se.configPath, data, 0644)
}

// AddGroup 添加语义组
func (se *SemanticExpander) AddGroup(name string, words []string, weight float64) {
	se.mu.Lock()
	defer se.mu.Unlock()

	se.groups.Groups[name] = &SemanticGroup{
		Words:  words,
		Weight: weight,
	}
	se.buildWordToGroupMap()
}
