// internal/driver/epson/epson_driver.go
package epson

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	// ✅ Artık pkg'den import - circular import yok!
	"device-service/internal/model"
	"device-service/internal/protocol/serial"
	"device-service/internal/utils"
	"device-service/pkg/driver"
)

// EPSONDriver implements driver.DeviceDriver and driver.PrinterDriver for EPSON printers
type EPSONDriver struct {
	config        *EPSONConfig
	connection    *serial.Connection
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
	DeviceID     string                 `json:"device_id"`
	Model        string                 `json:"model"`
	Port         string                 `json:"port"`
	BaudRate     int                    `json:"baud_rate"`
	DataBits     int                    `json:"data_bits"`
	StopBits     int                    `json:"stop_bits"`
	Parity       string                 `json:"parity"`
	Timeout      time.Duration          `json:"timeout"`
	PaperWidth   int                    `json:"paper_width"`   // 58mm, 80mm
	CharacterSet string                 `json:"character_set"` // PC437, PC850, etc.
	CutType      string                 `json:"cut_type"`      // FULL, PARTIAL
	DrawerPin    int                    `json:"drawer_pin"`    // 0 or 1
	EnableDrawer bool                   `json:"enable_drawer"`
	EnableCutter bool                   `json:"enable_cutter"`
	LogoEnabled  bool                   `json:"logo_enabled"`
	Options      map[string]interface{} `json:"options"`
}

// NewEPSONDriver creates a new EPSON printer driver
func NewEPSONDriver(config interface{}, logger *zap.Logger) (driver.DeviceDriver, error) {
	epsonConfig, err := parseEPSONConfig(config)
	if err != nil {
		return nil, fmt.Errorf("invalid EPSON configuration: %w", err)
	}

	deviceLogger := utils.NewDeviceLogger(logger, epsonConfig.DeviceID, "PRINTER", "EPSON")

	return &EPSONDriver{
		config:        epsonConfig,
		logger:        deviceLogger,
		healthMetrics: &driver.HealthMetrics{},
		deviceInfo: &driver.DeviceInfo{
			Brand:          model.BrandEpson,
			Model:          epsonConfig.Model,
			ConnectionType: model.ConnectionTypeSerial,
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

	// Create serial connection
	serialConfig := &serial.Config{
		Port:     d.config.Port,
		BaudRate: d.config.BaudRate,
		DataBits: d.config.DataBits,
		StopBits: d.config.StopBits,
		Parity:   d.config.Parity,
		Timeout:  d.config.Timeout,
	}

	conn, err := serial.NewConnection(serialConfig, d.logger.Logger)
	if err != nil {
		d.updateHealthMetrics(false, time.Since(startTime), err)
		return fmt.Errorf("failed to create serial connection: %w", err)
	}

	if err := conn.Open(ctx); err != nil {
		d.updateHealthMetrics(false, time.Since(startTime), err)
		return fmt.Errorf("failed to open serial connection: %w", err)
	}

	d.connection = conn
	d.isConnected = true
	d.lastPing = time.Now()

	// Initialize printer
	if err := d.initializePrinter(ctx); err != nil {
		d.connection.Close()
		d.isConnected = false
		d.updateHealthMetrics(false, time.Since(startTime), err)
		return fmt.Errorf("failed to initialize printer: %w", err)
	}

	// Get device information
	if err := d.retrieveDeviceInfo(ctx); err != nil {
		d.logger.Warn("Failed to retrieve device info", zap.Error(err))
	}

	d.updateHealthMetrics(true, time.Since(startTime), nil)
	d.notifyEvent("connected", nil)

	d.logger.Info("EPSON printer connected successfully",
		zap.String("port", d.config.Port),
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

	if d.connection != nil {
		if err := d.connection.Close(); err != nil {
			d.logger.Error("Failed to close connection", zap.Error(err))
		}
		d.connection = nil
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
	return d.isConnected
}

// GetDeviceInfo returns device information
func (d *EPSONDriver) GetDeviceInfo() (*driver.DeviceInfo, error) {
	return d.deviceInfo, nil
}

// GetCapabilities returns device capabilities
func (d *EPSONDriver) GetCapabilities() []model.Capability {
	capabilities := []model.Capability{
		model.CapabilityPrint,
		model.CapabilityStatus,
	}

	if d.config.EnableCutter {
		capabilities = append(capabilities, model.CapabilityCut)
	}
	if d.config.EnableDrawer {
		capabilities = append(capabilities, model.CapabilityDrawer)
	}
	if d.config.LogoEnabled {
		capabilities = append(capabilities, model.CapabilityLogo)
	}

	return capabilities
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

	// Get real printer status
	status, err := d.getPrinterStatus()
	if err != nil {
		return &driver.DeviceStatus{
			Status:       model.DeviceStatusError,
			IsReady:      false,
			HasError:     true,
			ErrorCode:    "STATUS_ERROR",
			ErrorMessage: err.Error(),
			LastResponse: d.lastPing,
		}, nil
	}

	return status, nil
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
		d.notifyEvent("operation_failed", err.Error())
		return nil, err
	}

	d.updateHealthMetrics(true, duration, nil)
	d.notifyEvent("operation_completed", operation.OperationType)

	result.Duration = duration.String()
	result.Timestamp = time.Now()

	return result, nil
}

// Print operation implementation
func (d *EPSONDriver) Print(ctx context.Context, content *driver.PrintContent) error {
	if !d.IsConnected() {
		return fmt.Errorf("device not connected")
	}

	// Convert content to ESC/POS commands
	commands, err := d.contentToESCPOS(content)
	if err != nil {
		return fmt.Errorf("failed to convert content: %w", err)
	}

	// Send commands to printer
	if err := d.sendCommands(ctx, commands); err != nil {
		return fmt.Errorf("failed to send print commands: %w", err)
	}

	d.logger.LogOperation("PRINT", "", time.Since(time.Now()), true, nil)
	return nil
}

// Cut operation implementation
func (d *EPSONDriver) Cut(ctx context.Context, cutType driver.CutType) error {
	if !d.IsConnected() {
		return fmt.Errorf("device not connected")
	}

	if !d.config.EnableCutter {
		return fmt.Errorf("cutter not enabled")
	}

	var command []byte
	switch cutType {
	case driver.CutTypeFull:
		command = ESC_POS_COMMANDS.CUT_FULL
	case driver.CutTypePartial:
		command = ESC_POS_COMMANDS.CUT_PARTIAL
	default:
		return fmt.Errorf("invalid cut type: %s", cutType)
	}

	if err := d.sendCommands(ctx, [][]byte{command}); err != nil {
		return fmt.Errorf("failed to cut paper: %w", err)
	}

	return nil
}

// Feed paper lines
func (d *EPSONDriver) Feed(ctx context.Context, lines int) error {
	if !d.IsConnected() {
		return fmt.Errorf("device not connected")
	}

	if lines < 0 || lines > 255 {
		return fmt.Errorf("invalid line count: %d", lines)
	}

	command := append(ESC_POS_COMMANDS.FEED_LINES, byte(lines))
	if err := d.sendCommands(ctx, [][]byte{command}); err != nil {
		return fmt.Errorf("failed to feed paper: %w", err)
	}

	return nil
}

// GetPaperStatus returns paper status
func (d *EPSONDriver) GetPaperStatus() driver.PaperStatus {
	status, err := d.getPrinterStatus()
	if err != nil {
		return driver.PaperStatusUnknown
	}

	// Parse paper status from printer response
	if status.HasError {
		if status.ErrorCode == "PAPER_OUT" {
			return driver.PaperStatusOut
		}
		if status.ErrorCode == "PAPER_JAM" {
			return driver.PaperStatusJam
		}
	}

	return driver.PaperStatusOK
}

// GetCutterStatus returns cutter status
func (d *EPSONDriver) GetCutterStatus() driver.CutterStatus {
	if !d.config.EnableCutter {
		return driver.CutterStatusUnknown
	}

	status, err := d.getPrinterStatus()
	if err != nil {
		return driver.CutterStatusUnknown
	}

	if status.HasError && status.ErrorCode == "CUTTER_ERROR" {
		return driver.CutterStatusError
	}

	return driver.CutterStatusOK
}

// OpenDrawer opens cash drawer
func (d *EPSONDriver) OpenDrawer(ctx context.Context, pin int) error {
	if !d.IsConnected() {
		return fmt.Errorf("device not connected")
	}

	if !d.config.EnableDrawer {
		return fmt.Errorf("cash drawer not enabled")
	}

	if pin < 0 || pin > 1 {
		pin = d.config.DrawerPin
	}

	var command []byte
	if pin == 0 {
		command = ESC_POS_COMMANDS.DRAWER_KICK_PIN2
	} else {
		command = ESC_POS_COMMANDS.DRAWER_KICK_PIN5
	}

	if err := d.sendCommands(ctx, [][]byte{command}); err != nil {
		return fmt.Errorf("failed to open drawer: %w", err)
	}

	d.logger.LogOperation("OPEN_DRAWER", "", time.Since(time.Now()), true, nil)
	return nil
}

// GetDrawerStatus returns drawer status
func (d *EPSONDriver) GetDrawerStatus() driver.DrawerStatus {
	if !d.config.EnableDrawer {
		return driver.DrawerStatusUnknown
	}

	// EPSON printers typically don't provide real-time drawer status
	// This would require additional hardware integration
	return driver.DrawerStatusUnknown
}

// Ping tests device connectivity
func (d *EPSONDriver) Ping(ctx context.Context) error {
	if !d.IsConnected() {
		return fmt.Errorf("device not connected")
	}

	startTime := time.Now()

	// Send status request
	if err := d.sendCommands(ctx, [][]byte{ESC_POS_COMMANDS.STATUS_REQUEST}); err != nil {
		d.updateHealthMetrics(false, time.Since(startTime), err)
		return fmt.Errorf("ping failed: %w", err)
	}

	// Read response with timeout
	response, err := d.readResponse(ctx, 2*time.Second)
	if err != nil {
		d.updateHealthMetrics(false, time.Since(startTime), err)
		return fmt.Errorf("ping response failed: %w", err)
	}

	if len(response) == 0 {
		err := fmt.Errorf("no response from device")
		d.updateHealthMetrics(false, time.Since(startTime), err)
		return err
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

// Helper methods will continue in the next part...

// Helper methods
func (d *EPSONDriver) initializePrinter(ctx context.Context) error {
	commands := [][]byte{
		ESC_POS_COMMANDS.INITIALIZE,
		ESC_POS_COMMANDS.SELECT_CHARSET_PC437,
	}

	// Set paper width
	if d.config.PaperWidth == 58 {
		commands = append(commands, ESC_POS_COMMANDS.SET_WIDTH_58MM)
	} else {
		commands = append(commands, ESC_POS_COMMANDS.SET_WIDTH_80MM)
	}

	return d.sendCommands(ctx, commands)
}

func (d *EPSONDriver) retrieveDeviceInfo(ctx context.Context) error {
	// Send device info request
	if err := d.sendCommands(ctx, [][]byte{ESC_POS_COMMANDS.GET_DEVICE_INFO}); err != nil {
		return err
	}

	// Read response
	response, err := d.readResponse(ctx, 3*time.Second)
	if err != nil {
		return err
	}

	// Parse device information
	if len(response) > 0 {
		d.deviceInfo.SerialNumber = parseSerialNumber(response)
		d.deviceInfo.FirmwareVersion = parseFirmwareVersion(response)
		d.deviceInfo.HardwareVersion = parseHardwareVersion(response)
	}

	return nil
}

func (d *EPSONDriver) getPrinterStatus() (*driver.DeviceStatus, error) {
	// Send status request
	if err := d.sendCommands(context.Background(), [][]byte{ESC_POS_COMMANDS.STATUS_REQUEST}); err != nil {
		return nil, err
	}

	// Read status response
	response, err := d.readResponse(context.Background(), 2*time.Second)
	if err != nil {
		return nil, err
	}

	return parseStatusResponse(response), nil
}

// ... (rest of the helper methods remain the same)

// Helper functions for parsing
func parseEPSONConfig(config interface{}) (*EPSONConfig, error) {
	configMap, ok := config.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid config type")
	}

	epsonConfig := &EPSONConfig{
		BaudRate:     9600,
		DataBits:     8,
		StopBits:     1,
		Parity:       "none",
		Timeout:      5 * time.Second,
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
	if model, ok := configMap["model"].(string); ok {
		epsonConfig.Model = model
	}
	if port, ok := configMap["port"].(string); ok {
		epsonConfig.Port = port
	}
	if baudRate, ok := configMap["baud_rate"].(float64); ok {
		epsonConfig.BaudRate = int(baudRate)
	}

	return epsonConfig, nil
}

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

// Simplified helper functions
func parseSerialNumber(response []byte) string {
	return "EPSON-" + fmt.Sprintf("%X", response[:4])
}

func parseFirmwareVersion(response []byte) string {
	return "1.0.0"
}

func parseHardwareVersion(response []byte) string {
	return "Rev.A"
}

func parseStatusResponse(response []byte) *driver.DeviceStatus {
	status := &driver.DeviceStatus{
		Status:       model.DeviceStatusOnline,
		IsReady:      true,
		HasError:     false,
		LastResponse: time.Now(),
	}

	if len(response) == 0 {
		status.Status = model.DeviceStatusError
		status.IsReady = false
		status.HasError = true
		status.ErrorCode = "NO_RESPONSE"
		status.ErrorMessage = "No response from device"
		return status
	}

	// Parse EPSON status bytes (simplified)
	if len(response) >= 4 {
		paperStatus := response[0]
		if paperStatus&0x03 != 0 {
			status.HasError = true
			status.ErrorCode = "PAPER_OUT"
			status.ErrorMessage = "Paper out or near end"
		}
	}

	return status
}
