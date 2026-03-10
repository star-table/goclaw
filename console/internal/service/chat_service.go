package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/smallnest/goclaw/console/internal/model"
	"github.com/smallnest/goclaw/session"
)

// ChatService manages chat sessions using goclaw session.Manager
type ChatService struct {
	manager *session.Manager
	baseDir string
	chats   map[string]*model.ChatSpec
	mu      sync.RWMutex
}

// NewChatService creates a new chat service
func NewChatService(mgr *session.Manager, baseDir string) *ChatService {
	s := &ChatService{
		manager: mgr,
		baseDir: baseDir,
		chats:   make(map[string]*model.ChatSpec),
	}
	// Load existing chats from disk
	s.loadChats()
	// Sync with session manager to recover any orphaned sessions (disabled to avoid deadlock)
	// s.syncWithSessions()
	return s
}

// loadChats loads chat metadata from disk
func (s *ChatService) loadChats() {
	if s.baseDir == "" {
		return
	}

	chatsFile := filepath.Join(s.baseDir, "chats.json")
	data, err := os.ReadFile(chatsFile)
	if err != nil {
		if !os.IsNotExist(err) {
			// Log error but don't fail
			return
		}
		// File doesn't exist, that's ok
		return
	}

	var chats []*model.ChatSpec
	if err := json.Unmarshal(data, &chats); err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, chat := range chats {
		// Set defaults for backward compatibility
		if chat.UserID == "" {
			chat.UserID = "default"
		}
		if chat.Channel == "" {
			chat.Channel = "console"
		}
		s.chats[chat.ID] = chat
	}
}

// saveChats persists chat metadata to disk
func (s *ChatService) saveChats() error {
	if s.baseDir == "" {
		return nil
	}

	// Ensure directory exists
	if err := os.MkdirAll(s.baseDir, 0755); err != nil {
		return err
	}

	s.mu.RLock()
	chats := make([]*model.ChatSpec, 0, len(s.chats))
	for _, chat := range s.chats {
		chats = append(chats, chat)
	}
	s.mu.RUnlock()

	chatsFile := filepath.Join(s.baseDir, "chats.json")
	data, err := json.MarshalIndent(chats, "", "  ")
	if err != nil {
		return err
	}

	// Try to write directly first
	if err := os.WriteFile(chatsFile, data, 0644); err != nil {
		// If direct write fails, try using temp directory
		tmpDir := os.TempDir()
		tmpFile := filepath.Join(tmpDir, "chats.json.tmp")
		if writeErr := os.WriteFile(tmpFile, data, 0644); writeErr != nil {
			return fmt.Errorf("failed to write to temp dir: %w", writeErr)
		}
		// Try to rename from temp
		if renameErr := os.Rename(tmpFile, chatsFile); renameErr != nil {
			// Clean up temp file
			os.Remove(tmpFile)
			return fmt.Errorf("failed to rename from temp: %w", renameErr)
		}
	}

	return nil
}

// syncWithSessions syncs chat metadata with existing sessions on disk
// This ensures that sessions created before a server restart are properly tracked
func (s *ChatService) syncWithSessions() {
	if s.manager == nil {
		return
	}

	// Use goroutine to avoid blocking initialization
	go func() {
		sessionKeys, err := s.manager.List()
		if err != nil {
			return
		}

		// Pre-load all sessions outside of lock to avoid deadlock
		sessionData := make(map[string]struct {
			name   string
			exists bool
		})
		for _, key := range sessionKeys {
			sess, err := s.manager.GetOrCreate(key)
			name := key
			if err == nil && sess != nil {
				if metaName, ok := sess.Metadata["name"].(string); ok && metaName != "" {
					name = metaName
				}
			}
			sessionData[key] = struct {
				name   string
				exists bool
			}{name: name, exists: err == nil}
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		needsSave := false
		for _, key := range sessionKeys {
			// Check if we already have this chat
			found := false
			for _, chat := range s.chats {
				if chat.ID == key || chat.SessionID == key {
					found = true
					break
				}
			}

			// If not found, create a chat entry for this session
			if !found {
				data := sessionData[key]
				chat := &model.ChatSpec{
					ID:        key,
					Name:      data.name,
					SessionID: key,
					UserID:    "default",
					Channel:   "console",
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
					Meta:      make(map[string]interface{}),
				}
				s.chats[key] = chat
				needsSave = true
			}
		}

		// Persist if we added new chats
		if needsSave {
			go s.saveChats()
		}
	}()
}

// ListChats returns all chats
func (s *ChatService) ListChats(userID, channel string) []*model.ChatSpec {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return stored chats only (avoid loading all sessions which can be slow)
	result := make([]*model.ChatSpec, 0)
	for _, chat := range s.chats {
		if userID != "" && chat.UserID != userID {
			continue
		}
		if channel != "" && chat.Channel != channel {
			continue
		}
		result = append(result, chat)
	}

	return result
}

// GetChat returns a specific chat by ID
func (s *ChatService) GetChat(id string) (*model.ChatSpec, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chat, ok := s.chats[id]
	if !ok {
		// Check if it exists in session manager
		if s.manager != nil {
			_, err := s.manager.GetOrCreate(id)
			if err != nil {
				return nil, ErrChatNotFound
			}
			return &model.ChatSpec{
				ID:        id,
				Name:      id,
				SessionID: id,
				UserID:    "default",
				Channel:   "console",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, nil
		}
		return nil, ErrChatNotFound
	}
	return chat, nil
}

// GetOrCreateChat gets existing chat or creates new one (similar to Python's get_or_create_chat)
func (s *ChatService) GetOrCreateChat(sessionID, userID, channel, name string) (*model.ChatSpec, error) {
	s.mu.Lock()

	// Try to find existing chat by session_id
	for _, chat := range s.chats {
		if chat.SessionID == sessionID && chat.UserID == userID && chat.Channel == channel {
			s.mu.Unlock()
			return chat, nil
		}
	}

	// Create new chat
	now := time.Now()
	chat := &model.ChatSpec{
		ID:        uuid.New().String(),
		Name:      name,
		SessionID: sessionID,
		UserID:    userID,
		Channel:   channel,
		CreatedAt: now,
		UpdatedAt: now,
		Meta:      make(map[string]interface{}),
	}

	s.chats[chat.ID] = chat
	s.mu.Unlock()

	// Check if session exists in session manager (outside of lock)
	if s.manager != nil {
		_, err := s.manager.GetOrCreate(sessionID)
		if err != nil {
			return nil, err
		}
	}

	// Persist to disk synchronously (outside of lock)
	if err := s.saveChats(); err != nil {
		fmt.Printf("Failed to save chats after create: %v\n", err)
	}

	return chat, nil
}

// GetChatHistory returns the history of a chat using session.Manager
func (s *ChatService) GetChatHistory(id string) (*model.ChatHistory, error) {
	s.mu.RLock()
	chat, exists := s.chats[id]
	s.mu.RUnlock()

	if s.manager == nil {
		return &model.ChatHistory{Messages: []model.ChatMessage{}}, nil
	}

	// Use session ID if chat exists, otherwise use the ID directly
	sessionID := id
	if exists && chat.SessionID != "" {
		sessionID = chat.SessionID
	}

	sess, err := s.manager.GetOrCreate(sessionID)
	if err != nil {
		return nil, ErrChatNotFound
	}

	// Check if session has any messages (indicates it was loaded from disk or has content)
	history := sess.GetHistory(0) // Get all messages

	// If chat doesn't exist in our map and session has no messages,
	// this is a newly created empty session, return not found
	if !exists && len(history) == 0 {
		return nil, ErrChatNotFound
	}

	// Convert session messages to chat messages (Python-compatible format)
	messages := make([]model.ChatMessage, 0, len(history))
	for _, msg := range history {
		chatMsg := convertSessionMessageToChatMessage(msg, sessionID)
		messages = append(messages, chatMsg)
	}

	return &model.ChatHistory{Messages: messages}, nil
}

// convertSessionMessageToChatMessage converts a session message to Python-compatible chat message format
func convertSessionMessageToChatMessage(msg session.Message, sessionID string) model.ChatMessage {
	msgID := generateMessageID()

	// Determine message type based on role and content
	msgType := "message"
	if msg.Role == "system" {
		msgType = "plugin_call_output"
	} else if msg.Role == "assistant" && containsPluginCall(msg.Content) {
		msgType = "plugin_call"
	}

	// Create content item
	contentItem := model.ContentItem{
		Object: "content",
		Status: "completed",
		Type:   "text",
		Index:  0,
		Delta:  false,
		MsgID:  msgID,
		Text:   msg.Content,
	}

	return model.ChatMessage{
		Object:  "message",
		Status:  "completed",
		ID:      msgID,
		Type:    msgType,
		Role:    msg.Role,
		Content: []model.ContentItem{contentItem},
		Code:    nil,
		Message: nil,
		Usage:   nil,
		Metadata: model.MessageMetadata{
			OriginalID:   msgID,
			OriginalName: msg.Role,
			Metadata:     msg.Metadata,
		},
	}
}

// containsPluginCall checks if content contains plugin call indicators
func containsPluginCall(content string) bool {
	// Simple heuristic: check for common plugin call patterns
	return len(content) > 0 && (content[0] == '{' || content[0] == '[')
}

// CreateChat creates a new chat
func (s *ChatService) CreateChat(req *model.CreateChatRequest) *model.ChatSpec {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Set defaults to match Python implementation
	userID := req.UserID
	if userID == "" {
		userID = "default"
	}
	channel := req.Channel
	if channel == "" {
		channel = "console"
	}

	chat := &model.ChatSpec{
		ID:        uuid.New().String(),
		Name:      "New Chat",
		SessionID: fmt.Sprintf("%d", now.UnixMilli()),
		UserID:    userID,
		Channel:   channel,
		CreatedAt: now,
		UpdatedAt: now,
		Meta:      req.Meta,
	}

	if chat.Meta == nil {
		chat.Meta = make(map[string]interface{})
	}

	s.chats[chat.ID] = chat

	// Create session in manager
	if s.manager != nil {
		sess, _ := s.manager.GetOrCreate(chat.SessionID)
		if sess != nil {
			sess.Metadata["name"] = chat.Name
			sess.Metadata["chat_id"] = chat.ID
			_ = s.manager.Save(sess)
		}
	}

	// Persist to disk synchronously to ensure it's saved
	_ = s.saveChats()

	return chat
}

// UpdateChat updates a chat
func (s *ChatService) UpdateChat(id string, req *model.UpdateChatRequest) (*model.ChatSpec, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	chat, ok := s.chats[id]
	if !ok {
		return nil, ErrChatNotFound
	}

	if req.Name != "" {
		chat.Name = req.Name
	}
	if req.Meta != nil {
		chat.Meta = req.Meta
	}
	chat.UpdatedAt = time.Now()

	// Save session if manager available
	if s.manager != nil {
		sess, _ := s.manager.GetOrCreate(chat.SessionID)
		if sess != nil {
			if req.Name != "" {
				sess.Metadata["name"] = req.Name
			}
			_ = s.manager.Save(sess)
		}
	}

	// Persist to disk synchronously
	_ = s.saveChats()

	return chat, nil
}

// DeleteChat deletes a chat
func (s *ChatService) DeleteChat(id string) (*model.DeleteChatResponse, error) {
	s.mu.Lock()

	chat, ok := s.chats[id]
	if !ok {
		s.mu.Unlock()
		return nil, ErrChatNotFound
	}

	// Delete session from manager (outside of lock)
	sessionID := chat.SessionID
	delete(s.chats, id)
	s.mu.Unlock()

	// Delete session from manager (outside of lock)
	if s.manager != nil {
		_ = s.manager.Delete(sessionID)
	}

	// Persist to disk synchronously (outside of lock)
	if err := s.saveChats(); err != nil {
		fmt.Printf("Failed to save chats after delete: %v\n", err)
	}

	return &model.DeleteChatResponse{
		Status: "deleted",
		ID:     id,
	}, nil
}

// BatchDeleteChats deletes multiple chats
func (s *ChatService) BatchDeleteChats(ids []string) *model.BatchDeleteChatsResponse {
	s.mu.Lock()

	// Collect session IDs to delete outside of lock
	sessionIDs := make([]string, 0)
	count := 0
	for _, id := range ids {
		chat, ok := s.chats[id]
		if ok {
			sessionIDs = append(sessionIDs, chat.SessionID)
			delete(s.chats, id)
			count++
		}
	}
	s.mu.Unlock()

	// Delete sessions from manager (outside of lock)
	if s.manager != nil {
		for _, sessionID := range sessionIDs {
			_ = s.manager.Delete(sessionID)
		}
	}

	// Persist to disk synchronously (outside of lock)
	if err := s.saveChats(); err != nil {
		fmt.Printf("Failed to save chats after batch delete: %v\n", err)
	}

	return &model.BatchDeleteChatsResponse{
		Success:      true,
		DeletedCount: count,
	}
}

// AddMessageToChat adds a message to a chat session and persists it
func (s *ChatService) AddMessageToChat(chatID string, role, content string) (*model.ChatMessage, error) {
	s.mu.RLock()
	chat, exists := s.chats[chatID]
	s.mu.RUnlock()

	if !exists {
		return nil, ErrChatNotFound
	}

	if s.manager == nil {
		return nil, ErrChatNotFound
	}

	sess, err := s.manager.GetOrCreate(chat.SessionID)
	if err != nil {
		return nil, err
	}

	msg := session.Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}

	sess.AddMessage(msg)

	// Persist session to disk
	if err := s.manager.Save(sess); err != nil {
		return nil, err
	}

	// Update chat timestamp
	s.mu.Lock()
	chat.UpdatedAt = time.Now()
	s.mu.Unlock()
	_ = s.saveChats()

	// Return message in Python-compatible format
	chatMsg := convertSessionMessageToChatMessage(msg, chat.SessionID)
	return &chatMsg, nil
}

// generateMessageID generates a unique message ID
func generateMessageID() string {
	return "msg-" + uuid.New().String()[:8]
}

// ErrChatNotFound is returned when a chat is not found
var ErrChatNotFound = &ChatNotFoundError{}

type ChatNotFoundError struct{}

func (e *ChatNotFoundError) Error() string {
	return "chat not found"
}

// Helper function to sanitize session keys for use as filenames
func sanitizeSessionKey(key string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, key)
}
