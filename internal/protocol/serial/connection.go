// internal/protocol/serial/connection.go
package serial

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"go.bug.st/serial"
	"go.uber.org/zap"
)

// Connection represents a serial port connection
type Connection struct {
	config *Config
	port   serial.Port
	logger *zap.Logger
	mutex  sync.RWMutex
	isOpen bool
}

// Config represents serial port configuration
type Config struct {
	Port     string        `json:"port"`
	BaudRate int           `json:"baud_rate"`
	DataBits int           `json:"data_bits"`
	StopBits int           `json:"stop_bits"`
	Parity   string        `json:"parity"`
	Timeout  time.Duration `json:"timeout"`
}

// NewConnection creates a new serial connection
func NewConnection(config *Config, logger *zap.Logger) (*Connection, error) {
	if config.Port == "" {
		return nil, fmt.Errorf("port is required")
	}

	return &Connection{
		config: config,
		logger: logger,
	}, nil
}

// Open opens the serial connection
func (c *Connection) Open(ctx context.Context) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.isOpen {
		return nil
	}

	// Configure serial port
	mode := &serial.Mode{
		BaudRate: c.config.BaudRate,
		DataBits: c.config.DataBits,
		StopBits: serial.StopBits(c.config.StopBits),
	}

	// Set parity
	switch c.config.Parity {
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
	port, err := serial.Open(c.config.Port, mode)
	if err != nil {
		c.logger.Error("Failed to open serial port",
			zap.Error(err),
			zap.String("port", c.config.Port),
		)
		return fmt.Errorf("failed to open serial port: %w", err)
	}

	// Set timeouts
	if err := port.SetReadTimeout(c.config.Timeout); err != nil {
		port.Close()
		return fmt.Errorf("failed to set read timeout: %w", err)
	}

	c.port = port
	c.isOpen = true

	c.logger.Info("Serial port opened successfully",
		zap.String("port", c.config.Port),
		zap.Int("baud_rate", c.config.BaudRate),
	)

	return nil
}

// Close closes the serial connection
func (c *Connection) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.isOpen || c.port == nil {
		return nil
	}

	if err := c.port.Close(); err != nil {
		c.logger.Error("Failed to close serial port", zap.Error(err))
		return fmt.Errorf("failed to close serial port: %w", err)
	}

	c.port = nil
	c.isOpen = false

	c.logger.Info("Serial port closed", zap.String("port", c.config.Port))
	return nil
}

// Write writes data to the serial port
func (c *Connection) Write(ctx context.Context, data []byte) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if !c.isOpen || c.port == nil {
		return fmt.Errorf("port not open")
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	n, err := c.port.Write(data)
	if err != nil {
		c.logger.Error("Failed to write to serial port",
			zap.Error(err),
			zap.Int("bytes_to_write", len(data)),
		)
		return fmt.Errorf("failed to write to serial port: %w", err)
	}

	if n != len(data) {
		return fmt.Errorf("incomplete write: wrote %d of %d bytes", n, len(data))
	}

	c.logger.Debug("Data written to serial port",
		zap.Int("bytes_written", n),
		zap.Binary("data", data),
	)

	return nil
}

// Read reads data from the serial port
func (c *Connection) Read(ctx context.Context, maxBytes int) ([]byte, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if !c.isOpen || c.port == nil {
		return nil, fmt.Errorf("port not open")
	}

	// Create buffer
	buffer := make([]byte, maxBytes)

	// Read with context cancellation support
	done := make(chan struct{})
	var n int
	var err error

	go func() {
		defer close(done)
		n, err = c.port.Read(buffer)
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
		if err != nil {
			if err == io.EOF {
				return buffer[:n], nil
			}
			c.logger.Error("Failed to read from serial port", zap.Error(err))
			return nil, fmt.Errorf("failed to read from serial port: %w", err)
		}

		result := make([]byte, n)
		copy(result, buffer[:n])

		c.logger.Debug("Data read from serial port",
			zap.Int("bytes_read", n),
			zap.Binary("data", result),
		)

		return result, nil
	}
}

// IsOpen returns whether the connection is open
func (c *Connection) IsOpen() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.isOpen
}

// GetConfig returns the connection configuration
func (c *Connection) GetConfig() *Config {
	return c.config
}
