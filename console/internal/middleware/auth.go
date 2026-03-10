package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// AuthHeader is the header key for authorization
	AuthHeader = "Authorization"
	// BearerPrefix is the prefix for bearer token
	BearerPrefix = "Bearer "
)

// Auth returns an authentication middleware
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for certain paths
		path := c.Request.URL.Path
		if isPublicPath(path) {
			c.Next()
			return
		}

		// Check Authorization header
		auth := c.GetHeader(AuthHeader)
		if auth == "" {
			// For development, allow requests without auth
			// In production, this should return 401
			c.Next()
			return
		}

		// Validate bearer token
		if !strings.HasPrefix(auth, BearerPrefix) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(auth, BearerPrefix)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			c.Abort()
			return
		}

		// Store token in context for later use
		c.Set("token", token)
		c.Next()
	}
}

// isPublicPath checks if the path should skip authentication
func isPublicPath(path string) bool {
	publicPaths := []string{
		"/",
		"/api/version",
		"/api/docs",
		"/logo.png",
		"/copaw-symbol.svg",
		"/health",
		"/api/agent/health",
	}

	for _, p := range publicPaths {
		if path == p {
			return true
		}
	}

	// Allow static assets and websocket
	if strings.HasPrefix(path, "/static/") ||
		strings.HasPrefix(path, "/assets/") ||
		strings.HasPrefix(path, "/ws") {
		return true
	}

	return false
}
