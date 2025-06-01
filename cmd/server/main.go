// cmd/server/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"

	_ "device-service/docs"
	"device-service/internal/config"
	"device-service/internal/database"
	"device-service/internal/driver"
	"device-service/internal/handler"
	"device-service/internal/middleware"
	"device-service/internal/model"
	"device-service/internal/repository"
	"device-service/internal/service"
	"device-service/internal/utils"
)

// Application represents the main application
type Application struct {
	config   *config.Config
	logger   *zap.Logger
	server   *http.Server
	database *database.DB

	// Services
	deviceService    *service.DeviceService
	operationService *service.OperationService
	discoveryService *service.DiscoveryService

	// Repositories
	deviceRepo    repository.DeviceRepository
	operationRepo repository.OperationRepository
	offlineRepo   repository.OfflineRepository

	// Driver registry
	driverRegistry *driver.Registry
}

// @title Device Service API
// @version 1.0.0
// @description Enterprise Device Management Service for POS systems, printers, and payment terminals
// @termsOfService http://swagger.io/terms/

// @contact.name Device Service API Support
// @contact.email support@deviceservice.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8084
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
func main() {
	// Initialize application
	app, err := NewApplication()
	if err != nil {
		fmt.Printf("Failed to initialize application: %v\n", err)
		os.Exit(1)
	}

	// Start the application
	if err := app.Start(); err != nil {
		app.logger.Fatal("Failed to start application", zap.Error(err))
	}
}

// NewApplication creates a new application instance
func NewApplication() (*Application, error) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize logger
	logger, err := utils.NewLogger(&cfg.Logging)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Create service logger
	serviceLogger := utils.NewServiceLogger(logger, "device-service")
	serviceLogger.LogServiceStart(cfg.App.Version, cfg)

	app := &Application{
		config: cfg,
		logger: logger,
	}

	// Initialize components
	if err := app.initializeDatabase(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	if err := app.initializeRepositories(); err != nil {
		return nil, fmt.Errorf("failed to initialize repositories: %w", err)
	}

	if err := app.initializeDriverRegistry(); err != nil {
		return nil, fmt.Errorf("failed to initialize driver registry: %w", err)
	}

	if err := app.initializeServices(); err != nil {
		return nil, fmt.Errorf("failed to initialize services: %w", err)
	}

	if err := app.initializeServer(); err != nil {
		return nil, fmt.Errorf("failed to initialize server: %w", err)
	}

	return app, nil
}

// initializeDatabase sets up database connection and runs migrations
func (app *Application) initializeDatabase() error {
	// Create database connection
	db, err := database.NewConnection(&app.config.Database, app.logger)
	if err != nil {
		return fmt.Errorf("failed to create database connection: %w", err)
	}

	app.database = db

	// Run migrations TODO: sonra
	// migrator := database.NewMigrator(db, app.logger, &app.config.Database)
	// if err := migrator.Up(); err != nil {
	// 	return fmt.Errorf("failed to run database migrations: %w", err)
	// }

	app.logger.Info("Database initialized successfully")
	return nil
}

// initializeRepositories creates repository instances
func (app *Application) initializeRepositories() error {
	app.deviceRepo = repository.NewDeviceRepository(app.database, app.logger)
	app.operationRepo = repository.NewOperationRepository(app.database, app.logger)
	app.offlineRepo = repository.NewOfflineRepository(app.database, app.logger)

	app.logger.Info("Repositories initialized successfully")
	return nil
}

// initializeDriverRegistry sets up device driver registry
func (app *Application) initializeDriverRegistry() error {
	app.driverRegistry = driver.NewRegistry(app.logger)

	// Register all supported drivers
	driver.RegisterDefaultDrivers(app.driverRegistry, app.logger)

	app.logger.Info("Driver registry initialized successfully",
		zap.Int("registered_drivers", len(app.driverRegistry.ListDrivers())),
	)
	return nil
}

// initializeServices creates service instances
func (app *Application) initializeServices() error {
	// Create device service
	app.deviceService = service.NewDeviceService(
		app.deviceRepo,
		app.operationRepo,
		app.driverRegistry,
		app.config,
		app.logger,
	)

	// Create operation service
	app.operationService = service.NewOperationService(
		app.operationRepo,
		app.deviceRepo,
		app.driverRegistry,
		app.config,
		app.logger,
	)

	// Create discovery service
	app.discoveryService = service.NewDiscoveryService(
		app.deviceRepo,
		app.driverRegistry,
		app.config,
		app.logger,
	)

	app.logger.Info("Services initialized successfully")
	return nil
}

// initializeServer sets up HTTP server and routes
func (app *Application) initializeServer() error {
	// Set Gin mode
	if app.config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Create Gin engine
	router := gin.New()

	// Add middleware
	app.addMiddleware(router)

	// Add routes
	app.addRoutes(router)

	// Create HTTP server
	app.server = &http.Server{
		Addr:         app.config.GetServerAddr(),
		Handler:      router,
		ReadTimeout:  app.config.Server.ReadTimeout,
		WriteTimeout: app.config.Server.WriteTimeout,
		IdleTimeout:  app.config.Server.IdleTimeout,
	}

	app.logger.Info("HTTP server initialized",
		zap.String("address", app.config.GetServerAddr()),
		zap.Bool("tls_enabled", app.config.Server.TLS.Enabled),
	)

	return nil
}

// addMiddleware adds middleware to the router
func (app *Application) addMiddleware(router *gin.Engine) {
	// Recovery middleware
	router.Use(middleware.RecoveryMiddleware(app.logger))

	// Request ID middleware
	router.Use(middleware.RequestIDMiddleware())

	// Logging middleware
	serviceLogger := utils.NewServiceLogger(app.logger, "http-server")
	router.Use(middleware.LoggingMiddleware(serviceLogger))

	// CORS middleware
	router.Use(middleware.CORSMiddleware(&app.config.Security))

	// Authentication middleware would go here
	// router.Use(middleware.AuthMiddleware(&app.config.Security))

	app.logger.Info("Middleware configured")
}

// addRoutes adds all routes to the router
// Updated addRoutes method in main.go
// cmd/server/main.go - addRoutes method düzeltilmiş
// cmd/server/main.go - addRoutes method tamamen düzeltilmiş
func (app *Application) addRoutes(router *gin.Engine) {
	// Health check routes (no authentication required)
	healthHandler := handler.NewHealthHandler(app.database, app.config, app.logger)
	healthHandler.RegisterRoutes(router.Group(""))

	// API routes
	api := router.Group("/api/v1")

	// Device routes
	deviceHandler := handler.NewDeviceHandler(app.deviceService, app.logger)
	deviceHandler.RegisterRoutes(api)

	// Operation routes (genel operation routes)
	operationHandler := handler.NewOperationHandler(app.operationService, app.logger)
	operationHandler.RegisterRoutes(api)

	// Device-specific operation routes (ayrı prefix ile)
	operationHandler.RegisterDeviceRoutes(api)

	// Discovery routes
	discoveryHandler := handler.NewDiscoveryHandler(app.discoveryService, app.logger)
	discoveryHandler.RegisterRoutes(api)

	// WebSocket routes
	wsHandler := handler.NewWebSocketHandler(app.deviceService, app.operationService, app.logger)
	wsHandler.RegisterRoutes(router.Group("/ws"))

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	// Swagger redirect for convenience
	router.GET("/docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})

	app.logger.Info("Routes configured including Swagger documentation")
}

// Alternatif çözüm: Tek method ile daha temiz yapı
func (app *Application) addRoutesAlternative(router *gin.Engine) {
	// Health check routes
	healthHandler := handler.NewHealthHandler(app.database, app.config, app.logger)
	healthHandler.RegisterRoutes(router.Group(""))

	// API v1 routes
	apiV1 := router.Group("/api/v1")

	// Device management routes
	deviceHandler := handler.NewDeviceHandler(app.deviceService, app.logger)
	devices := apiV1.Group("/devices")
	{
		// Device CRUD operations
		devices.POST("", deviceHandler.RegisterDevice)
		devices.GET("", deviceHandler.ListDevices)

		// Individual device operations
		device := devices.Group("/:device_id")
		{
			device.GET("", deviceHandler.GetDevice)
			device.PUT("", deviceHandler.UpdateDevice)
			device.DELETE("", deviceHandler.DeleteDevice)
			device.POST("/connect", deviceHandler.ConnectDevice)
			device.POST("/disconnect", deviceHandler.DisconnectDevice)
			device.POST("/test", deviceHandler.TestDevice)
			device.GET("/health", deviceHandler.GetDeviceHealth)
			device.PUT("/config", deviceHandler.UpdateDeviceConfig)
		}
	}

	// Operation management routes
	operationHandler := handler.NewOperationHandler(app.operationService, app.logger)
	operations := apiV1.Group("/operations")
	{
		operations.POST("", operationHandler.ExecuteOperation)
		operations.GET("", operationHandler.ListOperations)
		operations.GET("/:operation_id", operationHandler.GetOperation)
		operations.PUT("/:operation_id/cancel", operationHandler.CancelOperation)
	}

	// Device-specific operation routes
	deviceOps := apiV1.Group("/device-operations")
	{
		deviceOps.POST("/:device_id/execute", operationHandler.ExecuteDeviceOperation)
		deviceOps.GET("/:device_id/list", operationHandler.ListDeviceOperations)
		deviceOps.POST("/:device_id/print", operationHandler.PrintOperation)
		deviceOps.POST("/:device_id/payment", operationHandler.PaymentOperation)
		deviceOps.POST("/:device_id/scan", operationHandler.ScanOperation)
		deviceOps.POST("/:device_id/open-drawer", operationHandler.OpenDrawerOperation)
		deviceOps.POST("/:device_id/display", operationHandler.DisplayOperation)
	}

	// Discovery routes
	discoveryHandler := handler.NewDiscoveryHandler(app.discoveryService, app.logger)
	discovery := apiV1.Group("/discovery")
	{
		discovery.GET("/scan", discoveryHandler.ScanDevices)
		discovery.POST("/auto-setup", discoveryHandler.AutoSetupDevices)
		discovery.GET("/supported", discoveryHandler.GetSupportedDevices)
		discovery.GET("/capabilities/:brand/:type", discoveryHandler.GetCapabilities)
	}

	// WebSocket routes
	wsHandler := handler.NewWebSocketHandler(app.deviceService, app.operationService, app.logger)
	ws := router.Group("/ws")
	{
		ws.GET("/devices/:device_id", wsHandler.HandleDeviceConnection)
		ws.GET("/events", wsHandler.HandleEventConnection)
		ws.GET("/operations", wsHandler.HandleOperationConnection)
		ws.GET("/branches/:branch_id", wsHandler.HandleBranchConnection)
	}

	// Documentation routes
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	router.GET("/docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})

	app.logger.Info("Routes configured successfully")
}

// startBackgroundServices starts background services
func (app *Application) startBackgroundServices() {
	// Start device health monitoring
	go app.startDeviceHealthMonitoring()

	// Start offline operation sync
	go app.startOfflineSync()

	// Start cleanup service
	go app.startCleanupService()

	app.logger.Info("Background services started")
}

// startDeviceHealthMonitoring starts device health monitoring
func (app *Application) startDeviceHealthMonitoring() {
	ticker := time.NewTicker(app.config.Device.HealthCheckInterval)
	defer ticker.Stop()

	app.logger.Info("Device health monitoring started",
		zap.Duration("interval", app.config.Device.HealthCheckInterval),
	)

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		// Get all online devices
		devices, err := app.deviceRepo.ListByStatus(ctx, model.DeviceStatusOnline)
		if err != nil {
			app.logger.Error("Failed to get online devices for health check", zap.Error(err))
			cancel()
			continue
		}

		// Check health of each device
		for _, device := range devices {
			go app.checkDeviceHealth(ctx, device)
		}

		cancel()
	}
}

// checkDeviceHealth checks health of a single device
func (app *Application) checkDeviceHealth(ctx context.Context, device *model.Device) {
	// Create driver instance
	driverInstance, err := app.driverRegistry.CreateDriver(device, device.ConnectionConfig)
	if err != nil {
		app.logger.Error("Failed to create driver for health check",
			zap.Error(err),
			zap.String("device_id", device.DeviceID),
		)
		return
	}

	// Ping device
	startTime := time.Now()
	err = driverInstance.Ping(ctx)
	responseTime := time.Since(startTime)

	// Update device health
	if err != nil {
		// Device is not responding
		app.deviceRepo.UpdateStatus(ctx, device.ID, model.DeviceStatusError)
		app.logger.Warn("Device health check failed",
			zap.String("device_id", device.DeviceID),
			zap.Error(err),
		)
	} else {
		// Device is healthy
		app.deviceRepo.UpdateLastPing(ctx, device.ID, time.Now())

		// Create health log
		health := &model.DeviceHealth{
			DeviceID:    device.ID,
			HealthScore: 100,
			RecordedAt:  time.Now(),
		}

		responseTimeMs := int(responseTime.Milliseconds())
		health.ResponseTime = &responseTimeMs

		app.deviceRepo.CreateHealthLog(ctx, health)
	}
}

// startOfflineSync starts offline operation synchronization
func (app *Application) startOfflineSync() {
	ticker := time.NewTicker(app.config.Offline.SyncInterval)
	defer ticker.Stop()

	app.logger.Info("Offline sync started",
		zap.Duration("interval", app.config.Offline.SyncInterval),
	)

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

		// Get pending offline operations
		operations, err := app.offlineRepo.GetPendingOperations(ctx, app.config.Offline.RetryAttempts)
		if err != nil {
			app.logger.Error("Failed to get pending offline operations", zap.Error(err))
			cancel()
			continue
		}

		// Process each operation
		for _, operation := range operations {
			go app.syncOfflineOperation(ctx, operation)
		}

		cancel()
	}
}

// syncOfflineOperation syncs a single offline operation
func (app *Application) syncOfflineOperation(ctx context.Context, operation *model.OfflineOperation) {
	// Get device
	device, err := app.deviceRepo.GetByID(ctx, operation.DeviceID)
	if err != nil {
		app.logger.Error("Failed to get device for offline sync",
			zap.Error(err),
			zap.String("operation_id", operation.ID.String()),
		)
		return
	}

	// Check if device is online
	if device.Status != model.DeviceStatusOnline {
		return // Skip if device is not online
	}

	// Create operation request
	operationReq := &service.OperationRequest{
		DeviceID:      operation.DeviceID,
		OperationType: operation.OperationType,
		Data:          operation.OperationData,
		Priority:      operation.Priority,
	}

	// Execute operation
	_, err = app.operationService.ExecuteOperation(ctx, operationReq)
	if err != nil {
		// Mark as failed
		app.offlineRepo.MarkFailed(ctx, operation.ID, operation.SyncAttempts+1)
		app.logger.Error("Failed to sync offline operation",
			zap.Error(err),
			zap.String("operation_id", operation.ID.String()),
		)
	} else {
		// Mark as synced
		app.offlineRepo.MarkSynced(ctx, operation.ID)
		app.logger.Info("Offline operation synced successfully",
			zap.String("operation_id", operation.ID.String()),
		)
	}
}

// startCleanupService starts cleanup service for old records
func (app *Application) startCleanupService() {
	// Run cleanup every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	app.logger.Info("Cleanup service started")

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)

		// Cleanup old operations (30 days)
		oldDate := time.Now().AddDate(0, 0, -30)
		deletedOps, err := app.operationRepo.DeleteOldOperations(ctx, oldDate)
		if err != nil {
			app.logger.Error("Failed to cleanup old operations", zap.Error(err))
		} else if deletedOps > 0 {
			app.logger.Info("Cleaned up old operations", zap.Int64("deleted", deletedOps))
		}

		// Cleanup expired offline operations
		deletedOffline, err := app.offlineRepo.DeleteExpired(ctx)
		if err != nil {
			app.logger.Error("Failed to cleanup expired offline operations", zap.Error(err))
		} else if deletedOffline > 0 {
			app.logger.Info("Cleaned up expired offline operations", zap.Int64("deleted", deletedOffline))
		}

		cancel()
	}
}

// waitForShutdown waits for shutdown signal and performs graceful shutdown
func (app *Application) waitForShutdown() {
	// Create channel to receive OS signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	sig := <-quit
	app.logger.Info("Received shutdown signal", zap.String("signal", sig.String()))

	// Perform graceful shutdown
	app.shutdown()
}

// shutdown performs graceful shutdown
func (app *Application) shutdown() {
	serviceLogger := utils.NewServiceLogger(app.logger, "device-service")
	serviceLogger.LogServiceStop("shutdown signal received")

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.server.Shutdown(ctx); err != nil {
		app.logger.Error("HTTP server shutdown error", zap.Error(err))
	} else {
		app.logger.Info("HTTP server stopped")
	}

	// Close database connection
	if app.database != nil {
		if err := app.database.Close(); err != nil {
			app.logger.Error("Database close error", zap.Error(err))
		} else {
			app.logger.Info("Database connection closed")
		}
	}

	// Flush logger
	if err := utils.CloseLogger(app.logger); err != nil {
		fmt.Printf("Logger close error: %v\n", err)
	}

	app.logger.Info("Application shutdown completed")
}

func (app *Application) Start() error {
	// Start server in goroutine
	go func() {
		app.logger.Info("Starting HTTP server",
			zap.String("address", app.server.Addr),
		)

		var err error
		if app.config.Server.TLS.Enabled {
			err = app.server.ListenAndServeTLS(
				app.config.Server.TLS.CertFile,
				app.config.Server.TLS.KeyFile,
			)
		} else {
			err = app.server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			app.logger.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	// Start background services
	app.startBackgroundServices()

	// Wait for interrupt signal
	app.waitForShutdown()

	return nil
}
