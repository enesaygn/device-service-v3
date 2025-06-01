// internal/middleware/logging_middleware.go
package middleware

import (
	"device-service/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
)

func LoggingMiddleware(logger *utils.ServiceLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		c.Next()
		duration := time.Since(startTime)

		logger.LogAPIRequest(
			c.Request.Method,
			c.Request.URL.Path,
			c.Request.UserAgent(),
			c.ClientIP(),
			c.Writer.Status(),
			duration,
		)
	}
}
