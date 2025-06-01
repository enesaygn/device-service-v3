// internal/protocol/tcp_connection.go
package protocol

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"

	"device-service/internal/model"
)

// TCPConnection implements DeviceProtocol for TCP connections
type TCPConnection struct {
	config *TCPConfig
	conn   net.Conn
	logger *zap.Logger
	mutex  sync.RWMutex
	isOpen bool
	stats  *ProtocolStats
}

// NewTCPConnection creates a new TCP connection
func NewTCPConnection(config *TCPConfig, logger *zap.Logger) DeviceProtocol {
	return &TCPConnection{
		config: config,
		logger: logger.With(
			zap.String("protocol", "tcp"),
			zap.String("host", config.Host),
			zap.Int("port", config.Port),
		),
		stats: &ProtocolStats{
			IsConnected: false,
		},
	}
}

// Open opens the TCP connection
func (tc *TCPConnection) Open(ctx context.Context) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	if tc.isOpen {
		return nil
	}

	tc.logger.Info("Opening TCP connection",
		zap.String("host", tc.config.Host),
		zap.Int("port", tc.config.Port),
		zap.Bool("ssl", tc.config.SSL),
	)

	// Create dialer with timeout
	dialer := &net.Dialer{
		Timeout:   tc.config.Timeout,
		KeepAlive: 30 * time.Second,
	}

	address := fmt.Sprintf("%s:%d", tc.config.Host, tc.config.Port)

	var conn net.Conn
	var err error

	if tc.config.SSL {
		// SSL/TLS connection
		tlsConfig := &tls.Config{
			ServerName:         tc.config.Host,
			InsecureSkipVerify: false, // Set to true for testing only
		}
		conn, err = tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
	} else {
		// Plain TCP connection
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}

	if err != nil {
		tc.logger.Error("Failed to open TCP connection", zap.Error(err))
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	// Set connection options
	if tcpConn, ok := conn.(*net.TCPConn); ok && tc.config.KeepAlive {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	// Set timeouts
	if tc.config.ReadTimeout > 0 {
		conn.SetReadDeadline(time.Now().Add(tc.config.ReadTimeout))
	}
	if tc.config.WriteTimeout > 0 {
		conn.SetWriteDeadline(time.Now().Add(tc.config.WriteTimeout))
	}

	tc.conn = conn
	tc.isOpen = true
	tc.stats.IsConnected = true
	tc.stats.LastActivity = time.Now()

	tc.logger.Info("TCP connection opened successfully")
	return nil
}

// Close closes the TCP connection
func (tc *TCPConnection) Close() error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	if !tc.isOpen || tc.conn == nil {
		return nil
	}

	if err := tc.conn.Close(); err != nil {
		tc.logger.Error("Failed to close TCP connection", zap.Error(err))
		return fmt.Errorf("failed to close TCP connection: %w", err)
	}

	tc.conn = nil
	tc.isOpen = false
	tc.stats.IsConnected = false

	tc.logger.Info("TCP connection closed successfully")
	return nil
}

// IsOpen returns whether the connection is open
func (tc *TCPConnection) IsOpen() bool {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()
	return tc.isOpen && tc.conn != nil
}

// Write writes data to the TCP connection
func (tc *TCPConnection) Write(ctx context.Context, data []byte) error {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()

	if !tc.isOpen || tc.conn == nil {
		return fmt.Errorf("TCP connection not open")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Set write deadline
	if tc.config.WriteTimeout > 0 {
		tc.conn.SetWriteDeadline(time.Now().Add(tc.config.WriteTimeout))
	}

	startTime := time.Now()
	n, err := tc.conn.Write(data)
	if err != nil {
		tc.stats.ErrorCount++
		tc.logger.Error("TCP write failed", zap.Error(err))
		return fmt.Errorf("failed to write to TCP connection: %w", err)
	}

	if n != len(data) {
		return fmt.Errorf("incomplete write: wrote %d of %d bytes", n, len(data))
	}

	// Update statistics
	duration := time.Since(startTime)
	tc.stats.BytesWritten += int64(len(data))
	tc.stats.OperationCount++
	tc.stats.LastActivity = time.Now()
	tc.updateAverageLatency(duration)

	tc.logger.Debug("TCP write completed", zap.Int("bytes", len(data)))
	return nil
}

// Read reads data from the TCP connection
func (tc *TCPConnection) Read(ctx context.Context, maxBytes int) ([]byte, error) {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()

	if !tc.isOpen || tc.conn == nil {
		return nil, fmt.Errorf("TCP connection not open")
	}

	// Set read deadline
	if tc.config.ReadTimeout > 0 {
		tc.conn.SetReadDeadline(time.Now().Add(tc.config.ReadTimeout))
	}

	buffer := make([]byte, maxBytes)

	done := make(chan struct {
		data []byte
		err  error
	}, 1)

	go func() {
		n, err := tc.conn.Read(buffer)
		result := struct {
			data []byte
			err  error
		}{}

		if err != nil {
			result.err = fmt.Errorf("failed to read from TCP connection: %w", err)
		} else {
			result.data = make([]byte, n)
			copy(result.data, buffer[:n])
		}
		done <- result
	}()

	select {
	case result := <-done:
		if result.err != nil {
			tc.stats.ErrorCount++
			return nil, result.err
		}

		tc.stats.BytesRead += int64(len(result.data))
		tc.stats.OperationCount++
		tc.stats.LastActivity = time.Now()

		return result.data, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetProtocolType returns the protocol type
func (tc *TCPConnection) GetProtocolType() model.ConnectionType {
	return model.ConnectionTypeTCP
}

// Ping tests the connection
func (tc *TCPConnection) Ping(ctx context.Context) error {
	if !tc.IsOpen() {
		return fmt.Errorf("TCP connection not open")
	}

	// Simple ping with status request
	pingData := []byte{0x10, 0x04, 0x01}
	return tc.Write(ctx, pingData)
}

// updateAverageLatency updates the running average latency
func (tc *TCPConnection) updateAverageLatency(newLatency time.Duration) {
	if tc.stats.AverageLatency == 0 {
		tc.stats.AverageLatency = newLatency
	} else {
		tc.stats.AverageLatency = (tc.stats.AverageLatency + newLatency) / 2
	}
}
