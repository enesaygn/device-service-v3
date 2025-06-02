// internal/driver/registry.go
package driver

import (
	"fmt"
	"sync"

	"go.uber.org/zap"

	"device-service/internal/model"
	"device-service/pkg/driver" // ✅ Artık pkg'den import
)

// DriverFactory creates device drivers - ✅ Return type düzeltildi
type DriverFactory func(device *model.Device, connectionConfig interface{}, logger *zap.Logger) (driver.DeviceDriver, error)

// Registry manages device driver registration and creation
type Registry struct {
	drivers map[DriverKey]DriverFactory
	mu      sync.RWMutex
	logger  *zap.Logger
}

// DriverKey uniquely identifies a driver
type DriverKey struct {
	Brand      model.DeviceBrand
	DeviceType model.DeviceType
	Model      string
}

// NewRegistry creates a new driver registry
func NewRegistry(logger *zap.Logger) *Registry {
	return &Registry{
		drivers: make(map[DriverKey]DriverFactory),
		logger:  logger,
	}
}

// Register registers a driver factory
func (r *Registry) Register(brand model.DeviceBrand, deviceType model.DeviceType, model string, factory DriverFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := DriverKey{
		Brand:      brand,
		DeviceType: deviceType,
		Model:      model,
	}

	r.drivers[key] = factory
	r.logger.Info("Driver registered",
		zap.String("brand", string(brand)),
		zap.String("device_type", string(deviceType)),
		zap.String("model", model),
	)
}

// CreateDriver creates a driver instance
func (r *Registry) CreateDriver(device *model.Device, connectionConfig interface{}) (driver.DeviceDriver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try exact match first
	key := DriverKey{
		Brand:      device.Brand,
		DeviceType: device.DeviceType,
		Model:      device.Model,
	}

	if factory, exists := r.drivers[key]; exists {
		// ✅ FIXED: Pass both device and connectionConfig
		return factory(device, connectionConfig, r.logger)
	}

	// Try brand + device type match (any model)
	key.Model = "*"
	if factory, exists := r.drivers[key]; exists {
		return factory(device, connectionConfig, r.logger)
	}

	// Try generic driver
	key.Brand = model.BrandGeneric
	if factory, exists := r.drivers[key]; exists {
		return factory(device, connectionConfig, r.logger)
	}

	return nil, fmt.Errorf("no driver found for brand=%s, type=%s, model=%s",
		device.Brand, device.DeviceType, device.Model)
}

// ListDrivers returns all registered drivers
func (r *Registry) ListDrivers() []DriverKey {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := make([]DriverKey, 0, len(r.drivers))
	for key := range r.drivers {
		keys = append(keys, key)
	}
	return keys
}

// IsSupported checks if a device is supported
func (r *Registry) IsSupported(brand model.DeviceBrand, deviceType model.DeviceType, deviceModel string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check exact match
	key := DriverKey{Brand: brand, DeviceType: deviceType, Model: deviceModel}
	if _, exists := r.drivers[key]; exists {
		return true
	}

	// Check wildcard match
	key.Model = "*"
	if _, exists := r.drivers[key]; exists {
		return true
	}

	// Check generic driver
	key.Brand = model.BrandGeneric
	if _, exists := r.drivers[key]; exists {
		return true
	}

	return false
}

// GetSupportedBrands returns all supported brands for a device type
func (r *Registry) GetSupportedBrands(deviceType model.DeviceType) []model.DeviceBrand {
	r.mu.RLock()
	defer r.mu.RUnlock()

	brandSet := make(map[model.DeviceBrand]bool)
	for key := range r.drivers {
		if key.DeviceType == deviceType {
			brandSet[key.Brand] = true
		}
	}

	brands := make([]model.DeviceBrand, 0, len(brandSet))
	for brand := range brandSet {
		brands = append(brands, brand)
	}
	return brands
}
