// internal/middleware/cors_middleware.go
package middleware

import (
	"device-service/internal/config"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware creates CORS middleware
func CORSMiddleware(config *config.SecurityConfig) gin.HandlerFunc {
	corsConfig := cors.DefaultConfig()

	if len(config.AllowedOrigins) > 0 {
		corsConfig.AllowOrigins = config.AllowedOrigins
	} else {
		corsConfig.AllowAllOrigins = true
	}

	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"}
	corsConfig.ExposeHeaders = []string{"Content-Length"}
	corsConfig.AllowCredentials = true

	return cors.New(corsConfig)
}
