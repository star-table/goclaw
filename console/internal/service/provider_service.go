package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	goclawConfig "github.com/smallnest/goclaw/config"
	"github.com/smallnest/goclaw/console/internal/model"
	"github.com/smallnest/goclaw/internal/logger"
	"github.com/smallnest/goclaw/providers"
	"go.uber.org/zap"
)

// ProviderService manages model providers using goclaw config
type ProviderService struct {
	config       *goclawConfig.Config
	configPath   string
	providers    map[string]*model.ProviderInfo // 内置 provider 定义
	customAPIKey map[string]string              // 临时存储的 API Key（不持久化）
	mu           sync.RWMutex
}

// NewProviderService creates a new provider service
func NewProviderService(cfg *goclawConfig.Config, configPath string) *ProviderService {
	s := &ProviderService{
		config:       cfg,
		configPath:   configPath,
		providers:    make(map[string]*model.ProviderInfo),
		customAPIKey: make(map[string]string),
	}
	s.initBuiltinProviders()
	return s
}

// initBuiltinProviders 初始化内置 Provider 定义
func (s *ProviderService) initBuiltinProviders() {
	s.providers["openai"] = &model.ProviderInfo{
		ID:           "openai",
		Name:         "OpenAI",
		APIKeyPrefix: "sk-",
		Models: []model.ModelInfo{
			{ID: "gpt-4", Name: "GPT-4", ContextWindow: 8192, MaxTokens: 4096},
			{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", ContextWindow: 128000, MaxTokens: 4096},
			{ID: "gpt-4o", Name: "GPT-4o", ContextWindow: 128000, MaxTokens: 4096},
			{ID: "gpt-4o-mini", Name: "GPT-4o Mini", ContextWindow: 128000, MaxTokens: 4096},
			{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", ContextWindow: 16385, MaxTokens: 4096},
		},
		ExtraModels:    []model.ModelInfo{},
		IsCustom:       false,
		IsLocal:        false,
		NeedsBaseURL:   false,
		CurrentAPIKey:  "",
		CurrentBaseURL: "",
	}

	s.providers["anthropic"] = &model.ProviderInfo{
		ID:           "anthropic",
		Name:         "Anthropic",
		APIKeyPrefix: "sk-ant-",
		Models: []model.ModelInfo{
			{ID: "claude-opus-4-20250514", Name: "Claude Opus 4", ContextWindow: 200000, MaxTokens: 4096},
			{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", ContextWindow: 200000, MaxTokens: 4096},
			{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", ContextWindow: 200000, MaxTokens: 8192},
			{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", ContextWindow: 200000, MaxTokens: 8192},
			{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", ContextWindow: 200000, MaxTokens: 4096},
		},
		ExtraModels:    []model.ModelInfo{},
		IsCustom:       false,
		IsLocal:        false,
		NeedsBaseURL:   false,
		CurrentAPIKey:  "",
		CurrentBaseURL: "",
	}

	s.providers["openrouter"] = &model.ProviderInfo{
		ID:           "openrouter",
		Name:         "OpenRouter",
		APIKeyPrefix: "sk-or-",
		Models: []model.ModelInfo{
			{ID: "anthropic/claude-opus-4", Name: "Claude Opus 4 (OpenRouter)", ContextWindow: 200000, MaxTokens: 4096},
			{ID: "anthropic/claude-sonnet-4", Name: "Claude Sonnet 4 (OpenRouter)", ContextWindow: 200000, MaxTokens: 4096},
			{ID: "anthropic/claude-3.5-sonnet", Name: "Claude 3.5 Sonnet (OpenRouter)", ContextWindow: 200000, MaxTokens: 8192},
			{ID: "openai/gpt-4o", Name: "GPT-4o (OpenRouter)", ContextWindow: 128000, MaxTokens: 4096},
			{ID: "openai/gpt-4-turbo", Name: "GPT-4 Turbo (OpenRouter)", ContextWindow: 128000, MaxTokens: 4096},
			{ID: "google/gemini-pro-1.5", Name: "Gemini Pro 1.5 (OpenRouter)", ContextWindow: 1000000, MaxTokens: 8192},
		},
		ExtraModels:    []model.ModelInfo{},
		IsCustom:       false,
		IsLocal:        false,
		NeedsBaseURL:   true,
		CurrentAPIKey:  "",
		CurrentBaseURL: "https://openrouter.ai/api/v1",
	}
}

// ListProviders returns all providers (built-in + custom from config)
func (s *ProviderService) ListProviders() []*model.ProviderInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*model.ProviderInfo, 0, len(s.providers)+len(s.config.Providers.Profiles))

	// 添加内置 providers
	for id, p := range s.providers {
		// 复制一份，避免修改原始数据
		providerCopy := *p
		providerCopy.ExtraModels = make([]model.ModelInfo, len(p.ExtraModels))
		copy(providerCopy.ExtraModels, p.ExtraModels)

		// 从 config 填充当前配置
		s.fillProviderConfigFromConfig(id, &providerCopy)

		// 从临时存储填充 API Key
		if key, ok := s.customAPIKey[id]; ok {
			providerCopy.CurrentAPIKey = maskAPIKey(key)
		}

		result = append(result, &providerCopy)
	}

	// 从 config.Providers.Profiles 添加自定义 providers
	for _, profile := range s.config.Providers.Profiles {
		// 检查是否已存在（避免重复）
		if _, exists := s.providers[profile.Name]; exists {
			continue
		}

		p := &model.ProviderInfo{
			ID:             profile.Name,
			Name:           profile.Name,
			APIKeyPrefix:   "",
			Models:         []model.ModelInfo{},
			ExtraModels:    []model.ModelInfo{},
			IsCustom:       true,
			IsLocal:        false,
			NeedsBaseURL:   profile.BaseURL != "",
			CurrentAPIKey:  maskAPIKey(profile.APIKey),
			CurrentBaseURL: profile.BaseURL,
		}
		// 确保数组字段不为 nil
		if p.Models == nil {
			p.Models = []model.ModelInfo{}
		}
		if p.ExtraModels == nil {
			p.ExtraModels = []model.ModelInfo{}
		}
		result = append(result, p)
	}

	return result
}

// fillProviderConfigFromConfig 从 goclaw config 填充 provider 配置
func (s *ProviderService) fillProviderConfigFromConfig(id string, p *model.ProviderInfo) {
	switch id {
	case "openai":
		if s.config.Providers.OpenAI.APIKey != "" {
			p.CurrentAPIKey = maskAPIKey(s.config.Providers.OpenAI.APIKey)
		}
		if s.config.Providers.OpenAI.BaseURL != "" {
			p.CurrentBaseURL = s.config.Providers.OpenAI.BaseURL
		}
	case "anthropic":
		if s.config.Providers.Anthropic.APIKey != "" {
			p.CurrentAPIKey = maskAPIKey(s.config.Providers.Anthropic.APIKey)
		}
		if s.config.Providers.Anthropic.BaseURL != "" {
			p.CurrentBaseURL = s.config.Providers.Anthropic.BaseURL
		}
	case "openrouter":
		if s.config.Providers.OpenRouter.APIKey != "" {
			p.CurrentAPIKey = maskAPIKey(s.config.Providers.OpenRouter.APIKey)
		}
		if s.config.Providers.OpenRouter.BaseURL != "" {
			p.CurrentBaseURL = s.config.Providers.OpenRouter.BaseURL
		}
	}
}

// GetProvider returns a specific provider by ID
func (s *ProviderService) GetProvider(id string) (*model.ProviderInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 检查内置 providers
	if p, ok := s.providers[id]; ok {
		providerCopy := *p
		s.fillProviderConfigFromConfig(id, &providerCopy)
		if key, ok := s.customAPIKey[id]; ok {
			providerCopy.CurrentAPIKey = maskAPIKey(key)
		}
		return &providerCopy, nil
	}

	// 检查自定义 providers (from profiles)
	for _, profile := range s.config.Providers.Profiles {
		if profile.Name == id {
			return &model.ProviderInfo{
				ID:             profile.Name,
				Name:           profile.Name,
				APIKeyPrefix:   "",
				Models:         []model.ModelInfo{},
				ExtraModels:    []model.ModelInfo{},
				IsCustom:       true,
				IsLocal:        false,
				NeedsBaseURL:   profile.BaseURL != "",
				CurrentAPIKey:  maskAPIKey(profile.APIKey),
				CurrentBaseURL: profile.BaseURL,
			}, nil
		}
	}

	return nil, ErrProviderNotFound
}

// parseModelString 解析模型标识符为 providerID 和 model
func parseModelString(modelStr string) (providerID, model string) {
	if idx := strings.Index(modelStr, ":"); idx > 0 {
		return modelStr[:idx], modelStr[idx+1:]
	}
	return "", modelStr
}

// GetActiveModels returns the active model from config
func (s *ProviderService) GetActiveModels() *model.ActiveModelsInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	activeLLM := s.config.Agents.Defaults.Model
	if activeLLM == "" {
		activeLLM = "openai:gpt-4"
	}

	providerID, modelName := parseModelString(activeLLM)

	return &model.ActiveModelsInfo{
		ActiveLLM: model.ActiveLLMInfo{
			ProviderID: providerID,
			Model:      modelName,
		},
	}
}

// SetActiveModel sets the active model in config and saves
func (s *ProviderService) SetActiveModel(req *model.SetActiveModelRequest) *model.ActiveModelsInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 构建模型标识符
	var modelStr string
	if req.ProviderID == "openrouter" && !strings.HasPrefix(req.Model, "openrouter:") {
		modelStr = req.Model // OpenRouter 直接使用模型名
	} else if strings.HasPrefix(req.Model, req.ProviderID+":") {
		modelStr = req.Model
	} else {
		modelStr = req.ProviderID + ":" + req.Model
	}

	// 更新 config
	s.config.Agents.Defaults.Model = modelStr

	// 保存配置
	if s.configPath != "" {
		goclawConfig.Save(s.config, s.configPath)
	}

	return &model.ActiveModelsInfo{
		ActiveLLM: model.ActiveLLMInfo{
			ProviderID: req.ProviderID,
			Model:      req.Model,
		},
	}
}

// CreateCustomProvider creates a custom provider profile in config
// 使用 CustomProvider 处理自定义 provider，默认兼容 OpenAI API
func (s *ProviderService) CreateCustomProvider(req *model.CreateCustomProviderRequest) *model.ProviderInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 确定 Provider 类型，默认为 openai (CustomProviderTypeOpenAI)
	providerType := req.ProviderType
	if providerType == "" {
		providerType = string(providers.CustomProviderTypeOpenAI)
	}

	// 验证 provider 类型是否有效
	var customProviderType providers.CustomProviderType
	switch providerType {
	case "anthropic":
		customProviderType = providers.CustomProviderTypeAnthropic
	case "openai", "":
		customProviderType = providers.CustomProviderTypeOpenAI
	default:
		// 尝试根据 baseURL 自动检测
		if req.DefaultBaseURL != "" && strings.Contains(req.DefaultBaseURL, "anthropic") {
			customProviderType = providers.CustomProviderTypeAnthropic
			providerType = "anthropic"
		} else {
			customProviderType = providers.CustomProviderTypeOpenAI
			providerType = "openai"
		}
	}

	// 创建新的 profile，保存所有参数
	profile := goclawConfig.ProviderProfileConfig{
		Name:     req.ID,
		Provider: providerType, // 使用 CustomProvider 类型
		APIKey:   "",
		BaseURL:  req.DefaultBaseURL,
		Priority: len(s.config.Providers.Profiles) + 1,
	}

	s.config.Providers.Profiles = append(s.config.Providers.Profiles, profile)

	// 保存配置
	if s.configPath != "" {
		goclawConfig.Save(s.config, s.configPath)
	}

	// 添加到内置列表（方便后续操作）
	models := req.Models
	if models == nil {
		models = []model.ModelInfo{}
	}

	// 确保数组字段不为 nil
	extraModels := []model.ModelInfo{}

	s.providers[req.ID] = &model.ProviderInfo{
		ID:             req.ID,
		Name:           req.Name,
		APIKeyPrefix:   req.APIKeyPrefix,
		Models:         models,
		ExtraModels:    extraModels,
		IsCustom:       true,
		IsLocal:        false,
		NeedsBaseURL:   req.DefaultBaseURL != "",
		CurrentAPIKey:  "",
		CurrentBaseURL: req.DefaultBaseURL,
	}

	logger.Info("Created custom provider using CustomProvider",
		zap.String("id", req.ID),
		zap.String("name", req.Name),
		zap.String("providerType", providerType),
		zap.String("customProviderType", string(customProviderType)),
		zap.String("baseURL", req.DefaultBaseURL),
		zap.String("apiKeyPrefix", req.APIKeyPrefix))

	return s.providers[req.ID]
}

// DeleteCustomProvider deletes a custom provider profile from config
func (s *ProviderService) DeleteCustomProvider(id string) ([]*model.ProviderInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否是内置 provider
	if p, ok := s.providers[id]; ok && !p.IsCustom {
		return nil, ErrCannotDeleteBuiltin
	}

	// 从 profiles 中删除
	newProfiles := make([]goclawConfig.ProviderProfileConfig, 0)
	for _, profile := range s.config.Providers.Profiles {
		if profile.Name != id {
			newProfiles = append(newProfiles, profile)
		}
	}
	s.config.Providers.Profiles = newProfiles

	// 从内置列表删除
	delete(s.providers, id)
	delete(s.customAPIKey, id)

	// 保存配置
	if s.configPath != "" {
		goclawConfig.Save(s.config, s.configPath)
	}

	// 直接构建返回结果，避免在持有写锁时调用 ListProviders（会导致死锁）
	result := make([]*model.ProviderInfo, 0, len(s.providers)+len(s.config.Providers.Profiles))

	// 添加内置 providers
	for id, p := range s.providers {
		providerCopy := *p
		providerCopy.ExtraModels = make([]model.ModelInfo, len(p.ExtraModels))
		copy(providerCopy.ExtraModels, p.ExtraModels)
		s.fillProviderConfigFromConfig(id, &providerCopy)
		if key, ok := s.customAPIKey[id]; ok {
			providerCopy.CurrentAPIKey = maskAPIKey(key)
		}
		result = append(result, &providerCopy)
	}

	// 从 config.Providers.Profiles 添加自定义 providers
	for _, profile := range s.config.Providers.Profiles {
		if _, exists := s.providers[profile.Name]; exists {
			continue
		}
		p := &model.ProviderInfo{
			ID:             profile.Name,
			Name:           profile.Name,
			APIKeyPrefix:   "",
			Models:         []model.ModelInfo{},
			ExtraModels:    []model.ModelInfo{},
			IsCustom:       true,
			IsLocal:        false,
			NeedsBaseURL:   profile.BaseURL != "",
			CurrentAPIKey:  maskAPIKey(profile.APIKey),
			CurrentBaseURL: profile.BaseURL,
		}
		if p.Models == nil {
			p.Models = []model.ModelInfo{}
		}
		if p.ExtraModels == nil {
			p.ExtraModels = []model.ModelInfo{}
		}
		result = append(result, p)
	}

	return result, nil
}

// ConfigureProvider configures a provider's API key and saves to config
func (s *ProviderService) ConfigureProvider(id string, req *model.ProviderConfigRequest) (*model.ProviderInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 存储原始 API Key 到临时存储（用于测试）
	if req.APIKey != "" {
		s.customAPIKey[id] = req.APIKey
	}

	// 更新 goclaw config
	found := false
	switch id {
	case "openai":
		s.config.Providers.OpenAI.APIKey = req.APIKey
		if req.BaseURL != "" {
			s.config.Providers.OpenAI.BaseURL = req.BaseURL
		}
		found = true
	case "anthropic":
		s.config.Providers.Anthropic.APIKey = req.APIKey
		if req.BaseURL != "" {
			s.config.Providers.Anthropic.BaseURL = req.BaseURL
		}
		found = true
	case "openrouter":
		s.config.Providers.OpenRouter.APIKey = req.APIKey
		if req.BaseURL != "" {
			s.config.Providers.OpenRouter.BaseURL = req.BaseURL
		}
		found = true
	default:
		// 先检查 s.providers 中是否已存在该自定义 provider
		if provider, ok := s.providers[id]; ok {
			// 更新 s.providers 中的信息
			provider.CurrentAPIKey = maskAPIKey(req.APIKey)
			if req.BaseURL != "" {
				provider.CurrentBaseURL = req.BaseURL
			}
			// 更新 config.Profiles 中的对应 profile
			for i, profile := range s.config.Providers.Profiles {
				if profile.Name == id {
					s.config.Providers.Profiles[i].APIKey = req.APIKey
					if req.BaseURL != "" {
						s.config.Providers.Profiles[i].BaseURL = req.BaseURL
					}
					found = true
					break
				}
			}
			// 如果 config.Profiles 中没有，则添加
			if !found {
				newProfile := goclawConfig.ProviderProfileConfig{
					Name:     id,
					Provider: provider.APIKeyPrefix,
					APIKey:   req.APIKey,
					BaseURL:  req.BaseURL,
					Priority: len(s.config.Providers.Profiles) + 1,
				}
				s.config.Providers.Profiles = append(s.config.Providers.Profiles, newProfile)
				found = true
			}
		} else {
			// 如果 s.providers 中也没有，则创建新的
			for i, profile := range s.config.Providers.Profiles {
				if profile.Name == id {
					s.config.Providers.Profiles[i].APIKey = req.APIKey
					if req.BaseURL != "" {
						s.config.Providers.Profiles[i].BaseURL = req.BaseURL
					}
					found = true
					break
				}
			}
			// 如果没有找到对应的 profile，创建一个新的
			if !found {
				newProfile := goclawConfig.ProviderProfileConfig{
					Name:     id,
					Provider: id,
					APIKey:   req.APIKey,
					BaseURL:  req.BaseURL,
					Priority: len(s.config.Providers.Profiles) + 1,
				}
				s.config.Providers.Profiles = append(s.config.Providers.Profiles, newProfile)
				// 同时添加到 providers 列表中
				s.providers[id] = &model.ProviderInfo{
					ID:             id,
					Name:           id,
					APIKeyPrefix:   "",
					Models:         []model.ModelInfo{},
					ExtraModels:    []model.ModelInfo{},
					IsCustom:       true,
					IsLocal:        false,
					NeedsBaseURL:   req.BaseURL != "",
					CurrentAPIKey:  maskAPIKey(req.APIKey),
					CurrentBaseURL: req.BaseURL,
				}
				found = true
			}
		}
	}

	// 保存配置
	if s.configPath != "" {
		goclawConfig.Save(s.config, s.configPath)
	}

	// 直接构建返回结果，避免在持有写锁时调用 GetProvider（会导致死锁）
	providerInfo := &model.ProviderInfo{
		ID:             id,
		Name:           id,
		APIKeyPrefix:   "",
		Models:         []model.ModelInfo{},
		ExtraModels:    []model.ModelInfo{},
		IsCustom:       true,
		IsLocal:        false,
		NeedsBaseURL:   req.BaseURL != "",
		CurrentAPIKey:  maskAPIKey(req.APIKey),
		CurrentBaseURL: req.BaseURL,
	}
	if p, ok := s.providers[id]; ok {
		providerInfo.APIKeyPrefix = p.APIKeyPrefix
		providerInfo.Models = p.Models
		providerInfo.ExtraModels = p.ExtraModels
	}

	return providerInfo, nil
}

// AddModelToProvider adds a model to a provider's models
func (s *ProviderService) AddModelToProvider(providerID string, req *model.AddModelRequest) (*model.ProviderInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	provider, ok := s.providers[providerID]
	if !ok {
		// 检查是否在 config.Providers.Profiles 中
		for _, profile := range s.config.Providers.Profiles {
			if profile.Name == providerID {
				// 在内存中创建 provider
				provider = &model.ProviderInfo{
					ID:             profile.Name,
					Name:           profile.Name,
					APIKeyPrefix:   "",
					Models:         []model.ModelInfo{},
					ExtraModels:    []model.ModelInfo{},
					IsCustom:       true,
					IsLocal:        false,
					NeedsBaseURL:   profile.BaseURL != "",
					CurrentAPIKey:  maskAPIKey(profile.APIKey),
					CurrentBaseURL: profile.BaseURL,
				}
				s.providers[providerID] = provider
				ok = true
				break
			}
		}
		if !ok {
			return nil, ErrProviderNotFound
		}
	}

	// 检查模型 ID 是否已存在
	for _, m := range provider.Models {
		if m.ID == req.ID {
			return nil, ErrModelAlreadyExists
		}
	}
	for _, m := range provider.ExtraModels {
		if m.ID == req.ID {
			return nil, ErrModelAlreadyExists
		}
	}

	modelInfo := model.ModelInfo{
		ID:   req.ID,
		Name: req.Name,
	}

	// 保存到 Models 而不是 ExtraModels
	provider.Models = append(provider.Models, modelInfo)

	logger.Info("Added model to provider",
		zap.String("providerID", providerID),
		zap.String("modelID", req.ID),
		zap.String("modelName", req.Name))

	// 保存配置
	if s.configPath != "" {
		goclawConfig.Save(s.config, s.configPath)
	}

	return provider, nil
}

// RemoveModelFromProvider removes a model from a provider's extra models
func (s *ProviderService) RemoveModelFromProvider(providerID, modelID string) (*model.ProviderInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	provider, ok := s.providers[providerID]
	if !ok {
		return nil, ErrProviderNotFound
	}

	newModels := make([]model.ModelInfo, 0)
	for _, m := range provider.ExtraModels {
		if m.ID != modelID {
			newModels = append(newModels, m)
		}
	}
	provider.ExtraModels = newModels

	return provider, nil
}

// TestProviderConnection tests a provider connection using goclaw providers
func (s *ProviderService) TestProviderConnection(providerID string, req *model.TestProviderRequest) *model.TestProviderResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	startTime := time.Now()

	// 获取 API Key 和 BaseURL
	var apiKey, baseURL, providerType string

	// 优先使用请求中的配置
	if req.APIKey != "" {
		apiKey = req.APIKey
	} else if key, ok := s.customAPIKey[providerID]; ok {
		apiKey = key
	}

	if req.BaseURL != "" {
		baseURL = req.BaseURL
	}

	// 从 config 获取配置
	switch providerID {
	case "openai":
		providerType = "openai"
		if apiKey == "" {
			apiKey = s.config.Providers.OpenAI.APIKey
		}
		if baseURL == "" {
			baseURL = s.config.Providers.OpenAI.BaseURL
		}
	case "anthropic":
		providerType = "anthropic"
		if apiKey == "" {
			apiKey = s.config.Providers.Anthropic.APIKey
		}
		if baseURL == "" {
			baseURL = s.config.Providers.Anthropic.BaseURL
		}
	case "openrouter":
		providerType = "openrouter"
		if apiKey == "" {
			apiKey = s.config.Providers.OpenRouter.APIKey
		}
		if baseURL == "" {
			baseURL = s.config.Providers.OpenRouter.BaseURL
		}
	default:
		// 自定义 provider - 尝试从 profiles 获取
		providerType = "openai" // 自定义 provider 通常兼容 OpenAI API
		for _, profile := range s.config.Providers.Profiles {
			if profile.Name == providerID {
				if apiKey == "" {
					apiKey = profile.APIKey
				}
				if baseURL == "" {
					baseURL = profile.BaseURL
				}
				providerType = profile.Provider
				break
			}
		}
	}

	// 检测自定义 provider 的实际类型
	if providerID != "openai" && providerID != "anthropic" && providerID != "openrouter" {
		if strings.Contains(baseURL, "deepseek.com") {
			providerType = "deepseek"
		} else if strings.Contains(baseURL, "anthropic") {
			providerType = "anthropic"
		} else if strings.Contains(baseURL, "openrouter.ai") {
			providerType = "openrouter"
		}
	}

	if apiKey == "" {
		return &model.TestProviderResponse{
			Success: false,
			Message: "No API key configured for provider: " + providerID,
			Latency: int(time.Since(startTime).Milliseconds()),
		}
	}

	// 创建临时 provider 进行测试
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var prov providers.Provider
	var err error

	// 根据 provider 类型选择测试模型
	switch providerType {
	case "openai":
		prov, err = providers.NewOpenAIProviderWithTimeout(apiKey, baseURL, "gpt-4o-mini", 100, 10*time.Second)
	case "anthropic":
		prov, err = providers.NewAnthropicProviderWithTimeout(apiKey, baseURL, "claude-3-haiku-20240307", 100, 10*time.Second)
	case "openrouter":
		prov, err = providers.NewOpenRouterProviderWithTimeout(apiKey, baseURL, "openai/gpt-4o-mini", 100, 10*time.Second)
	case "deepseek":
		prov, err = providers.NewOpenAIProviderWithTimeout(apiKey, baseURL, "deepseek-chat", 100, 10*time.Second)
	default:
		// 自定义 provider 使用 CustomProvider 处理，默认兼容 OpenAI API
		config := providers.CustomProviderConfig{
			ProviderType: providers.CustomProviderTypeOpenAI,
			APIKey:       apiKey,
			BaseURL:      baseURL,
			Model:        "gpt-4o-mini",
			MaxTokens:    100,
			Timeout:      10 * time.Second,
		}
		// 如果 providerType 是 anthropic，使用 Anthropic 类型
		if providerType == "anthropic" {
			config.ProviderType = providers.CustomProviderTypeAnthropic
			config.Model = "claude-3-haiku-20240307"
		}
		prov, err = providers.NewCustomProviderWithTimeout(config, 10*time.Second)
	}

	if err != nil {
		return &model.TestProviderResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create provider: %v", err),
			Latency: int(time.Since(startTime).Milliseconds()),
		}
	}
	defer prov.Close()

	// 发送简单的测试消息
	response, err := prov.Chat(ctx, []providers.Message{
		{Role: "user", Content: "Say 'OK' if you can read this message."},
	}, nil)

	if err != nil {
		return &model.TestProviderResponse{
			Success: false,
			Message: fmt.Sprintf("Connection test failed: %v", err),
			Latency: int(time.Since(startTime).Milliseconds()),
		}
	}

	return &model.TestProviderResponse{
		Success: true,
		Message: fmt.Sprintf("Connection to %s successful. Response: %s", providerID, response.Content),
		Latency: int(time.Since(startTime).Milliseconds()),
	}
}

// TestModelConnection tests a specific model
func (s *ProviderService) TestModelConnection(providerID string, req *model.TestModelRequest) *model.TestModelResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	startTime := time.Now()

	// 获取 API Key 和 BaseURL
	var apiKey, baseURL, providerType string

	switch providerID {
	case "openai":
		providerType = "openai"
		apiKey = s.config.Providers.OpenAI.APIKey
		if key, ok := s.customAPIKey[providerID]; ok && key != "" {
			apiKey = key
		}
		baseURL = s.config.Providers.OpenAI.BaseURL
	case "anthropic":
		providerType = "anthropic"
		apiKey = s.config.Providers.Anthropic.APIKey
		if key, ok := s.customAPIKey[providerID]; ok && key != "" {
			apiKey = key
		}
		baseURL = s.config.Providers.Anthropic.BaseURL
	case "openrouter":
		providerType = "openrouter"
		apiKey = s.config.Providers.OpenRouter.APIKey
		if key, ok := s.customAPIKey[providerID]; ok && key != "" {
			apiKey = key
		}
		baseURL = s.config.Providers.OpenRouter.BaseURL
	default:
		providerType = "openai"
		for _, profile := range s.config.Providers.Profiles {
			if profile.Name == providerID {
				apiKey = profile.APIKey
				baseURL = profile.BaseURL
				providerType = profile.Provider
				break
			}
		}
		// 检测自定义 provider 的实际类型
		if providerID != "openai" && providerID != "anthropic" && providerID != "openrouter" {
			if baseURL == "" {
				// 从 s.providers 中获取 baseURL
				if p, ok := s.providers[providerID]; ok {
					baseURL = p.CurrentBaseURL
				}
			}
			if strings.Contains(baseURL, "deepseek.com") {
				providerType = "deepseek"
			} else if strings.Contains(baseURL, "anthropic") {
				providerType = "anthropic"
			} else if strings.Contains(baseURL, "openrouter.ai") {
				providerType = "openrouter"
			}
		}
	}

	if apiKey == "" {
		return &model.TestModelResponse{
			Success: false,
			Message: "No API key configured for provider: " + providerID,
			Latency: int(time.Since(startTime).Milliseconds()),
		}
	}

	// 创建临时 provider 进行测试
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var prov providers.Provider
	var err error

	modelID := req.ModelID
	if modelID == "" {
		modelID = "gpt-4o-mini"
	}

	switch providerType {
	case "openai":
		prov, err = providers.NewOpenAIProviderWithTimeout(apiKey, baseURL, modelID, 100, 15*time.Second)
	case "anthropic":
		prov, err = providers.NewAnthropicProviderWithTimeout(apiKey, baseURL, modelID, 100, 15*time.Second)
	case "openrouter":
		prov, err = providers.NewOpenRouterProviderWithTimeout(apiKey, baseURL, modelID, 100, 15*time.Second)
	case "deepseek":
		prov, err = providers.NewOpenAIProviderWithTimeout(apiKey, baseURL, modelID, 100, 15*time.Second)
	default:
		prov, err = providers.NewOpenAIProviderWithTimeout(apiKey, baseURL, modelID, 100, 15*time.Second)
	}

	if err != nil {
		return &model.TestModelResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create provider: %v", err),
			Latency: int(time.Since(startTime).Milliseconds()),
		}
	}
	defer prov.Close()

	// 发送简单的测试消息
	response, err := prov.Chat(ctx, []providers.Message{
		{Role: "user", Content: "Say 'OK' if you can read this message."},
	}, nil)

	if err != nil {
		return &model.TestModelResponse{
			Success: false,
			Message: fmt.Sprintf("Model test failed: %v", err),
			Latency: int(time.Since(startTime).Milliseconds()),
		}
	}

	return &model.TestModelResponse{
		Success: true,
		Message: fmt.Sprintf("Model %s on %s is working. Response: %s", req.ModelID, providerID, response.Content),
		Latency: int(time.Since(startTime).Milliseconds()),
	}
}

// maskAPIKey masks an API key for display
func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return key[:2] + "***"
	}
	return key[:3] + "***" + key[len(key)-3:]
}

// ErrProviderNotFound is returned when a provider is not found
var ErrProviderNotFound = &ProviderNotFoundError{}

type ProviderNotFoundError struct{}

func (e *ProviderNotFoundError) Error() string {
	return "provider not found"
}

// ErrCannotDeleteBuiltin is returned when trying to delete a built-in provider
var ErrCannotDeleteBuiltin = &CannotDeleteBuiltinError{}

type CannotDeleteBuiltinError struct{}

func (e *CannotDeleteBuiltinError) Error() string {
	return "cannot delete built-in provider"
}

// ErrModelAlreadyExists is returned when trying to add a model that already exists
var ErrModelAlreadyExists = &ModelAlreadyExistsError{}

type ModelAlreadyExistsError struct{}

func (e *ModelAlreadyExistsError) Error() string {
	return "model already exists"
}

// InitializeDefaultProviders creates default providers (for backward compatibility)
func (s *ProviderService) InitializeDefaultProviders() {
	// Providers are already initialized in initBuiltinProviders()
	// This method exists for backward compatibility
}
