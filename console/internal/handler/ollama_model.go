package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smallnest/goclaw/console/internal/model"
	"github.com/smallnest/goclaw/console/internal/service"
)

// OllamaModelHandler handles Ollama model API endpoints
type OllamaModelHandler struct {
	modelSvc *service.OllamaModelService
}

// NewOllamaModelHandler creates a new Ollama model handler
func NewOllamaModelHandler(modelSvc *service.OllamaModelService) *OllamaModelHandler {
	return &OllamaModelHandler{
		modelSvc: modelSvc,
	}
}

// HandleListModels handles GET /api/ollama-models
func (h *OllamaModelHandler) HandleListModels(c *gin.Context) {
	models := h.modelSvc.ListModels()
	c.JSON(http.StatusOK, models)
}

// HandleDownload handles POST /api/ollama-models/download
func (h *OllamaModelHandler) HandleDownload(c *gin.Context) {
	var req model.OllamaDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	task := h.modelSvc.StartDownload(&req)
	c.JSON(http.StatusAccepted, task)
}

// HandleGetDownloadStatus handles GET /api/ollama-models/download-status
func (h *OllamaModelHandler) HandleGetDownloadStatus(c *gin.Context) {
	tasks := h.modelSvc.GetDownloadStatus()
	c.JSON(http.StatusOK, tasks)
}

// HandleCancelDownload handles DELETE /api/ollama-models/download/:taskId
func (h *OllamaModelHandler) HandleCancelDownload(c *gin.Context) {
	taskID := c.Param("taskId")

	response, err := h.modelSvc.CancelDownload(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleDeleteModel handles DELETE /api/ollama-models/:name
func (h *OllamaModelHandler) HandleDeleteModel(c *gin.Context) {
	name := c.Param("name")

	response, err := h.modelSvc.DeleteModel(name)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}
