package agent

import (
	"os"
	"path/filepath"

	"github.com/smallnest/goclaw/internal/logger"
	"go.uber.org/zap"
)

// EnableHotReload 启用技能热重载
func (l *SkillsLoader) EnableHotReload(onChange func(skillName string, action string)) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.watcher != nil {
		// 已经启用了
		return nil
	}

	l.onSkillChanged = onChange

	// 创建监听器
	watcher, err := NewSkillWatcher(l.skillsDirs, l.handleSkillChange)
	if err != nil {
		return err
	}

	l.watcher = watcher
	l.watcher.Start()

	logger.Info("Skill hot reload enabled", zap.Strings("dirs", l.skillsDirs))
	return nil
}

// DisableHotReload 禁用技能热重载
func (l *SkillsLoader) DisableHotReload() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.watcher != nil {
		l.watcher.Stop()
		l.watcher = nil
		logger.Info("Skill hot reload disabled")
	}
}

// handleSkillChange 处理技能文件变化
func (l *SkillsLoader) handleSkillChange(skillName string, action string) {
	logger.Info("Handling skill change",
		zap.String("skill", skillName),
		zap.String("action", action))

	switch action {
	case "created", "modified":
		l.reloadSkill(skillName)
	case "deleted":
		l.removeSkill(skillName)
	case "renamed":
		// 重新加载所有技能
		l.ReloadAll()
	}

	// 触发外部回调
	if l.onSkillChanged != nil {
		l.onSkillChanged(skillName, action)
	}
}

// reloadSkill 重新加载单个技能
func (l *SkillsLoader) reloadSkill(skillName string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 查找技能所在的目录
	var skillPath string
	var source string

	for idx, dir := range l.skillsDirs {
		potentialPath := filepath.Join(dir, skillName)
		if _, err := os.Stat(potentialPath); err == nil {
			skillPath = potentialPath
			source = l.detectSourceType(dir, idx)
			break
		}
	}

	if skillPath == "" {
		logger.Warn("Skill not found for reload", zap.String("skill", skillName))
		return
	}

	// 加载技能
	if err := l.loadSkill(skillPath, source); err != nil {
		logger.Error("Failed to reload skill",
			zap.String("skill", skillName),
			zap.Error(err))
		return
	}

	logger.Info("Skill reloaded successfully", zap.String("skill", skillName))
}

// removeSkill 移除技能
func (l *SkillsLoader) removeSkill(skillName string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.skills[skillName]; exists {
		delete(l.skills, skillName)

		// 从 alwaysSkills 中移除
		for i, name := range l.alwaysSkills {
			if name == skillName {
				l.alwaysSkills = append(l.alwaysSkills[:i], l.alwaysSkills[i+1:]...)
				break
			}
		}

		logger.Info("Skill removed", zap.String("skill", skillName))
	}
}

// ReloadAll 重新加载所有技能
func (l *SkillsLoader) ReloadAll() {
	l.mu.Lock()
	defer l.mu.Unlock()

	logger.Info("Reloading all skills")

	// 清空现有技能
	l.skills = make(map[string]*Skill)
	l.alwaysSkills = []string{}

	// 重新发现技能
	if err := l.Discover(); err != nil {
		logger.Error("Failed to reload all skills", zap.Error(err))
		return
	}

	logger.Info("All skills reloaded", zap.Int("count", len(l.skills)))
}

// IsHotReloadEnabled 检查热重载是否已启用
func (l *SkillsLoader) IsHotReloadEnabled() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.watcher != nil
}

// GetSkillWithReload 获取技能（支持热重载）
func (l *SkillsLoader) GetSkillWithReload(name string) (*Skill, bool) {
	l.mu.RLock()
	skill, exists := l.skills[name]
	l.mu.RUnlock()

	if !exists && l.IsHotReloadEnabled() {
		// 尝试重新加载
		l.reloadSkill(name)

		l.mu.RLock()
		skill, exists = l.skills[name]
		l.mu.RUnlock()
	}

	return skill, exists
}

// WatchSkillDir 添加新的技能目录监听（热重载启用时）
func (l *SkillsLoader) WatchSkillDir(dir string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 添加到目录列表
	for _, d := range l.skillsDirs {
		if d == dir {
			return nil // 已经存在
		}
	}
	l.skillsDirs = append(l.skillsDirs, dir)

	// 如果热重载已启用，添加监听
	if l.watcher != nil {
		return l.watcher.AddSkillDir(dir)
	}

	return nil
}
