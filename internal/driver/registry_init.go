// internal/driver/registry_init.go
package driver

import (
	"go.uber.org/zap"

	"device-service/internal/driver/epson"
	"device-service/internal/model"
	// ✅ pkg'den import
)

// RegisterDefaultDrivers registers all default device drivers
func RegisterDefaultDrivers(registry *Registry, logger *zap.Logger) {
	// Register EPSON printer drivers
	registerEPSONDrivers(registry, logger)

	// Register other brand drivers here
	// registerSTARDrivers(registry, logger)
	// registerINGENICODrivers(registry, logger)
	// registerPAXDrivers(registry, logger)
}

// registerEPSONDrivers registers EPSON printer drivers
func registerEPSONDrivers(registry *Registry, logger *zap.Logger) {
	// EPSON TM-T88VI
	registry.Register(
		model.BrandEpson,
		model.DeviceTypePrinter,
		"TM-T88VI",
		epson.NewEPSONDriver,
	)

	// EPSON TM-T88V
	registry.Register(
		model.BrandEpson,
		model.DeviceTypePrinter,
		"TM-T88V",
		epson.NewEPSONDriver,
	)

	// EPSON TM-T20III
	registry.Register(
		model.BrandEpson,
		model.DeviceTypePrinter,
		"TM-T20III",
		epson.NewEPSONDriver,
	)

	// EPSON TM-T82III
	registry.Register(
		model.BrandEpson,
		model.DeviceTypePrinter,
		"TM-T82III",
		epson.NewEPSONDriver,
	)

	// EPSON TM-M30
	registry.Register(
		model.BrandEpson,
		model.DeviceTypePrinter,
		"TM-M30",
		epson.NewEPSONDriver,
	)

	// Generic EPSON printer (wildcard)
	registry.Register(
		model.BrandEpson,
		model.DeviceTypePrinter,
		"*",
		epson.NewEPSONDriver,
	)

	logger.Info("EPSON printer drivers registered",
		zap.Int("models", 6),
	)
}
