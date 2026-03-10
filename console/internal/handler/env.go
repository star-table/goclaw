package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smallnest/goclaw/console/internal/model"
	"github.com/smallnest/goclaw/console/internal/service"
)

// EnvHandler handles environment variable API endpoints
type EnvHandler struct {
	envSvc *service.EnvService
}

// NewEnvHandler creates a new env handler
func NewEnvHandler(envSvc *service.EnvService) *EnvHandler {
	return &EnvHandler{
		envSvc: envSvc,
	}
}

// HandleListEnvs handles GET /api/envs
func (h *EnvHandler) HandleListEnvs(c *gin.Context) {
	envs := h.envSvc.ListEnvs()
	c.JSON(http.StatusOK, envs)
}

// HandleUpdateEnvs handles PUT /api/envs
func (h *EnvHandler) HandleUpdateEnvs(c *gin.Context) {
	var req model.EnvUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	envs := h.envSvc.UpdateEnvs(req)
	c.JSON(http.StatusOK, envs)
}

// HandleDeleteEnv handles DELETE /api/envs/:key
func (h *EnvHandler) HandleDeleteEnv(c *gin.Context) {
	key := c.Param("key")

	envs := h.envSvc.DeleteEnv(key)
	c.JSON(http.StatusOK, envs)
}
