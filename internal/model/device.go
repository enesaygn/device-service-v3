// internal/model/device.go
package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// DeviceType represents the type of device
type DeviceType string

const (
	DeviceTypePOS          DeviceType = "POS"
	DeviceTypePrinter      DeviceType = "PRINTER"
	DeviceTypeScanner      DeviceType = "SCANNER"
	DeviceTypeCashRegister DeviceType = "CASH_REGISTER"
	DeviceTypeCashDrawer   DeviceType = "CASH_DRAWER"
	DeviceTypeDisplay      DeviceType = "DISPLAY"
)

// DeviceStatus represents the current status of a device
type DeviceStatus string

const (
	DeviceStatusOnline      DeviceStatus = "ONLINE"
	DeviceStatusOffline     DeviceStatus = "OFFLINE"
	DeviceStatusError       DeviceStatus = "ERROR"
	DeviceStatusMaintenance DeviceStatus = "MAINTENANCE"
	DeviceStatusConnecting  DeviceStatus = "CONNECTING"
)

// ConnectionType represents how the device is connected
type ConnectionType string

const (
	ConnectionTypeSerial    ConnectionType = "SERIAL"
	ConnectionTypeUSB       ConnectionType = "USB"
	ConnectionTypeTCP       ConnectionType = "TCP"
	ConnectionTypeBluetooth ConnectionType = "BLUETOOTH"
)

// DeviceBrand represents supported device brands
type DeviceBrand string

const (
	BrandEpson    DeviceBrand = "EPSON"
	BrandStar     DeviceBrand = "STAR"
	BrandIngenico DeviceBrand = "INGENICO"
	BrandPAX      DeviceBrand = "PAX"
	BrandCitizen  DeviceBrand = "CITIZEN"
	BrandBixolon  DeviceBrand = "BIXOLON"
	BrandVerifone DeviceBrand = "VERIFONE"
	BrandGeneric  DeviceBrand = "GENERIC"
)

// Capability represents what a device can do
type Capability string

const (
	CapabilityPrint   Capability = "PRINT"
	CapabilityCut     Capability = "CUT"
	CapabilityDrawer  Capability = "DRAWER"
	CapabilityDisplay Capability = "DISPLAY"
	CapabilityPayment Capability = "PAYMENT"
	CapabilityScan    Capability = "SCAN"
	CapabilityStatus  Capability = "STATUS"
	CapabilityBeep    Capability = "BEEP"
	CapabilityLogo    Capability = "LOGO"
	CapabilityBarcode Capability = "BARCODE"
	CapabilityQR      Capability = "QR"
)

// JSONArray type for PostgreSQL JSONB arrays
type JSONArray []interface{}

func (j *JSONArray) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONArray) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// JSONObject type for PostgreSQL JSONB objects
type JSONObject map[string]interface{}

func (j *JSONObject) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONObject) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Device represents a physical device in the system
type Device struct {
	ID                 uuid.UUID      `json:"id" db:"id"`
	DeviceID           string         `json:"device_id" db:"device_id"`
	DeviceType         DeviceType     `json:"device_type" db:"device_type"`
	Brand              DeviceBrand    `json:"brand" db:"brand"`
	Model              string         `json:"model" db:"model"`
	FirmwareVersion    *string        `json:"firmware_version" db:"firmware_version"`
	ConnectionType     ConnectionType `json:"connection_type" db:"connection_type"`
	ConnectionConfig   JSONObject     `json:"connection_config" db:"connection_config"`
	Capabilities       JSONArray      `json:"capabilities" db:"capabilities"`
	BranchID           uuid.UUID      `json:"branch_id" db:"branch_id"`
	Location           *string        `json:"location" db:"location"`
	Status             DeviceStatus   `json:"status" db:"status"`
	LastPing           *time.Time     `json:"last_ping" db:"last_ping"`
	ErrorInfo          JSONObject     `json:"error_info" db:"error_info"`
	PerformanceMetrics JSONObject     `json:"performance_metrics" db:"performance_metrics"`
	CreatedAt          time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at" db:"updated_at"`
}

// HasCapability checks if device has a specific capability
func (d *Device) HasCapability(capability Capability) bool {
	for _, cap := range d.Capabilities {
		if cap == string(capability) {
			return true
		}
	}
	return false
}

// IsOnline checks if device is currently online
func (d *Device) IsOnline() bool {
	return d.Status == DeviceStatusOnline
}

// ConnectionConfig structures for different connection types
type SerialConfig struct {
	Port     string `json:"port"`
	BaudRate int    `json:"baud_rate"`
	DataBits int    `json:"data_bits"`
	StopBits int    `json:"stop_bits"`
	Parity   string `json:"parity"`
}

type USBConfig struct {
	VendorID  string `json:"vendor_id"`
	ProductID string `json:"product_id"`
	Interface int    `json:"interface"`
}

type TCPConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	SSL  bool   `json:"ssl"`
}

type BluetoothConfig struct {
	MACAddress string `json:"mac_address"`
	PIN        string `json:"pin,omitempty"`
}

// DeviceHealth represents device health metrics
type DeviceHealth struct {
	DeviceID      uuid.UUID  `json:"device_id" db:"device_id"`
	HealthScore   int        `json:"health_score" db:"health_score"`
	ResponseTime  *int       `json:"response_time" db:"response_time"`
	ErrorRate     *float64   `json:"error_rate" db:"error_rate"`
	Uptime        *float64   `json:"uptime" db:"uptime"`
	LastErrorTime *time.Time `json:"last_error_time" db:"last_error_time"`
	RecordedAt    time.Time  `json:"recorded_at" db:"recorded_at"`
}

// PerformanceMetrics structure
type PerformanceMetrics struct {
	AverageResponseTime int     `json:"average_response_time_ms"`
	SuccessRate         float64 `json:"success_rate"`
	UptimePercentage    float64 `json:"uptime_percentage"`
	LastOperationTime   int     `json:"last_operation_time_ms"`
	TotalOperations     int64   `json:"total_operations"`
	ErrorCount          int64   `json:"error_count"`
}

// ErrorInfo structure
type ErrorInfo struct {
	LastError     *string    `json:"last_error,omitempty"`
	ErrorCode     *string    `json:"error_code,omitempty"`
	ErrorTime     *time.Time `json:"error_time,omitempty"`
	RecoveryInfo  *string    `json:"recovery_info,omitempty"`
	ErrorCount    int        `json:"error_count"`
	CriticalError bool       `json:"critical_error"`
}
