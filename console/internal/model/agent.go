package model

import "time"

// AgentStatus represents the agent service status
type AgentStatus struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// AgentHealth represents the agent health check response
type AgentHealth struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// ContentPartType defines content part types (matches Python agentscope_runtime)
type ContentPartType string

const (
	ContentTypeText    ContentPartType = "text"
	ContentTypeImage   ContentPartType = "image"
	ContentTypeRefusal ContentPartType = "refusal"
	ContentTypeVideo   ContentPartType = "video"
	ContentTypeAudio   ContentPartType = "audio"
	ContentTypeFile    ContentPartType = "file"
)

// ContentStatus defines content status
type ContentStatus string

const (
	ContentStatusCreated    ContentStatus = "created"
	ContentStatusInProgress ContentStatus = "in_progress"
	ContentStatusCompleted  ContentStatus = "completed"
)

// ContentPart represents a part of message content (matches Python runtime types)
type ContentPart struct {
	Object         string          `json:"object"`
	SequenceNumber int             `json:"sequence_number"`
	Status         ContentStatus   `json:"status,omitempty"`
	Error          *AgentError     `json:"error,omitempty"`
	Type           ContentPartType `json:"type"`
	Index          int             `json:"index,omitempty"`
	Delta          *bool           `json:"delta,omitempty"`
	MsgID          string          `json:"msg_id,omitempty"`
	Text           string          `json:"text,omitempty"`
	ImageURL       string          `json:"image_url,omitempty"`
	Data           interface{}     `json:"data,omitempty"`
}

// MessageType defines message types
type MessageType string

const (
	MessageTypeMessage MessageType = "message"
)

// MessageRole defines message roles
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
)

// AgentMessage represents a message in agent conversation (matches Python Message)
type AgentMessage struct {
	Object         string                 `json:"object"`
	SequenceNumber int                    `json:"sequence_number"`
	ID             string                 `json:"id"`
	Type           MessageType            `json:"type"`
	Role           MessageRole            `json:"role"`
	Status         ContentStatus          `json:"status,omitempty"`
	Error          *AgentError            `json:"error,omitempty"`
	Code           string                 `json:"code,omitempty"`
	Message        string                 `json:"message,omitempty"`
	Content        []ContentPart          `json:"content,omitempty"`
	Usage          map[string]interface{} `json:"usage,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// AgentProcessRequest represents a request to process agent input
// Matches Python AgentRequest format from agentscope_runtime
type AgentProcessRequest struct {
	Input     []AgentMessage `json:"input"`
	SessionID string         `json:"session_id"`
	UserID    string         `json:"user_id"`
	Channel   string         `json:"channel"`
	Stream    bool           `json:"stream"`
}

// AgentProcessResponse represents the response from agent processing
type AgentProcessResponse struct {
	SessionID string                 `json:"session_id"`
	Output    map[string]interface{} `json:"output"`
	Status    string                 `json:"status"`
}

// AgentEvent represents a streaming event (matches Python Event format)
type AgentEvent struct {
	Object       string        `json:"object"`
	ID           string        `json:"id"`
	Type         string        `json:"type,omitempty"`
	Status       string        `json:"status,omitempty"`
	Content      []ContentPart `json:"content,omitempty"`
	Error        *AgentError   `json:"error,omitempty"`
	Timestamp    int64         `json:"timestamp,omitempty"`
	// Additional fields for content-level events
	Role     string      `json:"role,omitempty"`
	MsgID    string      `json:"msg_id,omitempty"`
	Delta    bool        `json:"delta,omitempty"`
	Text     string      `json:"text,omitempty"`
	ImageURL string      `json:"image_url,omitempty"`
	Data     interface{} `json:"data,omitempty"`
	// Fields to match Python API response format
	SequenceNumber int                    `json:"sequence_number,omitempty"`
	CreatedAt      int64                  `json:"created_at,omitempty"`
	CompletedAt    int64                  `json:"completed_at,omitempty"`
	Output         []AgentMessage         `json:"output,omitempty"`
	Usage          map[string]interface{} `json:"usage,omitempty"`
	SessionID      string                 `json:"session_id,omitempty"`
	Code           string                 `json:"code,omitempty"`
	Message        string                 `json:"message,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// AgentError represents an error in agent response
type AgentError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// AgentAdminStatus represents the processing status
type AgentAdminStatus struct {
	Active    bool    `json:"active"`
	SessionID string  `json:"session_id"`
	Status    string  `json:"status"`
	StartTime *string `json:"start_time,omitempty"`
	Progress  int     `json:"progress"`
}

// AgentRunningConfig represents the agent running configuration
type AgentRunningConfig struct {
	MaxIters       int `json:"max_iters"`
	MaxInputLength int `json:"max_input_length"`
}

// LegacyAgentProcessRequest for backward compatibility
type LegacyAgentProcessRequest struct {
	Input     string `json:"input"`
	SessionID string `json:"session_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	Channel   string `json:"channel,omitempty"`
}
