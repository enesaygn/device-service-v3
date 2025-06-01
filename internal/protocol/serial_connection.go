// internal/protocol/serial_connection.go
package protocol

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"go.bug.st/serial"
	"go.uber.org/zap"

	"device-service/internal/model"
)

// SerialConnection implements DeviceProtocol for serial connections
type SerialConnection struct {
	config *SerialConfig
	port   serial.Port
	logger *zap.Logger
	mutex  sync.RWMutex
	isOpen bool
	stats  *ProtocolStats
}

// NewSerialConnection creates a new serial connection
func NewSerialConnection(config *SerialConfig, logger *zap.Logger) DeviceProtocol {
	return &SerialConnection{
		config: config,
		logger: logger.With(
			zap.String("protocol", "serial"),
			zap.String("port", config.Port),
		),
		stats: &ProtocolStats{
			IsConnected: false,
		},
	}
}

// Open opens the serial connection
func (sc *SerialConnection) Open(ctx context.Context) error {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	if sc.isOpen {
		return nil
	}

	sc.logger.Info("Opening serial port",
		zap.String("port", sc.config.Port),
		zap.Int("baud_rate", sc.config.BaudRate),
	)

	// Configure serial port mode
	mode := &serial.Mode{
		BaudRate: sc.config.BaudRate,
		DataBits: sc.config.DataBits,
		StopBits: serial.StopBits(sc.config.StopBits),
	}

	// Set parity
	switch sc.config.Parity {
	case "none":
		mode.Parity = serial.NoParity
	case "odd":
		mode.Parity = serial.OddParity
	case "even":
		mode.Parity = serial.EvenParity
	default:
		mode.Parity = serial.NoParity
	}

	// Open port
	port, err := serial.Open(sc.config.Port, mode)
	if err != nil {
		sc.logger.Error("Failed to open serial port", zap.Error(err))
		return fmt.Errorf("failed to open serial port: %w", err)
	}

	// Set read timeout
	if err := port.SetReadTimeout(sc.config.Timeout); err != nil {
		port.Close()
		return fmt.Errorf("failed to set read timeout: %w", err)
	}

	sc.port = port
	sc.isOpen = true
	sc.stats.IsConnected = true
	sc.stats.LastActivity = time.Now()

	sc.logger.Info("Serial port opened successfully")
	return nil
}

// Close closes the serial connection
func (sc *SerialConnection) Close() error {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	if !sc.isOpen || sc.port == nil {
		return nil
	}

	if err := sc.port.Close(); err != nil {
		sc.logger.Error("Failed to close serial port", zap.Error(err))
		return fmt.Errorf("failed to close serial port: %w", err)
	}

	sc.port = nil
	sc.isOpen = false
	sc.stats.IsConnected = false

	sc.logger.Info("Serial port closed successfully")
	return nil
}

// IsOpen returns whether the connection is open
func (sc *SerialConnection) IsOpen() bool {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()
	return sc.isOpen && sc.port != nil
}

// Write writes data to the serial port
func (sc *SerialConnection) Write(ctx context.Context, data []byte) error {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	if !sc.isOpen || sc.port == nil {
		return fmt.Errorf("serial port not open")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	startTime := time.Now()
	n, err := sc.port.Write(data)
	if err != nil {
		sc.stats.ErrorCount++
		sc.logger.Error("Serial write failed", zap.Error(err))
		return fmt.Errorf("failed to write to serial port: %w", err)
	}

	if n != len(data) {
		return fmt.Errorf("incomplete write: wrote %d of %d bytes", n, len(data))
	}

	// Update statistics
	duration := time.Since(startTime)
	sc.stats.BytesWritten += int64(len(data))
	sc.stats.OperationCount++
	sc.stats.LastActivity = time.Now()
	sc.updateAverageLatency(duration)

	sc.logger.Debug("Serial write completed", zap.Int("bytes", len(data)))
	return nil
}

// Read reads data from the serial port
func (sc *SerialConnection) Read(ctx context.Context, maxBytes int) ([]byte, error) {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	if !sc.isOpen || sc.port == nil {
		return nil, fmt.Errorf("serial port not open")
	}

	buffer := make([]byte, maxBytes)

	done := make(chan struct {
		data []byte
		err  error
	}, 1)

	go func() {
		n, err := sc.port.Read(buffer)
		result := struct {
			data []byte
			err  error
		}{}

		if err != nil {
			if err == io.EOF {
				result.data = buffer[:n]
			} else {
				result.err = fmt.Errorf("failed to read from serial port: %w", err)
			}
		} else {
			result.data = make([]byte, n)
			copy(result.data, buffer[:n])
		}
		done <- result
	}()

	select {
	case result := <-done:
		if result.err != nil {
			sc.stats.ErrorCount++
			return nil, result.err
		}

		sc.stats.BytesRead += int64(len(result.data))
		sc.stats.OperationCount++
		sc.stats.LastActivity = time.Now()

		return result.data, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetProtocolType returns the protocol type
func (sc *SerialConnection) GetProtocolType() model.ConnectionType {
	return model.ConnectionTypeSerial
}

// Ping tests the connection
func (sc *SerialConnection) Ping(ctx context.Context) error {
	if !sc.IsOpen() {
		return fmt.Errorf("serial port not open")
	}

	// Simple ping with status request
	pingData := []byte{0x10, 0x04, 0x01}
	return sc.Write(ctx, pingData)
}

// updateAverageLatency updates the running average latency
func (sc *SerialConnection) updateAverageLatency(newLatency time.Duration) {
	if sc.stats.AverageLatency == 0 {
		sc.stats.AverageLatency = newLatency
	} else {
		sc.stats.AverageLatency = (sc.stats.AverageLatency + newLatency) / 2
	}
}
