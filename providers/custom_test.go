package providers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCustomProvider_OpenAI 测试创建兼容 OpenAI 的自定义 Provider
func TestNewCustomProvider_OpenAI(t *testing.T) {
	config := CustomProviderConfig{
		ProviderType: CustomProviderTypeOpenAI,
		APIKey:       "sk-test-key",
		BaseURL:      "https://api.example.com/v1",
		Model:        "gpt-4",
		MaxTokens:    4096,
	}

	provider, err := NewCustomProvider(config)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, CustomProviderTypeOpenAI, provider.GetProviderType())
	assert.Equal(t, "gpt-4", provider.GetModel())
	assert.Equal(t, "https://api.example.com/v1", provider.GetBaseURL())

	err = provider.Close()
	assert.NoError(t, err)
}

// TestNewCustomProvider_Anthropic 测试创建兼容 Anthropic 的自定义 Provider
func TestNewCustomProvider_Anthropic(t *testing.T) {
	config := CustomProviderConfig{
		ProviderType: CustomProviderTypeAnthropic,
		APIKey:       "sk-ant-test-key",
		BaseURL:      "https://api.anthropic.com/v1",
		Model:        "claude-3-opus",
		MaxTokens:    4096,
	}

	provider, err := NewCustomProvider(config)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, CustomProviderTypeAnthropic, provider.GetProviderType())
	assert.Equal(t, "claude-3-opus", provider.GetModel())
	assert.Equal(t, "https://api.anthropic.com/v1", provider.GetBaseURL())

	err = provider.Close()
	assert.NoError(t, err)
}

// TestNewCustomProvider_MissingAPIKey 测试缺少 API Key 时的错误处理
func TestNewCustomProvider_MissingAPIKey(t *testing.T) {
	config := CustomProviderConfig{
		ProviderType: CustomProviderTypeOpenAI,
		APIKey:       "",
		BaseURL:      "https://api.example.com/v1",
		Model:        "gpt-4",
	}

	provider, err := NewCustomProvider(config)
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "API key is required")
}

// TestNewCustomProvider_DefaultModel 测试默认模型
func TestNewCustomProvider_DefaultModel(t *testing.T) {
	config := CustomProviderConfig{
		ProviderType: CustomProviderTypeOpenAI,
		APIKey:       "sk-test-key",
		BaseURL:      "https://api.example.com/v1",
		Model:        "", // 空模型应该使用默认值
		MaxTokens:    4096,
	}

	provider, err := NewCustomProvider(config)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "gpt-4", provider.GetModel()) // 默认模型

	err = provider.Close()
	assert.NoError(t, err)
}

// TestNewCustomProvider_DefaultProviderType 测试默认 Provider 类型
func TestNewCustomProvider_DefaultProviderType(t *testing.T) {
	config := CustomProviderConfig{
		ProviderType: "", // 空类型应该使用默认值
		APIKey:       "sk-test-key",
		BaseURL:      "https://api.example.com/v1",
		Model:        "gpt-4",
		MaxTokens:    4096,
	}

	provider, err := NewCustomProvider(config)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, CustomProviderTypeOpenAI, provider.GetProviderType()) // 默认类型

	err = provider.Close()
	assert.NoError(t, err)
}

// TestNewCustomProviderWithTimeout 测试带超时的 Provider 创建
func TestNewCustomProviderWithTimeout(t *testing.T) {
	config := CustomProviderConfig{
		ProviderType: CustomProviderTypeOpenAI,
		APIKey:       "sk-test-key",
		BaseURL:      "https://api.example.com/v1",
		Model:        "gpt-4",
		MaxTokens:    4096,
	}

	timeout := 30 * time.Second
	provider, err := NewCustomProviderWithTimeout(config, timeout)
	require.NoError(t, err)
	assert.NotNil(t, provider)

	err = provider.Close()
	assert.NoError(t, err)
}

// TestNewCustomProviderFromConfig 测试从配置创建 Provider
func TestNewCustomProviderFromConfig(t *testing.T) {
	tests := []struct {
		name         string
		apiKey       string
		baseURL      string
		model        string
		providerType string
		maxTokens    int
		wantErr      bool
	}{
		{
			name:         "OpenAI provider",
			apiKey:       "sk-test",
			baseURL:      "https://api.openai.com/v1",
			model:        "gpt-4",
			providerType: "openai",
			maxTokens:    4096,
			wantErr:      false,
		},
		{
			name:         "Anthropic provider",
			apiKey:       "sk-ant-test",
			baseURL:      "https://api.anthropic.com/v1",
			model:        "claude-3-opus",
			providerType: "anthropic",
			maxTokens:    4096,
			wantErr:      false,
		},
		{
			name:         "Auto detect from anthropic URL",
			apiKey:       "sk-test",
			baseURL:      "https://api.anthropic.com/v1",
			model:        "claude-3",
			providerType: "", // 空类型，应该根据 URL 自动检测
			maxTokens:    4096,
			wantErr:      false,
		},
		{
			name:         "Auto detect from generic URL",
			apiKey:       "sk-test",
			baseURL:      "https://api.example.com/v1",
			model:        "gpt-4",
			providerType: "", // 空类型，应该默认为 openai
			maxTokens:    4096,
			wantErr:      false,
		},
		{
			name:         "Missing API key",
			apiKey:       "",
			baseURL:      "https://api.example.com/v1",
			model:        "gpt-4",
			providerType: "openai",
			maxTokens:    4096,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewCustomProviderFromConfig(tt.apiKey, tt.baseURL, tt.model, tt.providerType, tt.maxTokens)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
				if provider != nil {
					err = provider.Close()
					assert.NoError(t, err)
				}
			}
		})
	}
}

// TestCustomProvider_Chat 测试聊天功能（需要实际 API key，通常跳过）
func TestCustomProvider_Chat(t *testing.T) {
	// 跳过需要真实 API key 的测试
	apiKey := "" // 设置为真实的 API key 以运行测试
	if apiKey == "" {
		t.Skip("Skipping test: no API key provided")
	}

	config := CustomProviderConfig{
		ProviderType: CustomProviderTypeOpenAI,
		APIKey:       apiKey,
		BaseURL:      "https://api.openai.com/v1",
		Model:        "gpt-3.5-turbo",
		MaxTokens:    100,
	}

	provider, err := NewCustomProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	messages := []Message{
		{Role: "user", Content: "Hello, this is a test."},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := provider.Chat(ctx, messages, nil)
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.NotEmpty(t, response.Content)
}

// TestCustomProvider_ChatWithTools 测试带工具的聊天功能（需要实际 API key，通常跳过）
func TestCustomProvider_ChatWithTools(t *testing.T) {
	// 跳过需要真实 API key 的测试
	apiKey := "" // 设置为真实的 API key 以运行测试
	if apiKey == "" {
		t.Skip("Skipping test: no API key provided")
	}

	config := CustomProviderConfig{
		ProviderType: CustomProviderTypeOpenAI,
		APIKey:       apiKey,
		BaseURL:      "https://api.openai.com/v1",
		Model:        "gpt-4",
		MaxTokens:    100,
	}

	provider, err := NewCustomProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	messages := []Message{
		{Role: "user", Content: "What's the weather like?"},
	}

	tools := []ToolDefinition{
		{
			Name:        "get_weather",
			Description: "Get the current weather for a location",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type":        "string",
						"description": "The city and state, e.g. San Francisco, CA",
					},
				},
				"required": []string{"location"},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := provider.ChatWithTools(ctx, messages, tools)
	require.NoError(t, err)
	assert.NotNil(t, response)
}

// TestContains 测试字符串包含函数
func TestContains(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "foo", false},
		{"", "foo", false},
		{"foo", "", true},
		{"foo", "foo", true},
		{"anthropic api", "anthropic", true},
		{"openai api", "anthropic", false},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			got := contains(tt.s, tt.substr)
			assert.Equal(t, tt.want, got)
		})
	}
}
