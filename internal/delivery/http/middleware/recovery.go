package middleware

import (
	"net/http"

	"gss/internal/infrastructure/logger"

	"github.com/gin-gonic/gin"
)

func Recovery(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Error("Panic recovered",
					"error", err,
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
				)

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    http.StatusInternalServerError,
					"message": "Internal Server Error",
				})
			}
		}()

		c.Next()
	}
}
