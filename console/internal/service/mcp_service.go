package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/smallnest/goclaw/console/internal/model"
	goclawConfig "github.com/smallnest/goclaw/config"
)

// MCPService manages MCP clients with file persistence
type MCPService struct {
	config     *goclawConfig.Config
	configPath string // 配置文件路径
	clients    map[string]*model.MCPClientInfo
	mu         sync.RWMutex
}

// NewMCPService creates a new MCP service
func NewMCPService(cfg *goclawConfig.Config) *MCPService {
	return &MCPService{
		config:  cfg,
		clients: make(map[string]*model.MCPClientInfo),
	}
}

// NewMCPServiceWithPath creates a new MCP service with persistence
func NewMCPServiceWithPath(cfg *goclawConfig.Config, configPath string) *MCPService {
	s := &MCPService{
		config:     cfg,
		configPath: configPath,
		clients:    make(map[string]*model.MCPClientInfo),
	}
	s.loadClients()
	return s
}

// loadClients 从配置文件加载 MCP 客户端
func (s *MCPService) loadClients() {
	if s.configPath == "" {
		return
	}

	// 使用专门的 MCP 配置文件
	mcpConfigPath := filepath.Join(filepath.Dir(s.configPath), "mcp_clients.json")
	data, err := os.ReadFile(mcpConfigPath)
	if err != nil {
		return
	}

	var clients []*model.MCPClientInfo
	if err := json.Unmarshal(data, &clients); err != nil {
		return
	}

	for _, c := range clients {
		s.clients[c.Key] = c
	}
}

// saveClients 保存 MCP 客户端到配置文件
func (s *MCPService) saveClients() error {
	if s.configPath == "" {
		return nil
	}

	clients := make([]*model.MCPClientInfo, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}

	data, err := json.MarshalIndent(clients, "", "  ")
	if err != nil {
		return err
	}

	mcpConfigPath := filepath.Join(filepath.Dir(s.configPath), "mcp_clients.json")
	return os.WriteFile(mcpConfigPath, data, 0644)
}

// ListClients returns all MCP clients
func (s *MCPService) ListClients() []*model.MCPClientInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*model.MCPClientInfo, 0, len(s.clients))
	for _, client := range s.clients {
		result = append(result, client)
	}
	return result
}

// GetClient returns a specific client by key
func (s *MCPService) GetClient(key string) (*model.MCPClientInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[key]
	if !ok {
		return nil, ErrMCPClientNotFound
	}
	return client, nil
}

// CreateClient creates a new MCP client
func (s *MCPService) CreateClient(req *model.CreateMCPClientRequest) *model.MCPClientInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	client := &model.MCPClientInfo{
		Key:         req.ClientKey,
		Name:        req.Client.Name,
		Description: req.Client.Description,
		Enabled:     req.Client.Enabled,
		Transport:   req.Client.Transport,
		URL:         req.Client.URL,
		Headers:     req.Client.Headers,
		Command:     req.Client.Command,
		Args:        req.Client.Args,
		Env:         req.Client.Env,
		CWD:         req.Client.CWD,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if client.Headers == nil {
		client.Headers = make(map[string]string)
	}
	if client.Args == nil {
		client.Args = []string{}
	}
	if client.Env == nil {
		client.Env = make(map[string]string)
	}

	s.clients[req.ClientKey] = client
	s.saveClients() // 持久化
	return client
}

// UpdateClient updates an MCP client
func (s *MCPService) UpdateClient(key string, req *model.MCPClientPayload) (*model.MCPClientInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, ok := s.clients[key]
	if !ok {
		return nil, ErrMCPClientNotFound
	}

	client.Name = req.Name
	client.Description = req.Description
	client.Enabled = req.Enabled
	client.Transport = req.Transport
	client.URL = req.URL
	client.Headers = req.Headers
	client.Command = req.Command
	client.Args = req.Args
	client.Env = req.Env
	client.CWD = req.CWD
	client.UpdatedAt = time.Now()

	s.saveClients() // 持久化
	return client, nil
}

// ToggleClient toggles an MCP client
func (s *MCPService) ToggleClient(key string) (*model.MCPClientInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, ok := s.clients[key]
	if !ok {
		return nil, ErrMCPClientNotFound
	}

	client.Enabled = !client.Enabled
	client.UpdatedAt = time.Now()

	s.saveClients() // 持久化
	return client, nil
}

// DeleteClient deletes an MCP client
func (s *MCPService) DeleteClient(key string) (*model.DeleteMCPClientResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.clients[key]; !ok {
		return nil, ErrMCPClientNotFound
	}

	delete(s.clients, key)
	s.saveClients() // 持久化
	return &model.DeleteMCPClientResponse{
		Message: "MCP client deleted successfully",
	}, nil
}

// ErrMCPClientNotFound is returned when an MCP client is not found
var ErrMCPClientNotFound = &MCPClientNotFoundError{}

type MCPClientNotFoundError struct{}

func (e *MCPClientNotFoundError) Error() string {
	return "MCP client not found"
}
