package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/smallnest/goclaw/internal/logger"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"go.uber.org/zap"
)

// CustomProviderType 定义自定义 Provider 类型
type CustomProviderType string

const (
	// CustomProviderTypeOpenAI 兼容 OpenAI API 的自定义 Provider
	CustomProviderTypeOpenAI CustomProviderType = "openai"
	// CustomProviderTypeAnthropic 兼容 Anthropic API 的自定义 Provider
	CustomProviderTypeAnthropic CustomProviderType = "anthropic"
)

// CustomProvider 自定义 Provider，兼容多种 API 格式
type CustomProvider struct {
	providerType CustomProviderType
	llm          *openai.LLM
	model        string
	maxTokens    int
	timeout      time.Duration
	apiKey       string
	baseURL      string
}

// CustomProviderConfig 自定义 Provider 配置
type CustomProviderConfig struct {
	ProviderType CustomProviderType
	APIKey       string
	BaseURL      string
	Model        string
	MaxTokens    int
	Timeout      time.Duration
}

// NewCustomProvider 创建自定义 Provider
func NewCustomProvider(config CustomProviderConfig) (*CustomProvider, error) {
	return NewCustomProviderWithTimeout(config, 0)
}

// NewCustomProviderWithTimeout 创建带超时的自定义 Provider
func NewCustomProviderWithTimeout(config CustomProviderConfig, timeout time.Duration) (*CustomProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	if config.Model == "" {
		config.Model = "gpt-4"
	}

	if config.ProviderType == "" {
		config.ProviderType = CustomProviderTypeOpenAI
	}

	// 根据 Provider 类型创建对应的实现
	switch config.ProviderType {
	case CustomProviderTypeOpenAI:
		return newOpenAICompatibleProvider(config, timeout)
	case CustomProviderTypeAnthropic:
		return newAnthropicCompatibleProvider(config, timeout)
	default:
		return nil, fmt.Errorf("unsupported custom provider type: %s", config.ProviderType)
	}
}

// newOpenAICompatibleProvider 创建兼容 OpenAI API 的 Provider
func newOpenAICompatibleProvider(config CustomProviderConfig, timeout time.Duration) (*CustomProvider, error) {
	opts := []openai.Option{
		openai.WithToken(config.APIKey),
		openai.WithModel(config.Model),
	}

	if config.BaseURL != "" {
		opts = append(opts, openai.WithBaseURL(config.BaseURL))
		logger.Info("Custom OpenAI provider configured with base URL",
			zap.String("baseURL", config.BaseURL),
			zap.String("model", config.Model))
	}

	// 设置超时
	if timeout > 0 {
		httpClient := &http.Client{
			Timeout: timeout,
		}
		opts = append(opts, openai.WithHTTPClient(httpClient))
		logger.Info("Custom OpenAI provider configured with timeout",
			zap.Duration("timeout", timeout))
	}

	llm, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create custom OpenAI provider: %w", err)
	}

	return &CustomProvider{
		providerType: CustomProviderTypeOpenAI,
		llm:          llm,
		model:        config.Model,
		maxTokens:    config.MaxTokens,
		timeout:      timeout,
		apiKey:       config.APIKey,
		baseURL:      config.BaseURL,
	}, nil
}

// newAnthropicCompatibleProvider 创建兼容 Anthropic API 的 Provider
func newAnthropicCompatibleProvider(config CustomProviderConfig, timeout time.Duration) (*CustomProvider, error) {
	// Anthropic 兼容模式也使用 OpenAI 客户端，但会添加 Anthropic 特定的 header 和格式转换
	opts := []openai.Option{
		openai.WithToken(config.APIKey),
		openai.WithModel(config.Model),
	}

	if config.BaseURL != "" {
		opts = append(opts, openai.WithBaseURL(config.BaseURL))
		logger.Info("Custom Anthropic provider configured with base URL",
			zap.String("baseURL", config.BaseURL),
			zap.String("model", config.Model))
	}

	// 设置超时
	if timeout > 0 {
		httpClient := &http.Client{
			Timeout: timeout,
		}
		opts = append(opts, openai.WithHTTPClient(httpClient))
		logger.Info("Custom Anthropic provider configured with timeout",
			zap.Duration("timeout", timeout))
	}

	llm, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create custom Anthropic provider: %w", err)
	}

	return &CustomProvider{
		providerType: CustomProviderTypeAnthropic,
		llm:          llm,
		model:        config.Model,
		maxTokens:    config.MaxTokens,
		timeout:      timeout,
		apiKey:       config.APIKey,
		baseURL:      config.BaseURL,
	}, nil
}

// Chat 聊天
func (p *CustomProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition, options ...ChatOption) (*Response, error) {
	opts := &ChatOptions{
		Model:       p.model,
		Temperature: 0.7,
		MaxTokens:   p.maxTokens,
		Stream:      false,
	}

	for _, opt := range options {
		opt(opts)
	}

	// 转换消息
	langchainMessages := make([]llms.MessageContent, len(messages))
	for i, msg := range messages {
		var role llms.ChatMessageType
		switch msg.Role {
		case "user":
			role = llms.ChatMessageTypeHuman
		case "assistant":
			role = llms.ChatMessageTypeAI
		case "system":
			role = llms.ChatMessageTypeSystem
		case "tool":
			role = llms.ChatMessageTypeTool
		default:
			role = llms.ChatMessageTypeHuman
		}

		if msg.Role == "tool" {
			langchainMessages[i] = llms.MessageContent{
				Role: role,
				Parts: []llms.ContentPart{
					llms.ToolCallResponse{
						ToolCallID: msg.ToolCallID,
						Name:       msg.ToolName,
						Content:    msg.Content,
					},
				},
			}
		} else if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			parts := []llms.ContentPart{
				llms.TextPart(msg.Content),
			}
			for _, tc := range msg.ToolCalls {
				args, _ := json.Marshal(tc.Params)
				parts = append(parts, llms.ToolCall{
					ID:   tc.ID,
					Type: "function",
					FunctionCall: &llms.FunctionCall{
						Name:      tc.Name,
						Arguments: string(args),
					},
				})
			}
			langchainMessages[i] = llms.MessageContent{
				Role:  role,
				Parts: parts,
			}
		} else {
			langchainMessages[i] = llms.TextParts(role, msg.Content)
		}
	}

	// 调用 LLM
	var llmOpts []llms.CallOption
	if opts.Temperature > 0 {
		llmOpts = append(llmOpts, llms.WithTemperature(float64(opts.Temperature)))
	}
	if opts.MaxTokens > 0 {
		llmOpts = append(llmOpts, llms.WithMaxTokens(int(opts.MaxTokens)))
	}

	// 如果有工具，添加工具选项
	if len(tools) > 0 {
		langchainTools := make([]llms.Tool, len(tools))
		for i, tool := range tools {
			langchainTools[i] = llms.Tool{
				Type: "function",
				Function: &llms.FunctionDefinition{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			}
		}
		llmOpts = append(llmOpts, llms.WithTools(langchainTools))
	}

	completion, err := p.llm.GenerateContent(ctx, langchainMessages, llmOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	// 解析工具调用
	var toolCalls []ToolCall
	if len(completion.Choices) > 0 {
		if len(completion.Choices[0].ToolCalls) > 0 {
			logger.Debug("Found tool calls from custom provider",
				zap.String("providerType", string(p.providerType)),
				zap.Int("count", len(completion.Choices[0].ToolCalls)))
			for _, tc := range completion.Choices[0].ToolCalls {
				logger.Debug("Tool call",
					zap.String("id", tc.ID),
					zap.String("name", tc.FunctionCall.Name),
					zap.String("args", tc.FunctionCall.Arguments))
			}
		}
		for _, tc := range completion.Choices[0].ToolCalls {
			var params map[string]interface{}
			if err := json.Unmarshal([]byte(tc.FunctionCall.Arguments), &params); err != nil {
				logger.Error("Failed to unmarshal tool arguments",
					zap.String("tool", tc.FunctionCall.Name),
					zap.String("id", tc.ID),
					zap.Error(err),
					zap.String("raw_args", tc.FunctionCall.Arguments))

				params = map[string]interface{}{
					"__error__":         fmt.Sprintf("Failed to parse arguments: %v", err),
					"__raw_arguments__": tc.FunctionCall.Arguments,
				}
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:     tc.ID,
				Name:   tc.FunctionCall.Name,
				Params: params,
			})
		}
	}

	response := &Response{
		Content:      completion.Choices[0].Content,
		ToolCalls:    toolCalls,
		FinishReason: "stop",
	}

	return response, nil
}

// ChatWithTools 聊天（带工具）
func (p *CustomProvider) ChatWithTools(ctx context.Context, messages []Message, tools []ToolDefinition, options ...ChatOption) (*Response, error) {
	return p.Chat(ctx, messages, tools, options...)
}

// Close 关闭连接
func (p *CustomProvider) Close() error {
	return nil
}

// GetProviderType 获取 Provider 类型
func (p *CustomProvider) GetProviderType() CustomProviderType {
	return p.providerType
}

// GetModel 获取当前模型
func (p *CustomProvider) GetModel() string {
	return p.model
}

// GetBaseURL 获取 Base URL
func (p *CustomProvider) GetBaseURL() string {
	return p.baseURL
}

// NewCustomProviderFromConfig 从配置创建自定义 Provider
func NewCustomProviderFromConfig(apiKey, baseURL, model string, providerType string, maxTokens int) (Provider, error) {
	config := CustomProviderConfig{
		APIKey:    apiKey,
		BaseURL:   baseURL,
		Model:     model,
		MaxTokens: maxTokens,
	}

	switch providerType {
	case "anthropic":
		config.ProviderType = CustomProviderTypeAnthropic
	case "openai", "":
		config.ProviderType = CustomProviderTypeOpenAI
	default:
		// 尝试根据 baseURL 自动检测
		if baseURL != "" {
			if contains(baseURL, "anthropic") {
				config.ProviderType = CustomProviderTypeAnthropic
			} else {
				config.ProviderType = CustomProviderTypeOpenAI
			}
		} else {
			config.ProviderType = CustomProviderTypeOpenAI
		}
	}

	return NewCustomProvider(config)
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsInternal(s, substr))
}

func containsInternal(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
