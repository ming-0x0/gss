package middleware

import (
	"context"
	"net/http"

	"gss/configs"

	"github.com/gin-gonic/gin"
)

func Timeout() gin.HandlerFunc {
	return func(c *gin.Context) {
		timeout := configs.Get().HTTP.RequestTimeout
		if timeout <= 0 {
			c.Next()
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()

		if ctx.Err() == context.DeadlineExceeded {
			c.AbortWithStatus(http.StatusGatewayTimeout)
		}
	}
}
