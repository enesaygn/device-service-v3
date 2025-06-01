// internal/service/device_service.go
package service

import (
	"context"
	"fmt"
	"time"

	"device-service/internal/config"
	internalDriver "device-service/internal/driver" // Registry için
	"device-service/internal/model"
	"device-service/internal/repository"
	"device-service/internal/utils"
	"device-service/pkg/driver" // DeviceDriver interface için

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// DeviceService handles device management business logic
type DeviceService struct {
	deviceRepo     repository.DeviceRepository
	operationRepo  repository.OperationRepository
	driverRegistry *internalDriver.Registry
	config         *config.Config
	logger         *utils.ServiceLogger
	auditLogger    *utils.AuditLogger
}

// NewDeviceService creates a new device service instance
func NewDeviceService(
	deviceRepo repository.DeviceRepository,
	operationRepo repository.OperationRepository,
	driverRegistry *internalDriver.Registry,
	config *config.Config,
	logger *zap.Logger,
) *DeviceService {
	return &DeviceService{
		deviceRepo:     deviceRepo,
		operationRepo:  operationRepo,
		driverRegistry: driverRegistry,
		config:         config,
		logger:         utils.NewServiceLogger(logger, "device-service"),
		auditLogger:    utils.NewAuditLogger(logger),
	}
}

// RegisterDevice registers a new device in the system
func (ds *DeviceService) RegisterDevice(ctx context.Context, req *RegisterDeviceRequest) (*model.Device, error) {
	// Validate request
	if err := ds.validateRegisterRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if device already exists
	existing, err := ds.deviceRepo.GetByDeviceID(ctx, req.DeviceID)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("device with ID %s already exists", req.DeviceID)
	}

	// Verify driver support
	if !ds.driverRegistry.IsSupported(req.Brand, req.DeviceType, req.Model) {
		return nil, fmt.Errorf("unsupported device: %s %s %s", req.Brand, req.DeviceType, req.Model)
	}

	// Create device model
	device := &model.Device{
		ID:               uuid.New(),
		DeviceID:         req.DeviceID,
		DeviceType:       req.DeviceType,
		Brand:            req.Brand,
		Model:            req.Model,
		FirmwareVersion:  req.FirmwareVersion,
		ConnectionType:   req.ConnectionType,
		ConnectionConfig: model.JSONObject(req.ConnectionConfig),
		Capabilities:     ds.getDeviceCapabilities(req.DeviceType, req.Brand),
		BranchID:         req.BranchID,
		Location:         req.Location,
		Status:           model.DeviceStatusOffline,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Save to database
	if err := ds.deviceRepo.Create(ctx, device); err != nil {
		ds.logger.Error("Failed to create device", zap.Error(err))
		return nil, fmt.Errorf("failed to create device: %w", err)
	}

	// Audit log
	ds.auditLogger.LogDeviceRegistration(
		device.DeviceID,
		string(device.DeviceType),
		string(device.Brand),
		req.UserID,
		true,
	)

	ds.logger.Info("Device registered successfully",
		zap.String("device_id", device.DeviceID),
		zap.String("device_type", string(device.DeviceType)),
		zap.String("brand", string(device.Brand)),
	)

	return device, nil
}

// ConnectDevice attempts to connect to a device
func (ds *DeviceService) ConnectDevice(ctx context.Context, deviceID string) error {
	// Get device from database
	device, err := ds.deviceRepo.GetByDeviceID(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	// Create device logger
	deviceLogger := utils.NewDeviceLogger(ds.logger.Logger, device.DeviceID, string(device.DeviceType), string(device.Brand))

	// Update status to connecting
	device.Status = model.DeviceStatusConnecting
	if err := ds.deviceRepo.UpdateStatus(ctx, device.ID, device.Status); err != nil {
		deviceLogger.Error("Failed to update device status", zap.Error(err))
	}

	// Create driver
	driverInstance, err := ds.driverRegistry.CreateDriver(device, device.ConnectionConfig)
	if err != nil {
		deviceLogger.LogConnection("create_driver", false, err)
		ds.updateDeviceError(ctx, device, err)
		return fmt.Errorf("failed to create driver: %w", err)
	}

	// Attempt connection
	connectCtx, cancel := context.WithTimeout(ctx, time.Duration(ds.config.Device.OperationTimeout))
	defer cancel()

	if err := driverInstance.Connect(connectCtx); err != nil {
		deviceLogger.LogConnection("connect", false, err)
		ds.updateDeviceError(ctx, device, err)
		return fmt.Errorf("failed to connect to device: %w", err)
	}

	// Update device status
	device.Status = model.DeviceStatusOnline
	device.LastPing = &[]time.Time{time.Now()}[0]
	device.ErrorInfo = model.JSONObject{}

	if err := ds.deviceRepo.Update(ctx, device); err != nil {
		deviceLogger.Error("Failed to update device after connection", zap.Error(err))
	}

	deviceLogger.LogConnection("connect", true, nil)

	// Start health monitoring
	go ds.startHealthMonitoring(device, driverInstance)

	return nil
}

// DisconnectDevice disconnects a device
func (ds *DeviceService) DisconnectDevice(ctx context.Context, deviceID string) error {
	device, err := ds.deviceRepo.GetByDeviceID(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	deviceLogger := utils.NewDeviceLogger(ds.logger.Logger, device.DeviceID, string(device.DeviceType), string(device.Brand))

	// Update status
	device.Status = model.DeviceStatusOffline
	if err := ds.deviceRepo.UpdateStatus(ctx, device.ID, device.Status); err != nil {
		deviceLogger.Error("Failed to update device status", zap.Error(err))
	}

	deviceLogger.LogConnection("disconnect", true, nil)
	return nil
}

// GetDevice retrieves device information
func (ds *DeviceService) GetDevice(ctx context.Context, deviceID string) (*model.Device, error) {
	device, err := ds.deviceRepo.GetByDeviceID(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}
	return device, nil
}

// ListDevices retrieves devices with filtering
func (ds *DeviceService) ListDevices(ctx context.Context, filter *DeviceFilter) ([]*model.Device, *PaginationResult, error) {
	devices, total, err := ds.deviceRepo.List(ctx, filter.toRepoFilter())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list devices: %w", err)
	}

	pagination := &PaginationResult{
		Total:      total,
		Page:       filter.Page,
		PerPage:    filter.PerPage,
		TotalPages: (total + filter.PerPage - 1) / filter.PerPage,
	}

	return devices, pagination, nil
}

// UpdateDeviceConfiguration updates device configuration
func (ds *DeviceService) UpdateDeviceConfiguration(ctx context.Context, deviceID string, config map[string]interface{}, userID string) error {
	device, err := ds.deviceRepo.GetByDeviceID(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	oldConfig := device.ConnectionConfig
	device.ConnectionConfig = model.JSONObject(config)
	device.UpdatedAt = time.Now()

	if err := ds.deviceRepo.Update(ctx, device); err != nil {
		return fmt.Errorf("failed to update device configuration: %w", err)
	}

	// Audit log
	ds.auditLogger.LogDeviceConfiguration(deviceID, userID, oldConfig, config)

	ds.logger.Info("Device configuration updated",
		zap.String("device_id", deviceID),
		zap.String("user_id", userID),
	)

	return nil
}

// DeleteDevice removes a device from the system
func (ds *DeviceService) DeleteDevice(ctx context.Context, deviceID string, userID string) error {
	device, err := ds.deviceRepo.GetByDeviceID(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	// Check if device is online
	if device.Status == model.DeviceStatusOnline {
		return fmt.Errorf("cannot delete online device, disconnect first")
	}

	if err := ds.deviceRepo.Delete(ctx, device.ID); err != nil {
		return fmt.Errorf("failed to delete device: %w", err)
	}

	ds.logger.Info("Device deleted",
		zap.String("device_id", deviceID),
		zap.String("user_id", userID),
	)

	return nil
}

// GetDeviceHealth retrieves device health metrics
func (ds *DeviceService) GetDeviceHealth(ctx context.Context, deviceID string) (*DeviceHealth, error) {
	device, err := ds.deviceRepo.GetByDeviceID(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	// Get latest health metrics
	healthLogs, err := ds.deviceRepo.GetHealthLogs(ctx, device.ID, 1)
	if err != nil || len(healthLogs) == 0 {
		return &DeviceHealth{
			DeviceID:    deviceID,
			HealthScore: 0,
			Status:      string(device.Status),
			LastCheck:   device.LastPing,
		}, nil
	}

	latestHealth := healthLogs[0]
	return &DeviceHealth{
		DeviceID:     deviceID,
		HealthScore:  latestHealth.HealthScore,
		Status:       string(device.Status),
		LastCheck:    device.LastPing,
		ResponseTime: latestHealth.ResponseTime,
		ErrorRate:    latestHealth.ErrorRate,
		Uptime:       latestHealth.Uptime,
		//TODO: Metrics:      latestHealth.Metrics,
	}, nil
}

// TestDevice performs a device connectivity test
func (ds *DeviceService) TestDevice(ctx context.Context, deviceID string) (*TestResult, error) {
	device, err := ds.deviceRepo.GetByDeviceID(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	deviceLogger := utils.NewDeviceLogger(ds.logger.Logger, device.DeviceID, string(device.DeviceType), string(device.Brand))

	startTime := time.Now()

	// Create driver
	driverInstance, err := ds.driverRegistry.CreateDriver(device, device.ConnectionConfig)
	if err != nil {
		return &TestResult{
			Success:      false,
			ErrorMessage: err.Error(),
			Duration:     time.Since(startTime).String(),
		}, nil
	}

	// Test connection
	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := driverInstance.Connect(testCtx); err != nil {
		deviceLogger.LogConnection("test", false, err)
		return &TestResult{
			Success:      false,
			ErrorMessage: err.Error(),
			Duration:     time.Since(startTime).String(),
		}, nil
	}
	defer driverInstance.Disconnect(testCtx)

	// Test ping
	if err := driverInstance.Ping(testCtx); err != nil {
		return &TestResult{
			Success:      false,
			ErrorMessage: err.Error(),
			Duration:     time.Since(startTime).String(),
		}, nil
	}

	// Get device info
	deviceInfo, err := driverInstance.GetDeviceInfo()
	if err != nil {
		deviceLogger.Warn("Failed to get device info during test", zap.Error(err))
	}

	deviceLogger.LogConnection("test", true, nil)

	return &TestResult{
		Success:    true,
		Duration:   time.Since(startTime).String(),
		DeviceInfo: deviceInfo,
	}, nil
}

// Helper methods

// validateRegisterRequest validates device registration request
func (ds *DeviceService) validateRegisterRequest(req *RegisterDeviceRequest) error {
	if req.DeviceID == "" {
		return fmt.Errorf("device_id is required")
	}
	if req.DeviceType == "" {
		return fmt.Errorf("device_type is required")
	}
	if req.Brand == "" {
		return fmt.Errorf("brand is required")
	}
	if req.Model == "" {
		return fmt.Errorf("model is required")
	}
	if req.ConnectionType == "" {
		return fmt.Errorf("connection_type is required")
	}
	if req.BranchID == uuid.Nil {
		return fmt.Errorf("branch_id is required")
	}
	if req.ConnectionConfig == nil {
		return fmt.Errorf("connection_config is required")
	}
	return nil
}

// getDeviceCapabilities returns capabilities for device type and brand
func (ds *DeviceService) getDeviceCapabilities(deviceType model.DeviceType, brand model.DeviceBrand) model.JSONArray {
	// Base capabilities from device type
	capabilities := []interface{}{}

	switch deviceType {
	case model.DeviceTypePrinter:
		capabilities = append(capabilities, "PRINT", "STATUS")
		if brand == model.BrandEpson || brand == model.BrandStar {
			capabilities = append(capabilities, "CUT", "DRAWER", "BEEP")
		}
	case model.DeviceTypePOS:
		capabilities = append(capabilities, "PAYMENT", "DISPLAY", "STATUS")
	case model.DeviceTypeScanner:
		capabilities = append(capabilities, "SCAN", "BEEP", "STATUS")
	case model.DeviceTypeCashDrawer:
		capabilities = append(capabilities, "DRAWER", "STATUS")
	case model.DeviceTypeDisplay:
		capabilities = append(capabilities, "DISPLAY", "STATUS")
	}

	return model.JSONArray(capabilities)
}

// updateDeviceError updates device with error information
func (ds *DeviceService) updateDeviceError(ctx context.Context, device *model.Device, err error) {
	device.Status = model.DeviceStatusError
	device.ErrorInfo = model.JSONObject{
		"last_error":     err.Error(),
		"error_time":     time.Now(),
		"error_count":    1,
		"critical_error": true,
	}

	if updateErr := ds.deviceRepo.Update(ctx, device); updateErr != nil {
		ds.logger.Error("Failed to update device error", zap.Error(updateErr))
	}
}

// startHealthMonitoring starts health monitoring for a device
func (ds *DeviceService) startHealthMonitoring(device *model.Device, driverInstance driver.DeviceDriver) {
	deviceLogger := utils.NewDeviceLogger(ds.logger.Logger, device.DeviceID, string(device.DeviceType), string(device.Brand))

	ticker := time.NewTicker(ds.config.Device.HealthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		startTime := time.Now()
		err := driverInstance.Ping(ctx)
		responseTime := time.Since(startTime)

		if err != nil {
			deviceLogger.Warn("Device ping failed", zap.Error(err))
			// Handle error and potentially reconnect
		} else {
			// Log health metrics
			deviceLogger.LogHealth(90, responseTime, 0.0) // Simplified health calculation
		}

		cancel()
	}
}

// Data Transfer Objects

// RegisterDeviceRequest represents device registration request
type RegisterDeviceRequest struct {
	DeviceID         string                 `json:"device_id"`
	DeviceType       model.DeviceType       `json:"device_type"`
	Brand            model.DeviceBrand      `json:"brand"`
	Model            string                 `json:"model"`
	FirmwareVersion  *string                `json:"firmware_version,omitempty"`
	ConnectionType   model.ConnectionType   `json:"connection_type"`
	ConnectionConfig map[string]interface{} `json:"connection_config"`
	BranchID         uuid.UUID              `json:"branch_id"`
	Location         *string                `json:"location,omitempty"`
	UserID           string                 `json:"user_id"`
}

// DeviceFilter represents device listing filters
type DeviceFilter struct {
	BranchID   *uuid.UUID          `json:"branch_id,omitempty"`
	DeviceType *model.DeviceType   `json:"device_type,omitempty"`
	Brand      *model.DeviceBrand  `json:"brand,omitempty"`
	Status     *model.DeviceStatus `json:"status,omitempty"`
	Location   *string             `json:"location,omitempty"`
	Page       int                 `json:"page"`
	PerPage    int                 `json:"per_page"`
	SortBy     string              `json:"sort_by"`
	SortOrder  string              `json:"sort_order"`
}

// toRepoFilter converts to repository filter
func (df *DeviceFilter) toRepoFilter() *repository.DeviceFilter {
	// Implementation would convert DTO to repository filter
	return &repository.DeviceFilter{
		BranchID:   df.BranchID,
		DeviceType: df.DeviceType,
		Brand:      df.Brand,
		Status:     df.Status,
		Location:   df.Location,
		Page:       df.Page,
		PerPage:    df.PerPage,
		SortBy:     df.SortBy,
		SortOrder:  df.SortOrder,
	}
}

// PaginationResult represents pagination information
type PaginationResult struct {
	Total      int `json:"total"`
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalPages int `json:"total_pages"`
}

// DeviceHealth represents device health information
type DeviceHealth struct {
	DeviceID     string                 `json:"device_id"`
	HealthScore  int                    `json:"health_score"`
	Status       string                 `json:"status"`
	LastCheck    *time.Time             `json:"last_check,omitempty"`
	ResponseTime *int                   `json:"response_time,omitempty"`
	ErrorRate    *float64               `json:"error_rate,omitempty"`
	Uptime       *float64               `json:"uptime,omitempty"`
	Metrics      map[string]interface{} `json:"metrics,omitempty"`
}

// TestResult represents device test result
type TestResult struct {
	Success      bool               `json:"success"`
	Duration     string             `json:"duration"`
	ErrorMessage string             `json:"error_message,omitempty"`
	DeviceInfo   *driver.DeviceInfo `json:"device_info,omitempty"`
}
