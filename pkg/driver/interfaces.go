// pkg/driver/interfaces.go
package driver

import (
	"context"

	"device-service/internal/model"
)

// DeviceDriver is the main interface that all hardware drivers must implement
type DeviceDriver interface {
	// Connection management
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	IsConnected() bool

	// Device information
	GetDeviceInfo() (*DeviceInfo, error)
	GetCapabilities() []model.Capability
	GetStatus() (*DeviceStatus, error)

	// Operations
	ExecuteOperation(ctx context.Context, operation *model.DeviceOperation) (*OperationResult, error)

	// Health and monitoring
	Ping(ctx context.Context) error
	GetHealthMetrics() (*HealthMetrics, error)

	// Configuration
	Configure(config interface{}) error
	Reset(ctx context.Context) error

	// Event handling
	SetEventHandler(handler EventHandler)

	// Cleanup
	Close() error
}

// PrinterDriver extends DeviceDriver for printer-specific operations
type PrinterDriver interface {
	DeviceDriver

	// Printing operations
	Print(ctx context.Context, content *PrintContent) error
	Cut(ctx context.Context, cutType CutType) error
	Feed(ctx context.Context, lines int) error

	// Printer status
	GetPaperStatus() PaperStatus
	GetCutterStatus() CutterStatus

	// Cash drawer (if supported)
	OpenDrawer(ctx context.Context, pin int) error
	GetDrawerStatus() DrawerStatus
}

// PaymentDriver extends DeviceDriver for payment terminal operations
type PaymentDriver interface {
	DeviceDriver

	// Payment operations
	ProcessPayment(ctx context.Context, payment *PaymentRequest) (*PaymentResult, error)
	ProcessRefund(ctx context.Context, refund *RefundRequest) (*RefundResult, error)
	CancelTransaction(ctx context.Context, transactionID string) error

	// Card operations
	ReadCard(ctx context.Context) (*CardInfo, error)

	// Terminal management
	EndOfDay(ctx context.Context) (*EODResult, error)
	GetLastTransaction() (*TransactionInfo, error)
}

// ScannerDriver extends DeviceDriver for scanner operations
type ScannerDriver interface {
	DeviceDriver

	// Scanning operations
	StartScan(ctx context.Context, scanType ScanType) error
	StopScan(ctx context.Context) error
	GetLastScan() (*ScanResult, error)

	// Scanner configuration
	SetScanMode(mode ScanMode) error
	SetTriggerMode(mode TriggerMode) error
}

// DisplayDriver extends DeviceDriver for customer display operations
type DisplayDriver interface {
	DeviceDriver

	// Display operations
	DisplayText(ctx context.Context, text *DisplayText) error
	ClearDisplay(ctx context.Context) error
	SetBacklight(ctx context.Context, enabled bool) error

	// Display configuration
	SetBrightness(ctx context.Context, level int) error
	SetContrast(ctx context.Context, level int) error
}
