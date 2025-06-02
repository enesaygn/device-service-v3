// internal/driver/epson/epson_driver.go
package epson

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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
func NewEPSONDriver(device *model.Device, connectionConfig interface{}, logger *zap.Logger) (driver.DeviceDriver, error) {
	// Parse connection configuration ONLY
	connConfig, err := parseConnectionConfig(connectionConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid connection configuration: %w", err)
	}

	// Create EPSON configuration using device + connection info
	epsonConfig := &EPSONConfig{
		// Device information comes from device parameter
		DeviceID:         device.DeviceID,
		Model:            device.Model,
		ConnectionType:   device.ConnectionType,
		ConnectionConfig: connConfig,

		// Driver-specific defaults
		PaperWidth:   80,
		CharacterSet: "PC437",
		CutType:      "FULL",
		DrawerPin:    0,
		EnableDrawer: true,
		EnableCutter: true,
		LogoEnabled:  false,
		Options:      make(map[string]interface{}),
	}

	// Override defaults from device capabilities if available
	if device.HasCapability(model.CapabilityDrawer) {
		epsonConfig.EnableDrawer = true
	}
	if device.HasCapability(model.CapabilityCut) {
		epsonConfig.EnableCutter = true
	}

	deviceLogger := utils.NewDeviceLogger(logger, device.DeviceID, string(device.DeviceType), string(device.Brand))

	// ✅ CREATE DRIVER INSTANCE
	epsonDriver := &EPSONDriver{
		config: epsonConfig,
		logger: deviceLogger,
		healthMetrics: &driver.HealthMetrics{
			HealthScore: 0,
		},
		deviceInfo: &driver.DeviceInfo{
			Brand:          device.Brand,
			Model:          device.Model,
			ConnectionType: device.ConnectionType,
			Capabilities:   getEPSONCapabilities(epsonConfig),
			Manufacturer:   "Seiko Epson Corporation",
		},
		isConnected: false, // Başlangıçta bağlı değil
	}

	// ✅ EAGER CONNECTION: Hemen protocol oluştur ve bağlan
	deviceLogger.Info("Creating protocol connection during driver initialization",
		zap.String("connection_type", string(device.ConnectionType)),
	)

	// Protocol oluştur
	protocolInstance, err := protocol.CreateProtocol(
		device.ConnectionType,
		connConfig,
		deviceLogger.Logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s protocol: %w", device.ConnectionType, err)
	}

	// Protocol connection'ı aç
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := protocolInstance.Open(ctx); err != nil {
		deviceLogger.Error("Failed to open protocol connection during driver creation", zap.Error(err))
		// Connection fail olsa bile driver'ı oluştur, sonra lazy connection yapacak
		epsonDriver.protocol = nil
		epsonDriver.isConnected = false
		deviceLogger.Warn("Driver created without active connection, will retry on first operation")
		return epsonDriver, nil
	}

	// Protocol'ü driver'a set et
	epsonDriver.protocol = protocolInstance
	epsonDriver.isConnected = true
	epsonDriver.lastPing = time.Now()

	// Printer'ı initialize et
	if err := epsonDriver.initializePrinter(ctx); err != nil {
		deviceLogger.Warn("Failed to initialize printer during driver creation", zap.Error(err))
		// Initialization fail olsa bile devam et
	}

	deviceLogger.Info("EPSON driver created with active connection",
		zap.String("connection_type", string(device.ConnectionType)),
		zap.Bool("protocol_connected", protocolInstance.IsOpen()),
	)

	return epsonDriver, nil
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

func parseConnectionConfig(config interface{}) (map[string]interface{}, error) {
	var configMap map[string]interface{}

	// Handle different input types
	switch v := config.(type) {
	case map[string]interface{}:
		configMap = v
	case model.JSONObject:
		configMap = map[string]interface{}(v)
	case *model.JSONObject:
		if v != nil {
			configMap = map[string]interface{}(*v)
		} else {
			return nil, fmt.Errorf("config is nil")
		}
	default:
		return nil, fmt.Errorf("invalid config type: %T, expected map[string]interface{} or model.JSONObject", config)
	}

	if configMap == nil {
		return nil, fmt.Errorf("config map is nil")
	}

	// Validate required connection fields based on connection type
	return configMap, nil
}

// parseEPSONConfig parses and validates EPSON configuration
func parseEPSONConfig(config interface{}) (*EPSONConfig, error) {
	var configMap map[string]interface{}

	// Handle different input types
	switch v := config.(type) {
	case map[string]interface{}:
		configMap = v
	case model.JSONObject:
		configMap = map[string]interface{}(v)
	case *model.JSONObject:
		if v != nil {
			configMap = map[string]interface{}(*v)
		} else {
			return nil, fmt.Errorf("config is nil")
		}
	default:
		return nil, fmt.Errorf("invalid config type: %T, expected map[string]interface{} or model.JSONObject", config)
	}

	if configMap == nil {
		return nil, fmt.Errorf("config map is nil")
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

// handlePrintOperation handles print operations with full ESC/POS support
func (d *EPSONDriver) handlePrintOperation(ctx context.Context, operation *model.DeviceOperation) (*driver.OperationResult, error) {
	d.logger.Info("Processing print operation", zap.String("operation_id", operation.ID.String()))

	// Parse operation data
	printData, err := d.parsePrintOperationData(operation.OperationData)
	if err != nil {
		return nil, fmt.Errorf("invalid print operation data: %w", err)
	}

	// Build command sequence
	commands, err := d.buildPrintCommands(printData)
	if err != nil {
		return nil, fmt.Errorf("failed to build print commands: %w", err)
	}

	// Send commands to printer
	startTime := time.Now()
	if err := d.sendCommands(ctx, commands); err != nil {
		return nil, fmt.Errorf("failed to send print commands: %w", err)
	}

	// Wait for print completion (optional)
	if err := d.waitForPrintCompletion(ctx, printData); err != nil {
		d.logger.Warn("Print completion wait failed", zap.Error(err))
		// Don't fail the operation, just log warning
	}

	duration := time.Since(startTime)

	d.logger.Info("Print operation completed successfully",
		zap.String("operation_id", operation.ID.String()),
		zap.Duration("duration", duration),
		zap.Int("lines", strings.Count(printData.Content, "\n")+1),
	)

	return &driver.OperationResult{
		Success: true,
		Data: map[string]interface{}{
			"printed":        true,
			"content_length": len(printData.Content),
			"lines_printed":  strings.Count(printData.Content, "\n") + 1,
			"copies":         printData.Copies,
			"cut_performed":  printData.Cut,
			"drawer_opened":  printData.OpenDrawer,
			"print_duration": duration.Milliseconds(),
		},
		Duration:  duration.String(),
		Timestamp: time.Now(),
	}, nil
}

// handleCutOperation handles paper cutting
func (d *EPSONDriver) handleCutOperation(ctx context.Context, operation *model.DeviceOperation) (*driver.OperationResult, error) {
	d.logger.Info("Processing cut operation", zap.String("operation_id", operation.ID.String()))

	if !d.config.EnableCutter {
		return nil, fmt.Errorf("cutter is disabled on this device")
	}

	// Parse cut type from operation data
	cutType := "FULL" // default
	if data, ok := operation.OperationData["cut_type"]; ok {
		if ct, ok := data.(string); ok {
			cutType = strings.ToUpper(ct)
		}
	}

	// Select appropriate cut command
	var cutCommand []byte
	switch cutType {
	case "FULL":
		cutCommand = ESC_POS_COMMANDS.CUT_FULL
	case "PARTIAL":
		cutCommand = ESC_POS_COMMANDS.CUT_PARTIAL
	default:
		return nil, fmt.Errorf("invalid cut type: %s", cutType)
	}

	// Send cut command
	startTime := time.Now()
	if err := d.sendCommands(ctx, [][]byte{cutCommand}); err != nil {
		return nil, fmt.Errorf("failed to send cut command: %w", err)
	}

	// Wait a bit for mechanical operation
	time.Sleep(500 * time.Millisecond)

	duration := time.Since(startTime)

	d.logger.Info("Cut operation completed successfully",
		zap.String("operation_id", operation.ID.String()),
		zap.String("cut_type", cutType),
		zap.Duration("duration", duration),
	)

	return &driver.OperationResult{
		Success: true,
		Data: map[string]interface{}{
			"cut":          true,
			"cut_type":     cutType,
			"cut_duration": duration.Milliseconds(),
		},
		Duration:  duration.String(),
		Timestamp: time.Now(),
	}, nil
}

// handleDrawerOperation handles cash drawer opening
func (d *EPSONDriver) handleDrawerOperation(ctx context.Context, operation *model.DeviceOperation) (*driver.OperationResult, error) {
	d.logger.Info("Processing drawer operation", zap.String("operation_id", operation.ID.String()))

	if !d.config.EnableDrawer {
		return nil, fmt.Errorf("cash drawer is disabled on this device")
	}

	// Parse drawer pin from operation data
	pin := d.config.DrawerPin // default
	if data, ok := operation.OperationData["pin"]; ok {
		switch v := data.(type) {
		case float64:
			pin = int(v)
		case int:
			pin = v
		case string:
			if p, err := strconv.Atoi(v); err == nil {
				pin = p
			}
		}
	}

	// Select appropriate drawer command
	var drawerCommand []byte
	switch pin {
	case 0, 2:
		drawerCommand = ESC_POS_COMMANDS.DRAWER_KICK_PIN2
	case 1, 5:
		drawerCommand = ESC_POS_COMMANDS.DRAWER_KICK_PIN5
	default:
		return nil, fmt.Errorf("invalid drawer pin: %d (supported: 0/2 or 1/5)", pin)
	}

	// Send drawer command
	startTime := time.Now()
	if err := d.sendCommands(ctx, [][]byte{drawerCommand}); err != nil {
		return nil, fmt.Errorf("failed to send drawer command: %w", err)
	}

	// Wait for drawer kick signal
	time.Sleep(200 * time.Millisecond)

	duration := time.Since(startTime)

	d.logger.Info("Drawer operation completed successfully",
		zap.String("operation_id", operation.ID.String()),
		zap.Int("pin", pin),
		zap.Duration("duration", duration),
	)

	return &driver.OperationResult{
		Success: true,
		Data: map[string]interface{}{
			"drawer_opened":   true,
			"pin_used":        pin,
			"drawer_duration": duration.Milliseconds(),
		},
		Duration:  duration.String(),
		Timestamp: time.Now(),
	}, nil
}

// handleStatusOperation handles status check operations
func (d *EPSONDriver) handleStatusOperation(ctx context.Context, operation *model.DeviceOperation) (*driver.OperationResult, error) {
	d.logger.Info("Processing status operation", zap.String("operation_id", operation.ID.String()))

	startTime := time.Now()

	// Get basic status
	status, err := d.GetStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to get device status: %w", err)
	}

	// Request detailed status from printer
	statusData, err := d.requestDetailedStatus(ctx)
	if err != nil {
		d.logger.Warn("Failed to get detailed status", zap.Error(err))
		// Don't fail, just use basic status
		statusData = map[string]interface{}{
			"basic_status_only": true,
		}
	}

	// Get device info
	deviceInfo, err := d.GetDeviceInfo()
	if err != nil {
		d.logger.Warn("Failed to get device info", zap.Error(err))
	}

	duration := time.Since(startTime)

	result := map[string]interface{}{
		"status":          status,
		"detailed_status": statusData,
		"last_ping":       d.lastPing,
		"connection_type": d.config.ConnectionType,
		"model":           d.config.Model,
		"capabilities":    d.GetCapabilities(),
		"status_duration": duration.Milliseconds(),
	}

	if deviceInfo != nil {
		result["device_info"] = deviceInfo
	}

	d.logger.Info("Status operation completed successfully",
		zap.String("operation_id", operation.ID.String()),
		zap.Duration("duration", duration),
	)

	return &driver.OperationResult{
		Success:   true,
		Data:      result,
		Duration:  duration.String(),
		Timestamp: time.Now(),
	}, nil
}

// handleBeepOperation handles beep/buzzer operations
func (d *EPSONDriver) handleBeepOperation(ctx context.Context, operation *model.DeviceOperation) (*driver.OperationResult, error) {
	d.logger.Info("Processing beep operation", zap.String("operation_id", operation.ID.String()))

	// Parse beep parameters
	beepCount := 1
	beepDuration := 100 // milliseconds

	if data, ok := operation.OperationData["count"]; ok {
		if c, ok := data.(float64); ok {
			beepCount = int(c)
		}
	}
	if data, ok := operation.OperationData["duration"]; ok {
		if d, ok := data.(float64); ok {
			beepDuration = int(d)
		}
	}

	// Limit beep count and duration
	if beepCount > 10 {
		beepCount = 10
	}
	if beepDuration > 2000 {
		beepDuration = 2000
	}

	startTime := time.Now()

	// Create beep sequence (using buzzer command if available)
	// EPSON printers typically use ESC ( A for buzzer
	beepCommand := []byte{0x1B, 0x28, 0x41, 0x04, 0x00, 0x01, 0x01, 0x01}

	commands := [][]byte{}
	for i := 0; i < beepCount; i++ {
		commands = append(commands, beepCommand)
		if i < beepCount-1 {
			// Add delay between beeps (implemented by sending line feeds)
			commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
		}
	}

	if err := d.sendCommands(ctx, commands); err != nil {
		return nil, fmt.Errorf("failed to send beep commands: %w", err)
	}

	duration := time.Since(startTime)

	d.logger.Info("Beep operation completed successfully",
		zap.String("operation_id", operation.ID.String()),
		zap.Int("beep_count", beepCount),
		zap.Duration("duration", duration),
	)

	return &driver.OperationResult{
		Success: true,
		Data: map[string]interface{}{
			"beep_executed":  true,
			"beep_count":     beepCount,
			"beep_duration":  beepDuration,
			"total_duration": duration.Milliseconds(),
		},
		Duration:  duration.String(),
		Timestamp: time.Now(),
	}, nil
}

// Helper methods for print operations

// parsePrintOperationData parses print operation data
func (d *EPSONDriver) parsePrintOperationData(data model.JSONObject) (*PrintOperationData, error) {
	printData := &PrintOperationData{
		ContentType: "TEXT",
		Copies:      1,
		Cut:         false,
		OpenDrawer:  false,
		Logo:        false,
		Options:     make(map[string]string),
	}

	// Parse content (required)
	if content, ok := data["content"]; ok {
		if c, ok := content.(string); ok {
			printData.Content = c
		} else {
			return nil, fmt.Errorf("content must be a string")
		}
	} else {
		return nil, fmt.Errorf("content is required")
	}

	// Parse optional fields
	if contentType, ok := data["content_type"]; ok {
		if ct, ok := contentType.(string); ok {
			printData.ContentType = strings.ToUpper(ct)
		}
	}

	if copies, ok := data["copies"]; ok {
		switch v := copies.(type) {
		case float64:
			printData.Copies = int(v)
		case int:
			printData.Copies = v
		}
	}

	if cut, ok := data["cut"]; ok {
		if c, ok := cut.(bool); ok {
			printData.Cut = c
		}
	}

	if openDrawer, ok := data["open_drawer"]; ok {
		if od, ok := openDrawer.(bool); ok {
			printData.OpenDrawer = od
		}
	}

	if logo, ok := data["logo"]; ok {
		if l, ok := logo.(bool); ok {
			printData.Logo = l
		}
	}

	if options, ok := data["options"]; ok {
		if opts, ok := options.(map[string]interface{}); ok {
			for k, v := range opts {
				if vStr, ok := v.(string); ok {
					printData.Options[k] = vStr
				}
			}
		}
	}

	// Validate
	if printData.Content == "" {
		return nil, fmt.Errorf("content cannot be empty")
	}
	if printData.Copies < 1 || printData.Copies > 10 {
		return nil, fmt.Errorf("copies must be between 1 and 10")
	}

	return printData, nil
}

// buildHTMLCommands builds commands for HTML content (simplified)
func (d *EPSONDriver) buildHTMLCommands(content string) ([][]byte, error) {
	// This is a simplified HTML to ESC/POS converter
	// In a real implementation, you'd use a proper HTML parser

	//TODO: commands := [][]byte{}

	// Remove HTML tags and convert basic formatting
	text := content
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = strings.ReplaceAll(text, "<br />", "\n")
	text = strings.ReplaceAll(text, "</p>", "\n")

	// Remove all other HTML tags (simplified)
	// In production, use proper HTML parsing
	for strings.Contains(text, "<") && strings.Contains(text, ">") {
		start := strings.Index(text, "<")
		end := strings.Index(text, ">")
		if start < end {
			text = text[:start] + text[end+1:]
		} else {
			break
		}
	}

	// Convert to text commands
	return d.buildTextCommands(text, make(map[string]string))
}

// waitForPrintCompletion waits for print operation to complete
func (d *EPSONDriver) waitForPrintCompletion(ctx context.Context, printData *PrintOperationData) error {
	// Calculate expected print time based on content length
	contentLength := len(printData.Content)
	estimatedTime := time.Duration(contentLength/100) * time.Millisecond * time.Duration(printData.Copies)

	// Minimum wait time
	if estimatedTime < 500*time.Millisecond {
		estimatedTime = 500 * time.Millisecond
	}

	// Maximum wait time
	if estimatedTime > 10*time.Second {
		estimatedTime = 10 * time.Second
	}

	select {
	case <-time.After(estimatedTime):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// requestDetailedStatus requests detailed status from printer
func (d *EPSONDriver) requestDetailedStatus(ctx context.Context) (map[string]interface{}, error) {
	// Send status request command
	if err := d.sendCommands(ctx, [][]byte{ESC_POS_COMMANDS.STATUS_REQUEST}); err != nil {
		return nil, fmt.Errorf("failed to send status request: %w", err)
	}

	// Read response with timeout
	responseCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	response, err := d.readResponse(responseCtx, 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to read status response: %w", err)
	}

	// Parse status response
	statusData := d.parseStatusResponse(response)
	return statusData, nil
}

// parseStatusResponse parses printer status response
func (d *EPSONDriver) parseStatusResponse(response []byte) map[string]interface{} {
	status := map[string]interface{}{
		"raw_response":    fmt.Sprintf("%x", response),
		"response_length": len(response),
	}

	if len(response) > 0 {
		// Parse basic status byte
		statusByte := response[0]

		status["online"] = (statusByte & 0x08) == 0
		status["paper_error"] = (statusByte & 0x20) != 0
		status["offline"] = (statusByte & 0x08) != 0
		status["error"] = (statusByte & 0x40) != 0

		// Additional status parsing could be added here
		// based on the specific EPSON printer model
	}

	status["timestamp"] = time.Now()
	return status
}

// Data structures for operations

// PrintOperationData represents print operation parameters
type PrintOperationData struct {
	Content     string            `json:"content"`
	ContentType string            `json:"content_type"` // TEXT, HTML, ESC_POS, RECEIPT
	Copies      int               `json:"copies"`
	Cut         bool              `json:"cut"`
	OpenDrawer  bool              `json:"open_drawer"`
	Logo        bool              `json:"logo"`
	Options     map[string]string `json:"options,omitempty"`
}

// ReceiptData represents structured receipt data
type ReceiptData struct {
	Header string        `json:"header"`
	Items  []ReceiptItem `json:"items"`
	Total  float64       `json:"total"`
	Footer string        `json:"footer"`
}

// ReceiptItem represents a single receipt item
type ReceiptItem struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Qty   int     `json:"qty,omitempty"`
}

// buildTextCommands - IMPROVED with better formatting
func (d *EPSONDriver) buildTextCommands(content string, options map[string]string) ([][]byte, error) {
	commands := [][]byte{}

	// ✅ ALWAYS start with center alignment for better layout
	commands = append(commands, ESC_POS_COMMANDS.ALIGN_CENTER)

	// ✅ Add header spacing
	commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)

	// Parse and apply formatting options
	if bold, ok := options["bold"]; ok && bold == "true" {
		commands = append(commands, ESC_POS_COMMANDS.TEXT_BOLD_ON)
	}

	if underline, ok := options["underline"]; ok && underline == "true" {
		commands = append(commands, ESC_POS_COMMANDS.TEXT_UNDERLINE_ON)
	}

	// ✅ DEFAULT: Make text larger for better readability
	textSize := "DOUBLE"
	if size, ok := options["size"]; ok {
		textSize = strings.ToUpper(size)
	}

	switch textSize {
	case "NORMAL":
		commands = append(commands, ESC_POS_COMMANDS.TEXT_SIZE_NORMAL)
	case "DOUBLE_WIDTH":
		commands = append(commands, ESC_POS_COMMANDS.TEXT_SIZE_DOUBLE_WIDTH)
	case "DOUBLE_HEIGHT":
		commands = append(commands, ESC_POS_COMMANDS.TEXT_SIZE_DOUBLE_HEIGHT)
	case "DOUBLE", "BIG":
		commands = append(commands, ESC_POS_COMMANDS.TEXT_SIZE_DOUBLE_BOTH)
	default:
		// Default to double size for better visibility
		commands = append(commands, ESC_POS_COMMANDS.TEXT_SIZE_DOUBLE_BOTH)
	}

	// Parse alignment (override center if specified)
	if align, ok := options["align"]; ok {
		switch strings.ToUpper(align) {
		case "LEFT":
			commands = append(commands, ESC_POS_COMMANDS.ALIGN_LEFT)
		case "RIGHT":
			commands = append(commands, ESC_POS_COMMANDS.ALIGN_RIGHT)
		case "CENTER":
			commands = append(commands, ESC_POS_COMMANDS.ALIGN_CENTER)
		}
	}

	// ✅ Process content line by line with proper spacing
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// Trim whitespace
		line = strings.TrimSpace(line)

		if line != "" {
			commands = append(commands, []byte(line))
		}

		// Add line feed after each line (including empty lines for spacing)
		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)

		// ✅ Add extra spacing between non-empty lines for better readability
		if line != "" && i < len(lines)-1 {
			commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
		}
	}

	// ✅ Add footer spacing
	commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
	commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)

	// Reset formatting
	commands = append(commands, ESC_POS_COMMANDS.TEXT_RESET)
	commands = append(commands, ESC_POS_COMMANDS.ALIGN_LEFT)

	return commands, nil
}

// buildReceiptCommands - IMPROVED with better formatting
func (d *EPSONDriver) buildReceiptCommands(content string, options map[string]string) ([][]byte, error) {
	commands := [][]byte{}

	// Parse receipt data
	var receipt ReceiptData
	if err := json.Unmarshal([]byte(content), &receipt); err != nil {
		// If not JSON, treat as formatted text with better defaults
		return d.buildFormattedTextCommands(content, options)
	}

	// ✅ RECEIPT HEADER with nice formatting
	if receipt.Header != "" {
		// Center alignment for header
		commands = append(commands, ESC_POS_COMMANDS.ALIGN_CENTER)
		commands = append(commands, ESC_POS_COMMANDS.TEXT_SIZE_DOUBLE_BOTH)
		commands = append(commands, ESC_POS_COMMANDS.TEXT_BOLD_ON)

		// Add top spacing
		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)

		commands = append(commands, []byte(receipt.Header))
		commands = append(commands, ESC_POS_COMMANDS.TEXT_BOLD_OFF)
		commands = append(commands, ESC_POS_COMMANDS.TEXT_SIZE_NORMAL)

		// Header separator
		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
		commands = append(commands, []byte("================================"))
		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
	}

	// ✅ ITEMS with better spacing and formatting
	commands = append(commands, ESC_POS_COMMANDS.ALIGN_LEFT)
	commands = append(commands, ESC_POS_COMMANDS.TEXT_SIZE_NORMAL)

	for i, item := range receipt.Items {
		// Item name and price formatting
		itemLine := formatReceiptLine(item.Name, item.Price, d.config.PaperWidth)
		commands = append(commands, []byte(itemLine))
		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)

		// Add spacing between items
		if i < len(receipt.Items)-1 {
			commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
		}
	}

	// ✅ TOTAL with emphasis
	if receipt.Total > 0 {
		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
		commands = append(commands, []byte("================================"))
		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)

		// Center and emphasize total
		commands = append(commands, ESC_POS_COMMANDS.ALIGN_CENTER)
		commands = append(commands, ESC_POS_COMMANDS.TEXT_SIZE_DOUBLE_WIDTH)
		commands = append(commands, ESC_POS_COMMANDS.TEXT_BOLD_ON)

		totalLine := fmt.Sprintf("TOPLAM: %.2f TL", receipt.Total)
		commands = append(commands, []byte(totalLine))

		commands = append(commands, ESC_POS_COMMANDS.TEXT_BOLD_OFF)
		commands = append(commands, ESC_POS_COMMANDS.TEXT_SIZE_NORMAL)
		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
	}

	// ✅ FOOTER with center alignment
	if receipt.Footer != "" {
		commands = append(commands, ESC_POS_COMMANDS.ALIGN_CENTER)
		commands = append(commands, ESC_POS_COMMANDS.TEXT_SIZE_NORMAL)
		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
		commands = append(commands, []byte(receipt.Footer))
		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
	}

	// ✅ Final spacing
	commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
	commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
	commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)

	// Reset alignment
	commands = append(commands, ESC_POS_COMMANDS.ALIGN_LEFT)

	return commands, nil
}

// ✅ NEW: Better formatted text commands for non-JSON content
func (d *EPSONDriver) buildFormattedTextCommands(content string, options map[string]string) ([][]byte, error) {
	commands := [][]byte{}

	// ✅ Start with nice header
	commands = append(commands, ESC_POS_COMMANDS.ALIGN_CENTER)
	commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
	commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)

	// ✅ Make text bigger and bold for better visibility
	commands = append(commands, ESC_POS_COMMANDS.TEXT_SIZE_DOUBLE_BOTH)
	commands = append(commands, ESC_POS_COMMANDS.TEXT_BOLD_ON)

	// Process each line
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)

		if line != "" {
			commands = append(commands, []byte(line))
		}

		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)

		// Extra spacing between non-empty lines
		if line != "" && i < len(lines)-1 {
			commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
		}
	}

	// ✅ Nice footer with current time
	commands = append(commands, ESC_POS_COMMANDS.TEXT_BOLD_OFF)
	commands = append(commands, ESC_POS_COMMANDS.TEXT_SIZE_NORMAL)
	commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
	commands = append(commands, []byte("--------------------------------"))
	commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)

	currentTime := time.Now().Format("02.01.2006 15:04:05")
	commands = append(commands, []byte(currentTime))
	commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
	commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
	commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)

	// Reset formatting
	commands = append(commands, ESC_POS_COMMANDS.TEXT_RESET)
	commands = append(commands, ESC_POS_COMMANDS.ALIGN_LEFT)

	return commands, nil
}

// ✅ Helper function to format receipt lines properly
func formatReceiptLine(name string, price float64, paperWidth int) string {
	maxNameWidth := paperWidth - 12 // Reserve space for price
	if maxNameWidth < 10 {
		maxNameWidth = 10
	}

	// Truncate name if too long
	if len(name) > maxNameWidth {
		name = name[:maxNameWidth-3] + "..."
	}

	// Format price
	priceStr := fmt.Sprintf("%.2f", price)

	// Calculate spacing
	totalUsed := len(name) + len(priceStr)
	spacesNeeded := paperWidth - totalUsed
	if spacesNeeded < 1 {
		spacesNeeded = 1
	}

	return name + strings.Repeat(" ", spacesNeeded) + priceStr
}

// ✅ IMPROVED: buildPrintCommands with better defaults
func (d *EPSONDriver) buildPrintCommands(printData *PrintOperationData) ([][]byte, error) {
	commands := [][]byte{}

	// Initialize printer
	commands = append(commands, ESC_POS_COMMANDS.INITIALIZE)

	// Set character set
	commands = append(commands, ESC_POS_COMMANDS.SELECT_CHARSET_PC437)

	// Set paper width
	if d.config.PaperWidth == 58 {
		commands = append(commands, ESC_POS_COMMANDS.SET_WIDTH_58MM)
	} else {
		commands = append(commands, ESC_POS_COMMANDS.SET_WIDTH_80MM)
	}

	// ✅ Print logo if requested and enabled
	if printData.Logo && d.config.LogoEnabled {
		commands = append(commands, ESC_POS_COMMANDS.ALIGN_CENTER)
		commands = append(commands, ESC_POS_COMMANDS.PRINT_LOGO)
		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
		commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
	}

	// ✅ Apply default formatting for better readability
	if printData.Options == nil {
		printData.Options = make(map[string]string)
	}

	// Set better defaults if not specified
	if _, exists := printData.Options["size"]; !exists {
		printData.Options["size"] = "DOUBLE" // Make text bigger by default
	}
	if _, exists := printData.Options["align"]; !exists {
		printData.Options["align"] = "CENTER" // Center by default
	}

	// Process content based on type
	switch printData.ContentType {
	case "TEXT":
		textCommands, err := d.buildFormattedTextCommands(printData.Content, printData.Options)
		if err != nil {
			return nil, fmt.Errorf("failed to build text commands: %w", err)
		}
		commands = append(commands, textCommands...)

	case "ESC_POS":
		// Raw ESC/POS data
		commands = append(commands, []byte(printData.Content))

	case "HTML":
		// Convert HTML to ESC/POS
		textCommands, err := d.buildHTMLCommands(printData.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to build HTML commands: %w", err)
		}
		commands = append(commands, textCommands...)

	case "RECEIPT":
		// Structured receipt format
		receiptCommands, err := d.buildReceiptCommands(printData.Content, printData.Options)
		if err != nil {
			return nil, fmt.Errorf("failed to build receipt commands: %w", err)
		}
		commands = append(commands, receiptCommands...)

	default:
		return nil, fmt.Errorf("unsupported content type: %s", printData.ContentType)
	}

	// ✅ Print multiple copies with better separation
	if printData.Copies > 1 {
		originalCommands := make([][]byte, len(commands))
		copy(originalCommands, commands)

		for i := 1; i < printData.Copies; i++ {
			// Nice separator between copies
			commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
			commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
			commands = append(commands, ESC_POS_COMMANDS.ALIGN_CENTER)
			commands = append(commands, []byte(fmt.Sprintf("--- KOPYA %d ---", i+1)))
			commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
			commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)

			// Add the original content
			commands = append(commands, originalCommands...)
		}
	}

	// ✅ Final spacing before cut/drawer
	commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
	commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)
	commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)

	// Cut paper if requested
	if printData.Cut && d.config.EnableCutter {
		switch d.config.CutType {
		case "PARTIAL":
			commands = append(commands, ESC_POS_COMMANDS.CUT_PARTIAL)
		default:
			commands = append(commands, ESC_POS_COMMANDS.CUT_FULL)
		}
	}

	// Open drawer if requested
	if printData.OpenDrawer && d.config.EnableDrawer {
		if d.config.DrawerPin == 1 || d.config.DrawerPin == 5 {
			commands = append(commands, ESC_POS_COMMANDS.DRAWER_KICK_PIN5)
		} else {
			commands = append(commands, ESC_POS_COMMANDS.DRAWER_KICK_PIN2)
		}
	}

	return commands, nil
}
