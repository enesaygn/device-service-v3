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

	"go.uber.org/zap"

	_ "device-service/docs"
	"device-service/internal/config"
	"device-service/internal/database"
	"device-service/internal/driver"
	"device-service/internal/model"
	"device-service/internal/repository"
	"device-service/internal/routes"
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
// cmd/server/main.go - initializeServer method'unu güncelle

func (app *Application) initializeServer() error {
	// Create router
	routerManager := routes.NewRouter(
		app.config,
		app.logger,
		app.database,
		app.deviceService,
		app.operationService,
		app.discoveryService,
	)

	// Setup router with all routes
	router := routerManager.SetupRouter()

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
