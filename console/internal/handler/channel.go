package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smallnest/goclaw/console/internal/model"
	"github.com/smallnest/goclaw/console/internal/service"
)

// ChannelHandler handles channel API endpoints
type ChannelHandler struct {
	channelSvc *service.ChannelService
}

// NewChannelHandler creates a new channel handler
func NewChannelHandler(channelSvc *service.ChannelService) *ChannelHandler {
	return &ChannelHandler{
		channelSvc: channelSvc,
	}
}

// HandleGetChannelTypes handles GET /api/config/channels/types
// Returns list of available channel types
func (h *ChannelHandler) HandleGetChannelTypes(c *gin.Context) {
	types := h.channelSvc.GetChannelTypes()
	c.JSON(http.StatusOK, types)
}

// HandleGetChannels handles GET /api/config/channels
// Returns all channel configurations
func (h *ChannelHandler) HandleGetChannels(c *gin.Context) {
	config := h.channelSvc.GetChannels()
	c.JSON(http.StatusOK, config)
}

// HandleUpdateChannels handles PUT /api/config/channels
// Updates all channel configurations
func (h *ChannelHandler) HandleUpdateChannels(c *gin.Context) {
	var req model.ChannelsConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	config := h.channelSvc.UpdateChannels(&req)
	c.JSON(http.StatusOK, config)
}

// HandleGetChannel handles GET /api/config/channels/:channelName
// Returns a specific channel configuration
func (h *ChannelHandler) HandleGetChannel(c *gin.Context) {
	channelName := c.Param("channelName")

	channel, err := h.channelSvc.GetChannel(channelName)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, channel)
}

// HandleUpdateChannel handles PUT /api/config/channels/:channelName
// Updates a specific channel configuration
func (h *ChannelHandler) HandleUpdateChannel(c *gin.Context) {
	channelName := c.Param("channelName")

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	channel, err := h.channelSvc.UpdateChannel(channelName, req)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, channel)
}
