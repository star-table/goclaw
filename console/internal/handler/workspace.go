package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smallnest/goclaw/console/internal/model"
	"github.com/smallnest/goclaw/console/internal/service"
)

// WorkspaceHandler handles workspace API endpoints
type WorkspaceHandler struct {
	workspaceSvc *service.WorkspaceService
}

// NewWorkspaceHandler creates a new workspace handler
func NewWorkspaceHandler(workspaceSvc *service.WorkspaceService) *WorkspaceHandler {
	return &WorkspaceHandler{
		workspaceSvc: workspaceSvc,
	}
}

// HandleListFiles handles GET /api/agent/files
func (h *WorkspaceHandler) HandleListFiles(c *gin.Context) {
	files := h.workspaceSvc.ListFiles()
	c.JSON(http.StatusOK, files)
}

// HandleGetFile handles GET /api/agent/files/:fileName
func (h *WorkspaceHandler) HandleGetFile(c *gin.Context) {
	fileName := c.Param("fileName")

	file, err := h.workspaceSvc.GetFile(fileName)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, file)
}

// HandleSaveFile handles PUT /api/agent/files/:fileName
func (h *WorkspaceHandler) HandleSaveFile(c *gin.Context) {
	fileName := c.Param("fileName")

	var req model.SaveFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	response := h.workspaceSvc.SaveFile(fileName, req.Content)
	c.JSON(http.StatusOK, response)
}

// HandleListMemories handles GET /api/agent/memory
func (h *WorkspaceHandler) HandleListMemories(c *gin.Context) {
	memories := h.workspaceSvc.ListMemories()
	c.JSON(http.StatusOK, memories)
}

// HandleGetMemory handles GET /api/agent/memory/:date
func (h *WorkspaceHandler) HandleGetMemory(c *gin.Context) {
	date := c.Param("date")

	memory, err := h.workspaceSvc.GetMemory(date)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, memory)
}

// HandleSaveMemory handles PUT /api/agent/memory/:date
func (h *WorkspaceHandler) HandleSaveMemory(c *gin.Context) {
	date := c.Param("date")

	var req model.SaveFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	response := h.workspaceSvc.SaveMemory(date, req.Content)
	c.JSON(http.StatusOK, response)
}

// HandleDownload handles GET /api/workspace/download
func (h *WorkspaceHandler) HandleDownload(c *gin.Context) {
	buf, filename, err := h.workspaceSvc.DownloadWorkspace()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

// HandleUpload handles POST /api/workspace/upload
func (h *WorkspaceHandler) HandleUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	// Validate content type
	contentType := file.Header.Get("Content-Type")
	if contentType != "" && contentType != "application/zip" &&
		contentType != "application/x-zip-compressed" &&
		contentType != "application/octet-stream" {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: fmt.Sprintf("Expected a zip file, got content-type: %s", contentType),
		})
		return
	}

	// Open uploaded file
	openedFile, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	defer openedFile.Close()

	// Read file content
	fileData := make([]byte, file.Size)
	_, err = openedFile.Read(fileData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}

	// Upload workspace
	err = h.workspaceSvc.UploadWorkspace(fileData)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.UploadFileResponse{
		Success: true,
		Message: "Workspace uploaded successfully",
	})
}
