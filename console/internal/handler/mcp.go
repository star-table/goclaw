package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smallnest/goclaw/console/internal/model"
	"github.com/smallnest/goclaw/console/internal/service"
)

// MCPHandler handles MCP API endpoints
type MCPHandler struct {
	mcpSvc *service.MCPService
}

// NewMCPHandler creates a new MCP handler
func NewMCPHandler(mcpSvc *service.MCPService) *MCPHandler {
	return &MCPHandler{
		mcpSvc: mcpSvc,
	}
}

// HandleListClients handles GET /api/mcp
func (h *MCPHandler) HandleListClients(c *gin.Context) {
	clients := h.mcpSvc.ListClients()
	c.JSON(http.StatusOK, clients)
}

// HandleGetClient handles GET /api/mcp/:clientKey
func (h *MCPHandler) HandleGetClient(c *gin.Context) {
	clientKey := c.Param("clientKey")

	client, err := h.mcpSvc.GetClient(clientKey)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, client)
}

// HandleCreateClient handles POST /api/mcp
func (h *MCPHandler) HandleCreateClient(c *gin.Context) {
	var req model.CreateMCPClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	client := h.mcpSvc.CreateClient(&req)
	c.JSON(http.StatusCreated, client)
}

// HandleUpdateClient handles PUT /api/mcp/:clientKey
func (h *MCPHandler) HandleUpdateClient(c *gin.Context) {
	clientKey := c.Param("clientKey")

	var req model.MCPClientPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	client, err := h.mcpSvc.UpdateClient(clientKey, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, client)
}

// HandleToggleClient handles PATCH /api/mcp/:clientKey/toggle
func (h *MCPHandler) HandleToggleClient(c *gin.Context) {
	clientKey := c.Param("clientKey")

	client, err := h.mcpSvc.ToggleClient(clientKey)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, client)
}

// HandleDeleteClient handles DELETE /api/mcp/:clientKey
func (h *MCPHandler) HandleDeleteClient(c *gin.Context) {
	clientKey := c.Param("clientKey")

	response, err := h.mcpSvc.DeleteClient(clientKey)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}
