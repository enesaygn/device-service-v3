// internal/driver/epson/epson_driver.go
package epson

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"device-service/internal/model"
	"device-service/internal/protocol"
	"device-service/internal/utils"
	"device-service/pkg/driver"
)

// EPSONDriver implements driver.DeviceDriver and driver.PrinterDriver for EPSON printers
type EPSONDriver struct {
	config        *EPSONConfig
	protocol      protocol.DeviceProtocol
	logger        *utils.DeviceLogger
	eventHandler  driver.EventHandler
	isConnected   bool
	lastPing      time.Time
	healthMetrics *driver.HealthMetrics
	mutex         sync.RWMutex
	deviceInfo    *driver.DeviceInfo
}

// EPSONConfig represents EPSON printer configuration
type EPSONConfig struct {
	DeviceID         string                 `json:"device_id"`
	Model            string                 `json:"model"`
	ConnectionType   model.ConnectionType   `json:"connection_type"`
	ConnectionConfig map[string]interface{} `json:"connection_config"`
	PaperWidth       int                    `json:"paper_width"`
	CharacterSet     string                 `json:"character_set"`
	CutType          string                 `json:"cut_type"`
	DrawerPin        int                    `json:"drawer_pin"`
	EnableDrawer     bool                   `json:"enable_drawer"`
	EnableCutter     bool                   `json:"enable_cutter"`
	LogoEnabled      bool                   `json:"logo_enabled"`
	Options          map[string]interface{} `json:"options"`
}

// NewEPSONDriver creates a new EPSON printer driver
func NewEPSONDriver(config interface{}, logger *zap.Logger) (driver.DeviceDriver, error) {
	epsonConfig, err := parseEPSONConfig(config)
	if err != nil {
		return nil, fmt.Errorf("invalid EPSON configuration: %w", err)
	}

	deviceLogger := utils.NewDeviceLogger(logger, epsonConfig.DeviceID, "PRINTER", "EPSON")

	return &EPSONDriver{
		config: epsonConfig,
		logger: deviceLogger,
		healthMetrics: &driver.HealthMetrics{
			HealthScore: 0,
		},
		deviceInfo: &driver.DeviceInfo{
			Brand:          model.BrandEpson,
			Model:          epsonConfig.Model,
			ConnectionType: epsonConfig.ConnectionType,
			Capabilities:   getEPSONCapabilities(epsonConfig),
			Manufacturer:   "Seiko Epson Corporation",
		},
	}, nil
}

// Connect establishes connection to EPSON printer
func (d *EPSONDriver) Connect(ctx context.Context) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.isConnected {
		return nil
	}

	startTime := time.Now()

	// Create protocol based on connection type
	protocolInstance, err := protocol.CreateProtocol(
		d.config.ConnectionType,
		d.config.ConnectionConfig,
		d.logger.Logger,
	)
	if err != nil {
		d.updateHealthMetrics(false, time.Since(startTime), err)
		return fmt.Errorf("failed to create %s protocol: %w", d.config.ConnectionType, err)
	}

	// Open protocol connection
	if err := protocolInstance.Open(ctx); err != nil {
		d.updateHealthMetrics(false, time.Since(startTime), err)
		return fmt.Errorf("failed to open %s connection: %w", d.config.ConnectionType, err)
	}

	d.protocol = protocolInstance
	d.isConnected = true
	d.lastPing = time.Now()

	// Initialize printer
	if err := d.initializePrinter(ctx); err != nil {
		d.protocol.Close()
		d.isConnected = false
		d.updateHealthMetrics(false, time.Since(startTime), err)
		return fmt.Errorf("failed to initialize printer: %w", err)
	}

	d.updateHealthMetrics(true, time.Since(startTime), nil)
	d.notifyEvent("connected", nil)

	d.logger.Info("EPSON printer connected successfully",
		zap.String("connection_type", string(d.config.ConnectionType)),
		zap.String("model", d.config.Model),
	)

	return nil
}

// Disconnect closes connection to EPSON printer
func (d *EPSONDriver) Disconnect(ctx context.Context) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if !d.isConnected {
		return nil
	}

	if d.protocol != nil {
		if err := d.protocol.Close(); err != nil {
			d.logger.Error("Failed to close protocol", zap.Error(err))
		}
		d.protocol = nil
	}

	d.isConnected = false
	d.notifyEvent("disconnected", "manual disconnect")

	d.logger.Info("EPSON printer disconnected")
	return nil
}

// IsConnected returns connection status
func (d *EPSONDriver) IsConnected() bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.isConnected && d.protocol != nil && d.protocol.IsOpen()
}

// GetDeviceInfo returns device information
func (d *EPSONDriver) GetDeviceInfo() (*driver.DeviceInfo, error) {
	return d.deviceInfo, nil
}

// GetCapabilities returns device capabilities
func (d *EPSONDriver) GetCapabilities() []model.Capability {
	return getEPSONCapabilities(d.config)
}

// GetStatus returns current device status
func (d *EPSONDriver) GetStatus() (*driver.DeviceStatus, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	if !d.isConnected {
		return &driver.DeviceStatus{
			Status:       model.DeviceStatusOffline,
			IsReady:      false,
			HasError:     false,
			LastResponse: d.lastPing,
		}, nil
	}

	return &driver.DeviceStatus{
		Status:       model.DeviceStatusOnline,
		IsReady:      true,
		HasError:     false,
		LastResponse: d.lastPing,
	}, nil
}

// ExecuteOperation executes a device operation
func (d *EPSONDriver) ExecuteOperation(ctx context.Context, operation *model.DeviceOperation) (*driver.OperationResult, error) {
	startTime := time.Now()

	if !d.IsConnected() {
		return nil, fmt.Errorf("device not connected")
	}

	var result *driver.OperationResult
	var err error

	switch operation.OperationType {
	case model.OperationTypePrint:
		result, err = d.handlePrintOperation(ctx, operation)
	case model.OperationTypeCut:
		result, err = d.handleCutOperation(ctx, operation)
	case model.OperationTypeOpenDrawer:
		result, err = d.handleDrawerOperation(ctx, operation)
	case model.OperationTypeStatusCheck:
		result, err = d.handleStatusOperation(ctx, operation)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation.OperationType)
	}

	duration := time.Since(startTime)

	if err != nil {
		d.updateHealthMetrics(false, duration, err)
		return nil, err
	}

	d.updateHealthMetrics(true, duration, nil)
	result.Duration = duration.String()
	result.Timestamp = time.Now()

	return result, nil
}

// Ping tests device connectivity
func (d *EPSONDriver) Ping(ctx context.Context) error {
	if !d.IsConnected() {
		return fmt.Errorf("device not connected")
	}

	startTime := time.Now()
	err := d.protocol.Ping(ctx)

	if err != nil {
		d.updateHealthMetrics(false, time.Since(startTime), err)
		return fmt.Errorf("ping failed: %w", err)
	}

	d.lastPing = time.Now()
	d.updateHealthMetrics(true, time.Since(startTime), nil)
	return nil
}

// GetHealthMetrics returns health metrics
func (d *EPSONDriver) GetHealthMetrics() (*driver.HealthMetrics, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	metrics := *d.healthMetrics
	return &metrics, nil
}

// Configure updates device configuration
func (d *EPSONDriver) Configure(config interface{}) error {
	newConfig, err := parseEPSONConfig(config)
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.config = newConfig
	d.deviceInfo.Capabilities = getEPSONCapabilities(newConfig)

	d.logger.Info("EPSON printer reconfigured")
	return nil
}

// Reset resets the device
func (d *EPSONDriver) Reset(ctx context.Context) error {
	if !d.IsConnected() {
		return fmt.Errorf("device not connected")
	}

	if err := d.sendCommands(ctx, [][]byte{ESC_POS_COMMANDS.INITIALIZE}); err != nil {
		return fmt.Errorf("failed to reset printer: %w", err)
	}

	d.logger.Info("EPSON printer reset")
	return nil
}

// SetEventHandler sets event handler
func (d *EPSONDriver) SetEventHandler(handler driver.EventHandler) {
	d.eventHandler = handler
}

// Close cleans up resources
func (d *EPSONDriver) Close() error {
	return d.Disconnect(context.Background())
}

// Helper methods

// sendCommands sends commands to printer using protocol
func (d *EPSONDriver) sendCommands(ctx context.Context, commands [][]byte) error {
	if d.protocol == nil {
		return fmt.Errorf("no protocol connection")
	}

	for _, cmd := range commands {
		if err := d.protocol.Write(ctx, cmd); err != nil {
			return fmt.Errorf("failed to send command: %w", err)
		}
	}

	return nil
}

// readResponse reads response from printer using protocol
func (d *EPSONDriver) readResponse(ctx context.Context, timeout time.Duration) ([]byte, error) {
	if d.protocol == nil {
		return nil, fmt.Errorf("no protocol connection")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return d.protocol.Read(ctx, 1024)
}

// initializePrinter initializes the printer
func (d *EPSONDriver) initializePrinter(ctx context.Context) error {
	commands := [][]byte{
		ESC_POS_COMMANDS.INITIALIZE,
		ESC_POS_COMMANDS.SELECT_CHARSET_PC437,
	}

	if d.config.PaperWidth == 58 {
		commands = append(commands, ESC_POS_COMMANDS.SET_WIDTH_58MM)
	} else {
		commands = append(commands, ESC_POS_COMMANDS.SET_WIDTH_80MM)
	}

	return d.sendCommands(ctx, commands)
}

// Operation handlers (placeholder implementations)
func (d *EPSONDriver) handlePrintOperation(ctx context.Context, operation *model.DeviceOperation) (*driver.OperationResult, error) {
	return &driver.OperationResult{
		Success: true,
		Data:    map[string]interface{}{"printed": true},
	}, nil
}

func (d *EPSONDriver) handleCutOperation(ctx context.Context, operation *model.DeviceOperation) (*driver.OperationResult, error) {
	return &driver.OperationResult{
		Success: true,
		Data:    map[string]interface{}{"cut": true},
	}, nil
}

func (d *EPSONDriver) handleDrawerOperation(ctx context.Context, operation *model.DeviceOperation) (*driver.OperationResult, error) {
	return &driver.OperationResult{
		Success: true,
		Data:    map[string]interface{}{"drawer_opened": true},
	}, nil
}

func (d *EPSONDriver) handleStatusOperation(ctx context.Context, operation *model.DeviceOperation) (*driver.OperationResult, error) {
	status, err := d.GetStatus()
	if err != nil {
		return nil, err
	}

	return &driver.OperationResult{
		Success: true,
		Data:    map[string]interface{}{"status": status},
	}, nil
}

// updateHealthMetrics updates device health metrics
func (d *EPSONDriver) updateHealthMetrics(success bool, responseTime time.Duration, err error) {
	d.healthMetrics.TotalOperations++
	d.healthMetrics.ResponseTime = responseTime

	if success {
		d.healthMetrics.SuccessRate = float64(d.healthMetrics.TotalOperations-d.healthMetrics.ErrorCount) / float64(d.healthMetrics.TotalOperations)
		now := time.Now()
		d.healthMetrics.LastSuccessTime = &now
	} else {
		d.healthMetrics.ErrorCount++
		d.healthMetrics.SuccessRate = float64(d.healthMetrics.TotalOperations-d.healthMetrics.ErrorCount) / float64(d.healthMetrics.TotalOperations)
		now := time.Now()
		d.healthMetrics.LastErrorTime = &now
	}

	d.healthMetrics.HealthScore = int(d.healthMetrics.SuccessRate * 100)
	if responseTime > 5*time.Second {
		d.healthMetrics.HealthScore -= 10
	}
	if d.healthMetrics.HealthScore < 0 {
		d.healthMetrics.HealthScore = 0
	}
}

// notifyEvent notifies event handler
func (d *EPSONDriver) notifyEvent(eventType string, data interface{}) {
	if d.eventHandler != nil {
		switch eventType {
		case "connected":
			d.eventHandler.OnDeviceConnected(d.config.DeviceID)
		case "disconnected":
			d.eventHandler.OnDeviceDisconnected(d.config.DeviceID, data.(string))
		}
	}
}

// parseEPSONConfig parses and validates EPSON configuration
func parseEPSONConfig(config interface{}) (*EPSONConfig, error) {
	configMap, ok := config.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid config type")
	}

	epsonConfig := &EPSONConfig{
		PaperWidth:   80,
		CharacterSet: "PC437",
		CutType:      "FULL",
		DrawerPin:    0,
		EnableDrawer: true,
		EnableCutter: true,
		LogoEnabled:  false,
	}

	if deviceID, ok := configMap["device_id"].(string); ok {
		epsonConfig.DeviceID = deviceID
	}
	if deviceModel, ok := configMap["model"].(string); ok {
		epsonConfig.Model = deviceModel
	}
	if connType, ok := configMap["connection_type"].(string); ok {
		epsonConfig.ConnectionType = model.ConnectionType(connType)
	}
	if connConfig, ok := configMap["connection_config"].(map[string]interface{}); ok {
		epsonConfig.ConnectionConfig = connConfig
	}

	return epsonConfig, nil
}

// getEPSONCapabilities returns device capabilities based on configuration
func getEPSONCapabilities(config *EPSONConfig) []model.Capability {
	capabilities := []model.Capability{
		model.CapabilityPrint,
		model.CapabilityStatus,
	}

	if config.EnableCutter {
		capabilities = append(capabilities, model.CapabilityCut)
	}
	if config.EnableDrawer {
		capabilities = append(capabilities, model.CapabilityDrawer)
	}
	if config.LogoEnabled {
		capabilities = append(capabilities, model.CapabilityLogo)
	}

	return capabilities
}
