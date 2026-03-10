package model

// ModelInfo represents a model in a provider
type ModelInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ContextWindow int    `json:"context_window,omitempty"`
	MaxTokens     int    `json:"max_tokens,omitempty"`
	InputPrice    string `json:"input_price,omitempty"`
	OutputPrice   string `json:"output_price,omitempty"`
	Description   string `json:"description,omitempty"`
}

// ProviderInfo represents a model provider
type ProviderInfo struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	APIKeyPrefix   string      `json:"api_key_prefix"`
	Models         []ModelInfo `json:"models"`
	ExtraModels    []ModelInfo `json:"extra_models"`
	IsCustom       bool        `json:"is_custom"`
	IsLocal        bool        `json:"is_local"`
	NeedsBaseURL   bool        `json:"needs_base_url"`
	CurrentAPIKey  string      `json:"current_api_key"`
	CurrentBaseURL string      `json:"current_base_url"`
}

// ActiveLLMInfo represents the active LLM with provider and model
type ActiveLLMInfo struct {
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
}

// ActiveModelsInfo represents active models configuration
type ActiveModelsInfo struct {
	ActiveLLM ActiveLLMInfo `json:"active_llm"`
}

// SetActiveModelRequest represents a request to set active model
type SetActiveModelRequest struct {
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
}

// CreateCustomProviderRequest represents a request to create custom provider
type CreateCustomProviderRequest struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	DefaultBaseURL string      `json:"default_base_url"`
	APIKeyPrefix   string      `json:"api_key_prefix"`
	ProviderType   string      `json:"provider_type"` // "openai" or "anthropic"
	Models         []ModelInfo `json:"models"`
}

// ProviderConfigRequest represents a request to configure provider
type ProviderConfigRequest struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url,omitempty"`
}

// AddModelRequest represents a request to add model to provider
type AddModelRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// TestProviderRequest represents a request to test provider
type TestProviderRequest struct {
	APIKey  string `json:"api_key,omitempty"`
	BaseURL string `json:"base_url,omitempty"`
}

// TestProviderResponse represents test provider response
type TestProviderResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Latency int    `json:"latency"`
}

// TestModelRequest represents a request to test model
type TestModelRequest struct {
	ModelID string `json:"model_id"`
}

// TestModelResponse represents test model response
type TestModelResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Latency int    `json:"latency"`
}
