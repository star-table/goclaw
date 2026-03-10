package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smallnest/goclaw/console/internal/model"
	"github.com/smallnest/goclaw/console/internal/service"
)

// ProviderHandler handles provider/model API endpoints
type ProviderHandler struct {
	providerSvc *service.ProviderService
}

// NewProviderHandler creates a new provider handler
func NewProviderHandler(providerSvc *service.ProviderService) *ProviderHandler {
	return &ProviderHandler{
		providerSvc: providerSvc,
	}
}

// HandleListProviders handles GET /api/models
func (h *ProviderHandler) HandleListProviders(c *gin.Context) {
	providers := h.providerSvc.ListProviders()
	c.JSON(http.StatusOK, providers)
}

// HandleGetActiveModels handles GET /api/models/active
func (h *ProviderHandler) HandleGetActiveModels(c *gin.Context) {
	info := h.providerSvc.GetActiveModels()
	c.JSON(http.StatusOK, info)
}

// HandleSetActiveModel handles PUT /api/models/active
func (h *ProviderHandler) HandleSetActiveModel(c *gin.Context) {
	var req model.SetActiveModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	info := h.providerSvc.SetActiveModel(&req)
	c.JSON(http.StatusOK, info)
}

// HandleCreateCustomProvider handles POST /api/models/custom-providers
func (h *ProviderHandler) HandleCreateCustomProvider(c *gin.Context) {
	var req model.CreateCustomProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	provider := h.providerSvc.CreateCustomProvider(&req)
	c.JSON(http.StatusCreated, provider)
}

// HandleDeleteCustomProvider handles DELETE /api/models/custom-providers/:providerId
func (h *ProviderHandler) HandleDeleteCustomProvider(c *gin.Context) {
	providerID := c.Param("providerId")

	providers, err := h.providerSvc.DeleteCustomProvider(providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, providers)
}

// HandleConfigureProvider handles PUT /api/models/:providerId/config
func (h *ProviderHandler) HandleConfigureProvider(c *gin.Context) {
	providerID := c.Param("providerId")

	var req model.ProviderConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	provider, err := h.providerSvc.ConfigureProvider(providerID, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, provider)
}

// HandleAddModel handles POST /api/models/:providerId/models
func (h *ProviderHandler) HandleAddModel(c *gin.Context) {
	providerID := c.Param("providerId")

	var req model.AddModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	provider, err := h.providerSvc.AddModelToProvider(providerID, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, provider)
}

// HandleRemoveModel handles DELETE /api/models/:providerId/models/:modelId
func (h *ProviderHandler) HandleRemoveModel(c *gin.Context) {
	providerID := c.Param("providerId")
	modelID := c.Param("modelId")

	provider, err := h.providerSvc.RemoveModelFromProvider(providerID, modelID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, provider)
}

// HandleTestProvider handles POST /api/models/:providerId/test
func (h *ProviderHandler) HandleTestProvider(c *gin.Context) {
	providerID := c.Param("providerId")

	var req model.TestProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Optional body, ignore error
		req = model.TestProviderRequest{}
	}

	response := h.providerSvc.TestProviderConnection(providerID, &req)
	c.JSON(http.StatusOK, response)
}

// HandleTestModel handles POST /api/models/:providerId/models/test
func (h *ProviderHandler) HandleTestModel(c *gin.Context) {
	providerID := c.Param("providerId")

	var req model.TestModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	response := h.providerSvc.TestModelConnection(providerID, &req)
	c.JSON(http.StatusOK, response)
}
