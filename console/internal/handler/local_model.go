package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smallnest/goclaw/console/internal/model"
	"github.com/smallnest/goclaw/console/internal/service"
)

// LocalModelHandler handles local model API endpoints
type LocalModelHandler struct {
	modelSvc *service.LocalModelService
}

// NewLocalModelHandler creates a new local model handler
func NewLocalModelHandler(modelSvc *service.LocalModelService) *LocalModelHandler {
	return &LocalModelHandler{
		modelSvc: modelSvc,
	}
}

// HandleListModels handles GET /api/local-models
func (h *LocalModelHandler) HandleListModels(c *gin.Context) {
	backend := c.Query("backend")
	models := h.modelSvc.ListModels(backend)
	c.JSON(http.StatusOK, models)
}

// HandleDownload handles POST /api/local-models/download
func (h *LocalModelHandler) HandleDownload(c *gin.Context) {
	var req model.LocalModelDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	task := h.modelSvc.StartDownload(&req)
	c.JSON(http.StatusAccepted, task)
}

// HandleGetDownloadStatus handles GET /api/local-models/download-status
func (h *LocalModelHandler) HandleGetDownloadStatus(c *gin.Context) {
	backend := c.Query("backend")
	tasks := h.modelSvc.GetDownloadStatus(backend)
	c.JSON(http.StatusOK, tasks)
}

// HandleCancelDownload handles POST /api/local-models/cancel-download/:taskId
func (h *LocalModelHandler) HandleCancelDownload(c *gin.Context) {
	taskID := c.Param("taskId")

	response, err := h.modelSvc.CancelDownload(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleDeleteModel handles DELETE /api/local-models/:modelId
func (h *LocalModelHandler) HandleDeleteModel(c *gin.Context) {
	modelID := c.Param("modelId")

	response, err := h.modelSvc.DeleteModel(modelID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}
