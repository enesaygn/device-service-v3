// internal/handler/health_handler.go
package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"device-service/internal/config"
	"device-service/internal/database"
	"device-service/internal/utils"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	db     *database.DB
	config *config.Config
	logger *utils.ServiceLogger
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *database.DB, config *config.Config, logger *zap.Logger) *HealthHandler {
	return &HealthHandler{
		db:     db,
		config: config,
		logger: utils.NewServiceLogger(logger, "health-handler"),
	}
}

// RegisterRoutes registers health check routes
func (h *HealthHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/health", h.HealthCheck)
	router.GET("/health/db", h.DatabaseHealthCheck)
	router.GET("/ready", h.ReadinessCheck)
	router.GET("/live", h.LivenessCheck)
}

// HealthCheck performs general health check
// @Summary Health check
// @Description Get overall service health status including database connectivity
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} HealthResponse "Service is healthy"
// @Failure 503 {object} HealthResponse "Service is unhealthy"
// @Router /health [get]
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	startTime := time.Now()

	health := &HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Service:   h.config.App.Name,
		Version:   h.config.App.Version,
		Uptime:    time.Since(startTime).String(),
		Checks:    make(map[string]CheckResult),
	}

	// Database check
	dbErr := h.db.HealthCheck()
	if dbErr != nil {
		health.Status = "unhealthy"
		health.Checks["database"] = CheckResult{
			Status:  "unhealthy",
			Message: dbErr.Error(),
		}
	} else {
		health.Checks["database"] = CheckResult{
			Status:  "healthy",
			Message: "Database connection OK",
		}
	}

	// Add database stats
	stats := h.db.GetStats()
	health.Checks["database_stats"] = CheckResult{
		Status: "healthy",
		Data: map[string]interface{}{
			"open_connections": stats.OpenConnections,
			"in_use":           stats.InUse,
			"idle":             stats.Idle,
		},
	}

	statusCode := http.StatusOK
	if health.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, health)
}

// DatabaseHealthCheck checks database connectivity
// @Summary Database health check
// @Description Check database connectivity and performance
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} utils.APIResponse "Database is healthy"
// @Failure 503 {object} utils.APIResponse "Database is unhealthy"
// @Router /health/db [get]
func (h *HealthHandler) DatabaseHealthCheck(c *gin.Context) {
	startTime := time.Now()

	if err := h.db.HealthCheck(); err != nil {
		h.logger.Error("Database health check failed", zap.Error(err))
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Database unhealthy", err)
		return
	}

	stats := h.db.GetStats()
	response := gin.H{
		"status":           "healthy",
		"response_time_ms": time.Since(startTime).Milliseconds(),
		"stats": gin.H{
			"open_connections":     stats.OpenConnections,
			"in_use":               stats.InUse,
			"idle":                 stats.Idle,
			"wait_count":           stats.WaitCount,
			"wait_duration":        stats.WaitDuration,
			"max_idle_closed":      stats.MaxIdleClosed,
			"max_idle_time_closed": stats.MaxIdleTimeClosed,
			"max_lifetime_closed":  stats.MaxLifetimeClosed,
		},
	}

	utils.SuccessResponse(c, http.StatusOK, "Database is healthy", response)
}

// ReadinessCheck for Kubernetes readiness probe
// @Summary Readiness check
// @Description Check if service is ready to accept traffic
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} object{status=string,timestamp=string} "Service is ready"
// @Failure 503 {object} object{status=string,reason=string} "Service is not ready"
// @Router /ready [get]
func (h *HealthHandler) ReadinessCheck(c *gin.Context) {
	// Check if service is ready to accept traffic
	if err := h.db.HealthCheck(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"reason": "database not available",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ready",
		"timestamp": time.Now(),
	})
}

// LivenessCheck for Kubernetes liveness probe
// @Summary Liveness check
// @Description Check if service is alive
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} object{status=string,timestamp=string} "Service is alive"
// @Router /live [get]
func (h *HealthHandler) LivenessCheck(c *gin.Context) {
	// Simple liveness check - service is alive if it can respond
	c.JSON(http.StatusOK, gin.H{
		"status":    "alive",
		"timestamp": time.Now(),
	})
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Service   string                 `json:"service"`
	Version   string                 `json:"version"`
	Uptime    string                 `json:"uptime"`
	Checks    map[string]CheckResult `json:"checks"`
}

// CheckResult represents individual check result
type CheckResult struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}
