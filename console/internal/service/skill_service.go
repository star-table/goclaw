package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/smallnest/goclaw/agent"
	"github.com/smallnest/goclaw/console/internal/model"
	"gopkg.in/yaml.v3"
)

// SkillService manages skills using goclaw SkillsLoader
type SkillService struct {
	skillsDir       string // base skills directory (~/.goclaw/skills)
	activeSkillsDir string // active skills directory (~/.goclaw/active_skills)
	loader          *agent.SkillsLoader
	enabledMap      map[string]bool
	mu              sync.RWMutex
}

// NewSkillService creates a new skill service
func NewSkillService(loader *agent.SkillsLoader, skillsDir string, activeSkillsDir string) *SkillService {
	if activeSkillsDir == "" {
		// Default active skills dir is sibling of skills dir
		activeSkillsDir = filepath.Join(filepath.Dir(skillsDir), "active_skills")
	}
	os.MkdirAll(activeSkillsDir, 0755)

	svc := &SkillService{
		loader:          loader,
		skillsDir:       skillsDir,
		activeSkillsDir: activeSkillsDir,
		enabledMap:      make(map[string]bool),
	}

	// Load enabled state from active_skills directory
	svc.loadEnabledState()

	return svc
}

// loadEnabledState loads enabled state from active_skills directory
func (s *SkillService) loadEnabledState() {
	entries, err := os.ReadDir(s.activeSkillsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Check if SKILL.md exists
			skillPath := filepath.Join(s.activeSkillsDir, entry.Name(), "SKILL.md")
			if _, err := os.Stat(skillPath); err == nil {
				s.enabledMap[entry.Name()] = true
			}
		}
	}
}

// ListSkills returns all skills
func (s *SkillService) ListSkills() []*model.SkillSpec {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.loader == nil {
		return []*model.SkillSpec{}
	}

	skills := s.loader.List()
	result := make([]*model.SkillSpec, 0, len(skills))
	for _, skill := range skills {
		enabled := s.enabledMap[skill.Name]
		source := s.getSkillSource(skill.Name)
		spec := convertSkillToSpec(skill, enabled, source, s.skillsDir)
		result = append(result, spec)
	}
	return result
}

// ListAvailableSkills returns only enabled skills
func (s *SkillService) ListAvailableSkills() []*model.SkillSpec {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.loader == nil {
		return []*model.SkillSpec{}
	}

	skills := s.loader.List()
	result := make([]*model.SkillSpec, 0)
	for _, skill := range skills {
		if s.enabledMap[skill.Name] {
			source := s.getSkillSource(skill.Name)
			spec := convertSkillToSpec(skill, true, source, s.skillsDir)
			result = append(result, spec)
		}
	}
	return result
}

// GetSkill returns a specific skill by name
func (s *SkillService) GetSkill(name string) (*model.SkillSpec, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.loader == nil {
		return nil, ErrSkillNotFound
	}

	skill, ok := s.loader.Get(name)
	if !ok {
		return nil, ErrSkillNotFound
	}

	enabled := s.enabledMap[name]
	source := s.getSkillSource(name)
	return convertSkillToSpec(skill, enabled, source, s.skillsDir), nil
}

// GetBuiltinSkills returns all builtin skills
func (s *SkillService) GetBuiltinSkills() []*model.SkillSpec {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.loader == nil {
		return []*model.SkillSpec{}
	}

	skills := s.loader.List()
	result := make([]*model.SkillSpec, 0)
	for _, skill := range skills {
		if skill.Source == "builtin" {
			enabled := s.enabledMap[skill.Name]
			spec := convertSkillToSpec(skill, enabled, "builtin", s.skillsDir)
			result = append(result, spec)
		}
	}
	return result
}

// getSkillSource determines the source of a skill
func (s *SkillService) getSkillSource(name string) string {
	// Check if in active_skills
	activePath := filepath.Join(s.activeSkillsDir, name, "SKILL.md")
	if _, err := os.Stat(activePath); err == nil {
		return "active"
	}

	// Check if in skills directory (customized)
	customPath := filepath.Join(s.skillsDir, name, "SKILL.md")
	if _, err := os.Stat(customPath); err == nil {
		return "customized"
	}

	// Otherwise it's builtin
	return "builtin"
}

// CreateSkill creates a new skill (supports skill importer)
func (s *SkillService) CreateSkill(req *model.CreateSkillRequest) *model.CreateSkillResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create skill directory in skills dir (customized)
	skillDir := filepath.Join(s.skillsDir, req.Name)

	// Check if skill already exists
	if _, err := os.Stat(skillDir); err == nil && !req.Overwrite {
		return &model.CreateSkillResponse{
			Created: false,
			Name:    req.Name,
		}
	}

	// Clean up existing directory if overwriting
	if req.Overwrite {
		os.RemoveAll(skillDir)
	}

	os.MkdirAll(skillDir, 0755)

	// Write SKILL.md
	skillPath := filepath.Join(skillDir, "SKILL.md")
	os.WriteFile(skillPath, []byte(req.Content), 0644)

	// Write references files
	if req.References != nil {
		s.writeTreeFiles(skillDir, "references", req.References)
	}

	// Write scripts files
	if req.Scripts != nil {
		s.writeTreeFiles(skillDir, "scripts", req.Scripts)
	}

	// Write extra files
	if req.ExtraFiles != nil {
		s.writeTreeFiles(skillDir, "", req.ExtraFiles)
	}

	// Re-discover skills if loader is available
	if s.loader != nil {
		s.loader.Discover()
	}

	return &model.CreateSkillResponse{
		Created: true,
		Name:    req.Name,
	}
}

// writeTreeFiles writes files from a tree structure (matches Python logic)
func (s *SkillService) writeTreeFiles(baseDir, subdir string, tree map[string]interface{}) {
	dirPath := filepath.Join(baseDir, subdir)
	os.MkdirAll(dirPath, 0755)

	for name, value := range tree {
		switch v := value.(type) {
		case string:
			// It's a file with content
			filePath := filepath.Join(dirPath, name)
			os.WriteFile(filePath, []byte(v), 0644)
		case map[string]interface{}:
			// It's a nested directory
			s.writeTreeFiles(baseDir, filepath.Join(subdir, name), v)
		case nil:
			// Empty file (for creating empty files)
			filePath := filepath.Join(dirPath, name)
			os.WriteFile(filePath, []byte{}, 0644)
		}
	}
}

// EnableSkill enables a skill by copying to active_skills
func (s *SkillService) EnableSkill(name string) (*model.EnableSkillResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loader == nil {
		return nil, ErrSkillNotFound
	}

	if _, ok := s.loader.Get(name); !ok {
		return nil, ErrSkillNotFound
	}

	s.enabledMap[name] = true
	s.syncSkillToActive(name)

	return &model.EnableSkillResponse{Enabled: true}, nil
}

// syncSkillToActive copies skill to active_skills directory
func (s *SkillService) syncSkillToActive(name string) {
	// Find source directory
	var sourceDir string
	source := s.getSkillSource(name)

	switch source {
	case "customized":
		sourceDir = filepath.Join(s.skillsDir, name)
	case "builtin":
		// For builtin skills, we need to find them in the loader's skills map
		if skill, ok := s.loader.Get(name); ok {
			// Create in active_skills with just SKILL.md content
			activeDir := filepath.Join(s.activeSkillsDir, name)
			os.MkdirAll(activeDir, 0755)
			skillPath := filepath.Join(activeDir, "SKILL.md")
			os.WriteFile(skillPath, []byte(skill.Content), 0644)
			return
		}
	default:
		// Already active or unknown source
		sourceDir = filepath.Join(s.skillsDir, name)
	}

	if sourceDir != "" {
		// Copy to active_skills
		activeDir := filepath.Join(s.activeSkillsDir, name)
		os.MkdirAll(activeDir, 0755)

		// Copy SKILL.md
		srcPath := filepath.Join(sourceDir, "SKILL.md")
		dstPath := filepath.Join(activeDir, "SKILL.md")
		content, err := os.ReadFile(srcPath)
		if err == nil {
			os.WriteFile(dstPath, content, 0644)
		}

		// Copy references directory
		srcRefs := filepath.Join(sourceDir, "references")
		if entries, err := os.ReadDir(srcRefs); err == nil {
			dstRefs := filepath.Join(activeDir, "references")
			os.MkdirAll(dstRefs, 0755)
			for _, entry := range entries {
				copyFileOrDir(srcRefs, dstRefs, entry.Name())
			}
		}

		// Copy scripts directory
		srcScripts := filepath.Join(sourceDir, "scripts")
		if entries, err := os.ReadDir(srcScripts); err == nil {
			dstScripts := filepath.Join(activeDir, "scripts")
			os.MkdirAll(dstScripts, 0755)
			for _, entry := range entries {
				copyFileOrDir(srcScripts, dstScripts, entry.Name())
			}
		}
	}
}

// copyFileOrDir copies a file or directory
func copyFileOrDir(srcDir, dstDir, name string) {
	srcPath := filepath.Join(srcDir, name)
	dstPath := filepath.Join(dstDir, name)

	info, err := os.Stat(srcPath)
	if err != nil {
		return
	}

	if info.IsDir() {
		os.MkdirAll(dstPath, 0755)
		if entries, err := os.ReadDir(srcPath); err == nil {
			for _, entry := range entries {
				copyFileOrDir(srcPath, dstPath, entry.Name())
			}
		}
	} else {
		content, err := os.ReadFile(srcPath)
		if err == nil {
			os.WriteFile(dstPath, content, 0644)
		}
	}
}

// DisableSkill disables a skill by removing from active_skills
func (s *SkillService) DisableSkill(name string) (*model.DisableSkillResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loader == nil {
		return nil, ErrSkillNotFound
	}

	if _, ok := s.loader.Get(name); !ok {
		return nil, ErrSkillNotFound
	}

	s.enabledMap[name] = false

	// Remove from active_skills
	activeDir := filepath.Join(s.activeSkillsDir, name)
	os.RemoveAll(activeDir)

	return &model.DisableSkillResponse{Disabled: true}, nil
}

// BatchEnableSkills enables multiple skills
func (s *SkillService) BatchEnableSkills(names []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, name := range names {
		s.enabledMap[name] = true
		s.syncSkillToActive(name)
	}
}

// BatchDisableSkills disables multiple skills
func (s *SkillService) BatchDisableSkills(names []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, name := range names {
		s.enabledMap[name] = false
		activeDir := filepath.Join(s.activeSkillsDir, name)
		os.RemoveAll(activeDir)
	}
}

// DeleteSkill deletes a skill
func (s *SkillService) DeleteSkill(name string) (*model.DeleteSkillResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loader == nil {
		return nil, ErrSkillNotFound
	}

	if _, ok := s.loader.Get(name); !ok {
		return nil, ErrSkillNotFound
	}

	// Remove from skills directory
	skillDir := filepath.Join(s.skillsDir, name)
	os.RemoveAll(skillDir)

	// Remove from active_skills
	activeDir := filepath.Join(s.activeSkillsDir, name)
	os.RemoveAll(activeDir)

	delete(s.enabledMap, name)

	// Re-discover skills
	s.loader.Discover()

	return &model.DeleteSkillResponse{Deleted: true}, nil
}

// SearchHubSkills searches skills in the hub
func (s *SkillService) SearchHubSkills(query string, limit int) []*model.HubSkillSpec {
	// Use SkillsLoader.Search if available
	if s.loader != nil {
		results := s.loader.Search(query)
		hubSkills := make([]*model.HubSkillSpec, 0, len(results))
		for _, r := range results {
			if r.Skill != nil {
				hubSkills = append(hubSkills, &model.HubSkillSpec{
					Slug:        r.Skill.Name,
					Name:        r.Skill.Name,
					Description: r.Skill.Description,
					Version:     r.Skill.Version,
					SourceURL:   r.Skill.Homepage,
				})
			}
		}
		return hubSkills
	}

	// Fallback mock
	return []*model.HubSkillSpec{
		{
			Slug:        "code-assistant",
			Name:        "Code Assistant",
			Description: "Help with coding tasks, debugging, and code review",
			Version:     "1.0.0",
			SourceURL:   "https://hub.copaw.io/skills/code-assistant/bundle.tar.gz",
		},
	}
}

// InstallSkill installs a skill from the hub
func (s *SkillService) InstallSkill(req *model.InstallSkillRequest) *model.InstallSkillResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate a name from bundle URL or use UUID
	name := extractSkillNameFromURL(req.BundleURL)
	if name == "" {
		name = "installed-skill-" + uuid.New().String()[:8]
	}

	// Check if overwrite is needed
	skillDir := filepath.Join(s.skillsDir, name)
	if _, err := os.Stat(skillDir); err == nil && !req.Overwrite {
		return &model.InstallSkillResponse{
			Installed: false,
			Name:      name,
			Enabled:   false,
			SourceURL: req.BundleURL,
			Message:   "Skill already exists. Use overwrite=true to replace.",
		}
	}

	// Create skill directory
	os.MkdirAll(skillDir, 0755)

	// Create skill file (in real implementation, would download from bundle_url)
	skillPath := filepath.Join(skillDir, "SKILL.md")
	content := "---\nname: " + name + "\nsource: hub\n---\n\nInstalled from " + req.BundleURL
	os.WriteFile(skillPath, []byte(content), 0644)

	s.enabledMap[name] = req.Enable
	if req.Enable {
		s.syncSkillToActive(name)
	}

	// Re-discover skills
	if s.loader != nil {
		s.loader.Discover()
	}

	return &model.InstallSkillResponse{
		Installed: true,
		Name:      name,
		Enabled:   req.Enable,
		SourceURL: req.BundleURL,
	}
}

// extractSkillNameFromURL extracts skill name from bundle URL
func extractSkillNameFromURL(url string) string {
	// Try to extract from URL path
	parts := strings.Split(url, "/")
	for i, part := range parts {
		if part == "skills" && i+1 < len(parts) {
			name := parts[i+1]
			// Remove file extensions
			name = strings.TrimSuffix(name, ".tar.gz")
			name = strings.TrimSuffix(name, ".zip")
			return name
		}
	}
	return ""
}

// LoadSkillFile loads a skill file
func (s *SkillService) LoadSkillFile(skillName, source, filePath string) (*model.SkillFileContent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.loader == nil {
		return nil, ErrSkillNotFound
	}

	skill, ok := s.loader.Get(skillName)
	if !ok {
		return nil, ErrSkillNotFound
	}

	// Return skill content if file path is SKILL.md or empty
	if filePath == "SKILL.md" || filePath == "" || filePath == "/" {
		return &model.SkillFileContent{
			Content: skill.Content,
		}, nil
	}

	// Try to read from active_skills first, then skills directory
	searchDirs := []string{
		filepath.Join(s.activeSkillsDir, skillName),
		filepath.Join(s.skillsDir, skillName),
	}

	for _, dir := range searchDirs {
		fullPath := filepath.Join(dir, filePath)
		content, err := os.ReadFile(fullPath)
		if err == nil {
			return &model.SkillFileContent{
				Content: string(content),
			}, nil
		}
	}

	// Fallback to skill content
	return &model.SkillFileContent{
		Content: skill.Content,
	}, nil
}

// InstallDependencies installs dependencies for a skill
func (s *SkillService) InstallDependencies(skillName string) error {
	if s.loader == nil {
		return ErrSkillNotFound
	}
	return s.loader.InstallDependencies(skillName)
}

// Helper function to convert agent.Skill to model.SkillSpec
func convertSkillToSpec(skill *agent.Skill, enabled bool, source, skillsDir string) *model.SkillSpec {
	// Determine skill path
	skillPath := "/skills/" + skill.Name
	if source == "active" {
		skillPath = filepath.Join(filepath.Dir(skillsDir), "active_skills", skill.Name)
	} else if source == "customized" {
		skillPath = filepath.Join(skillsDir, skill.Name)
	}

	return &model.SkillSpec{
		Name:       skill.Name,
		Content:    skill.Content,
		Source:     source,
		Path:       skillPath,
		Enabled:    enabled,
		References: readDirectoryTree(skillPath, "references"),
		Scripts:    readDirectoryTree(skillPath, "scripts"),
	}
}

// readDirectoryTree reads a directory and returns its structure as a tree
// Files are represented as {filename: nil}, directories as {dirname: {nested}}
func readDirectoryTree(basePath, subdir string) map[string]interface{} {
	result := make(map[string]interface{})

	dirPath := filepath.Join(basePath, subdir)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Recursively read subdirectory
			subResult := readDirectoryTree(dirPath, entry.Name())
			result[entry.Name()] = subResult
		} else {
			// File - in list mode, we just indicate existence with nil
			// But for content mode, we could read the file content
			result[entry.Name()] = nil
		}
	}

	return result
}

// ErrSkillNotFound is returned when a skill is not found
var ErrSkillNotFound = &SkillNotFoundError{}

type SkillNotFoundError struct{}

func (e *SkillNotFoundError) Error() string {
	return "skill not found"
}

// ValidateSkillResponse represents skill validation response
type ValidateSkillResponse struct {
	Valid    bool     `json:"valid"`
	Name     string   `json:"name"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ValidateSkill validates a skill by name
func (s *SkillService) ValidateSkill(name string) *ValidateSkillResponse {
	result := &ValidateSkillResponse{
		Valid:    true,
		Name:     name,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.loader == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "Skill loader not initialized")
		return result
	}

	skill, ok := s.loader.Get(name)
	if !ok {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Skill '%s' not found", name))
		return result
	}

	// Validate name
	if skill.Name == "" {
		result.Errors = append(result.Errors, "Skill name is empty")
		result.Valid = false
	}

	// Validate content
	if skill.Content == "" {
		result.Errors = append(result.Errors, "Skill content is empty")
		result.Valid = false
	}

	// Validate YAML front matter
	if !strings.HasPrefix(skill.Content, "---") {
		result.Warnings = append(result.Warnings, "Skill content missing YAML front matter")
	} else {
		// Try to parse front matter
		parts := strings.SplitN(skill.Content, "---", 3)
		if len(parts) >= 3 {
			var meta map[string]interface{}
			if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Invalid YAML front matter: %v", err))
				result.Valid = false
			} else {
				// Check required fields
				if _, ok := meta["name"]; !ok {
					result.Warnings = append(result.Warnings, "Missing 'name' in front matter")
				}
				if _, ok := meta["description"]; !ok {
					result.Warnings = append(result.Warnings, "Missing 'description' in front matter")
				}
			}
		}
	}

	// Check for missing dependencies
	if skill.MissingDeps != nil {
		if len(skill.MissingDeps.Bins) > 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Missing binaries: %v", skill.MissingDeps.Bins))
		}
		if len(skill.MissingDeps.PythonPkgs) > 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Missing Python packages: %v", skill.MissingDeps.PythonPkgs))
		}
		if len(skill.MissingDeps.NodePkgs) > 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Missing Node.js packages: %v", skill.MissingDeps.NodePkgs))
		}
	}

	return result
}

// SkillConfig represents skill configuration for persistence
type SkillConfig struct {
	EnabledSkills []string `json:"enabled_skills"`
}

// SaveConfig saves the enabled skills configuration
func (s *SkillService) SaveConfig() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config := SkillConfig{
		EnabledSkills: make([]string, 0),
	}

	for name, enabled := range s.enabledMap {
		if enabled {
			config.EnabledSkills = append(config.EnabledSkills, name)
		}
	}

	configPath := filepath.Join(filepath.Dir(s.skillsDir), "skill_config.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// LoadConfig loads the enabled skills configuration
func (s *SkillService) LoadConfig() error {
	configPath := filepath.Join(filepath.Dir(s.skillsDir), "skill_config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var config SkillConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, name := range config.EnabledSkills {
		s.enabledMap[name] = true
	}

	return nil
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	SyncedCount  int      `json:"synced_count"`
	SkippedCount int      `json:"skipped_count"`
	Errors       []string `json:"errors,omitempty"`
}

// SyncSkillsToActive syncs skills from skillsDir to active_skills directory
// This is equivalent to Python's sync_skills_to_working_dir
func (s *SkillService) SyncSkillsToActive(skillNames []string, force bool) *SyncResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := &SyncResult{
		SyncedCount:  0,
		SkippedCount: 0,
		Errors:       make([]string, 0),
	}

	// Collect skills from skillsDir
	skillsToSync := s.collectSkillsFromDir(s.skillsDir)
	if len(skillsToSync) == 0 {
		return result
	}

	// Filter by skillNames if specified
	if len(skillNames) > 0 {
		filtered := make(map[string]string)
		for _, name := range skillNames {
			if path, ok := skillsToSync[name]; ok {
				filtered[name] = path
			}
		}
		skillsToSync = filtered
	}

	// Sync each skill
	for skillName, skillDir := range skillsToSync {
		targetDir := filepath.Join(s.activeSkillsDir, skillName)

		// Check if skill already exists
		if _, err := os.Stat(targetDir); err == nil && !force {
			result.SkippedCount++
			continue
		}

		// Remove existing if force
		if force {
			os.RemoveAll(targetDir)
		}

		// Copy skill directory
		if err := s.copyDir(skillDir, targetDir); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to sync skill '%s': %v", skillName, err))
			continue
		}

		result.SyncedCount++
		s.enabledMap[skillName] = true
	}

	return result
}

// SyncBuiltinSkillsToActive syncs builtin skills from a specific source directory to active_skills
func (s *SkillService) SyncBuiltinSkillsToActive(builtinSkillsDir string, skillNames []string, force bool) *SyncResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := &SyncResult{
		SyncedCount:  0,
		SkippedCount: 0,
		Errors:       make([]string, 0),
	}

	// Collect skills from builtin directory
	skillsToSync := s.collectSkillsFromDir(builtinSkillsDir)
	if len(skillsToSync) == 0 {
		return result
	}

	// Filter by skillNames if specified
	if len(skillNames) > 0 {
		filtered := make(map[string]string)
		for _, name := range skillNames {
			if path, ok := skillsToSync[name]; ok {
				filtered[name] = path
			}
		}
		skillsToSync = filtered
	}

	// Sync each skill
	for skillName, skillDir := range skillsToSync {
		targetDir := filepath.Join(s.activeSkillsDir, skillName)

		// Check if skill already exists
		if _, err := os.Stat(targetDir); err == nil && !force {
			result.SkippedCount++
			continue
		}

		// Remove existing if force
		if force {
			os.RemoveAll(targetDir)
		}

		// Copy skill directory
		if err := s.copyDir(skillDir, targetDir); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to sync skill '%s': %v", skillName, err))
			continue
		}

		result.SyncedCount++
		s.enabledMap[skillName] = true
	}

	return result
}

// collectSkillsFromDir collects skills from a directory
func (s *SkillService) collectSkillsFromDir(dir string) map[string]string {
	skills := make(map[string]string)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return skills
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillDir := filepath.Join(dir, entry.Name())
		skillFile := filepath.Join(skillDir, "SKILL.md")
		if _, err := os.Stat(skillFile); err == nil {
			skills[entry.Name()] = skillDir
		}
	}

	return skills
}

// copyDir copies a directory recursively
func (s *SkillService) copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		// Copy file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(targetPath, data, 0644)
	})
}
