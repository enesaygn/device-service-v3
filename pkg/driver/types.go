// pkg/driver/types.go
package driver

import (
	"time"

	"device-service/internal/model"
)

// Core data structures

// DeviceInfo contains basic device information
type DeviceInfo struct {
	Brand           model.DeviceBrand    `json:"brand"`
	Model           string               `json:"model"`
	SerialNumber    string               `json:"serial_number"`
	FirmwareVersion string               `json:"firmware_version"`
	HardwareVersion string               `json:"hardware_version"`
	Capabilities    []model.Capability   `json:"capabilities"`
	ConnectionType  model.ConnectionType `json:"connection_type"`
	Manufacturer    string               `json:"manufacturer"`
}

// DeviceStatus represents current device status
type DeviceStatus struct {
	Status       model.DeviceStatus `json:"status"`
	IsReady      bool               `json:"is_ready"`
	HasError     bool               `json:"has_error"`
	ErrorCode    string             `json:"error_code,omitempty"`
	ErrorMessage string             `json:"error_message,omitempty"`
	LastResponse time.Time          `json:"last_response"`
	Temperature  *float64           `json:"temperature,omitempty"`
	Voltage      *float64           `json:"voltage,omitempty"`
}

// OperationResult represents the result of a device operation
type OperationResult struct {
	Success      bool                   `json:"success"`
	ErrorCode    string                 `json:"error_code,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Data         map[string]interface{} `json:"data,omitempty"`
	Duration     string                 `json:"duration"`
	Timestamp    time.Time              `json:"timestamp"`
}

// HealthMetrics contains device health information
type HealthMetrics struct {
	HealthScore     int           `json:"health_score"` // 0-100
	ResponseTime    time.Duration `json:"response_time"`
	SuccessRate     float64       `json:"success_rate"` // 0.0-1.0
	ErrorCount      int64         `json:"error_count"`
	TotalOperations int64         `json:"total_operations"`
	UptimePercent   float64       `json:"uptime_percent"`
	LastErrorTime   *time.Time    `json:"last_error_time,omitempty"`
	LastSuccessTime *time.Time    `json:"last_success_time,omitempty"`
}

// EventHandler handles device events
type EventHandler interface {
	OnDeviceConnected(deviceID string)
	OnDeviceDisconnected(deviceID string, reason string)
	OnDeviceError(deviceID string, err error)
	OnOperationCompleted(deviceID string, operationID string, result *OperationResult)
	OnStatusChanged(deviceID string, oldStatus, newStatus model.DeviceStatus)
}

// Printer-specific types

// PrintContent represents content to be printed
type PrintContent struct {
	Type     ContentType            `json:"type"`
	Data     interface{}            `json:"data"`
	Options  map[string]interface{} `json:"options,omitempty"`
	Encoding string                 `json:"encoding,omitempty"`
}

// ContentType defines the type of print content
type ContentType string

const (
	ContentTypeText    ContentType = "TEXT"
	ContentTypeHTML    ContentType = "HTML"
	ContentTypeESCPOS  ContentType = "ESC_POS"
	ContentTypeImage   ContentType = "IMAGE"
	ContentTypeReceipt ContentType = "RECEIPT"
)

// CutType defines paper cutting options
type CutType string

const (
	CutTypeFull    CutType = "FULL"
	CutTypePartial CutType = "PARTIAL"
	CutTypeNone    CutType = "NONE"
)

// PaperStatus represents paper status
type PaperStatus string

const (
	PaperStatusOK      PaperStatus = "OK"
	PaperStatusLow     PaperStatus = "LOW"
	PaperStatusOut     PaperStatus = "OUT"
	PaperStatusJam     PaperStatus = "JAM"
	PaperStatusUnknown PaperStatus = "UNKNOWN"
)

// CutterStatus represents cutter status
type CutterStatus string

const (
	CutterStatusOK      CutterStatus = "OK"
	CutterStatusError   CutterStatus = "ERROR"
	CutterStatusUnknown CutterStatus = "UNKNOWN"
)

// DrawerStatus represents cash drawer status
type DrawerStatus string

const (
	DrawerStatusClosed  DrawerStatus = "CLOSED"
	DrawerStatusOpen    DrawerStatus = "OPEN"
	DrawerStatusUnknown DrawerStatus = "UNKNOWN"
)

// Payment-specific types

// PaymentRequest represents a payment request
type PaymentRequest struct {
	Amount        float64                `json:"amount"`
	Currency      string                 `json:"currency"`
	PaymentMethod PaymentMethod          `json:"payment_method"`
	Reference     string                 `json:"reference"`
	Description   string                 `json:"description,omitempty"`
	Timeout       time.Duration          `json:"timeout"`
	Options       map[string]interface{} `json:"options,omitempty"`
}

// PaymentMethod defines payment methods
type PaymentMethod string

const (
	PaymentMethodCard        PaymentMethod = "CARD"
	PaymentMethodContactless PaymentMethod = "CONTACTLESS"
	PaymentMethodCash        PaymentMethod = "CASH"
	PaymentMethodMobile      PaymentMethod = "MOBILE"
)

// PaymentResult represents payment result
type PaymentResult struct {
	Success         bool                   `json:"success"`
	TransactionID   string                 `json:"transaction_id"`
	AuthCode        string                 `json:"auth_code,omitempty"`
	ReferenceNumber string                 `json:"reference_number,omitempty"`
	CardInfo        *CardInfo              `json:"card_info,omitempty"`
	Amount          float64                `json:"amount"`
	Currency        string                 `json:"currency"`
	Timestamp       time.Time              `json:"timestamp"`
	ErrorCode       string                 `json:"error_code,omitempty"`
	ErrorMessage    string                 `json:"error_message,omitempty"`
	Receipt         string                 `json:"receipt,omitempty"`
	AdditionalData  map[string]interface{} `json:"additional_data,omitempty"`
}

// RefundRequest represents a refund request
type RefundRequest struct {
	OriginalTransactionID string                 `json:"original_transaction_id"`
	Amount                float64                `json:"amount"`
	Currency              string                 `json:"currency"`
	Reference             string                 `json:"reference"`
	Reason                string                 `json:"reason,omitempty"`
	Options               map[string]interface{} `json:"options,omitempty"`
}

// RefundResult represents refund result
type RefundResult struct {
	Success      bool      `json:"success"`
	RefundID     string    `json:"refund_id"`
	Amount       float64   `json:"amount"`
	Currency     string    `json:"currency"`
	Timestamp    time.Time `json:"timestamp"`
	ErrorCode    string    `json:"error_code,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// CardInfo represents card information (without sensitive data)
type CardInfo struct {
	Last4Digits string `json:"last_4_digits"`
	CardType    string `json:"card_type"`    // VISA, MASTERCARD, etc.
	PaymentType string `json:"payment_type"` // CREDIT, DEBIT
	BankCode    string `json:"bank_code,omitempty"`
	BankName    string `json:"bank_name,omitempty"`
	ExpiryMonth string `json:"expiry_month,omitempty"`
	ExpiryYear  string `json:"expiry_year,omitempty"`
}

// TransactionInfo represents transaction information
type TransactionInfo struct {
	TransactionID string        `json:"transaction_id"`
	Amount        float64       `json:"amount"`
	Currency      string        `json:"currency"`
	PaymentMethod PaymentMethod `json:"payment_method"`
	Status        string        `json:"status"`
	Timestamp     time.Time     `json:"timestamp"`
}

// EODResult represents end-of-day result
type EODResult struct {
	TotalTransactions int       `json:"total_transactions"`
	TotalAmount       float64   `json:"total_amount"`
	Currency          string    `json:"currency"`
	Timestamp         time.Time `json:"timestamp"`
}

// Scanner-specific types

// ScanType defines scan types
type ScanType string

const (
	ScanTypeBarcode    ScanType = "BARCODE"
	ScanTypeQR         ScanType = "QR"
	ScanTypeDataMatrix ScanType = "DATA_MATRIX"
	ScanTypePDF417     ScanType = "PDF417"
)

// ScanMode defines scanning modes
type ScanMode string

const (
	ScanModeContinuous ScanMode = "CONTINUOUS"
	ScanModeSingle     ScanMode = "SINGLE"
	ScanModeManual     ScanMode = "MANUAL"
)

// TriggerMode defines trigger modes
type TriggerMode string

const (
	TriggerModeAuto   TriggerMode = "AUTO"
	TriggerModeManual TriggerMode = "MANUAL"
	TriggerModeLevel  TriggerMode = "LEVEL"
)

// ScanResult represents scan result
type ScanResult struct {
	Success      bool      `json:"success"`
	Data         string    `json:"data"`
	Type         ScanType  `json:"type"`
	Quality      int       `json:"quality"` // 0-100
	Timestamp    time.Time `json:"timestamp"`
	ErrorCode    string    `json:"error_code,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// Display-specific types

// DisplayText represents text to display
type DisplayText struct {
	Line1      string                 `json:"line1"`
	Line2      string                 `json:"line2,omitempty"`
	Duration   time.Duration          `json:"duration,omitempty"`
	ClearAfter bool                   `json:"clear_after"`
	Options    map[string]interface{} `json:"options,omitempty"`
}
