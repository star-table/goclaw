package model

// EnvVar represents an environment variable
type EnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// EnvUpdateRequest represents a request to update environment variables
type EnvUpdateRequest map[string]string
