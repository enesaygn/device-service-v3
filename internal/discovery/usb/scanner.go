// 📁 internal/discovery/usb/scanner.go - Complete USB Scanner Implementation
package usb

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/gousb"
	"go.uber.org/zap"

	"device-service/internal/discovery"
	"device-service/internal/model"
)

// Scanner struct'ından ÖNCE, dosyanın başına ekle
type deviceResult struct {
	device *discovery.DiscoveredDevice
	error  error
}

// Scanner implements USB device scanning
type Scanner struct {
	logger       *zap.Logger
	knownDevices *DeviceDatabase
	timeout      time.Duration
	config       *Config
}

// Config for USB scanner
type Config struct {
	ScanTimeout    time.Duration `json:"scan_timeout"`
	EnableDebug    bool          `json:"enable_debug"`
	SkipPermCheck  bool          `json:"skip_permission_check"`
	FilterByClass  bool          `json:"filter_by_class"`
	TestConnection bool          `json:"test_connection"`
	MaxConcurrent  int           `json:"max_concurrent"`
}

const (
	USBClassHID        = 3
	USBClassPrinter    = 7
	USBClassVendorSpec = 255
)

// NewScanner creates a new USB scanner
func NewScanner(logger *zap.Logger, config *Config) *Scanner {
	if config == nil {
		config = &Config{
			ScanTimeout:    10 * time.Second,
			EnableDebug:    false,
			FilterByClass:  true,
			TestConnection: false, // USB connection test can be risky
			MaxConcurrent:  5,
		}
	}

	return &Scanner{
		logger:       logger.With(zap.String("scanner", "usb")),
		knownDevices: NewDeviceDatabase(),
		timeout:      config.ScanTimeout,
		config:       config,
	}
}

// GetScannerType returns scanner type identifier
func (s *Scanner) GetScannerType() string {
	return "usb"
}

// IsAvailable checks if USB scanning is available on this system
func (s *Scanner) IsAvailable() bool {
	switch runtime.GOOS {
	case "windows":
		return s.checkWindowsUSBAccess()
	case "linux":
		return s.checkLinuxUSBAccess()
	case "darwin": // macOS
		return s.checkMacOSUSBAccess()
	default:
		s.logger.Warn("USB scanning support unknown for OS", zap.String("os", runtime.GOOS))
		return false
	}
}

// Scan performs USB device discovery
func (s *Scanner) Scan(ctx context.Context) ([]*discovery.DiscoveredDevice, error) {
	startTime := time.Now()
	s.logger.Info("Starting USB device scan")

	// Create scan context with timeout
	scanCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// Pre-scan checks
	if err := s.preScanChecks(); err != nil {
		return nil, fmt.Errorf("pre-scan checks failed: %w", err)
	}

	// Initialize USB context
	usbCtx := gousb.NewContext()
	defer func() {
		if err := usbCtx.Close(); err != nil {
			s.logger.Warn("Failed to close USB context", zap.Error(err))
		}
	}()

	// Configure USB context
	s.configureUSBContext(usbCtx)

	// Enumerate and process devices
	discovered, err := s.enumerateAndProcessDevices(scanCtx, usbCtx)
	if err != nil {
		return nil, fmt.Errorf("device enumeration failed: %w", err)
	}

	// Post-process discovered devices
	processed := s.postProcessDevices(discovered)

	s.logger.Info("USB scan completed",
		zap.Int("devices_found", len(processed)),
		zap.Duration("scan_duration", time.Since(startTime)),
	)

	return processed, nil
}

// preScanChecks performs pre-scan validation
func (s *Scanner) preScanChecks() error {
	// Check if USB subsystem is accessible
	testCtx := gousb.NewContext()
	defer testCtx.Close()

	// Try to get device list - this will fail if no permissions
	_, err := testCtx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		return false // Don't actually open anything, just test access
	})

	if err != nil {
		s.logger.Error("USB subsystem access test failed", zap.Error(err))
		return fmt.Errorf("USB subsystem not accessible: %w", err)
	}

	return nil
}

// configureUSBContext sets up USB context with appropriate settings
func (s *Scanner) configureUSBContext(ctx *gousb.Context) {
	// Set debug level
	debugLevel := 0
	if s.config.EnableDebug {
		debugLevel = 3
	}
	ctx.Debug(debugLevel)

	s.logger.Debug("USB context configured",
		zap.Int("debug_level", debugLevel),
		zap.Bool("filter_by_class", s.config.FilterByClass),
	)
}

// enumerateAndProcessDevices handles the main enumeration and processing loop
func (s *Scanner) enumerateAndProcessDevices(ctx context.Context, usbCtx *gousb.Context) ([]*discovery.DiscoveredDevice, error) {
	// Enumerate devices with filter
	devices, err := usbCtx.OpenDevices(s.createDeviceFilter())
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate USB devices: %w", err)
	}
	defer s.closeAllDevices(devices)

	s.logger.Info("Found USB devices to examine", zap.Int("device_count", len(devices)))

	// Process devices with concurrency control
	return s.processDevicesConcurrently(ctx, devices)
}

// createDeviceFilter returns a device filter function
func (s *Scanner) createDeviceFilter() func(*gousb.DeviceDesc) bool {
	return func(desc *gousb.DeviceDesc) bool {
		return s.shouldExamineDevice(desc)
	}
}

// shouldExamineDevice determines if a device should be examined
func (s *Scanner) shouldExamineDevice(desc *gousb.DeviceDesc) bool {
	// Check if vendor is in our known devices database
	if s.knownDevices.IsKnownVendor(desc.Vendor) {
		s.logger.Debug("Found known vendor device",
			zap.String("vendor_id", fmt.Sprintf("0x%04X", desc.Vendor)),
			zap.String("product_id", fmt.Sprintf("0x%04X", desc.Product)),
		)
		return true
	}

	// Check for device classes that might be POS devices
	if s.config.FilterByClass {
		if s.isPotentialPOSDevice(desc) {
			s.logger.Debug("Found potential POS device by class",
				zap.String("vendor_id", fmt.Sprintf("0x%04X", desc.Vendor)),
				zap.String("product_id", fmt.Sprintf("0x%04X", desc.Product)),
				zap.String("class", desc.Class.String()),
			)
			return true
		}
	}

	return false
}

// isPotentialPOSDevice checks if device could be a POS device based on USB class
func (s *Scanner) isPotentialPOSDevice(desc *gousb.DeviceDesc) bool {
	switch desc.Class {
	case USBClassPrinter:
		return true
	case USBClassHID:
		// HID devices might include barcode scanners
		return true
	case USBClassVendorSpec:
		// Vendor-specific devices might be POS equipment
		return true
	default:
		return false
	}
}

// processDevicesConcurrently processes devices with controlled concurrency
func (s *Scanner) processDevicesConcurrently(ctx context.Context, devices []*gousb.Device) ([]*discovery.DiscoveredDevice, error) {
	if len(devices) == 0 {
		return []*discovery.DiscoveredDevice{}, nil
	}

	// Create worker pool for concurrent processing
	type deviceResult struct {
		device *discovery.DiscoveredDevice
		error  error
	}

	maxWorkers := s.config.MaxConcurrent
	if maxWorkers <= 0 {
		maxWorkers = 5
	}

	deviceChan := make(chan *gousb.Device, len(devices))
	resultChan := make(chan deviceResult, len(devices))

	// Start workers
	for i := 0; i < maxWorkers; i++ {
		go s.deviceWorker(ctx, deviceChan, resultChan)
	}

	// Send devices to workers
	for _, device := range devices {
		select {
		case deviceChan <- device:
		case <-ctx.Done():
			close(deviceChan)
			return nil, ctx.Err()
		}
	}
	close(deviceChan)

	// Collect results
	var discovered []*discovery.DiscoveredDevice
	for i := 0; i < len(devices); i++ {
		select {
		case result := <-resultChan:
			if result.error != nil {
				s.logger.Warn("Device processing failed", zap.Error(result.error))
			} else if result.device != nil {
				discovered = append(discovered, result.device)
			}
		case <-ctx.Done():
			return discovered, ctx.Err()
		}
	}

	return discovered, nil
}

// deviceWorker processes devices in worker pool
func (s *Scanner) deviceWorker(ctx context.Context, deviceChan <-chan *gousb.Device, resultChan chan<- deviceResult) {
	for {
		select {
		case device, ok := <-deviceChan:
			if !ok {
				return
			}

			discoveredDevice := s.processDevice(device)
			resultChan <- deviceResult{
				device: discoveredDevice,
				error:  nil,
			}

		case <-ctx.Done():
			return
		}
	}
}

// processDevice examines a single USB device and creates DiscoveredDevice if applicable
func (s *Scanner) processDevice(device *gousb.Device) *discovery.DiscoveredDevice {
	desc := device.Desc
	if desc == nil {
		s.logger.Warn("Failed to get device descriptor", zap.Error(errors.New("device descriptor is nil")))
		return nil
	}

	s.logger.Debug("Processing USB device",
		zap.String("vendor_id", fmt.Sprintf("0x%04X", desc.Vendor)),
		zap.String("product_id", fmt.Sprintf("0x%04X", desc.Product)),
		zap.String("class", desc.Class.String()),
	)

	// Try to identify the device using our known devices database
	if knownDevice := s.identifyKnownDevice(desc, device); knownDevice != nil {
		return knownDevice
	}

	// Try to identify as generic device based on class/interface
	if genericDevice := s.identifyGenericDevice(desc, device); genericDevice != nil {
		return genericDevice
	}

	s.logger.Debug("Device not identified as POS equipment",
		zap.String("vendor_id", fmt.Sprintf("0x%04X", desc.Vendor)),
		zap.String("product_id", fmt.Sprintf("0x%04X", desc.Product)),
	)

	return nil
}

// identifyKnownDevice checks if device matches our known devices database
func (s *Scanner) identifyKnownDevice(desc *gousb.DeviceDesc, device *gousb.Device) *discovery.DiscoveredDevice {
	// Look up vendor information
	vendorInfo := s.knownDevices.GetVendorInfo(desc.Vendor)
	if vendorInfo == nil {
		return nil
	}

	// Look up product information
	productInfo := vendorInfo.GetProductInfo(desc.Product)
	if productInfo == nil {
		// Known vendor but unknown product - create generic entry with lower confidence
		return &discovery.DiscoveredDevice{
			ConnectionType: model.ConnectionTypeUSB,
			ConnectionInfo: s.createUSBConnectionInfo(desc),
			Brand:          vendorInfo.Brand,
			Model:          fmt.Sprintf("Unknown-%04X", desc.Product),
			DeviceType:     model.DeviceTypePrinter, // Assume printer for known vendors
			Capabilities:   []string{"PRINT", "STATUS"},
			Confidence:     0.5, // Lower confidence for unknown products
			SerialNumber:   s.getSerialNumber(device),
			Location:       s.createLocationString(desc),
		}
	}

	// Known vendor and product - high confidence
	return &discovery.DiscoveredDevice{
		ConnectionType: model.ConnectionTypeUSB,
		ConnectionInfo: s.createUSBConnectionInfo(desc),
		Brand:          vendorInfo.Brand,
		Model:          productInfo.Model,
		DeviceType:     productInfo.DeviceType,
		Capabilities:   productInfo.Capabilities,
		Confidence:     productInfo.Confidence,
		SerialNumber:   s.getSerialNumber(device),
		Location:       s.createLocationString(desc),
	}
}

// identifyGenericDevice attempts to identify device by USB class/interface
func (s *Scanner) identifyGenericDevice(desc *gousb.DeviceDesc, device *gousb.Device) *discovery.DiscoveredDevice {
	var deviceType model.DeviceType
	var capabilities []string
	var confidence float64

	switch desc.Class {
	case USBClassPrinter:
		deviceType = model.DeviceTypePrinter
		capabilities = []string{"PRINT"}
		confidence = 0.4
	case USBClassHID:
		// HID might be a barcode scanner
		deviceType = model.DeviceTypeScanner
		capabilities = []string{"SCAN"}
		confidence = 0.3
	default:
		return nil
	}

	manufacturer, err := device.Manufacturer()
	if err != nil {
		panic(err)
	}
	product, err := device.Product()
	if err != nil {
		panic(err)
	}

	return &discovery.DiscoveredDevice{
		ConnectionType: model.ConnectionTypeUSB,
		ConnectionInfo: s.createUSBConnectionInfo(desc),
		Brand:          model.BrandGeneric,
		Model:          s.createGenericModelName(manufacturer, product, desc),
		DeviceType:     deviceType,
		Capabilities:   capabilities,
		Confidence:     confidence,
		SerialNumber:   s.getSerialNumber(device),
		Location:       s.createLocationString(desc),
	}
}

// createUSBConnectionInfo creates connection configuration for USB device
func (s *Scanner) createUSBConnectionInfo(desc *gousb.DeviceDesc) map[string]interface{} {
	return map[string]interface{}{
		"vendor_id":      fmt.Sprintf("0x%04X", desc.Vendor),
		"product_id":     fmt.Sprintf("0x%04X", desc.Product),
		"bus":            desc.Bus,
		"address":        desc.Address,
		"device_version": fmt.Sprintf("%d.%02d", desc.Device>>8, desc.Device&0xFF),
		"usb_version":    fmt.Sprintf("%d.%02d", desc.Spec>>8, desc.Spec&0xFF),
		"class":          desc.Class.String(),
		"sub_class":      desc.SubClass,
		"protocol":       desc.Protocol,
		"interface":      0,    // Default interface
		"timeout":        5000, // 5 second timeout in ms
		//"max_packet_size": desc.MaxPacketSize0,
	}
}

// getSerialNumber attempts to retrieve device serial number
func (s *Scanner) getSerialNumber(device *gousb.Device) string {
	// Try to get actual serial number from device
	serialNumber, err := device.SerialNumber()
	serialNumberInt, err := strconv.Atoi(serialNumber)
	if err != nil {
		panic(err)
	}
	if err != nil {
		panic(err)
	}
	if serialNumber != "0" {
		if serial := s.getStringDescriptor(device, serialNumberInt); serial != "" {
			return serial
		}
	}

	// Fallback to synthetic serial number
	return fmt.Sprintf("USB-%04X%04X-%d", device.Desc.Vendor, device.Desc.Product, device.Desc.Address)
}

// getStringDescriptor safely retrieves a string descriptor from device
func (s *Scanner) getStringDescriptor(device *gousb.Device, index int) string {
	if index == 0 {
		return ""
	}

	// Try to get string descriptor with error handling
	str, err := device.GetStringDescriptor(index)
	if err != nil {
		s.logger.Debug("Failed to get string descriptor",
			zap.Int("index", index),
			zap.Error(err),
		)
		return ""
	}

	return strings.TrimSpace(str)
}

// createGenericModelName creates a model name for generic devices
func (s *Scanner) createGenericModelName(manufacturer, product string, desc *gousb.DeviceDesc) string {
	// Priority order: product name, manufacturer name, VID:PID
	if product != "" && product != "Unknown" {
		return fmt.Sprintf("Generic-%s", product)
	}

	if manufacturer != "" && manufacturer != "Unknown" {
		return fmt.Sprintf("Generic-%s-%04X", manufacturer, desc.Product)
	}

	return fmt.Sprintf("Generic-USB-%04X:%04X", desc.Vendor, desc.Product)
}

// createLocationString creates a location identifier for the device
func (s *Scanner) createLocationString(desc *gousb.DeviceDesc) string {
	return fmt.Sprintf("USB-Bus%d-Port%d", desc.Bus, desc.Address)
}

// closeAllDevices safely closes all opened USB devices
func (s *Scanner) closeAllDevices(devices []*gousb.Device) {
	for i, device := range devices {
		if device != nil {
			if err := device.Close(); err != nil {
				s.logger.Warn("Failed to close USB device",
					zap.Int("device_index", i),
					zap.Error(err),
				)
			}
		}
	}
}

// postProcessDevices performs final processing on discovered devices
func (s *Scanner) postProcessDevices(devices []*discovery.DiscoveredDevice) []*discovery.DiscoveredDevice {
	// Remove duplicates (same vendor_id + product_id + serial)
	seen := make(map[string]bool)
	var unique []*discovery.DiscoveredDevice

	for _, device := range devices {
		key := s.createDeviceKey(device)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, device)
		} else {
			s.logger.Debug("Removing duplicate device", zap.String("key", key))
		}
	}

	// Sort by confidence score (highest first)
	for i := 0; i < len(unique)-1; i++ {
		for j := i + 1; j < len(unique); j++ {
			if unique[i].Confidence < unique[j].Confidence {
				unique[i], unique[j] = unique[j], unique[i]
			}
		}
	}

	return unique
}

// createDeviceKey creates a unique key for device deduplication
func (s *Scanner) createDeviceKey(device *discovery.DiscoveredDevice) string {
	vendorID := device.ConnectionInfo["vendor_id"].(string)
	productID := device.ConnectionInfo["product_id"].(string)
	return fmt.Sprintf("%s:%s:%s", vendorID, productID, device.SerialNumber)
}

// Platform-specific availability checks

// checkWindowsUSBAccess checks USB access on Windows
func (s *Scanner) checkWindowsUSBAccess() bool {
	// On Windows, USB access is generally available
	// May require driver installation for some devices
	return true
}

// checkLinuxUSBAccess checks USB access on Linux
func (s *Scanner) checkLinuxUSBAccess() bool {
	// On Linux, check if user has access to USB devices
	// This typically requires being in the 'plugdev' group or running as root

	// Simple test: try to create USB context
	testCtx := gousb.NewContext()
	defer testCtx.Close()

	// If we can create context, USB access is likely available
	return true
}

// checkMacOSUSBAccess checks USB access on macOS
func (s *Scanner) checkMacOSUSBAccess() bool {
	if s.config.SkipPermCheck {
		return true
	}

	// On macOS, USB access may require special entitlements or running as root
	// For now, we'll assume it's available but log a warning
	s.logger.Warn("USB scanning on macOS may require additional permissions")
	return true
}
