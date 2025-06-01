// internal/model/event.go
package model

import (
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of event
type EventType string

const (
	EventDeviceConnected    EventType = "DEVICE_CONNECTED"
	EventDeviceDisconnected EventType = "DEVICE_DISCONNECTED"
	EventDeviceError        EventType = "DEVICE_ERROR"
	EventOperationStarted   EventType = "OPERATION_STARTED"
	EventOperationCompleted EventType = "OPERATION_COMPLETED"
	EventOperationFailed    EventType = "OPERATION_FAILED"
	EventHealthUpdate       EventType = "HEALTH_UPDATE"
	EventStatusChange       EventType = "STATUS_CHANGE"
	EventConfigUpdate       EventType = "CONFIG_UPDATE"
)

// DeviceEvent represents an event in the system
type DeviceEvent struct {
	ID        uuid.UUID  `json:"id"`
	EventType EventType  `json:"event_type"`
	DeviceID  uuid.UUID  `json:"device_id"`
	Data      JSONObject `json:"data"`
	Timestamp time.Time  `json:"timestamp"`
	Source    string     `json:"source"`
	Severity  string     `json:"severity"` // INFO, WARNING, ERROR, CRITICAL
}

// EventData structures for different event types

// DeviceConnectedEventData represents device connection event
type DeviceConnectedEventData struct {
	DeviceInfo     Device         `json:"device_info"`
	ConnectionTime time.Time      `json:"connection_time"`
	PreviousStatus DeviceStatus   `json:"previous_status"`
	ConnectionType ConnectionType `json:"connection_type"`
}

// DeviceErrorEventData represents device error event
type DeviceErrorEventData struct {
	ErrorCode    string    `json:"error_code"`
	ErrorMessage string    `json:"error_message"`
	ErrorTime    time.Time `json:"error_time"`
	Severity     string    `json:"severity"`
	Recovery     bool      `json:"auto_recovery_possible"`
}

// OperationEventData represents operation-related events
type OperationEventData struct {
	OperationID   uuid.UUID       `json:"operation_id"`
	OperationType OperationType   `json:"operation_type"`
	Status        OperationStatus `json:"status"`
	Duration      *int            `json:"duration_ms,omitempty"`
	ErrorMessage  *string         `json:"error_message,omitempty"`
}

// HealthUpdateEventData represents health status updates
type HealthUpdateEventData struct {
	HealthScore   int      `json:"health_score"`
	PreviousScore int      `json:"previous_score"`
	ResponseTime  int      `json:"response_time_ms"`
	ErrorRate     float64  `json:"error_rate"`
	UptimePercent float64  `json:"uptime_percent"`
	Alerts        []string `json:"alerts,omitempty"`
}
