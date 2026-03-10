package model

// SkillSpec represents a skill specification (matches Python SkillSpec)
type SkillSpec struct {
	Name       string                 `json:"name"`
	Content    string                 `json:"content"`
	Source     string                 `json:"source"`     // "builtin", "customized", or "active"
	Path       string                 `json:"path"`
	Enabled    bool                   `json:"enabled"`
	References map[string]interface{} `json:"references"` // directory tree structure
	Scripts    map[string]interface{} `json:"scripts"`    // directory tree structure
}

// CreateSkillRequest represents a request to create a skill (matches Python CreateSkillRequest)
type CreateSkillRequest struct {
	Name       string                 `json:"name"`
	Content    string                 `json:"content"`
	References map[string]interface{} `json:"references,omitempty"` // {filename: content} for files
	Scripts    map[string]interface{} `json:"scripts,omitempty"`    // {filename: content} for files
	ExtraFiles map[string]interface{} `json:"extra_files,omitempty"` // Additional files for imported skills
	Overwrite  bool                   `json:"overwrite,omitempty"`   // Overwrite existing skill
}

// CreateSkillResponse represents create skill response
type CreateSkillResponse struct {
	Created bool   `json:"created"`
	Name    string `json:"name"`
}

// EnableSkillResponse represents enable skill response
type EnableSkillResponse struct {
	Enabled bool `json:"enabled"`
}

// DisableSkillResponse represents disable skill response
type DisableSkillResponse struct {
	Disabled bool `json:"disabled"`
}

// DeleteSkillResponse represents delete skill response
type DeleteSkillResponse struct {
	Deleted bool `json:"deleted"`
}

// HubSkillSpec represents a skill from the hub (matches Python HubSkillResult)
type HubSkillSpec struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"` // matches Python "descriptor"
	Version     string `json:"version"`
	SourceURL   string `json:"source_url"`
}

// InstallSkillRequest represents a request to install skill from hub (matches Python HubInstallRequest)
type InstallSkillRequest struct {
	BundleURL string `json:"bundle_url"`
	Version   string `json:"version,omitempty"`
	Enable    bool   `json:"enable"`
	Overwrite bool   `json:"overwrite"`
}

// InstallSkillResponse represents install skill response (matches Python HubInstallResult)
type InstallSkillResponse struct {
	Installed bool   `json:"installed"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	SourceURL string `json:"source_url"`
	Message   string `json:"message,omitempty"`
}

// SkillFileContent represents skill file content
type SkillFileContent struct {
	Content string `json:"content"`
}

// BatchSkillRequest represents a request for batch operations
type BatchSkillRequest struct {
	Names []string `json:"names"`
}
