// internal/routes/routes.go
package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"

	"device-service/internal/config"
	"device-service/internal/database"
	"device-service/internal/handler"
	"device-service/internal/middleware"
	"device-service/internal/service"
	"device-service/internal/utils"
)

// Router holds all dependencies for routing
type Router struct {
	config           *config.Config
	logger           *zap.Logger
	db               *database.DB
	deviceService    *service.DeviceService
	operationService *service.OperationService
	discoveryService *service.DiscoveryService
}

// NewRouter creates a new router instance
func NewRouter(
	config *config.Config,
	logger *zap.Logger,
	db *database.DB,
	deviceService *service.DeviceService,
	operationService *service.OperationService,
	discoveryService *service.DiscoveryService,
) *Router {
	return &Router{
		config:           config,
		logger:           logger,
		db:               db,
		deviceService:    deviceService,
		operationService: operationService,
		discoveryService: discoveryService,
	}
}

// SetupRouter creates and configures the Gin router
func (r *Router) SetupRouter() *gin.Engine {
	// Set Gin mode
	if r.config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Create Gin engine
	router := gin.New()

	// Add middleware
	r.addMiddleware(router)

	// Add routes
	r.addRoutes(router)

	return router
}

// addMiddleware adds middleware to the router
func (r *Router) addMiddleware(router *gin.Engine) {
	// Recovery middleware
	router.Use(middleware.RecoveryMiddleware(r.logger))

	// Request ID middleware
	router.Use(middleware.RequestIDMiddleware())

	// Logging middleware
	serviceLogger := utils.NewServiceLogger(r.logger, "http-server")
	router.Use(middleware.LoggingMiddleware(serviceLogger))

	// CORS middleware
	router.Use(middleware.CORSMiddleware(&r.config.Security))

	// Authentication middleware would go here
	// router.Use(middleware.AuthMiddleware(&r.config.Security))

	r.logger.Info("Middleware configured")
}

// addRoutes sets up all application routes
func (r *Router) addRoutes(router *gin.Engine) {
	// Create handlers
	healthHandler := handler.NewHealthHandler(r.db, r.config, r.logger)
	deviceHandler := handler.NewDeviceHandler(r.deviceService, r.logger)
	operationHandler := handler.NewOperationHandler(r.operationService, r.logger)
	discoveryHandler := handler.NewDiscoveryHandler(r.discoveryService, r.logger)
	wsHandler := handler.NewWebSocketHandler(r.deviceService, r.operationService, r.logger)

	// Health check routes (no auth required)
	r.addHealthRoutes(router, healthHandler)

	// API v1 routes
	apiV1 := router.Group("/api/v1")
	r.addDeviceRoutes(apiV1, deviceHandler, operationHandler)
	r.addOperationRoutes(apiV1, operationHandler)
	r.addDiscoveryRoutes(apiV1, discoveryHandler)

	// WebSocket routes
	r.addWebSocketRoutes(router, wsHandler)

	// Documentation routes
	r.addDocumentationRoutes(router)

	r.logger.Info("All routes configured successfully")
}

// addHealthRoutes sets up health check routes
func (r *Router) addHealthRoutes(router *gin.Engine, handler *handler.HealthHandler) {
	health := router.Group("")
	{
		health.GET("/health", handler.HealthCheck)
		health.GET("/health/db", handler.DatabaseHealthCheck)
		health.GET("/ready", handler.ReadinessCheck)
		health.GET("/live", handler.LivenessCheck)
	}
}

// addDeviceRoutes sets up device management routes
func (r *Router) addDeviceRoutes(api *gin.RouterGroup, deviceHandler *handler.DeviceHandler, operationHandler *handler.OperationHandler) {
	devices := api.Group("/devices")
	{
		// Device CRUD operations
		devices.POST("", deviceHandler.RegisterDevice)
		devices.GET("", deviceHandler.ListDevices)

		// Individual device operations
		device := devices.Group("/:device_id")
		{
			// Device management
			device.GET("", deviceHandler.GetDevice)
			device.PUT("", deviceHandler.UpdateDevice)
			device.DELETE("", deviceHandler.DeleteDevice)
			device.POST("/connect", deviceHandler.ConnectDevice)
			device.POST("/disconnect", deviceHandler.DisconnectDevice)
			device.POST("/test", deviceHandler.TestDevice)
			device.GET("/health", deviceHandler.GetDeviceHealth)
			device.PUT("/config", deviceHandler.UpdateDeviceConfig)

			// Device operations - DİREKT DEVICE ALTINDA
			device.POST("/print", operationHandler.PrintOperation)
			device.POST("/payment", operationHandler.PaymentOperation)
			device.POST("/scan", operationHandler.ScanOperation)
			device.POST("/open-drawer", operationHandler.OpenDrawerOperation)
			device.POST("/display", operationHandler.DisplayOperation)
			device.GET("/operations", operationHandler.ListDeviceOperations)
		}
	}
}

// addOperationRoutes sets up general operation routes
func (r *Router) addOperationRoutes(api *gin.RouterGroup, handler *handler.OperationHandler) {
	operations := api.Group("/operations")
	{
		operations.POST("", handler.ExecuteOperation)
		operations.GET("", handler.ListOperations)
		operations.GET("/:operation_id", handler.GetOperation)
		operations.PUT("/:operation_id/cancel", handler.CancelOperation)
	}
}

// addDiscoveryRoutes sets up device discovery routes
func (r *Router) addDiscoveryRoutes(api *gin.RouterGroup, handler *handler.DiscoveryHandler) {
	discovery := api.Group("/discovery")
	{
		discovery.GET("/scan", handler.ScanDevices)
		discovery.POST("/auto-setup", handler.AutoSetupDevices)
		discovery.GET("/supported", handler.GetSupportedDevices)
		discovery.GET("/capabilities/:brand/:type", handler.GetCapabilities)
	}
}

// addWebSocketRoutes sets up WebSocket routes
func (r *Router) addWebSocketRoutes(router *gin.Engine, handler *handler.WebSocketHandler) {
	ws := router.Group("/ws")
	{
		ws.GET("/devices/:device_id", handler.HandleDeviceConnection)
		ws.GET("/events", handler.HandleEventConnection)
		ws.GET("/operations", handler.HandleOperationConnection)
		ws.GET("/branches/:branch_id", handler.HandleBranchConnection)
	}
}

// addDocumentationRoutes sets up documentation routes
func (r *Router) addDocumentationRoutes(router *gin.Engine) {
	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	// Swagger redirect for convenience
	router.GET("/docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
}
