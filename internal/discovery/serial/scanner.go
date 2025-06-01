// 📁 internal/discovery/serial/scanner.go - Serial Scanner Implementation
package serial

// import (
// 	"context"
// 	"fmt"
// 	"runtime"
// 	"time"

// 	"go.bug.st/serial"
// 	"go.uber.org/zap"

// 	"device-service/internal/discovery"
// 	"device-service/internal/model"
// )

// // Scanner implements serial port device scanning
// type Scanner struct {
// 	logger  *zap.Logger
// 	config  *Config
// 	timeout time.Duration
// }

// // Config for serial scanner
// type Config struct {
// 	ScanTimeout    time.Duration `json:"scan_timeout"`
// 	BaudRates      []int         `json:"baud_rates"`
// 	TestCommands   bool          `json:"test_commands"`
// 	PortPatterns   []string      `json:"port_patterns"`
// }

// // NewScanner creates a new serial scanner
// func NewScanner(logger *zap.Logger, config *Config) *Scanner {
// 	if config == nil {
// 		config = &Config{
// 			ScanTimeout:  30 * time.Second,
// 			BaudRates:    []int{9600, 19200, 38400, 115200},
// 			TestCommands: true,
// 			PortPatterns: getDefaultPortPatterns(),
// 		}
// 	}

// 	return &Scanner{
// 		logger:  logger.With(zap.String("scanner", "serial")),
// 		config:  config,
// 		timeout: config.ScanTimeout,
// 	}
// }

// // GetScannerType returns scanner type
// func (s *Scanner) GetScannerType() string {
// 	return "serial"
// }

// // IsAvailable checks if serial scanning is available
// func (s *Scanner) IsAvailable() bool {
// 	// Serial port scanning tüm platformlarda mevcut
// 	return true
// }

// // Scan performs serial port device discovery
// func (s *Scanner) Scan(ctx context.Context) ([]*discovery.DiscoveredDevice, error) {
// 	s.logger.Info("Starting serial port scan")

// 	// Get available ports
// 	ports, err := serial.GetPortsList()
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get serial ports: %w", err)
// 	}

// 	if len(ports) == 0 {
// 		s.logger.Info("No serial ports found")
// 		return []*discovery.DiscoveredDevice{}, nil
// 	}

// 	s.logger.Info("Found serial ports", zap.Strings("ports", ports))

// 	// Filter ports by patterns
// 	filteredPorts := s.filterPorts(ports)

// 	// Test each port
// 	var discovered []*discovery.DiscoveredDevice
// 	for _, port := range filteredPorts {
// 		select {
// 		case <-ctx.Done():
// 			return discovered, ctx.Err()
// 		default:
// 		}

// 		if device := s.testPort(ctx, port); device != nil {
// 			discovered = append(discovered, device)
// 		}
// 	}

// 	s.logger.Info("Serial scan completed", zap.Int("devices_found", len(discovered)))
// 	return discovered, nil
// }
