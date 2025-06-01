// internal/protocol/protocol.go
package protocol

import (
	"context"
	"time"

	"device-service/internal/model"
)

// DeviceProtocol represents a communication protocol to a device
type DeviceProtocol interface {
	// Connection lifecycle
	Open(ctx context.Context) error
	Close() error
	IsOpen() bool

	// Data communication
	Write(ctx context.Context, data []byte) error
	Read(ctx context.Context, maxBytes int) ([]byte, error)

	// Protocol information
	GetProtocolType() model.ConnectionType

	// Health and diagnostics
	Ping(ctx context.Context) error
}

// ProtocolStats provides protocol-level statistics
type ProtocolStats struct {
	BytesWritten   int64         `json:"bytes_written"`
	BytesRead      int64         `json:"bytes_read"`
	OperationCount int64         `json:"operation_count"`
	ErrorCount     int64         `json:"error_count"`
	LastActivity   time.Time     `json:"last_activity"`
	AverageLatency time.Duration `json:"average_latency"`
	IsConnected    bool          `json:"is_connected"`
}
