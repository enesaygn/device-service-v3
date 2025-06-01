// internal/protocol/usb_connection.go
package protocol

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/gousb"
	"go.uber.org/zap"

	"device-service/internal/model"
)

// USBConnection implements DeviceProtocol for USB connections
type USBConnection struct {
	config   *USBConfig
	ctx      *gousb.Context
	device   *gousb.Device
	intf     *gousb.Interface
	outEndpt *gousb.OutEndpoint
	inEndpt  *gousb.InEndpoint
	logger   *zap.Logger
	mutex    sync.RWMutex
	isOpen   bool
	stats    *ProtocolStats
}

// NewUSBConnection creates a new USB connection
func NewUSBConnection(config *USBConfig, logger *zap.Logger) DeviceProtocol {
	return &USBConnection{
		config: config,
		logger: logger.With(
			zap.String("protocol", "usb"),
			zap.String("vendor_id", config.VendorID),
			zap.String("product_id", config.ProductID),
		),
		stats: &ProtocolStats{
			IsConnected: false,
		},
	}
}

// Open opens the USB connection
func (uc *USBConnection) Open(ctx context.Context) error {
	uc.mutex.Lock()
	defer uc.mutex.Unlock()

	if uc.isOpen {
		return nil
	}

	uc.logger.Info("Opening USB connection",
		zap.String("vendor_id", uc.config.VendorID),
		zap.String("product_id", uc.config.ProductID),
		zap.Int("interface", uc.config.Interface),
	)

	// Parse vendor and product IDs
	vendorID, err := uc.parseHexID(uc.config.VendorID)
	if err != nil {
		return fmt.Errorf("invalid vendor ID: %w", err)
	}

	productID, err := uc.parseHexID(uc.config.ProductID)
	if err != nil {
		return fmt.Errorf("invalid product ID: %w", err)
	}

	// Initialize USB context
	uc.ctx = gousb.NewContext()

	// Find and open device
	device, err := uc.findAndOpenDevice(vendorID, productID)
	if err != nil {
		uc.ctx.Close()
		return fmt.Errorf("failed to find USB device: %w", err)
	}

	// Claim interface
	intf, done, err := device.DefaultInterface()
	if err != nil {
		device.Close()
		uc.ctx.Close()
		return fmt.Errorf("failed to claim interface: %w", err)
	}

	// Find endpoints
	outEndpt, err := intf.OutEndpoint(uc.config.Endpoint)
	if err != nil {
		done()
		device.Close()
		uc.ctx.Close()
		return fmt.Errorf("failed to get out endpoint: %w", err)
	}

	inEndpt, err := intf.InEndpoint(uc.config.Endpoint)
	if err != nil {
		// Some devices might not have in endpoint, that's ok
		uc.logger.Warn("No in endpoint found", zap.Error(err))
	}

	uc.device = device
	uc.intf = intf
	uc.outEndpt = outEndpt
	uc.inEndpt = inEndpt
	uc.isOpen = true
	uc.stats.IsConnected = true
	uc.stats.LastActivity = time.Now()

	uc.logger.Info("USB connection opened successfully")
	return nil
}

// Close closes the USB connection
func (uc *USBConnection) Close() error {
	uc.mutex.Lock()
	defer uc.mutex.Unlock()

	if !uc.isOpen {
		return nil
	}

	if uc.intf != nil {
		uc.intf.Close()
		uc.intf = nil
	}

	if uc.device != nil {
		uc.device.Close()
		uc.device = nil
	}

	if uc.ctx != nil {
		uc.ctx.Close()
		uc.ctx = nil
	}

	uc.outEndpt = nil
	uc.inEndpt = nil
	uc.isOpen = false
	uc.stats.IsConnected = false

	uc.logger.Info("USB connection closed successfully")
	return nil
}

// IsOpen returns whether the connection is open
func (uc *USBConnection) IsOpen() bool {
	uc.mutex.RLock()
	defer uc.mutex.RUnlock()
	return uc.isOpen && uc.device != nil && uc.outEndpt != nil
}

// Write writes data to the USB connection
func (uc *USBConnection) Write(ctx context.Context, data []byte) error {
	uc.mutex.RLock()
	defer uc.mutex.RUnlock()

	if !uc.isOpen || uc.outEndpt == nil {
		return fmt.Errorf("USB connection not open")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	startTime := time.Now()
	n, err := uc.outEndpt.Write(data)
	if err != nil {
		uc.stats.ErrorCount++
		uc.logger.Error("USB write failed", zap.Error(err))
		return fmt.Errorf("failed to write to USB device: %w", err)
	}

	if n != len(data) {
		return fmt.Errorf("incomplete write: wrote %d of %d bytes", n, len(data))
	}

	// Update statistics
	duration := time.Since(startTime)
	uc.stats.BytesWritten += int64(len(data))
	uc.stats.OperationCount++
	uc.stats.LastActivity = time.Now()
	uc.updateAverageLatency(duration)

	uc.logger.Debug("USB write completed", zap.Int("bytes", len(data)))
	return nil
}

// Read reads data from the USB connection
func (uc *USBConnection) Read(ctx context.Context, maxBytes int) ([]byte, error) {
	uc.mutex.RLock()
	defer uc.mutex.RUnlock()

	if !uc.isOpen || uc.inEndpt == nil {
		return nil, fmt.Errorf("USB connection not open or no in endpoint")
	}

	buffer := make([]byte, maxBytes)

	done := make(chan struct {
		data []byte
		err  error
	}, 1)

	go func() {
		n, err := uc.inEndpt.Read(buffer)
		result := struct {
			data []byte
			err  error
		}{}

		if err != nil {
			result.err = fmt.Errorf("failed to read from USB device: %w", err)
		} else {
			result.data = make([]byte, n)
			copy(result.data, buffer[:n])
		}
		done <- result
	}()

	select {
	case result := <-done:
		if result.err != nil {
			uc.stats.ErrorCount++
			return nil, result.err
		}

		uc.stats.BytesRead += int64(len(result.data))
		uc.stats.OperationCount++
		uc.stats.LastActivity = time.Now()

		return result.data, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetProtocolType returns the protocol type
func (uc *USBConnection) GetProtocolType() model.ConnectionType {
	return model.ConnectionTypeUSB
}

// Ping tests the connection
func (uc *USBConnection) Ping(ctx context.Context) error {
	if !uc.IsOpen() {
		return fmt.Errorf("USB connection not open")
	}

	// Simple ping with status request
	pingData := []byte{0x10, 0x04, 0x01}
	return uc.Write(ctx, pingData)
}

// Helper methods

// parseHexID parses hex ID string (0x1234 or 1234)
func (uc *USBConnection) parseHexID(hexStr string) (gousb.ID, error) {
	// Remove 0x prefix if present
	if len(hexStr) > 2 && hexStr[:2] == "0x" {
		hexStr = hexStr[2:]
	}

	id, err := strconv.ParseUint(hexStr, 16, 16)
	if err != nil {
		return 0, err
	}

	return gousb.ID(id), nil
}

// findAndOpenDevice finds and opens the USB device
func (uc *USBConnection) findAndOpenDevice(vendorID, productID gousb.ID) (*gousb.Device, error) {
	// Find device by vendor and product ID
	devices, err := uc.ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		if desc.Vendor != vendorID || desc.Product != productID {
			return false
		}

		// If serial number is specified, match it
		if uc.config.SerialNumber != "" {
			// This would require opening the device to get serial number
			// For now, just match by VID/PID
		}

		return true
	})

	if err != nil {
		return nil, fmt.Errorf("failed to enumerate USB devices: %w", err)
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("USB device not found (VID: %04X, PID: %04X)", vendorID, productID)
	}

	if len(devices) > 1 {
		// Close extra devices
		for i := 1; i < len(devices); i++ {
			devices[i].Close()
		}
		uc.logger.Warn("Multiple matching USB devices found, using first one")
	}

	return devices[0], nil
}

// updateAverageLatency updates the running average latency
func (uc *USBConnection) updateAverageLatency(newLatency time.Duration) {
	if uc.stats.AverageLatency == 0 {
		uc.stats.AverageLatency = newLatency
	} else {
		uc.stats.AverageLatency = (uc.stats.AverageLatency + newLatency) / 2
	}
}
