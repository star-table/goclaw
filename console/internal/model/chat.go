package model

import "time"

// ChatSpec represents a chat specification
type ChatSpec struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	SessionID string                 `json:"session_id"`
	UserID    string                 `json:"user_id"`
	Channel   string                 `json:"channel"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Meta      map[string]interface{} `json:"meta"`
}

// CreateChatRequest represents a request to create a chat
type CreateChatRequest struct {
	UserID  string                 `json:"user_id"`
	Channel string                 `json:"channel"`
	Meta    map[string]interface{} `json:"meta"`
}

// UpdateChatRequest represents a request to update a chat
type UpdateChatRequest struct {
	Name string                 `json:"name,omitempty"`
	Meta map[string]interface{} `json:"meta,omitempty"`
}

// ContentItem represents a single content item in a message (matches Python format)
type ContentItem struct {
	SequenceNumber *int                    `json:"sequence_number,omitempty"`
	Object         string                  `json:"object,omitempty"`
	Status         string                  `json:"status,omitempty"`
	Error          interface{}             `json:"error,omitempty"`
	Type           string                  `json:"type"` // "text", "data", etc.
	Index          int                     `json:"index,omitempty"`
	Delta          bool                    `json:"delta,omitempty"`
	MsgID          string                  `json:"msg_id,omitempty"`
	Text           string                  `json:"text,omitempty"`
	Data           map[string]interface{}  `json:"data,omitempty"`
}

// ChatMessage represents a message in chat history (matches Python format)
type ChatMessage struct {
	SequenceNumber *int                    `json:"sequence_number,omitempty"`
	Object         string                  `json:"object,omitempty"`
	Status         string                  `json:"status,omitempty"`
	Error          interface{}             `json:"error,omitempty"`
	ID             string                  `json:"id"`
	Type           string                  `json:"type,omitempty"` // "message", "plugin_call", "plugin_call_output"
	Role           string                  `json:"role"`
	Content        []ContentItem           `json:"content"`
	Code           interface{}             `json:"code,omitempty"`
	Message        interface{}             `json:"message,omitempty"`
	Usage          interface{}             `json:"usage,omitempty"`
	Metadata       MessageMetadata         `json:"metadata,omitempty"`
}

// MessageMetadata represents message metadata
type MessageMetadata struct {
	OriginalID   string      `json:"original_id,omitempty"`
	OriginalName string      `json:"original_name,omitempty"`
	Metadata     interface{} `json:"metadata,omitempty"`
}

// ChatHistory represents chat history with messages
type ChatHistory struct {
	Messages []ChatMessage `json:"messages"`
}

// DeleteChatResponse represents delete chat response
type DeleteChatResponse struct {
	Status string `json:"status"`
	ID     string `json:"id"`
}

// BatchDeleteChatsRequest represents a request to batch delete chats
type BatchDeleteChatsRequest struct {
	ChatIDs []string `json:"chat_ids"`
}

// BatchDeleteChatsResponse represents batch delete response
type BatchDeleteChatsResponse struct {
	Success      bool `json:"success"`
	DeletedCount int  `json:"deleted_count"`
}
