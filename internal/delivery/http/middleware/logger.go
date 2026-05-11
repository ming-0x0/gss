package middleware

import (
	"time"

	"gss/internal/infrastructure/logger"

	"github.com/gin-gonic/gin"
)

func Logger(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()

		if raw != "" {
			path = path + "?" + raw
		}

		log.Info("HTTP request",
			"status", status,
			"method", method,
			"path", path,
			"latency", latency.String(),
			"ip", clientIP,
		)
	}
}
