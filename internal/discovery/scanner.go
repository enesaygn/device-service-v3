// 📁 internal/discovery/scanner.go - Main Scanner Interface
package discovery

import (
	"context"
	"device-service/internal/model"
	"fmt"

	"go.uber.org/zap"
)

// DeviceScanner interface - Strategy Pattern
type DeviceScanner interface {
	Scan(ctx context.Context) ([]*DiscoveredDevice, error)
	GetScannerType() string
	IsAvailable() bool
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

// ScannerManager manages all device scanners - Facade Pattern
type ScannerManager struct {
	scanners map[string]DeviceScanner
	logger   *zap.Logger
}

// NewScannerManager creates a new scanner manager
func NewScannerManager(logger *zap.Logger) *ScannerManager {
	return &ScannerManager{
		scanners: make(map[string]DeviceScanner),
		logger:   logger,
	}
}

// RegisterScanner registers a device scanner
func (sm *ScannerManager) RegisterScanner(scanner DeviceScanner) {
	scannerType := scanner.GetScannerType()
	sm.scanners[scannerType] = scanner
	sm.logger.Info("Scanner registered", zap.String("type", scannerType))
}

// ScanAll scans all registered scanner types
func (sm *ScannerManager) ScanAll(ctx context.Context) ([]*DiscoveredDevice, error) {
	var allDevices []*DiscoveredDevice

	for scannerType, scanner := range sm.scanners {
		if !scanner.IsAvailable() {
			sm.logger.Debug("Scanner not available, skipping", zap.String("type", scannerType))
			continue
		}

		devices, err := scanner.Scan(ctx)
		if err != nil {
			sm.logger.Error("Scanner failed", zap.String("type", scannerType), zap.Error(err))
			continue
		}

		allDevices = append(allDevices, devices...)
		sm.logger.Info("Scanner completed",
			zap.String("type", scannerType),
			zap.Int("devices_found", len(devices)),
		)
	}

	return allDevices, nil
}

// ScanByType scans specific scanner type
func (sm *ScannerManager) ScanByType(ctx context.Context, scannerType string) ([]*DiscoveredDevice, error) {
	scanner, exists := sm.scanners[scannerType]
	if !exists {
		return nil, fmt.Errorf("scanner type not found: %s", scannerType)
	}

	if !scanner.IsAvailable() {
		return nil, fmt.Errorf("scanner not available: %s", scannerType)
	}

	return scanner.Scan(ctx)
}

// GetAvailableScanners returns list of available scanner types
func (sm *ScannerManager) GetAvailableScanners() []string {
	var available []string
	for scannerType, scanner := range sm.scanners {
		if scanner.IsAvailable() {
			available = append(available, scannerType)
		}
	}
	return available
}
