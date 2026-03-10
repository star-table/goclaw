package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/smallnest/goclaw/internal/logger"
	"go.uber.org/zap"
)

// SkillWatcher 技能文件监听器
type SkillWatcher struct {
	watcher    *fsnotify.Watcher
	skillsDirs []string
	onChange   func(skillName string, action string)
	mu         sync.RWMutex
	running    bool
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewSkillWatcher 创建技能文件监听器
func NewSkillWatcher(skillsDirs []string, onChange func(skillName string, action string)) (*SkillWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	sw := &SkillWatcher{
		watcher:    watcher,
		skillsDirs: skillsDirs,
		onChange:   onChange,
		ctx:        ctx,
		cancel:     cancel,
	}

	// 添加所有技能目录到监听器
	for _, dir := range skillsDirs {
		if err := sw.addDirRecursive(dir); err != nil {
			logger.Warn("Failed to watch skills directory", zap.String("dir", dir), zap.Error(err))
		}
	}

	return sw, nil
}

// addDirRecursive 递归添加目录到监听器
func (sw *SkillWatcher) addDirRecursive(dir string) error {
	// 检查目录是否存在
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}

	// 添加当前目录
	if err := sw.watcher.Add(dir); err != nil {
		return err
	}

	// 递归添加子目录
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(dir, entry.Name())
			if err := sw.addDirRecursive(subDir); err != nil {
				logger.Warn("Failed to watch subdirectory", zap.String("dir", subDir), zap.Error(err))
			}
		}
	}

	return nil
}

// Start 启动监听
func (sw *SkillWatcher) Start() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.running {
		return
	}

	sw.running = true

	go sw.watchLoop()
}

// Stop 停止监听
func (sw *SkillWatcher) Stop() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if !sw.running {
		return
	}

	sw.cancel()
	sw.watcher.Close()
	sw.running = false
}

// watchLoop 监听循环
func (sw *SkillWatcher) watchLoop() {
	// 防抖处理
	debounceTimer := make(map[string]*time.Timer)
	var debounceMu sync.Mutex

	for {
		select {
		case <-sw.ctx.Done():
			return

		case event, ok := <-sw.watcher.Events:
			if !ok {
				return
			}

			// 只处理 SKILL.md 文件的变化
			if !strings.HasSuffix(event.Name, "SKILL.md") && !strings.HasSuffix(event.Name, "skill.md") {
				continue
			}

			// 获取技能名称（父目录名）
			skillName := filepath.Base(filepath.Dir(event.Name))

			// 防抖处理
			debounceMu.Lock()
			if timer, exists := debounceTimer[skillName]; exists {
				timer.Stop()
			}

			debounceTimer[skillName] = time.AfterFunc(500*time.Millisecond, func() {
				debounceMu.Lock()
				delete(debounceTimer, skillName)
				debounceMu.Unlock()

				// 确定操作类型
				action := "modified"
				if event.Op&fsnotify.Create == fsnotify.Create {
					action = "created"
				} else if event.Op&fsnotify.Remove == fsnotify.Remove {
					action = "deleted"
				} else if event.Op&fsnotify.Rename == fsnotify.Rename {
					action = "renamed"
				}

				logger.Info("Skill file changed",
					zap.String("skill", skillName),
					zap.String("action", action),
					zap.String("file", event.Name))

				// 触发回调
				if sw.onChange != nil {
					sw.onChange(skillName, action)
				}
			})
			debounceMu.Unlock()

		case err, ok := <-sw.watcher.Errors:
			if !ok {
				return
			}
			logger.Error("Skill watcher error", zap.Error(err))
		}
	}
}

// AddSkillDir 添加新的技能目录
func (sw *SkillWatcher) AddSkillDir(dir string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.skillsDirs = append(sw.skillsDirs, dir)
	return sw.addDirRecursive(dir)
}

// RemoveSkillDir 移除技能目录监听
func (sw *SkillWatcher) RemoveSkillDir(dir string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	// 从列表中移除
	for i, d := range sw.skillsDirs {
		if d == dir {
			sw.skillsDirs = append(sw.skillsDirs[:i], sw.skillsDirs[i+1:]...)
			break
		}
	}

	return sw.watcher.Remove(dir)
}
