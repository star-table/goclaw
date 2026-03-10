package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smallnest/goclaw/console/internal/model"
)

// RootHandler handles root API endpoints
type RootHandler struct{}

// NewRootHandler creates a new root handler
func NewRootHandler() *RootHandler {
	return &RootHandler{}
}

// HandleRoot handles GET /
func (h *RootHandler) HandleRoot(c *gin.Context) {
	c.JSON(http.StatusOK, model.RootResponse{
		Message: "StarClaw Console API Server",
		Docs:    "/api/docs",
	})
}

// HandleAPIRoot handles GET /api/
func (h *RootHandler) HandleAPIRoot(c *gin.Context) {
	c.JSON(http.StatusOK, model.RootResponse{
		Message: "StarClaw Console API Server",
		Docs:    "/api/docs",
	})
}

// HandleVersion handles GET /api/version
func (h *RootHandler) HandleVersion(c *gin.Context) {
	c.JSON(http.StatusOK, model.VersionInfo{
		Version: "1.0.0",
	})
}

// HandleSPAFallback handles GET /* for SPA routing
func (h *RootHandler) HandleSPAFallback(c *gin.Context) {
	c.JSON(http.StatusOK, model.SPAFallbackResponse{
		Message: "SPA Fallback - In production, this would serve index.html",
		Path:    c.Request.URL.Path,
	})
}

// HandleLogo handles GET /logo.png
func (h *RootHandler) HandleLogo(c *gin.Context) {
	// Return a placeholder response
	c.Data(http.StatusOK, "image/png", []byte{})
}

// HandleSymbol handles GET /copaw-symbol.svg
func (h *RootHandler) HandleSymbol(c *gin.Context) {
	// Return a placeholder SVG
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40" fill="#4F46E5"/></svg>`
	c.Data(http.StatusOK, "image/svg+xml", []byte(svg))
}

// HealthHandler handles health check endpoints
type HealthHandler struct{}

// NewHealthHandler creates a new health handler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// HandleHealth handles GET /health
func (h *HealthHandler) HandleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}
