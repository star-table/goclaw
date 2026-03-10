package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smallnest/goclaw/console/internal/model"
	"github.com/smallnest/goclaw/console/internal/service"
)

// ChatHandler handles chat API endpoints
type ChatHandler struct {
	chatSvc *service.ChatService
}

// NewChatHandler creates a new chat handler
func NewChatHandler(chatSvc *service.ChatService) *ChatHandler {
	return &ChatHandler{
		chatSvc: chatSvc,
	}
}

// HandleListChats handles GET /api/chats
func (h *ChatHandler) HandleListChats(c *gin.Context) {
	userID := c.Query("user_id")
	channel := c.Query("channel")

	chats := h.chatSvc.ListChats(userID, channel)
	c.JSON(http.StatusOK, chats)
}

// HandleCreateChat handles POST /api/chats
func (h *ChatHandler) HandleCreateChat(c *gin.Context) {
	var req model.CreateChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	chat := h.chatSvc.CreateChat(&req)
	c.JSON(http.StatusCreated, chat)
}

// HandleGetChat handles GET /api/chats/:chatId
// Returns chat message history (adapted for frontend compatibility)
func (h *ChatHandler) HandleGetChat(c *gin.Context) {
	chatID := c.Param("chatId")

	history, err := h.chatSvc.GetChatHistory(chatID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, history)
}

// HandleGetChatHistory handles GET /api/chats/:chatId/history
// Returns chat message history
func (h *ChatHandler) HandleGetChatHistory(c *gin.Context) {
	chatID := c.Param("chatId")

	history, err := h.chatSvc.GetChatHistory(chatID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, history)
}

// HandleUpdateChat handles PUT /api/chats/:chatId
func (h *ChatHandler) HandleUpdateChat(c *gin.Context) {
	chatID := c.Param("chatId")

	var req model.UpdateChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	chat, err := h.chatSvc.UpdateChat(chatID, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, chat)
}

// HandleDeleteChat handles DELETE /api/chats/:chatId
func (h *ChatHandler) HandleDeleteChat(c *gin.Context) {
	chatID := c.Param("chatId")

	response, err := h.chatSvc.DeleteChat(chatID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleBatchDelete handles POST /api/chats/batch-delete
func (h *ChatHandler) HandleBatchDelete(c *gin.Context) {
	var req model.BatchDeleteChatsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	response := h.chatSvc.BatchDeleteChats(req.ChatIDs)
	c.JSON(http.StatusOK, response)
}
