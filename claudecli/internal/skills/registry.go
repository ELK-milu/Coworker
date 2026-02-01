package skills

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Registry 技能注册表
type Registry struct {
	skills map[string]*Skill
	mu     sync.RWMutex
}

// NewRegistry 创建技能注册表
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]*Skill),
	}
}

// Register 注册技能
func (r *Registry) Register(skill *Skill) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[skill.Name] = skill
	return nil
}

// Get 获取技能
func (r *Registry) Get(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	skill, ok := r.skills[name]
	return skill, ok
}

// GetAll 获取所有技能
func (r *Registry) GetAll() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Skill, 0, len(r.skills))
	for _, s := range r.skills {
		result = append(result, s)
	}
	return result
}

// LoadFromDir 从目录加载技能
func (r *Registry) LoadFromDir(dir, source string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			path := filepath.Join(dir, entry.Name())
			skill, err := ParseSkillFile(path)
			if err != nil {
				continue
			}
			skill.Source = source
			r.Register(skill)
		}
	}
	return nil
}
