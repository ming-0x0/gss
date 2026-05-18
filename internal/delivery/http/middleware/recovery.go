package middleware

import (
	"net/http"

	"gss/internal/infrastructure/logger"

	"github.com/gin-gonic/gin"
)

func Recovery(l *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				l.Error("panic recovered",
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"error", r,
				)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
