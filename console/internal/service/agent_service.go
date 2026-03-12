package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/smallnest/goclaw/agent"
	"github.com/smallnest/goclaw/agent/tools"
	"github.com/smallnest/goclaw/bus"
	goclawConfig "github.com/smallnest/goclaw/config"
	"github.com/smallnest/goclaw/console/internal/model"
	"github.com/smallnest/goclaw/providers"
	"github.com/smallnest/goclaw/session"
)

// AgentService handles agent operations using goclaw agent.Manager
type AgentService struct {
	manager        *agent.AgentManager
	config         *goclawConfig.Config
	sessionMgr     *session.Manager
	chatSvc        *ChatService
	bus            *bus.MessageBus
	skillsLoader   *agent.SkillsLoader
	provider       providers.Provider
	toolRegistry   *agent.ToolRegistry
	contextBuilder *agent.ContextBuilder

	// Admin session tracking
	adminSession *AdminSession
	mu           sync.RWMutex

	// Current message tracking for content-level events
	currentMsgID string
	msgMu        sync.RWMutex
}

// streamContext holds per-stream state including sequence numbers
type streamContext struct {
	sequenceNum        int
	startTime          int64
	sessionID          string
	responseID         string
	currentMsgID       string
	accumulatedContent strings.Builder
}

// AdminSession tracks admin session state
type AdminSession struct {
	Active    bool
	SessionID string
	StartTime *time.Time
	Progress  int
	Input     string
}

// NewAgentService creates a new agent service
func NewAgentService(cfg *goclawConfig.Config, sessionMgr *session.Manager, bus *bus.MessageBus, skillsLoader *agent.SkillsLoader) *AgentService {
	return &AgentService{
		config:       cfg,
		sessionMgr:   sessionMgr,
		bus:          bus,
		skillsLoader: skillsLoader,
		adminSession: &AdminSession{},
	}
}

// SetChatService sets the chat service for auto-creating chats
func (s *AgentService) SetChatService(chatSvc *ChatService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chatSvc = chatSvc
}

// Initialize initializes the agent service with provider and tools
func (s *AgentService) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create provider from config
	provider, err := s.createProvider()
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}
	s.provider = provider

	// Create tool registry
	s.toolRegistry = agent.NewToolRegistry()

	// Register default tools
	s.registerDefaultTools()

	// Create memory store
	memoryStore := agent.NewMemoryStore(s.config.Workspace.Path)

	// Create context builder
	s.contextBuilder = agent.NewContextBuilder(memoryStore, s.config.Workspace.Path)

	// Create agent manager
	managerCfg := &agent.NewAgentManagerConfig{
		Bus:            s.bus,
		Provider:       s.provider,
		SessionMgr:     s.sessionMgr,
		Tools:          s.toolRegistry,
		DataDir:        s.config.Workspace.Path,
		ContextBuilder: s.contextBuilder,
		SkillsLoader:   s.skillsLoader,
	}
	s.manager = agent.NewAgentManager(managerCfg)

	// Setup from config
	if err := s.manager.SetupFromConfig(s.config, s.contextBuilder); err != nil {
		return fmt.Errorf("failed to setup agent manager: %w", err)
	}

	// Start the manager
	if err := s.manager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start agent manager: %w", err)
	}

	return nil
}

// registerDefaultTools registers default tools
func (s *AgentService) registerDefaultTools() {
	// Register use_skill tool
	s.toolRegistry.RegisterExisting(tools.NewUseSkillTool())

	// Register shell tool
	// Get shell tool config from tools config
	shellEnabled := s.config.Tools.Shell.Enabled
	shellTimeout := s.config.Tools.Shell.Timeout
	if shellTimeout <= 0 {
		shellTimeout = 120 // default 120 seconds
	}

	shellTool := tools.NewShellTool(
		shellEnabled,
		s.config.Tools.Shell.AllowedCmds,
		s.config.Tools.Shell.DeniedCmds,
		shellTimeout,
		s.config.Workspace.Path,
		s.config.Tools.Shell.Sandbox,
	)

	// Register all tools from shell tool
	for _, tool := range shellTool.GetTools() {
		s.toolRegistry.RegisterExisting(tool)
	}

	// Register filesystem tools
	// Allow access to workspace and temp directories
	allowedPaths := []string{
		s.config.Workspace.Path,
		"/tmp",
		"/var/tmp",
	}
	deniedPaths := []string{}

	fsTool := tools.NewFileSystemTool(allowedPaths, deniedPaths, s.config.Workspace.Path)

	// Register all tools from filesystem tool
	for _, tool := range fsTool.GetTools() {
		s.toolRegistry.RegisterExisting(tool)
	}
}

// createProvider creates a provider from config
func (s *AgentService) createProvider() (providers.Provider, error) {
	modelStr := s.config.Agents.Defaults.Model
	if modelStr == "" {
		modelStr = "openai:gpt-4"
	}

	// Parse provider and model
	parts := strings.SplitN(modelStr, ":", 2)
	providerType := parts[0]
	modelName := "gpt-4"
	if len(parts) > 1 {
		modelName = parts[1]
	}

	// Default max tokens
	maxTokens := 4096
	if s.config.Agents.Defaults.MaxTokens > 0 {
		maxTokens = s.config.Agents.Defaults.MaxTokens
	}

	// Default timeout (5 minutes for LLM calls)
	timeout := 5 * time.Minute

	fmt.Printf("Creating provider: type=%s, model=%s, timeout=%v\n", providerType, modelName, timeout)

	switch providerType {
	case "openai":
		apiKey := s.config.Providers.OpenAI.APIKey
		baseURL := s.config.Providers.OpenAI.BaseURL
		if s.config.Providers.OpenAI.Timeout > 0 {
			timeout = time.Duration(s.config.Providers.OpenAI.Timeout) * time.Second
		}
		maskedKey := "***"
		if len(apiKey) > 8 {
			maskedKey = apiKey[:4] + "***" + apiKey[len(apiKey)-4:]
		}
		fmt.Printf("OpenAI provider: apiKey=%s, baseURL=%s, timeout=%v\n", maskedKey, baseURL, timeout)
		return providers.NewOpenAIProviderWithTimeout(apiKey, baseURL, modelName, maxTokens, timeout)
	case "anthropic":
		apiKey := s.config.Providers.Anthropic.APIKey
		baseURL := s.config.Providers.Anthropic.BaseURL
		if s.config.Providers.Anthropic.Timeout > 0 {
			timeout = time.Duration(s.config.Providers.Anthropic.Timeout) * time.Second
		}
		return providers.NewAnthropicProviderWithTimeout(apiKey, baseURL, modelName, maxTokens, timeout)
	case "openrouter":
		apiKey := s.config.Providers.OpenRouter.APIKey
		baseURL := s.config.Providers.OpenRouter.BaseURL
		if baseURL == "" {
			baseURL = "https://openrouter.ai/api/v1"
		}
		if s.config.Providers.OpenRouter.Timeout > 0 {
			timeout = time.Duration(s.config.Providers.OpenRouter.Timeout) * time.Second
		}
		return providers.NewOpenRouterProviderWithTimeout(apiKey, baseURL, modelName, maxTokens, timeout)
	default:
		// Check profiles for custom providers
		for _, profile := range s.config.Providers.Profiles {
			if profile.Name == providerType {
				fmt.Printf("Custom provider from profile: name=%s, baseURL=%s, timeout=%v\n", profile.Name, profile.BaseURL, timeout)
				return providers.NewOpenAIProviderWithTimeout(profile.APIKey, profile.BaseURL, modelName, maxTokens, timeout)
			}
		}
		return nil, fmt.Errorf("unknown provider: %s", providerType)
	}
}

// ProcessMessage processes a message using the agent manager (non-streaming)
func (s *AgentService) ProcessMessage(ctx context.Context, req *model.AgentProcessRequest) (*model.AgentProcessResponse, error) {
	s.mu.RLock()
	manager := s.manager
	sessionMgr := s.sessionMgr
	chatSvc := s.chatSvc
	s.mu.RUnlock()

	if manager == nil {
		return nil, fmt.Errorf("agent manager not initialized")
	}

	// Generate session ID if not provided
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = "session-" + uuid.New().String()[:8]
	}

	// Ensure user_id and channel have defaults
	userID := req.UserID
	if userID == "" {
		userID = "default"
	}
	channel := req.Channel
	if channel == "" {
		channel = "console"
	}

	// Auto-create or get chat (similar to Python's get_or_create_chat)
	if chatSvc != nil {
		// Generate chat name from first message content
		chatName := "New Chat"
		if len(req.Input) > 0 {
			firstMsg := req.Input[0]
			if firstMsg.Role == "user" && len(firstMsg.Content) > 0 {
				contentText := ""
				for _, part := range firstMsg.Content {
					if part.Type == model.ContentTypeText {
						contentText += part.Text
					}
				}
				if contentText != "" {
					// Take first 10 chars as name (matching Python behavior)
					// Use rune slicing to handle UTF-8 correctly
					runes := []rune(contentText)
					if len(runes) > 10 {
						chatName = string(runes[:10])
					} else {
						chatName = contentText
					}
				}
			}
		}

		// Try to get existing chat or create new one
		_, err := chatSvc.GetOrCreateChat(sessionID, userID, channel, chatName)
		if err != nil {
			fmt.Printf("Failed to auto-create chat: %v\n", err)
		}
	}

	// Get or create session
	sess, err := sessionMgr.GetOrCreate(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Extract text content from input messages and save to session
	var contentText string
	for _, msg := range req.Input {
		for _, part := range msg.Content {
			if part.Type == model.ContentTypeText {
				contentText += part.Text
			}
		}
		if msg.Role == "user" {
			sess.AddMessage(session.Message{
				Role:      "user",
				Content:   contentText,
				Timestamp: time.Now(),
			})
		}
	}
	// Save session after adding user messages
	if err := s.sessionMgr.Save(sess); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// Create inbound message
	msg := &bus.InboundMessage{
		ID:        uuid.New().String(),
		Channel:   req.Channel,
		AccountID: req.UserID,
		ChatID:    sessionID,
		Content:   contentText,
		Timestamp: time.Now(),
	}

	// Route through agent manager
	if err := s.manager.RouteInbound(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to process message: %w", err)
	}

	return &model.AgentProcessResponse{
		SessionID: sessionID,
		Output: map[string]interface{}{
			"message": "Message processed successfully",
			"input":   req.Input,
			"model":   s.config.Agents.Defaults.Model,
			"status":  "completed",
		},
		Status: "completed",
	}, nil
}

// ProcessMessageStream processes a message and returns a channel of events (streaming)
func (s *AgentService) ProcessMessageStream(ctx context.Context, req *model.AgentProcessRequest) (<-chan *model.AgentEvent, error) {
	s.mu.RLock()
	manager := s.manager
	chatSvc := s.chatSvc
	s.mu.RUnlock()

	if manager == nil {
		return nil, fmt.Errorf("agent manager not initialized")
	}

	// Use provided session_id or generate one
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("%d", time.Now().UnixMilli())
	}

	// Ensure user_id and channel have defaults
	userID := req.UserID
	if userID == "" {
		userID = "default"
	}
	channel := req.Channel
	if channel == "" {
		channel = "console"
	}

	// Auto-create or get chat (similar to Python's get_or_create_chat)
	if chatSvc != nil {
		// Generate chat name from first message content
		chatName := "New Chat"
		if len(req.Input) > 0 {
			firstMsg := req.Input[0]
			if firstMsg.Role == "user" && len(firstMsg.Content) > 0 {
				contentText := ""
				for _, part := range firstMsg.Content {
					if part.Type == model.ContentTypeText {
						contentText += part.Text
					}
				}
				if contentText != "" {
					// Take first 10 chars as name (matching Python behavior)
					// Use rune slicing to handle UTF-8 correctly
					runes := []rune(contentText)
					if len(runes) > 10 {
						chatName = string(runes[:10])
					} else {
						chatName = contentText
					}
				}
			}
		}

		// Try to get existing chat or create new one
		_, err := chatSvc.GetOrCreateChat(sessionID, userID, channel, chatName)
		if err != nil {
			fmt.Printf("Failed to auto-create chat: %v\n", err)
		}
	}

	// Get default agent
	agentInstance := s.manager.GetDefaultAgent()
	if agentInstance == nil {
		return nil, fmt.Errorf("no default agent available")
	}

	// Get or create session
	sess, err := s.sessionMgr.GetOrCreate(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Convert input messages to agent messages
	agentMsgs := s.convertToAgentMessages(req.Input)

	// Save user input messages to session first
	for _, msg := range req.Input {
		if msg.Role == "user" {
			contentText := ""
			for _, part := range msg.Content {
				if part.Type == model.ContentTypeText {
					contentText += part.Text
				}
			}
			sess.AddMessage(session.Message{
				Role:      "user",
				Content:   contentText,
				Timestamp: time.Now(),
			})
		}
	}
	// Save session after adding user messages
	if err := s.sessionMgr.Save(sess); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// Load history
	maxHistory := s.config.Agents.Defaults.MaxHistoryMessages
	if maxHistory <= 0 {
		maxHistory = 100
	}
	history := sess.GetHistorySafe(maxHistory)
	historyAgentMsgs := sessionMessagesToAgentMessages(history)
	allMessages := append(historyAgentMsgs, agentMsgs...)

	// Create event channel with larger buffer for streaming
	eventChan := make(chan *model.AgentEvent, 500)

	// Subscribe to agent events
	agentEventChan := agentInstance.Subscribe()

	// Create a detached context for LLM operations that won't be affected by HTTP request timeout
	// LLM calls can take a long time, so we use a 10-minute timeout
	llmCtx, llmCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	deadline, hasDeadline := llmCtx.Deadline()
	fmt.Printf("Created LLM context with deadline: %v, has deadline: %v, time until deadline: %v\n", deadline, hasDeadline, time.Until(deadline))

	// Initialize stream context for sequence numbers and state tracking
	streamCtx := &streamContext{
		sequenceNum:  0,
		startTime:    time.Now().UnixMilli(),
		sessionID:    sessionID,
		responseID:   "response_" + uuid.New().String(),
		currentMsgID: "",
	}

	// Helper function to create event with sequence number
	nextEvent := func(objType, status string) *model.AgentEvent {
		event := &model.AgentEvent{
			Object:         objType,
			ID:             streamCtx.responseID,
			Status:         status,
			SequenceNumber: streamCtx.sequenceNum,
			CreatedAt:      streamCtx.startTime,
			SessionID:      sessionID,
			Timestamp:      time.Now().UnixMilli(),
		}
		streamCtx.sequenceNum++
		return event
	}

	// Note: response events (created/in_progress/completed) are now handled
	// by convertEventWithContext through EventAgentStart/EventAgentEnd events
	// No need to send manual response events here

	// Start processing in goroutine
	go func() {
		defer close(eventChan)
		defer agentInstance.Unsubscribe(agentEventChan)
		defer llmCancel()

		// Get orchestrator
		orchestrator := agentInstance.GetOrchestrator()

		// Run agent in background
		done := make(chan struct{})
		var orchestratorResult []agent.AgentMessage
		var orchestratorErr error

		go func() {
			defer close(done)
			fmt.Printf("Starting orchestrator run with context deadline: %v\n", time.Until(deadline))
			orchestratorResult, orchestratorErr = orchestrator.Run(llmCtx, allMessages)
			if orchestratorErr != nil {
				fmt.Printf("Orchestrator error: %v\n", orchestratorErr)
			}
			fmt.Printf("Orchestrator completed, messages: %d\n", len(orchestratorResult))
		}()

		// Track if we received any content events (for streaming detection)
		receivedContent := false
		var assistantContent strings.Builder
		var currentMsgID string

		// Forward events in real-time
		for {
			select {
			case <-ctx.Done():
				fmt.Println("HTTP request context cancelled, but LLM context continues")
				<-done
				return
			case <-llmCtx.Done():
				fmt.Println("LLM context cancelled")
				return
			case <-done:
				// Handle orchestrator error
				if orchestratorErr != nil {
					fmt.Printf("Orchestrator failed with error: %v\n", orchestratorErr)
					errEvent := nextEvent("response", "failed")
					errEvent.Error = &model.AgentError{
						Type:    "orchestrator_error",
						Message: orchestratorErr.Error(),
					}
					eventChan <- errEvent
					return
				}

				// If no streaming content was received, process final results
				if !receivedContent && len(orchestratorResult) > 0 {
					for _, msg := range orchestratorResult {
						if msg.Role == agent.RoleAssistant {
							contentText := extractTextContent(msg)
							if contentText != "" {
								// Send message start event
								msgEvent := nextEvent("message", "in_progress")
								msgEvent.Type = "message"
								msgEvent.Role = "assistant"
								currentMsgID = "msg_" + uuid.New().String()
								msgEvent.ID = currentMsgID
								eventChan <- msgEvent

								// Send content event
								contentEvent := nextEvent("content", "in_progress")
								contentEvent.Type = "text"
								contentEvent.Delta = true
								contentEvent.Text = contentText
								contentEvent.MsgID = currentMsgID
								eventChan <- contentEvent

								// Send completed content event with full text
								completedContentEvent := nextEvent("content", "completed")
								completedContentEvent.Type = "text"
								completedContentEvent.Delta = false
								completedContentEvent.Text = contentText
								completedContentEvent.MsgID = currentMsgID
								eventChan <- completedContentEvent

								// Send completed message event
								completedMsgEvent := nextEvent("message", "completed")
								completedMsgEvent.Type = "message"
								completedMsgEvent.Role = "assistant"
								completedMsgEvent.ID = currentMsgID
								deltaFalse := false
								completedMsgEvent.Content = []model.ContentPart{
									{
										Object:         "content",
										SequenceNumber: completedContentEvent.SequenceNumber,
										Status:         "completed",
										Type:           "text",
										Delta:          &deltaFalse,
										MsgID:          currentMsgID,
										Text:           contentText,
									},
								}
								eventChan <- completedMsgEvent

								// Save to session
								sess.AddMessage(session.Message{
									Role:      "assistant",
									Content:   contentText,
									Timestamp: time.Now(),
								})
							}
						}
					}
				} else if receivedContent {
					// Save accumulated streaming content to session
					content := assistantContent.String()
					if content != "" {
						sess.AddMessage(session.Message{
							Role:      "assistant",
							Content:   content,
							Timestamp: time.Now(),
						})
					}

					// Send completed content event if we have a current message
					if currentMsgID != "" {
						completedContentEvent := nextEvent("content", "completed")
						completedContentEvent.Type = "text"
						completedContentEvent.Delta = false
						completedContentEvent.Text = content
						completedContentEvent.MsgID = currentMsgID
						eventChan <- completedContentEvent

						// Send completed message event
						completedMsgEvent := nextEvent("message", "completed")
						completedMsgEvent.Type = "message"
						completedMsgEvent.Role = "assistant"
						completedMsgEvent.ID = currentMsgID
						deltaFalse2 := false
						completedMsgEvent.Content = []model.ContentPart{
							{
								Object:         "content",
								SequenceNumber: completedContentEvent.SequenceNumber,
								Status:         "completed",
								Type:           "text",
								Delta:          &deltaFalse2,
								MsgID:          currentMsgID,
								Text:           content,
							},
						}
						eventChan <- completedMsgEvent
					}
				}

				// Save session
				if err := s.sessionMgr.Save(sess); err != nil {
					fmt.Printf("Failed to save session: %v\n", err)
				}

				// Build output for final response
				var output []model.AgentMessage
				if currentMsgID != "" {
					content := assistantContent.String()
					if content == "" && len(orchestratorResult) > 0 {
						for _, msg := range orchestratorResult {
							if msg.Role == agent.RoleAssistant {
								content = extractTextContent(msg)
								break
							}
						}
					}
					if content != "" {
						deltaFalse := false
						output = append(output, model.AgentMessage{
							Object:         "message",
							ID:             currentMsgID,
							Type:           "message",
							Role:           "assistant",
							Status:         "completed",
							SequenceNumber: streamCtx.sequenceNum - 1,
							Content: []model.ContentPart{
								{
									Object:         "content",
									SequenceNumber: streamCtx.sequenceNum - 2,
									Status:         "completed",
									Type:           "text",
									Delta:          &deltaFalse,
									MsgID:          currentMsgID,
									Text:           content,
								},
							},
						})
					}
				}

				// Send final response completed event
				fmt.Println("Sending final response event")
				finalEvent := nextEvent("response", "completed")
				finalEvent.CompletedAt = time.Now().UnixMilli()
				finalEvent.Output = output
				eventChan <- finalEvent
				return

			case event, ok := <-agentEventChan:
				if !ok {
					fmt.Println("Agent event channel closed")
					return
				}
				fmt.Printf("Received agent event: %s\n", event.Type)

				// Track content events for streaming detection
				switch event.Type {
				case agent.EventStreamContent:
					receivedContent = true
					if event.StreamContent != "" {
						assistantContent.WriteString(event.StreamContent)
					}
				}

				// Convert agent event to API event with sequence number
				apiEvent := s.convertEventWithContext(event, streamCtx)
				if apiEvent != nil {
					// Track current message ID for content events
					if apiEvent.Object == "message" && apiEvent.Status == "in_progress" {
						currentMsgID = apiEvent.ID
					}
					if apiEvent.Object == "content" && currentMsgID != "" {
						apiEvent.MsgID = currentMsgID
					}
					eventChan <- apiEvent
				}
			}
		}
	}()

	return eventChan, nil
}

// convertToAgentMessages converts API messages to agent messages
func (s *AgentService) convertToAgentMessages(messages []model.AgentMessage) []agent.AgentMessage {
	result := make([]agent.AgentMessage, 0, len(messages))
	for _, msg := range messages {
		content := make([]agent.ContentBlock, 0)
		for _, part := range msg.Content {
			switch part.Type {
			case model.ContentTypeText:
				content = append(content, agent.TextContent{Text: part.Text})
			case model.ContentTypeImage:
				content = append(content, agent.ImageContent{URL: part.ImageURL})
			}
		}
		result = append(result, agent.AgentMessage{
			Role:      agent.MessageRole(msg.Role),
			Content:   content,
			Timestamp: time.Now().UnixMilli(),
		})
	}
	return result
}

// convertEvent converts an agent event to an API event
// Follows the AgentScope Runtime protocol format
func (s *AgentService) convertEvent(event *agent.Event, sessionID string) *model.AgentEvent {
	switch event.Type {
	case agent.EventAgentStart:
		// Response-level event: agent run started
		return &model.AgentEvent{
			Object:    "response",
			ID:        uuid.New().String(),
			Status:    "in_progress",
			Timestamp: event.Timestamp,
		}

	case agent.EventAgentEnd:
		// Response-level event: agent run completed
		return &model.AgentEvent{
			Object:    "response",
			ID:        uuid.New().String(),
			Status:    "completed",
			Timestamp: event.Timestamp,
		}

	case agent.EventMessageStart:
		// Message-level event: new message started
		msgID := uuid.New().String()
		s.msgMu.Lock()
		s.currentMsgID = msgID
		s.msgMu.Unlock()
		return &model.AgentEvent{
			Object:    "message",
			ID:        msgID,
			Role:      "assistant",
			Type:      "message",
			Status:    "in_progress",
			Timestamp: event.Timestamp,
			Content:   []model.ContentPart{},
		}

	case agent.EventStreamContent:
		// Content-level event: incremental text content
		s.msgMu.RLock()
		msgID := s.currentMsgID
		s.msgMu.RUnlock()
		if msgID == "" {
			msgID = uuid.New().String()
		}
		return &model.AgentEvent{
			Object:    "content",
			ID:        uuid.New().String(),
			MsgID:     msgID,
			Type:      "text",
			Delta:     true,
			Text:      event.StreamContent,
			Status:    "in_progress",
			Timestamp: event.Timestamp,
		}

	case agent.EventStreamThinking:
		// Content-level event: thinking/reasoning content
		s.msgMu.RLock()
		msgID := s.currentMsgID
		s.msgMu.RUnlock()
		if msgID == "" {
			msgID = uuid.New().String()
		}
		return &model.AgentEvent{
			Object:    "content",
			ID:        uuid.New().String(),
			MsgID:     msgID,
			Type:      "reasoning",
			Delta:     true,
			Text:      event.StreamContent,
			Status:    "in_progress",
			Timestamp: event.Timestamp,
		}

	case agent.EventStreamFinal:
		// Content-level event: final content
		s.msgMu.RLock()
		msgID := s.currentMsgID
		s.msgMu.RUnlock()
		if msgID == "" {
			msgID = uuid.New().String()
		}
		return &model.AgentEvent{
			Object:    "content",
			ID:        uuid.New().String(),
			MsgID:     msgID,
			Type:      "text",
			Delta:     false,
			Text:      event.StreamContent,
			Status:    "completed",
			Timestamp: event.Timestamp,
		}

	case agent.EventStreamDone:
		// Stream done - no event needed, handled by message_end
		return nil

	case agent.EventMessageEnd:
		// Message-level event: message completed
		s.msgMu.RLock()
		msgID := s.currentMsgID
		s.msgMu.RUnlock()
		if msgID == "" {
			msgID = uuid.New().String()
		}
		apiEvent := &model.AgentEvent{
			Object:    "message",
			ID:        msgID,
			Role:      "assistant",
			Type:      "message",
			Status:    "completed",
			Timestamp: event.Timestamp,
		}
		if event.Message != nil {
			apiEvent.Content = s.convertContentBlocks(event.Message.Content)
		}
		return apiEvent

	case agent.EventToolExecutionStart:
		// Message-level event: tool call started
		return &model.AgentEvent{
			Object:    "message",
			ID:        event.ToolID,
			Role:      "assistant",
			Type:      "function_call",
			Status:    "in_progress",
			Timestamp: event.Timestamp,
			Content: []model.ContentPart{
				{
					Type: "data",
					Data: map[string]interface{}{
						"call_id":   event.ToolID,
						"name":      event.ToolName,
						"arguments": event.ToolArgs,
					},
				},
			},
		}

	case agent.EventToolExecutionEnd:
		// Message-level event: tool call completed
		return &model.AgentEvent{
			Object:    "message",
			ID:        event.ToolID,
			Role:      "assistant",
			Type:      "function_call_output",
			Status:    "completed",
			Timestamp: event.Timestamp,
		}

	case agent.EventTurnStart, agent.EventTurnEnd:
		// Turn events - map to response-level events
		return &model.AgentEvent{
			Object:    "response",
			ID:        uuid.New().String(),
			Status:    "in_progress",
			Timestamp: event.Timestamp,
		}

	default:
		return nil
	}
}

// convertContentBlocks converts agent content blocks to API content parts
func (s *AgentService) convertContentBlocks(blocks []agent.ContentBlock) []model.ContentPart {
	result := make([]model.ContentPart, 0)
	for _, block := range blocks {
		switch b := block.(type) {
		case agent.TextContent:
			result = append(result, model.ContentPart{
				Type: model.ContentTypeText,
				Text: b.Text,
			})
		case agent.ImageContent:
			result = append(result, model.ContentPart{
				Type:     model.ContentTypeImage,
				ImageURL: b.URL,
			})
		}
	}
	return result
}

// convertEventWithContext converts an agent event to an API event with stream context
func (s *AgentService) convertEventWithContext(event *agent.Event, streamCtx *streamContext) *model.AgentEvent {
	streamCtx.sequenceNum++
	switch event.Type {
	case agent.EventAgentStart:
		seqNum := streamCtx.sequenceNum
		return &model.AgentEvent{
			Object:         "response",
			ID:             streamCtx.responseID,
			Status:         "in_progress",
			SequenceNumber: seqNum,
			CreatedAt:      streamCtx.startTime,
			SessionID:      streamCtx.sessionID,
			Timestamp:      event.Timestamp,
		}

	case agent.EventAgentEnd:
		seqNum := streamCtx.sequenceNum
		return &model.AgentEvent{
			Object:         "response",
			ID:             streamCtx.responseID,
			Status:         "completed",
			SequenceNumber: seqNum,
			CreatedAt:      streamCtx.startTime,
			CompletedAt:    time.Now().UnixMilli(),
			SessionID:      streamCtx.sessionID,
			Timestamp:      event.Timestamp,
		}

	case agent.EventMessageStart:
		seqNum := streamCtx.sequenceNum
		msgID := "msg_" + uuid.New().String()
		streamCtx.currentMsgID = msgID
		return &model.AgentEvent{
			Object:         "message",
			ID:             msgID,
			Role:           "assistant",
			Type:           "message",
			Status:         "in_progress",
			SequenceNumber: seqNum,
			CreatedAt:      streamCtx.startTime,
			SessionID:      streamCtx.sessionID,
			Timestamp:      event.Timestamp,
		}

	case agent.EventStreamContent:
		seqNum := streamCtx.sequenceNum
		msgID := streamCtx.currentMsgID
		if msgID == "" {
			msgID = "msg_" + uuid.New().String()
			streamCtx.currentMsgID = msgID
		}
		return &model.AgentEvent{
			Object:         "content",
			ID:             uuid.New().String(),
			MsgID:          msgID,
			Type:           "text",
			Delta:          true,
			Text:           event.StreamContent,
			Status:         "in_progress",
			SequenceNumber: seqNum,
			CreatedAt:      streamCtx.startTime,
			SessionID:      streamCtx.sessionID,
			Timestamp:      event.Timestamp,
		}

	case agent.EventStreamThinking:
		seqNum := streamCtx.sequenceNum
		msgID := streamCtx.currentMsgID
		if msgID == "" {
			msgID = "msg_" + uuid.New().String()
			streamCtx.currentMsgID = msgID
		}
		return &model.AgentEvent{
			Object:         "content",
			ID:             uuid.New().String(),
			MsgID:          msgID,
			Type:           "reasoning",
			Delta:          true,
			Text:           event.StreamContent,
			Status:         "in_progress",
			SequenceNumber: seqNum,
			CreatedAt:      streamCtx.startTime,
			SessionID:      streamCtx.sessionID,
			Timestamp:      event.Timestamp,
		}

	case agent.EventStreamFinal:
		seqNum := streamCtx.sequenceNum
		msgID := streamCtx.currentMsgID
		if msgID == "" {
			msgID = "msg_" + uuid.New().String()
			streamCtx.currentMsgID = msgID
		}
		return &model.AgentEvent{
			Object:         "content",
			ID:             uuid.New().String(),
			MsgID:          msgID,
			Type:           "final",
			Delta:          true,
			Text:           event.StreamContent,
			Status:         "in_progress",
			SequenceNumber: seqNum,
			CreatedAt:      streamCtx.startTime,
			SessionID:      streamCtx.sessionID,
			Timestamp:      event.Timestamp,
		}

	case agent.EventStreamDone:
		seqNum := streamCtx.sequenceNum
		return &model.AgentEvent{
			Object:         "content",
			ID:             uuid.New().String(),
			Type:           "done",
			Status:         "completed",
			SequenceNumber: seqNum,
			CreatedAt:      streamCtx.startTime,
			SessionID:      streamCtx.sessionID,
			Timestamp:      event.Timestamp,
		}

	case agent.EventMessageEnd:
		seqNum := streamCtx.sequenceNum
		msgID := streamCtx.currentMsgID
		if msgID == "" {
			msgID = "msg_" + uuid.New().String()
		}
		return &model.AgentEvent{
			Object:         "message",
			ID:             msgID,
			Role:           "assistant",
			Type:           "message",
			Status:         "completed",
			SequenceNumber: seqNum,
			CreatedAt:      streamCtx.startTime,
			SessionID:      streamCtx.sessionID,
			Timestamp:      event.Timestamp,
		}

	case agent.EventToolExecutionStart:
		// Skip tool execution start events - these are internal LLM data not meant for frontend
		return nil

	case agent.EventToolExecutionEnd:
		// Skip tool execution end events - these are internal LLM data not meant for frontend
		return nil

	case agent.EventTurnStart, agent.EventTurnEnd:
		seqNum := streamCtx.sequenceNum
		return &model.AgentEvent{
			Object:         "response",
			ID:             streamCtx.responseID,
			Status:         "in_progress",
			SequenceNumber: seqNum,
			CreatedAt:      streamCtx.startTime,
			SessionID:      streamCtx.sessionID,
			Timestamp:      event.Timestamp,
		}

	default:
		return nil
	}
}

// GetStatus returns the agent status
func (s *AgentService) GetStatus() *model.AgentStatus {
	return &model.AgentStatus{
		Status:  "running",
		Message: "Agent service is available",
	}
}

// GetHealth returns the agent health
func (s *AgentService) GetHealth() *model.AgentHealth {
	return &model.AgentHealth{
		Status:    "healthy",
		Timestamp: time.Now(),
	}
}

// GetAdminStatus returns the admin session status
func (s *AgentService) GetAdminStatus() *model.AgentAdminStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.adminSession == nil {
		return &model.AgentAdminStatus{
			Active:    false,
			SessionID: "",
			Status:    "idle",
			StartTime: nil,
			Progress:  0,
		}
	}

	var startTimeStr *string
	if s.adminSession.StartTime != nil {
		formatted := s.adminSession.StartTime.Format(time.RFC3339)
		startTimeStr = &formatted
	}

	return &model.AgentAdminStatus{
		Active:    s.adminSession.Active,
		SessionID: s.adminSession.SessionID,
		Status:    "idle",
		StartTime: startTimeStr,
		Progress:  s.adminSession.Progress,
	}
}

// StartAdminSession starts an admin session
func (s *AgentService) StartAdminSession(input string) *model.AgentAdminStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.adminSession = &AdminSession{
		Active:    true,
		SessionID: "admin-" + uuid.New().String()[:8],
		StartTime: &now,
		Progress:  0,
		Input:     input,
	}

	startTimeStr := now.Format(time.RFC3339)
	return &model.AgentAdminStatus{
		Active:    true,
		SessionID: s.adminSession.SessionID,
		Status:    "running",
		StartTime: &startTimeStr,
		Progress:  0,
	}
}

// StopAdminSession stops the admin session
func (s *AgentService) StopAdminSession() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.adminSession = &AdminSession{
		Active: false,
	}
}

// Shutdown shuts down the agent service
func (s *AgentService) Shutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.manager != nil {
		return s.manager.Stop()
	}
	return nil
}

// GetRunningConfig returns the running configuration
func (s *AgentService) GetRunningConfig() *model.AgentRunningConfig {
	maxIters := s.config.Agents.Defaults.MaxIterations
	if maxIters == 0 {
		maxIters = 15
	}

	return &model.AgentRunningConfig{
		MaxIters:       maxIters,
		MaxInputLength: 4000,
	}
}

// UpdateRunningConfig updates the running configuration
func (s *AgentService) UpdateRunningConfig(req *model.AgentRunningConfig, configPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.Agents.Defaults.MaxIterations = req.MaxIters

	// Save config
	if configPath != "" {
		goclawConfig.Save(s.config, configPath)
	}

	return nil
}

// GetToolsInfo returns the available tools
func (s *AgentService) GetToolsInfo() (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.manager == nil {
		return map[string]interface{}{}, nil
	}

	return s.manager.GetToolsInfo()
}

// sessionMessagesToAgentMessages converts session messages to agent messages
func sessionMessagesToAgentMessages(sessMsgs []session.Message) []agent.AgentMessage {
	result := make([]agent.AgentMessage, 0, len(sessMsgs))
	for _, sessMsg := range sessMsgs {
		agentMsg := agent.AgentMessage{
			Role:      agent.MessageRole(sessMsg.Role),
			Content:   []agent.ContentBlock{agent.TextContent{Text: sessMsg.Content}},
			Timestamp: sessMsg.Timestamp.UnixMilli(),
		}

		// Handle tool calls in assistant messages
		if sessMsg.Role == "assistant" && len(sessMsg.ToolCalls) > 0 {
			agentMsg.Content = []agent.ContentBlock{}
			for _, tc := range sessMsg.ToolCalls {
				agentMsg.Content = append(agentMsg.Content, agent.ToolCallContent{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Params,
				})
			}
		}

		// Handle tool result messages
		if sessMsg.Role == "tool" {
			agentMsg.Role = agent.RoleToolResult
			if sessMsg.ToolCallID != "" {
				if agentMsg.Metadata == nil {
					agentMsg.Metadata = make(map[string]any)
				}
				agentMsg.Metadata["tool_call_id"] = sessMsg.ToolCallID
			}
		}

		result = append(result, agentMsg)
	}
	return result
}

// extractTextContent extracts text content from agent message
func extractTextContent(msg agent.AgentMessage) string {
	for _, block := range msg.Content {
		if text, ok := block.(agent.TextContent); ok {
			return text.Text
		}
	}
	return ""
}
