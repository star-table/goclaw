package service

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/smallnest/goclaw/console/internal/model"
)

const (
	// MaxAgeSeconds is the max age of messages in seconds (60s like Python)
	MaxAgeSeconds = 60
	// MaxMessages is the max number of messages to keep (500 like Python)
	MaxMessages = 500
)

// ConsoleService manages console push messages (matches Python console_push_store)
type ConsoleService struct {
	messages []*model.PushMessage
	mu       sync.RWMutex
}

// NewConsoleService creates a new console service
func NewConsoleService() *ConsoleService {
	return &ConsoleService{
		messages: make([]*model.PushMessage, 0),
	}
}

// Append adds a message to the store (bounded: oldest dropped if over MaxMessages)
func (s *ConsoleService) Append(sessionID, text string) {
	if sessionID == "" || text == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	msg := &model.PushMessage{
		ID:        uuid.New().String(),
		Text:      text,
		Timestamp: float64(time.Now().UnixNano()) / 1e9, // seconds with decimals
		SessionID: sessionID,
	}

	s.messages = append(s.messages, msg)

	// Bound by count: drop oldest if over MaxMessages
	if len(s.messages) > MaxMessages {
		// Sort by timestamp and keep most recent
		s.sortByTimestamp()
		s.messages = s.messages[len(s.messages)-MaxMessages:]
	}
}

// Take returns and removes all messages for the session (consumes them)
func (s *ConsoleService) Take(sessionID string) []*model.ConsoleMessage {
	if sessionID == "" {
		return []*model.ConsoleMessage{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var out []*model.PushMessage
	var remaining []*model.PushMessage

	for _, m := range s.messages {
		if m.SessionID == sessionID {
			out = append(out, m)
		} else {
			remaining = append(remaining, m)
		}
	}

	s.messages = remaining
	return s.stripTimestamp(out)
}

// TakeAll returns and removes all messages
func (s *ConsoleService) TakeAll() []*model.ConsoleMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := s.messages
	s.messages = make([]*model.PushMessage, 0)
	return s.stripTimestamp(out)
}

// GetRecent returns recent messages (not consumed), drops older than MaxAgeSeconds
func (s *ConsoleService) GetRecent() []*model.ConsoleMessage {
	return s.GetRecentWithAge(MaxAgeSeconds)
}

// GetRecentWithAge returns recent messages with custom max age
func (s *ConsoleService) GetRecentWithAge(maxAgeSeconds int) []*model.ConsoleMessage {
	now := float64(time.Now().UnixNano()) / 1e9
	cutoff := now - float64(maxAgeSeconds)

	s.mu.Lock()
	defer s.mu.Unlock()

	var out []*model.PushMessage
	for _, m := range s.messages {
		if m.Timestamp >= cutoff {
			out = append(out, m)
		}
	}

	// Update store to only keep recent messages (bound memory)
	s.messages = out
	return s.stripTimestamp(out)
}

// GetPushMessages returns push messages based on sessionID (for API compatibility)
func (s *ConsoleService) GetPushMessages(sessionID string) *model.PushMessagesResponse {
	var messages []*model.ConsoleMessage

	if sessionID != "" {
		messages = s.Take(sessionID)
	} else {
		messages = s.GetRecent()
	}

	// Convert to PushMessage format for response
	result := make([]*model.PushMessage, len(messages))
	for i, m := range messages {
		result[i] = &model.PushMessage{
			ID:   m.ID,
			Text: m.Text,
		}
	}

	return &model.PushMessagesResponse{
		Messages: result,
	}
}

// stripTimestamp removes timestamp and session_id from messages for API response
func (s *ConsoleService) stripTimestamp(msgs []*model.PushMessage) []*model.ConsoleMessage {
	result := make([]*model.ConsoleMessage, len(msgs))
	for i, m := range msgs {
		result[i] = &model.ConsoleMessage{
			ID:   m.ID,
			Text: m.Text,
		}
	}
	return result
}

// sortByTimestamp sorts messages by timestamp (oldest first)
func (s *ConsoleService) sortByTimestamp() {
	// Simple insertion sort for small arrays
	for i := 1; i < len(s.messages); i++ {
		for j := i; j > 0 && s.messages[j-1].Timestamp > s.messages[j].Timestamp; j-- {
			s.messages[j], s.messages[j-1] = s.messages[j-1], s.messages[j]
		}
	}
}

// InitializeDefaultMessages creates default messages (for backward compatibility)
func (s *ConsoleService) InitializeDefaultMessages() {
	// No default messages needed for push store
	// Messages are pushed by cron jobs and other services
}
