// internal/model/operation.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// OperationType represents the type of operation
type OperationType string

const (
	OperationTypePrint       OperationType = "PRINT"
	OperationTypePayment     OperationType = "PAYMENT"
	OperationTypeScan        OperationType = "SCAN"
	OperationTypeStatusCheck OperationType = "STATUS_CHECK"
	OperationTypeOpenDrawer  OperationType = "OPEN_DRAWER"
	OperationTypeDisplayText OperationType = "DISPLAY_TEXT"
	OperationTypeBeep        OperationType = "BEEP"
	OperationTypeRefund      OperationType = "REFUND"
	OperationTypeCut         OperationType = "CUT"
)

// OperationStatus represents the status of an operation
type OperationStatus string

const (
	OperationStatusPending    OperationStatus = "PENDING"
	OperationStatusProcessing OperationStatus = "PROCESSING"
	OperationStatusSuccess    OperationStatus = "SUCCESS"
	OperationStatusFailed     OperationStatus = "FAILED"
	OperationStatusTimeout    OperationStatus = "TIMEOUT"
	OperationStatusCancelled  OperationStatus = "CANCELLED"
)

// OperationPriority represents operation priority
type OperationPriority int

const (
	PriorityUltraCritical OperationPriority = 1 // Payment responses, emergency stops
	PriorityHigh          OperationPriority = 2 // Receipt printing, cash drawer
	PriorityNormal        OperationPriority = 3 // Status updates, configurations
	PriorityLow           OperationPriority = 4 // Logging, analytics
	PriorityBackground    OperationPriority = 5 // Bulk operations
)

// DeviceOperation represents an operation performed on a device
type DeviceOperation struct {
	ID            uuid.UUID         `json:"id" db:"id"`
	DeviceID      uuid.UUID         `json:"device_id" db:"device_id"`
	OperationType OperationType     `json:"operation_type" db:"operation_type"`
	OperationData JSONObject        `json:"operation_data" db:"operation_data"`
	Priority      OperationPriority `json:"priority" db:"priority"`
	Status        OperationStatus   `json:"status" db:"status"`
	StartedAt     time.Time         `json:"started_at" db:"started_at"`
	CompletedAt   *time.Time        `json:"completed_at" db:"completed_at"`
	DurationMs    *int              `json:"duration_ms" db:"duration_ms"`
	ErrorMessage  *string           `json:"error_message" db:"error_message"`
	RetryCount    int               `json:"retry_count" db:"retry_count"`
	CorrelationID *uuid.UUID        `json:"correlation_id" db:"correlation_id"`
	Result        JSONObject        `json:"result" db:"result"`
	CreatedAt     time.Time         `json:"created_at" db:"created_at"`
}

// IsCompleted checks if operation is completed (success or failed)
func (op *DeviceOperation) IsCompleted() bool {
	return op.Status == OperationStatusSuccess ||
		op.Status == OperationStatusFailed ||
		op.Status == OperationStatusTimeout ||
		op.Status == OperationStatusCancelled
}

// IsCritical checks if operation has critical priority
func (op *DeviceOperation) IsCritical() bool {
	return op.Priority <= PriorityHigh
}

// Operation data structures for different operation types

// PrintOperationData represents print operation data
type PrintOperationData struct {
	Content     string            `json:"content"`
	ContentType string            `json:"content_type"` // TEXT, HTML, ESC_POS
	Copies      int               `json:"copies"`
	Cut         bool              `json:"cut"`
	OpenDrawer  bool              `json:"open_drawer"`
	Logo        bool              `json:"logo"`
	Options     map[string]string `json:"options,omitempty"`
}

// PaymentOperationData represents payment operation data
type PaymentOperationData struct {
	Amount        decimal.Decimal   `json:"amount"`
	Currency      string            `json:"currency"`
	PaymentMethod string            `json:"payment_method"` // CARD, CASH, CONTACTLESS
	OrderID       uuid.UUID         `json:"order_id"`
	Reference     string            `json:"reference"`
	Timeout       int               `json:"timeout_seconds"`
	Options       map[string]string `json:"options,omitempty"`
}

// ScanOperationData represents scan operation data
type ScanOperationData struct {
	ScanType string `json:"scan_type"` // BARCODE, QR
	Timeout  int    `json:"timeout_seconds"`
}

// DisplayOperationData represents display operation data
type DisplayOperationData struct {
	Line1    string `json:"line1"`
	Line2    string `json:"line2,omitempty"`
	Duration int    `json:"duration_seconds"`
	Clear    bool   `json:"clear_after"`
}

// OfflineOperation represents operations queued for offline devices
type OfflineOperation struct {
	ID              uuid.UUID         `json:"id" db:"id"`
	DeviceID        uuid.UUID         `json:"device_id" db:"device_id"`
	OperationType   OperationType     `json:"operation_type" db:"operation_type"`
	OperationData   JSONObject        `json:"operation_data" db:"operation_data"`
	CreatedAt       time.Time         `json:"created_at" db:"created_at"`
	SyncStatus      string            `json:"sync_status" db:"sync_status"`
	SyncAttempts    int               `json:"sync_attempts" db:"sync_attempts"`
	LastSyncAttempt *time.Time        `json:"last_sync_attempt" db:"last_sync_attempt"`
	Priority        OperationPriority `json:"priority" db:"priority"`
	ExpiresAt       *time.Time        `json:"expires_at" db:"expires_at"`
}
