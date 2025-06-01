// internal/service/discovery_service.go
package service

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"device-service/internal/config"
	"device-service/internal/driver"
	"device-service/internal/model"
	"device-service/internal/repository"
	"device-service/internal/utils"
	"device-service/pkg/devicetypes"
)

// DiscoveryService handles device discovery operations
type DiscoveryService struct {
	deviceRepo     repository.DeviceRepository
	driverRegistry *driver.Registry
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
	return &DiscoveryService{
		deviceRepo:     deviceRepo,
		driverRegistry: driverRegistry,
		config:         config,
		logger:         utils.NewServiceLogger(logger, "discovery-service"),
	}
}

// ScanDevices scans for available devices
func (ds *DiscoveryService) ScanDevices(ctx context.Context, req *ScanRequest) ([]*DiscoveredDevice, error) {
	ds.logger.Info("Starting device scan", zap.String("type", req.ScanType))

	var devices []*DiscoveredDevice

	// Doğru
	switch req.ScanType {
	case "all":
		// Tüm tipları tara
		serialDevices, _ := ds.scanSerialDevices(ctx)
		usbDevices, _ := ds.scanUSBDevices(ctx)
		tcpDevices, _ := ds.scanTCPDevices(ctx)
		devices = append(devices, serialDevices...)
		devices = append(devices, usbDevices...)
		devices = append(devices, tcpDevices...)
	case "serial":
		devices, _ = ds.scanSerialDevices(ctx)
	case "usb":
		devices, _ = ds.scanUSBDevices(ctx)
	case "tcp":
		devices, _ = ds.scanTCPDevices(ctx)
	}

	ds.logger.Info("Device scan completed",
		zap.Int("devices_found", len(devices)),
		zap.String("scan_type", req.ScanType),
	)

	return devices, nil
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

	//TODO:
	// branchID, err := uuid.Parse(req.BranchID)
	// if err != nil {
	// 	return nil, fmt.Errorf("invalid branch ID: %w", err)
	// }

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

	// Setup each discovered device
	for i, device := range devices {
		deviceID := fmt.Sprintf("AUTO_%s_%d", device.DeviceType, i+1)

		setupResult := &SetupDeviceResult{
			DeviceID:       deviceID,
			ConnectionType: device.ConnectionType,
			Brand:          device.Brand,
			Model:          device.Model,
			Status:         "SUCCESS",
		}

		// Create device registration request
		// TODO:
		// regReq := &RegisterDeviceRequest{
		// 	DeviceID:         deviceID,
		// 	DeviceType:       device.DeviceType,
		// 	Brand:            device.Brand,
		// 	Model:            device.Model,
		// 	ConnectionType:   device.ConnectionType,
		// 	ConnectionConfig: device.ConnectionInfo,
		// 	BranchID:         branchID,
		// 	UserID:           "auto-setup",
		// }

		// Register device
		// Note: This would typically use DeviceService
		// For now, just mark as successful
		result.SuccessfullySetup++
		result.SetupDevices = append(result.SetupDevices, setupResult)

		ds.logger.Info("Device auto-setup completed",
			zap.String("device_id", deviceID),
			zap.String("brand", string(device.Brand)),
			zap.String("model", device.Model),
		)
	}

	return result, nil
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
