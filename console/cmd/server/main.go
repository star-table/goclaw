package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallnest/goclaw/agent"
	"github.com/smallnest/goclaw/bus"
	goclawConfig "github.com/smallnest/goclaw/config"
	"github.com/smallnest/goclaw/console/internal/handler"
	"github.com/smallnest/goclaw/console/internal/middleware"
	"github.com/smallnest/goclaw/console/internal/service"
	"github.com/smallnest/goclaw/cron"
	"github.com/smallnest/goclaw/internal"
	"github.com/smallnest/goclaw/internal/workspace"
	"github.com/smallnest/goclaw/memory"
	"github.com/smallnest/goclaw/session"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	// Get workspace directory
	workspaceDir := getEnvOrDefault("GOCLAW_WORKSPACE", "")
	if workspaceDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		workspaceDir = filepath.Join(homeDir, ".goclaw")
	}
	os.MkdirAll(workspaceDir, 0755)

	// Initialize workspace with bootstrap files
	wsMgr := workspace.NewManager(filepath.Join(workspaceDir, "workspace"))
	if err := wsMgr.Ensure(); err != nil {
		logger.Warn("Failed to initialize workspace", zap.Error(err))
	}

	// Ensure builtin skills are copied to user directory (same as cli/root.go)
	if err := internal.EnsureBuiltinSkills(); err != nil {
		logger.Warn("Failed to ensure builtin skills", zap.Error(err))
	} else {
		logger.Info("Builtin skills ready")
	}

	// Ensure config file exists (same as cli/root.go)
	configCreated, err := internal.EnsureConfig()
	if err != nil {
		logger.Warn("Failed to ensure config", zap.Error(err))
	}
	if configCreated {
		logger.Info("Config file created", zap.String("path", internal.GetConfigPath()))
	}

	// Load goclaw config
	cfg, err := loadConfig(workspaceDir)
	if err != nil {
		logger.Warn("Failed to load config, using defaults", zap.Error(err))
		cfg = &goclawConfig.Config{}
	}

	// Create message bus
	messageBus := bus.NewMessageBus(100)

	// Initialize session manager
	sessionDir := filepath.Join(workspaceDir, "sessions")
	sessionMgr, err := session.NewManager(sessionDir)
	if err != nil {
		logger.Warn("Failed to create session manager", zap.Error(err))
	}

	// Initialize cron service
	cronConfig := cron.DefaultCronConfig()
	cronConfig.StorePath = filepath.Join(workspaceDir, "cron", "jobs.json")
	cronSvc, err := cron.NewService(cronConfig, messageBus)
	if err != nil {
		logger.Warn("Failed to create cron service", zap.Error(err))
	}
	if cronSvc != nil {
		ctx := context.Background()
		if err := cronSvc.Start(ctx); err != nil {
			logger.Warn("Failed to start cron service", zap.Error(err))
		}
	}

	// Initialize skills loader (same directories as cli/root.go)
	// Loading order (later ones override earlier ones with same name):
	// 1. ~/.goclaw/skills (global, lowest priority)
	// 2. ${WORKSPACE}/skills (workspace)
	// 3. ~/.goclaw/active_skills (enabled skills, highest priority)
	goclawDir := workspaceDir
	skillsDir := filepath.Join(goclawDir, "skills")
	activeSkillsDir := filepath.Join(goclawDir, "active_skills")

	// Ensure directories exist
	os.MkdirAll(skillsDir, 0755)
	os.MkdirAll(activeSkillsDir, 0755)

	skillsLoader := agent.NewSkillsLoader(goclawDir, []string{
		skillsDir,       // Global skills (lowest priority)
		activeSkillsDir, // Enabled skills (highest priority)
	})
	if err := skillsLoader.Discover(); err != nil {
		logger.Warn("Failed to discover skills", zap.Error(err))
	} else {
		skills := skillsLoader.List()
		logger.Info("Skills loaded", zap.Int("count", len(skills)))
	}

	// Initialize memory store (optional - requires embedding provider)
	var memoryStore memory.Store
	// Note: In production, you would initialize with a real embedding provider
	// memoryStore, err = memory.NewSQLiteStore(memory.DefaultStoreConfig(dbPath, provider))
	// if err != nil {
	// 	logger.Warn("Failed to create memory store", zap.Error(err))
	// }

	// Config file path for persistence
	configPath := filepath.Join(workspaceDir, "config.json")

	// Initialize console services with goclaw core modules
	chatSvc := service.NewChatService(sessionMgr, filepath.Join(workspaceDir, "chats"))
	cronSvcWrapper := service.NewCronService(cronSvc)
	skillSvc := service.NewSkillService(skillsLoader, skillsDir, activeSkillsDir)
	providerSvc := service.NewProviderService(cfg, configPath)
	workspaceSvc := service.NewWorkspaceService(memoryStore, cfg.Workspace.Path)
	mcpSvc := service.NewMCPServiceWithPath(cfg, configPath)
	localModelSvc := service.NewLocalModelService(workspaceDir)
	ollamaModelSvc := service.NewOllamaModelService()
	envSvc := service.NewEnvService()
	channelSvc := service.NewChannelService(cfg, workspaceDir)
	heartbeatSvc := service.NewHeartbeatService(filepath.Join(workspaceDir, "heartbeat"))
	consoleSvc := service.NewConsoleService()

	// Initialize agent service (requires valid provider configuration)
	agentSvc := service.NewAgentService(cfg, sessionMgr, messageBus, skillsLoader)
	if err := agentSvc.Initialize(context.Background()); err != nil {
		logger.Warn("Failed to initialize agent service, running in degraded mode", zap.Error(err))
	}

	// Initialize default data (only for services that don't use goclaw modules)
	ollamaModelSvc.InitializeDefaultOllamaModels()
	envSvc.InitializeDefaultEnvs()
	consoleSvc.InitializeDefaultMessages()

	// Initialize handlers
	rootHandler := handler.NewRootHandler()
	healthHandler := handler.NewHealthHandler()
	agentHandler := handler.NewAgentHandler(cfg, configPath)
	agentHandler.SetAgentService(agentSvc) // Set the agent service
	chatHandler := handler.NewChatHandler(chatSvc)
	cronJobHandler := handler.NewCronJobHandler(cronSvcWrapper)
	channelHandler := handler.NewChannelHandler(channelSvc)
	envHandler := handler.NewEnvHandler(envSvc)
	heartbeatHandler := handler.NewHeartbeatHandler(heartbeatSvc)
	providerHandler := handler.NewProviderHandler(providerSvc)
	skillHandler := handler.NewSkillHandler(skillSvc)
	workspaceHandler := handler.NewWorkspaceHandler(workspaceSvc)
	mcpHandler := handler.NewMCPHandler(mcpSvc)
	localModelHandler := handler.NewLocalModelHandler(localModelSvc)
	ollamaModelHandler := handler.NewOllamaModelHandler(ollamaModelSvc)
	consoleHandler := handler.NewConsoleHandler(consoleSvc)

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(middleware.Logger(logger))

	// Root routes (no auth)
	r.GET("", rootHandler.HandleRoot)
	r.GET("/api/version", rootHandler.HandleVersion)
	r.GET("/logo.png", rootHandler.HandleLogo)
	r.GET("/copaw-symbol.svg", rootHandler.HandleSymbol)
	r.GET("/health", healthHandler.HandleHealth)

	// API routes with auth
	api := r.Group("/api")
	api.Use(middleware.Auth())
	{
		// API root
		api.GET("", rootHandler.HandleAPIRoot)

		// Agent routes
		agentGroup := api.Group("/agent")
		{
			agentGroup.GET("", agentHandler.HandleStatus)
			agentGroup.GET("/health", agentHandler.HandleHealth)
			agentGroup.POST("/process", agentHandler.HandleProcess)
			agentGroup.GET("/admin/status", agentHandler.HandleAdminStatus)
			agentGroup.POST("/shutdown", agentHandler.HandleShutdown)
			agentGroup.POST("/admin/shutdown", agentHandler.HandleAdminShutdown)
			agentGroup.GET("/running-config", agentHandler.HandleGetRunningConfig)
			agentGroup.PUT("/running-config", agentHandler.HandleUpdateRunningConfig)
			agentGroup.GET("/files", workspaceHandler.HandleListFiles)
			agentGroup.GET("/files/:fileName", workspaceHandler.HandleGetFile)
			agentGroup.PUT("/files/:fileName", workspaceHandler.HandleSaveFile)
			agentGroup.GET("/memory", workspaceHandler.HandleListMemories)
			agentGroup.GET("/memory/:date", workspaceHandler.HandleGetMemory)
			agentGroup.PUT("/memory/:date", workspaceHandler.HandleSaveMemory)
		}

		// Chat routes
		chats := api.Group("/chats")
		{
			chats.GET("", chatHandler.HandleListChats)
			chats.POST("", chatHandler.HandleCreateChat)
			chats.POST("/batch-delete", chatHandler.HandleBatchDelete)
			chats.GET("/:chatId", chatHandler.HandleGetChat)
			chats.PUT("/:chatId", chatHandler.HandleUpdateChat)
			chats.DELETE("/:chatId", chatHandler.HandleDeleteChat)
		}

		// Cron routes
		cronGroup := api.Group("/cron/jobs")
		{
			cronGroup.GET("", cronJobHandler.HandleListJobs)
			cronGroup.POST("", cronJobHandler.HandleCreateJob)
			cronGroup.GET("/:jobId", cronJobHandler.HandleGetJob)
			cronGroup.PUT("/:jobId", cronJobHandler.HandleUpdateJob)
			cronGroup.DELETE("/:jobId", cronJobHandler.HandleDeleteJob)
			cronGroup.POST("/:jobId/pause", cronJobHandler.HandlePauseJob)
			cronGroup.POST("/:jobId/resume", cronJobHandler.HandleResumeJob)
			cronGroup.POST("/:jobId/run", cronJobHandler.HandleRunJob)
			cronGroup.GET("/:jobId/state", cronJobHandler.HandleGetJobState)
		}

		// Config routes
		configGroup := api.Group("/config")
		{
			configGroup.GET("/channels/types", channelHandler.HandleGetChannelTypes)
			configGroup.GET("/channels", channelHandler.HandleGetChannels)
			configGroup.PUT("/channels", channelHandler.HandleUpdateChannels)
			configGroup.GET("/channels/:channelName", channelHandler.HandleGetChannel)
			configGroup.PUT("/channels/:channelName", channelHandler.HandleUpdateChannel)
			configGroup.GET("/heartbeat", heartbeatHandler.HandleGetConfig)
			configGroup.PUT("/heartbeat", heartbeatHandler.HandleUpdateConfig)
		}

		// Environment routes
		envs := api.Group("/envs")
		{
			envs.GET("", envHandler.HandleListEnvs)
			envs.PUT("", envHandler.HandleUpdateEnvs)
			envs.DELETE("/:key", envHandler.HandleDeleteEnv)
		}

		// Model/Provider routes
		models := api.Group("/models")
		{
			models.GET("", providerHandler.HandleListProviders)
			models.GET("/active", providerHandler.HandleGetActiveModels)
			models.PUT("/active", providerHandler.HandleSetActiveModel)
			models.POST("/custom-providers", providerHandler.HandleCreateCustomProvider)
			models.DELETE("/custom-providers/:providerId", providerHandler.HandleDeleteCustomProvider)
			models.PUT("/:providerId/config", providerHandler.HandleConfigureProvider)
			models.POST("/:providerId/models", providerHandler.HandleAddModel)
			models.DELETE("/:providerId/models/:modelId", providerHandler.HandleRemoveModel)
			models.POST("/:providerId/test", providerHandler.HandleTestProvider)
			models.POST("/:providerId/models/test", providerHandler.HandleTestModel)
		}

		// Skill routes
		skills := api.Group("/skills")
		{
			skills.GET("", skillHandler.HandleListSkills)
			skills.GET("/available", skillHandler.HandleListAvailableSkills)
			skills.POST("", skillHandler.HandleCreateSkill)
			skills.POST("/:skillName/enable", skillHandler.HandleEnableSkill)
			skills.POST("/:skillName/disable", skillHandler.HandleDisableSkill)
			skills.POST("/batch-enable", skillHandler.HandleBatchEnableSkills)
			skills.POST("/batch-disable", skillHandler.HandleBatchDisableSkills)
			skills.DELETE("/:skillName", skillHandler.HandleDeleteSkill)
			skills.GET("/hub/search", skillHandler.HandleSearchHub)
			skills.POST("/hub/install", skillHandler.HandleInstallSkill)
			skills.GET("/:skillName/files/:source/*filePath", skillHandler.HandleLoadSkillFile)
		}

		// Workspace routes
		workspace := api.Group("/workspace")
		{
			workspace.GET("/download", workspaceHandler.HandleDownload)
			workspace.POST("/upload", workspaceHandler.HandleUpload)
		}

		// Local model routes
		localModels := api.Group("/local-models")
		{
			localModels.GET("", localModelHandler.HandleListModels)
			localModels.POST("/download", localModelHandler.HandleDownload)
			localModels.GET("/download-status", localModelHandler.HandleGetDownloadStatus)
			localModels.POST("/cancel-download/:taskId", localModelHandler.HandleCancelDownload)
			localModels.DELETE("/:modelId", localModelHandler.HandleDeleteModel)
		}

		// Ollama model routes
		ollamaModels := api.Group("/ollama-models")
		{
			ollamaModels.GET("", ollamaModelHandler.HandleListModels)
			ollamaModels.POST("/download", ollamaModelHandler.HandleDownload)
			ollamaModels.GET("/download-status", ollamaModelHandler.HandleGetDownloadStatus)
			ollamaModels.DELETE("/download/:taskId", ollamaModelHandler.HandleCancelDownload)
			ollamaModels.DELETE("/:name", ollamaModelHandler.HandleDeleteModel)
		}

		// MCP routes
		mcp := api.Group("/mcp")
		{
			mcp.GET("", mcpHandler.HandleListClients)
			mcp.GET("/:clientKey", mcpHandler.HandleGetClient)
			mcp.POST("", mcpHandler.HandleCreateClient)
			mcp.PUT("/:clientKey", mcpHandler.HandleUpdateClient)
			mcp.PATCH("/:clientKey/toggle", mcpHandler.HandleToggleClient)
			mcp.DELETE("/:clientKey", mcpHandler.HandleDeleteClient)
		}

		// Console routes
		console := api.Group("/console")
		{
			console.GET("/push-messages", consoleHandler.HandleGetPushMessages)
		}
	}

	// SPA fallback for unmatched routes
	r.NoRoute(rootHandler.HandleSPAFallback)

	// Start server
	port := getEnvOrDefault("PORT", "8099")
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		logger.Info("Starting CoPaw Console API Server", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Stop cron service
	if cronSvc != nil {
		cronSvc.Stop()
	}

	// Close memory store
	if memoryStore != nil {
		memoryStore.Close()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

func loadConfig(workspaceDir string) (*goclawConfig.Config, error) {
	configPath := filepath.Join(workspaceDir, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// 尝试 config.yaml
		configPath = filepath.Join(workspaceDir, "config.yaml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			// 配置文件不存在，返回默认配置
			return &goclawConfig.Config{}, nil
		}
	}

	// 使用 goclaw 的 config loader 加载配置
	cfg, err := goclawConfig.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
