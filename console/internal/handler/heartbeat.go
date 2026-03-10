package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smallnest/goclaw/console/internal/model"
	"github.com/smallnest/goclaw/console/internal/service"
)

// HeartbeatHandler handles heartbeat API endpoints
type HeartbeatHandler struct {
	heartbeatSvc *service.HeartbeatService
}

// NewHeartbeatHandler creates a new heartbeat handler
func NewHeartbeatHandler(heartbeatSvc *service.HeartbeatService) *HeartbeatHandler {
	return &HeartbeatHandler{
		heartbeatSvc: heartbeatSvc,
	}
}

// HandleGetConfig handles GET /api/config/heartbeat
func (h *HeartbeatHandler) HandleGetConfig(c *gin.Context) {
	config := h.heartbeatSvc.GetConfig()
	c.JSON(http.StatusOK, config)
}

// HandleUpdateConfig handles PUT /api/config/heartbeat
func (h *HeartbeatHandler) HandleUpdateConfig(c *gin.Context) {
	var req model.HeartbeatConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	config := h.heartbeatSvc.UpdateConfig(&req)
	c.JSON(http.StatusOK, config)
}
