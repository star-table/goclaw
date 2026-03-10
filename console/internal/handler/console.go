package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smallnest/goclaw/console/internal/service"
)

// ConsoleHandler handles console API endpoints
type ConsoleHandler struct {
	consoleSvc *service.ConsoleService
}

// NewConsoleHandler creates a new console handler
func NewConsoleHandler(consoleSvc *service.ConsoleService) *ConsoleHandler {
	return &ConsoleHandler{
		consoleSvc: consoleSvc,
	}
}

// HandleGetPushMessages handles GET /api/console/push-messages
// Query params:
//   - session_id: optional, if provided, consumes messages for that session
//   - without session_id: returns recent messages (last 60s), not consumed
func (h *ConsoleHandler) HandleGetPushMessages(c *gin.Context) {
	sessionID := c.Query("session_id")
	response := h.consoleSvc.GetPushMessages(sessionID)
	c.JSON(http.StatusOK, response)
}
