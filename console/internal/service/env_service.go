package service

import (
	"os"
	"sync"

	"github.com/smallnest/goclaw/console/internal/model"
)

// EnvService manages environment variables
type EnvService struct {
	// 存储 API 密钥等敏感信息（持久化到 config）
	secrets map[string]string
	mu      sync.RWMutex
}

// NewEnvService creates a new environment service
func NewEnvService() *EnvService {
	return &EnvService{
		secrets: make(map[string]string),
	}
}

// ListEnvs returns all environment variables (from system env + stored secrets)
func (s *EnvService) ListEnvs() []*model.EnvVar {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 收集所有环境变量
	result := make([]*model.EnvVar, 0)
	seen := make(map[string]bool)

	// 添加系统环境变量（过滤敏感信息）
	for _, env := range os.Environ() {
		// 解析 key=value
		for i := 0; i < len(env); i++ {
			if env[i] == '=' {
				key := env[:i]
				value := env[i+1:]
				
				// 过滤敏感环境变量
				if isSensitiveEnv(key) {
					result = append(result, &model.EnvVar{
						Key:   key,
						Value: maskSensitiveValue(value),
					})
				} else {
					result = append(result, &model.EnvVar{
						Key:   key,
						Value: value,
					})
				}
				seen[key] = true
				break
			}
		}
	}

	// 添加存储的 secrets
	for key, value := range s.secrets {
		if !seen[key] {
			result = append(result, &model.EnvVar{
				Key:   key,
				Value: maskSensitiveValue(value),
			})
		}
	}

	return result
}

// GetEnv returns a specific environment variable
func (s *EnvService) GetEnv(key string) (*model.EnvVar, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 先检查存储的 secrets
	if value, ok := s.secrets[key]; ok {
		return &model.EnvVar{
			Key:   key,
			Value: value,
		}, nil
	}

	// 检查系统环境变量
	value := os.Getenv(key)
	if value != "" {
		return &model.EnvVar{
			Key:   key,
			Value: value,
		}, nil
	}

	return nil, ErrEnvNotFound
}

// UpdateEnvs updates environment variables
// 注意：系统环境变量在运行时不能修改，这里只更新存储的 secrets
func (s *EnvService) UpdateEnvs(req model.EnvUpdateRequest) []*model.EnvVar {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, value := range req {
		s.secrets[key] = value
	}

	return s.ListEnvs()
}

// DeleteEnv deletes an environment variable from storage
func (s *EnvService) DeleteEnv(key string) []*model.EnvVar {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.secrets, key)

	return s.ListEnvs()
}

// SetEnv sets an environment variable for the current process
func (s *EnvService) SetEnv(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 存储到 secrets
	s.secrets[key] = value

	// 尝试设置系统环境变量（对当前进程有效）
	return os.Setenv(key, value)
}

// isSensitiveEnv 判断是否是敏感环境变量
func isSensitiveEnv(key string) bool {
	sensitiveKeys := []string{
		"API_KEY", "SECRET", "PASSWORD", "TOKEN", "CREDENTIAL",
		"PRIVATE_KEY", "ACCESS_KEY", "AUTH",
	}
	
	for _, sk := range sensitiveKeys {
		for i := 0; i <= len(key)-len(sk); i++ {
			match := true
			for j := 0; j < len(sk); j++ {
				c1 := key[i+j]
				c2 := sk[j]
				if c1 >= 'a' && c1 <= 'z' {
					c1 -= 32
				}
				if c1 != c2 {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

// maskSensitiveValue 遮蔽敏感值
func maskSensitiveValue(value string) string {
	if len(value) <= 8 {
		return "***"
	}
	return value[:3] + "***" + value[len(value)-3:]
}

// ErrEnvNotFound is returned when an environment variable is not found
var ErrEnvNotFound = &EnvNotFoundError{}

type EnvNotFoundError struct{}

func (e *EnvNotFoundError) Error() string {
	return "environment variable not found"
}

// InitializeDefaultEnvs creates default environment variables
func (s *EnvService) InitializeDefaultEnvs() {
	// 不再设置假的默认值
	// 从系统环境变量读取实际的 API keys
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		s.secrets["OPENAI_API_KEY"] = apiKey
	}
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		s.secrets["ANTHROPIC_API_KEY"] = apiKey
	}
	if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" {
		s.secrets["OPENROUTER_API_KEY"] = apiKey
	}
}
