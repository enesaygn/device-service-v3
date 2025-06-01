// internal/middleware/recovery_middleware.go
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"device-service/internal/utils"
)

// RecoveryMiddleware creates panic recovery middleware
func RecoveryMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		logger.Error("Panic recovered",
			zap.Any("panic", recovered),
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
			zap.Stack("stacktrace"),
		)

		utils.ErrorResponse(c, http.StatusInternalServerError, "Internal server error", nil)
	})
}
