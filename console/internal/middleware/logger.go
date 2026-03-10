package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Logger returns a logging middleware
func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		// Skip logging for high-frequency polling endpoints
		if strings.Contains(path, "/push-messages") {
			return
		}

		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()

		if query != "" {
			path = path + "?" + query
		}

		if status >= 400 {
			logger.Warn("HTTP Request",
				zap.Int("status", status),
				zap.String("method", method),
				zap.String("path", path),
				zap.String("ip", clientIP),
				zap.Duration("latency", latency),
				zap.String("user-agent", c.Request.UserAgent()),
				zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
			)
		} else {
			logger.Info("HTTP Request",
				zap.Int("status", status),
				zap.String("method", method),
				zap.String("path", path),
				zap.String("ip", clientIP),
				zap.Duration("latency", latency),
			)
		}
	}
}
