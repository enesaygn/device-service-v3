// internal/service/discovery_service.go
package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"device-service/internal/config"
	"device-service/internal/discovery"
	"device-service/internal/discovery/tcp"
	"device-service/internal/discovery/usb"
	"device-service/internal/driver"
	"device-service/internal/model"
	"device-service/internal/repository"
	"device-service/internal/utils"
	"device-service/pkg/devicetypes"
)

// DiscoveryService handles device discovery operations - Now much cleaner!
type DiscoveryService struct {
	deviceRepo     repository.DeviceRepository
	driverRegistry *driver.Registry
	scannerManager *discovery.ScannerManager
	config         *config.Config
	logger         *utils.ServiceLogger
}

// NewDiscoveryService creates a new discovery service
func NewDiscoveryService(
	deviceRepo repository.DeviceRepository,
	driverRegistry *driver.Registry,
	config *config.Config,
	logger *zap.Logger,
) *DiscoveryService {
	serviceLogger := utils.NewServiceLogger(logger, "discovery-service")

	// Create scanner manager
	scannerManager := discovery.NewScannerManager(logger)

	// Register available scanners
	ds := &DiscoveryService{
		deviceRepo:     deviceRepo,
		driverRegistry: driverRegistry,
		scannerManager: scannerManager,
		config:         config,
		logger:         serviceLogger,
	}

	// Initialize scanners
	ds.initializeScanners()

	return ds
}

// initializeScanners registers all available scanners
func (ds *DiscoveryService) initializeScanners() {
	// Register USB scanner
	if usbScanner := usb.NewScanner(ds.logger.Logger, nil); usbScanner.IsAvailable() {
		ds.scannerManager.RegisterScanner(usbScanner)
	}

	//TODO: Register Serial scanner
	// if serialScanner := serial.NewScanner(ds.logger.Logger, nil); serialScanner.IsAvailable() {
	// 	ds.scannerManager.RegisterScanner(serialScanner)
	// }

	// Register TCP scanner
	if tcpScanner := tcp.NewScanner(ds.logger.Logger, nil); tcpScanner.IsAvailable() {
		ds.scannerManager.RegisterScanner(tcpScanner)
	}

	ds.logger.Info("Discovery scanners initialized",
		zap.Strings("available_scanners", ds.scannerManager.GetAvailableScanners()),
	)
}

// ScanDevices scans for available devices - Much simpler now!
func (ds *DiscoveryService) ScanDevices(ctx context.Context, req *ScanRequest) ([]*DiscoveredDevice, error) {
	ds.logger.Info("Starting device scan", zap.String("type", req.ScanType))

	var devices []*discovery.DiscoveredDevice
	var err error

	switch req.ScanType {
	case "all":
		devices, err = ds.scannerManager.ScanAll(ctx)
	case "serial", "usb", "tcp":
		devices, err = ds.scannerManager.ScanByType(ctx, req.ScanType)
	default:
		return nil, fmt.Errorf("unsupported scan type: %s", req.ScanType)
	}

	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	// Convert to service DTOs
	result := make([]*DiscoveredDevice, len(devices))
	for i, device := range devices {
		result[i] = ds.convertToServiceDTO(device)
	}

	ds.logger.Info("Device scan completed",
		zap.Int("devices_found", len(result)),
		zap.String("scan_type", req.ScanType),
	)

	return result, nil
}

// convertToServiceDTO converts discovery device to service DTO
func (ds *DiscoveryService) convertToServiceDTO(device *discovery.DiscoveredDevice) *DiscoveredDevice {
	return &DiscoveredDevice{
		ConnectionType: device.ConnectionType,
		ConnectionInfo: device.ConnectionInfo,
		Brand:          device.Brand,
		Model:          device.Model,
		DeviceType:     device.DeviceType,
		Capabilities:   device.Capabilities,
		Confidence:     device.Confidence,
		SerialNumber:   device.SerialNumber,
		Location:       device.Location,
	}
}

// scanSerialDevices scans for serial port devices
func (ds *DiscoveryService) scanSerialDevices(ctx context.Context) ([]*DiscoveredDevice, error) {
	// This would implement actual serial port scanning
	// For now, return mock data
	devices := []*DiscoveredDevice{
		{
			ConnectionType: model.ConnectionTypeSerial,
			ConnectionInfo: map[string]interface{}{
				"port":      "/dev/ttyUSB0",
				"baud_rate": 9600,
			},
			Brand:        model.BrandEpson,
			Model:        "TM-T88VI",
			DeviceType:   model.DeviceTypePrinter,
			Capabilities: []string{"PRINT", "CUT", "DRAWER"},
			Confidence:   0.8,
		},
	}

	return devices, nil
}

// scanUSBDevices scans for USB devices
func (ds *DiscoveryService) scanUSBDevices(ctx context.Context) ([]*DiscoveredDevice, error) {
	// USB device scanning implementation
	return []*DiscoveredDevice{}, nil
}

// scanTCPDevices scans for network devices
func (ds *DiscoveryService) scanTCPDevices(ctx context.Context) ([]*DiscoveredDevice, error) {
	// TCP/IP device scanning implementation
	return []*DiscoveredDevice{}, nil
}

// AutoSetupDevices automatically sets up discovered devices
func (ds *DiscoveryService) AutoSetupDevices(ctx context.Context, req *AutoSetupRequest) (*AutoSetupResult, error) {
	ds.logger.Info("Starting auto-setup process", zap.String("branch_id", req.BranchID))

	// Parse and validate branch ID
	branchID, err := uuid.Parse(req.BranchID)
	if err != nil {
		return nil, fmt.Errorf("invalid branch ID: %w", err)
	}

	// Scan for devices
	scanReq := &ScanRequest{
		ScanType: "all",
		Timeout:  "30s",
	}

	devices, err := ds.ScanDevices(ctx, scanReq)
	if err != nil {
		return nil, fmt.Errorf("device scan failed: %w", err)
	}

	result := &AutoSetupResult{
		TotalScanned:      len(devices),
		SuccessfullySetup: 0,
		Failed:            0,
		SetupDevices:      []*SetupDeviceResult{},
		Errors:            []string{},
	}

	// If no devices found, return early
	if len(devices) == 0 {
		ds.logger.Info("No devices found during auto-setup scan")
		return result, nil
	}

	// Setup each discovered device
	for i, device := range devices {
		// Generate unique device ID
		deviceID := fmt.Sprintf("AUTO_%s_%s_%d",
			string(device.Brand),
			string(device.DeviceType),
			i+1)

		setupResult := &SetupDeviceResult{
			DeviceID:       deviceID,
			ConnectionType: device.ConnectionType,
			Brand:          device.Brand,
			Model:          device.Model,
			Status:         "PENDING",
		}

		// Apply device filter if specified
		if !ds.shouldSetupDevice(device, req.DeviceFilter) {
			ds.logger.Debug("Device filtered out by device filter",
				zap.String("device_id", deviceID),
				zap.String("brand", string(device.Brand)),
				zap.String("model", device.Model),
			)
			continue
		}

		// Check if device already exists
		existingDevice, err := ds.deviceRepo.GetByDeviceID(ctx, deviceID)
		if err == nil && existingDevice != nil {
			setupResult.Status = "ALREADY_EXISTS"
			setupResult.Error = "Device already registered in system"
			result.SetupDevices = append(result.SetupDevices, setupResult)
			continue
		}

		// Create device registration request
		regReq := &RegisterDeviceRequest{
			DeviceID:         deviceID,
			DeviceType:       device.DeviceType,
			Brand:            device.Brand,
			Model:            device.Model,
			ConnectionType:   device.ConnectionType,
			ConnectionConfig: device.ConnectionInfo,
			BranchID:         branchID,
			Location:         nil, //TODO:
			UserID:           "auto-setup",
		}

		// Set firmware version if available
		if device.SerialNumber != "" {
			//TODO: regReq.FirmwareVersion = &device.SerialNumber
		}

		// Register device using DeviceService
		registeredDevice, err := ds.registerDeviceWithService(ctx, regReq)
		if err != nil {
			setupResult.Status = "FAILED"
			setupResult.Error = err.Error()
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("Device %s: %v", deviceID, err))

			ds.logger.Error("Failed to register device during auto-setup",
				zap.String("device_id", deviceID),
				zap.Error(err),
			)
		} else {
			setupResult.Status = "SUCCESS"
			result.SuccessfullySetup++

			ds.logger.Info("Device auto-setup completed successfully",
				zap.String("device_id", deviceID),
				zap.String("brand", string(device.Brand)),
				zap.String("model", device.Model),
				zap.Float64("confidence", device.Confidence),
			)

			// Auto-connect if requested
			if req.AutoConnect {
				if err := ds.autoConnectDevice(ctx, registeredDevice.DeviceID); err != nil {
					ds.logger.Warn("Auto-connect failed after registration",
						zap.String("device_id", deviceID),
						zap.Error(err),
					)
					// Don't fail the setup, just log the warning
				} else {
					ds.logger.Info("Device auto-connected successfully",
						zap.String("device_id", deviceID),
					)
				}
			}
		}

		result.SetupDevices = append(result.SetupDevices, setupResult)
	}

	ds.logger.Info("Auto-setup process completed",
		zap.Int("total_scanned", result.TotalScanned),
		zap.Int("successfully_setup", result.SuccessfullySetup),
		zap.Int("failed", result.Failed),
	)

	return result, nil
}

// shouldSetupDevice checks if device matches the filter criteria
func (ds *DiscoveryService) shouldSetupDevice(device *DiscoveredDevice, filter map[string]string) bool {
	if filter == nil {
		return true
	}

	// Check brand filter
	if brandFilter, exists := filter["brand"]; exists {
		if string(device.Brand) != brandFilter {
			return false
		}
	}

	// Check device type filter
	if typeFilter, exists := filter["device_type"]; exists {
		if string(device.DeviceType) != typeFilter {
			return false
		}
	}

	// Check minimum confidence filter
	if confidenceFilter, exists := filter["min_confidence"]; exists {
		if minConfidence, err := strconv.ParseFloat(confidenceFilter, 64); err == nil {
			if device.Confidence < minConfidence {
				return false
			}
		}
	}

	// Check connection type filter
	if connectionFilter, exists := filter["connection_type"]; exists {
		if string(device.ConnectionType) != connectionFilter {
			return false
		}
	}

	return true
}

// registerDeviceWithService registers device using DeviceService
// registerDeviceWithService registers device using proper service layer
func (ds *DiscoveryService) registerDeviceWithService(ctx context.Context, req *RegisterDeviceRequest) (*model.Device, error) {
	ds.logger.Info("Registering device through discovery service",
		zap.String("device_id", req.DeviceID),
		zap.String("brand", string(req.Brand)),
		zap.String("model", req.Model),
		zap.String("device_type", string(req.DeviceType)),
	)

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
		ErrorInfo:        model.JSONObject{},
		PerformanceMetrics: model.JSONObject{
			"total_operations": 0,
			"success_rate":     0.0,
			"error_count":      0,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save to database
	if err := ds.deviceRepo.Create(ctx, device); err != nil {
		ds.logger.Error("Failed to create device in discovery service",
			zap.Error(err),
			zap.String("device_id", req.DeviceID),
		)
		return nil, fmt.Errorf("failed to create device: %w", err)
	}

	// Log successful registration
	ds.logger.Info("Device registered successfully through discovery service",
		zap.String("device_id", device.DeviceID),
		zap.String("device_type", string(device.DeviceType)),
		zap.String("brand", string(device.Brand)),
		zap.String("model", device.Model),
		zap.String("connection_type", string(device.ConnectionType)),
		zap.String("user_id", req.UserID),
	)

	return device, nil
}

// validateRegisterRequest validates device registration request
func (ds *DiscoveryService) validateRegisterRequest(req *RegisterDeviceRequest) error {
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
	if req.UserID == "" {
		req.UserID = "auto-setup" // Default for discovery service
	}

	// Validate device type
	validTypes := []model.DeviceType{
		model.DeviceTypePOS,
		model.DeviceTypePrinter,
		model.DeviceTypeScanner,
		model.DeviceTypeCashRegister,
		model.DeviceTypeCashDrawer,
		model.DeviceTypeDisplay,
	}
	isValidType := false
	for _, validType := range validTypes {
		if req.DeviceType == validType {
			isValidType = true
			break
		}
	}
	if !isValidType {
		return fmt.Errorf("invalid device_type: %s", req.DeviceType)
	}

	// Validate brand
	validBrands := []model.DeviceBrand{
		model.BrandEpson,
		model.BrandStar,
		model.BrandIngenico,
		model.BrandPAX,
		model.BrandCitizen,
		model.BrandBixolon,
		model.BrandVerifone,
		model.BrandGeneric,
		model.BrandKodpos,
	}
	isValidBrand := false
	for _, validBrand := range validBrands {
		if req.Brand == validBrand {
			isValidBrand = true
			break
		}
	}
	if !isValidBrand {
		return fmt.Errorf("invalid brand: %s", req.Brand)
	}

	// Validate connection type
	validConnTypes := []model.ConnectionType{
		model.ConnectionTypeSerial,
		model.ConnectionTypeUSB,
		model.ConnectionTypeTCP,
		model.ConnectionTypeBluetooth,
	}
	isValidConnType := false
	for _, validConnType := range validConnTypes {
		if req.ConnectionType == validConnType {
			isValidConnType = true
			break
		}
	}
	if !isValidConnType {
		return fmt.Errorf("invalid connection_type: %s", req.ConnectionType)
	}

	return nil
}

// createDeviceFromDiscovered creates a RegisterDeviceRequest from discovered device
func (ds *DiscoveryService) createDeviceFromDiscovered(device *DiscoveredDevice, branchID uuid.UUID, index int) *RegisterDeviceRequest {
	// Generate unique device ID
	deviceID := fmt.Sprintf("AUTO_%s_%s_%d",
		string(device.Brand),
		string(device.DeviceType),
		index+1)

	// Set firmware version if serial number is available
	var firmwareVersion *string
	if device.SerialNumber != "" {
		firmwareVersion = &device.SerialNumber
	}

	// Set location if available
	var location *string
	if device.Location != "" {
		location = &device.Location
	}

	return &RegisterDeviceRequest{
		DeviceID:         deviceID,
		DeviceType:       device.DeviceType,
		Brand:            device.Brand,
		Model:            device.Model,
		FirmwareVersion:  firmwareVersion,
		ConnectionType:   device.ConnectionType,
		ConnectionConfig: device.ConnectionInfo,
		BranchID:         branchID,
		Location:         location,
		UserID:           "auto-setup",
	}
}

// Enhanced autoConnectDevice with better error handling and logging
func (ds *DiscoveryService) autoConnectDevice(ctx context.Context, deviceID string) error {
	ds.logger.Info("Attempting auto-connect for device", zap.String("device_id", deviceID))

	// Get device from database
	device, err := ds.deviceRepo.GetByDeviceID(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	// Create device logger for better tracking
	deviceLogger := utils.NewDeviceLogger(ds.logger.Logger, device.DeviceID, string(device.DeviceType), string(device.Brand))

	// Validate connection configuration
	if len(device.ConnectionConfig) == 0 {
		return fmt.Errorf("device has no connection configuration")
	}

	// Create driver instance
	driverInstance, err := ds.driverRegistry.CreateDriver(device, device.ConnectionConfig)
	if err != nil {
		deviceLogger.LogConnection("create_driver", false, err)
		return fmt.Errorf("failed to create driver: %w", err)
	}

	// Update status to connecting
	device.Status = model.DeviceStatusConnecting
	if err := ds.deviceRepo.UpdateStatus(ctx, device.ID, device.Status); err != nil {
		deviceLogger.Error("Failed to update device status to connecting", zap.Error(err))
	}

	// Attempt connection with timeout
	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	startTime := time.Now()
	if err := driverInstance.Connect(connectCtx); err != nil {
		// Update device with error status
		device.Status = model.DeviceStatusError
		device.ErrorInfo = model.JSONObject{
			"last_error":     err.Error(),
			"error_time":     time.Now(),
			"error_context":  "auto_connect",
			"critical_error": true,
		}

		if updateErr := ds.deviceRepo.Update(ctx, device); updateErr != nil {
			deviceLogger.Error("Failed to update device after connection error", zap.Error(updateErr))
		}

		deviceLogger.LogConnection("connect", false, err)
		return fmt.Errorf("failed to connect to device: %w", err)
	}

	// Test basic functionality
	if err := driverInstance.Ping(connectCtx); err != nil {
		deviceLogger.Warn("Device connected but ping failed", zap.Error(err))
		// Don't fail auto-connect for ping failure, just log it
	}

	// Update device status to online
	device.Status = model.DeviceStatusOnline
	device.LastPing = &[]time.Time{time.Now()}[0]
	device.ErrorInfo = model.JSONObject{} // Clear any previous errors
	device.PerformanceMetrics = model.JSONObject{
		"last_connect_time":    time.Now(),
		"connection_duration":  time.Since(startTime).Milliseconds(),
		"auto_connect_success": true,
	}

	if err := ds.deviceRepo.Update(ctx, device); err != nil {
		deviceLogger.Error("Failed to update device after successful connection", zap.Error(err))
		// Don't return error, connection was successful
	}

	deviceLogger.LogConnection("connect", true, nil)
	ds.logger.Info("Device auto-connected successfully",
		zap.String("device_id", deviceID),
		zap.Duration("connection_time", time.Since(startTime)),
	)

	// Close the driver connection for now - it will be reopened when needed
	go func() {
		time.Sleep(1 * time.Second) // Give it a moment
		if err := driverInstance.Disconnect(context.Background()); err != nil {
			deviceLogger.Warn("Failed to disconnect after auto-connect test", zap.Error(err))
		}
	}()

	return nil
}

// getDeviceCapabilities returns capabilities for device type and brand
func (ds *DiscoveryService) getDeviceCapabilities(deviceType model.DeviceType, brand model.DeviceBrand) model.JSONArray {
	capabilities := []interface{}{}

	switch deviceType {
	case model.DeviceTypePrinter:
		capabilities = append(capabilities, "PRINT", "STATUS")
		if brand == model.BrandEpson || brand == model.BrandStar || brand == model.BrandKodpos {
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

// GetSupportedDevices returns list of supported devices
func (ds *DiscoveryService) GetSupportedDevices() *SupportedDevicesResponse {
	drivers := ds.driverRegistry.ListDrivers()

	deviceMap := make(map[string]map[string][]string)

	for _, driverKey := range drivers {
		brandStr := string(driverKey.Brand)
		typeStr := string(driverKey.DeviceType)

		if deviceMap[brandStr] == nil {
			deviceMap[brandStr] = make(map[string][]string)
		}

		if deviceMap[brandStr][typeStr] == nil {
			deviceMap[brandStr][typeStr] = []string{}
		}

		if driverKey.Model != "*" {
			deviceMap[brandStr][typeStr] = append(deviceMap[brandStr][typeStr], driverKey.Model)
		}
	}

	return &SupportedDevicesResponse{
		TotalBrands:  len(deviceMap),
		Devices:      deviceMap,
		Capabilities: devicetypes.DeviceCapabilities,
	}
}

// GetDeviceCapabilities returns capabilities for a specific device
func (ds *DiscoveryService) GetDeviceCapabilities(brand, deviceType string) ([]string, error) {
	if caps, exists := devicetypes.DeviceCapabilities[deviceType]; exists {
		return caps, nil
	}

	return nil, fmt.Errorf("device type not supported: %s", deviceType)
}

// DTOs for Discovery Service

// ScanRequest represents device scan request
type ScanRequest struct {
	ScanType string `json:"scan_type"` // all, serial, usb, tcp, bluetooth
	Timeout  string `json:"timeout"`
}

// DiscoveredDevice represents a discovered device
type DiscoveredDevice struct {
	ConnectionType model.ConnectionType   `json:"connection_type"`
	ConnectionInfo map[string]interface{} `json:"connection_info"`
	Brand          model.DeviceBrand      `json:"brand"`
	Model          string                 `json:"model"`
	DeviceType     model.DeviceType       `json:"device_type"`
	Capabilities   []string               `json:"capabilities"`
	Confidence     float64                `json:"confidence"` // 0.0-1.0
	SerialNumber   string                 `json:"serial_number,omitempty"`
	Location       string                 `json:"location,omitempty"`
}

// AutoSetupRequest represents auto-setup request
type AutoSetupRequest struct {
	BranchID     string            `json:"branch_id"`
	DeviceFilter map[string]string `json:"device_filter,omitempty"`
	AutoConnect  bool              `json:"auto_connect"`
}

// AutoSetupResult represents auto-setup result
type AutoSetupResult struct {
	TotalScanned      int                  `json:"total_scanned"`
	SuccessfullySetup int                  `json:"successfully_setup"`
	Failed            int                  `json:"failed"`
	SetupDevices      []*SetupDeviceResult `json:"setup_devices"`
	Errors            []string             `json:"errors,omitempty"`
}

// SetupDeviceResult represents individual device setup result
type SetupDeviceResult struct {
	DeviceID       string               `json:"device_id"`
	ConnectionType model.ConnectionType `json:"connection_type"`
	Brand          model.DeviceBrand    `json:"brand"`
	Model          string               `json:"model"`
	Status         string               `json:"status"` // SUCCESS, FAILED
	Error          string               `json:"error,omitempty"`
}

// SupportedDevicesResponse represents supported devices response
type SupportedDevicesResponse struct {
	TotalBrands  int                            `json:"total_brands"`
	Devices      map[string]map[string][]string `json:"devices"`
	Capabilities map[string][]string            `json:"capabilities"`
}
