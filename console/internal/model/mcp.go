package model

import "time"

// MCPClientInfo represents an MCP client configuration
type MCPClientInfo struct {
	Key         string                 `json:"key"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Enabled     bool                   `json:"enabled"`
	Transport   string                 `json:"transport"`
	URL         string                 `json:"url"`
	Headers     map[string]string      `json:"headers"`
	Command     string                 `json:"command"`
	Args        []string               `json:"args"`
	Env         map[string]string      `json:"env"`
	CWD         string                 `json:"cwd"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// CreateMCPClientRequest represents a request to create MCP client
type CreateMCPClientRequest struct {
	ClientKey string            `json:"client_key"`
	Client    MCPClientPayload  `json:"client"`
}

// MCPClientPayload represents MCP client payload for create/update
type MCPClientPayload struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
	Transport   string            `json:"transport"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env"`
	CWD         string            `json:"cwd"`
}

// DeleteMCPClientResponse represents delete MCP client response
type DeleteMCPClientResponse struct {
	Message string `json:"message"`
}
