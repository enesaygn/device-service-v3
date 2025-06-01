// internal/protocol/factory.go
package protocol

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"device-service/internal/model"
)

// CreateProtocol creates a protocol based on connection type and configuration
func CreateProtocol(connectionType model.ConnectionType, config map[string]interface{}, logger *zap.Logger) (DeviceProtocol, error) {
	switch connectionType {
	case model.ConnectionTypeSerial:
		return createSerialProtocol(config, logger)
	case model.ConnectionTypeUSB:
		return createUSBProtocol(config, logger)
	case model.ConnectionTypeTCP:
		return createTCPProtocol(config, logger)
	case model.ConnectionTypeBluetooth:
		return createBluetoothProtocol(config, logger)
	default:
		return nil, fmt.Errorf("unsupported protocol type: %s", connectionType)
	}
}

// createSerialProtocol creates a serial protocol
func createSerialProtocol(config map[string]interface{}, logger *zap.Logger) (DeviceProtocol, error) {
	serialConfig := &SerialConfig{
		BaudRate: 9600,
		DataBits: 8,
		StopBits: 1,
		Parity:   "none",
		Timeout:  5 * time.Second,
	}

	// Parse port
	if port, ok := config["port"].(string); ok {
		serialConfig.Port = port
	} else {
		return nil, fmt.Errorf("serial port is required")
	}

	// Parse baud rate
	if baudRate, ok := config["baud_rate"]; ok {
		switch v := baudRate.(type) {
		case float64:
			serialConfig.BaudRate = int(v)
		case int:
			serialConfig.BaudRate = v
		}
	}

	// Parse data bits
	if dataBits, ok := config["data_bits"]; ok {
		switch v := dataBits.(type) {
		case float64:
			serialConfig.DataBits = int(v)
		case int:
			serialConfig.DataBits = v
		}
	}

	// Parse stop bits
	if stopBits, ok := config["stop_bits"]; ok {
		switch v := stopBits.(type) {
		case float64:
			serialConfig.StopBits = int(v)
		case int:
			serialConfig.StopBits = v
		}
	}

	// Parse parity
	if parity, ok := config["parity"].(string); ok {
		serialConfig.Parity = parity
	}

	// Parse timeout
	if timeout, ok := config["timeout"].(string); ok {
		if dur, err := time.ParseDuration(timeout); err == nil {
			serialConfig.Timeout = dur
		}
	}

	logger.Info("Creating serial protocol",
		zap.String("port", serialConfig.Port),
		zap.Int("baud_rate", serialConfig.BaudRate),
	)

	return NewSerialConnection(serialConfig, logger), nil
}

// createUSBProtocol creates a USB protocol
func createUSBProtocol(config map[string]interface{}, logger *zap.Logger) (DeviceProtocol, error) {
	usbConfig := &USBConfig{
		Interface: 0,
		Endpoint:  1,
		Timeout:   5 * time.Second,
	}

	// Parse vendor ID
	if vendorID, ok := config["vendor_id"].(string); ok {
		usbConfig.VendorID = vendorID
	} else {
		return nil, fmt.Errorf("USB vendor_id is required")
	}

	// Parse product ID
	if productID, ok := config["product_id"].(string); ok {
		usbConfig.ProductID = productID
	} else {
		return nil, fmt.Errorf("USB product_id is required")
	}

	// Parse interface
	if intf, ok := config["interface"]; ok {
		switch v := intf.(type) {
		case float64:
			usbConfig.Interface = int(v)
		case int:
			usbConfig.Interface = v
		}
	}

	// Parse endpoint
	if endpoint, ok := config["endpoint"]; ok {
		switch v := endpoint.(type) {
		case float64:
			usbConfig.Endpoint = int(v)
		case int:
			usbConfig.Endpoint = v
		}
	}

	// Parse serial number
	if serialNumber, ok := config["serial_number"].(string); ok {
		usbConfig.SerialNumber = serialNumber
	}

	// Parse timeout
	if timeout, ok := config["timeout"].(string); ok {
		if dur, err := time.ParseDuration(timeout); err == nil {
			usbConfig.Timeout = dur
		}
	}

	logger.Info("Creating USB protocol",
		zap.String("vendor_id", usbConfig.VendorID),
		zap.String("product_id", usbConfig.ProductID),
		zap.Int("interface", usbConfig.Interface),
	)

	return NewUSBConnection(usbConfig, logger), nil
}

// createTCPProtocol creates a TCP protocol
func createTCPProtocol(config map[string]interface{}, logger *zap.Logger) (DeviceProtocol, error) {
	tcpConfig := &TCPConfig{
		Port:         9100, // Default printer port
		SSL:          false,
		KeepAlive:    true,
		BufferSize:   4096,
		Timeout:      10 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Parse host
	if host, ok := config["host"].(string); ok {
		tcpConfig.Host = host
	} else {
		return nil, fmt.Errorf("TCP host is required")
	}

	// Parse port
	if port, ok := config["port"]; ok {
		switch v := port.(type) {
		case float64:
			tcpConfig.Port = int(v)
		case int:
			tcpConfig.Port = v
		}
	}

	// Parse SSL
	if ssl, ok := config["ssl"].(bool); ok {
		tcpConfig.SSL = ssl
	}

	// Parse keep alive
	if keepAlive, ok := config["keep_alive"].(bool); ok {
		tcpConfig.KeepAlive = keepAlive
	}

	// Parse buffer size
	if bufferSize, ok := config["buffer_size"]; ok {
		switch v := bufferSize.(type) {
		case float64:
			tcpConfig.BufferSize = int(v)
		case int:
			tcpConfig.BufferSize = v
		}
	}

	// Parse timeout
	if timeout, ok := config["timeout"].(string); ok {
		if dur, err := time.ParseDuration(timeout); err == nil {
			tcpConfig.Timeout = dur
		}
	}

	// Parse read timeout
	if readTimeout, ok := config["read_timeout"].(string); ok {
		if dur, err := time.ParseDuration(readTimeout); err == nil {
			tcpConfig.ReadTimeout = dur
		}
	}

	// Parse write timeout
	if writeTimeout, ok := config["write_timeout"].(string); ok {
		if dur, err := time.ParseDuration(writeTimeout); err == nil {
			tcpConfig.WriteTimeout = dur
		}
	}

	logger.Info("Creating TCP protocol",
		zap.String("host", tcpConfig.Host),
		zap.Int("port", tcpConfig.Port),
		zap.Bool("ssl", tcpConfig.SSL),
	)

	return NewTCPConnection(tcpConfig, logger), nil
}

// createBluetoothProtocol creates a Bluetooth protocol
func createBluetoothProtocol(config map[string]interface{}, logger *zap.Logger) (DeviceProtocol, error) {
	// TODO: Implement Bluetooth protocol
	return nil, fmt.Errorf("Bluetooth protocol not implemented yet")
}

// ValidateConfig validates configuration for a specific protocol type
func ValidateConfig(connectionType model.ConnectionType, config map[string]interface{}) error {
	switch connectionType {
	case model.ConnectionTypeSerial:
		return validateSerialConfig(config)
	case model.ConnectionTypeUSB:
		return validateUSBConfig(config)
	case model.ConnectionTypeTCP:
		return validateTCPConfig(config)
	case model.ConnectionTypeBluetooth:
		return validateBluetoothConfig(config)
	default:
		return fmt.Errorf("unsupported connection type: %s", connectionType)
	}
}

// validateSerialConfig validates serial configuration
func validateSerialConfig(config map[string]interface{}) error {
	if _, ok := config["port"].(string); !ok {
		return fmt.Errorf("serial port is required")
	}

	if baudRate, ok := config["baud_rate"]; ok {
		var rate int
		switch v := baudRate.(type) {
		case float64:
			rate = int(v)
		case int:
			rate = v
		default:
			return fmt.Errorf("invalid baud_rate type")
		}

		validRates := []int{1200, 2400, 4800, 9600, 19200, 38400, 57600, 115200}
		valid := false
		for _, validRate := range validRates {
			if rate == validRate {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid baud rate: %d", rate)
		}
	}

	return nil
}

// validateUSBConfig validates USB configuration
func validateUSBConfig(config map[string]interface{}) error {
	if _, ok := config["vendor_id"].(string); !ok {
		return fmt.Errorf("USB vendor_id is required")
	}

	if _, ok := config["product_id"].(string); !ok {
		return fmt.Errorf("USB product_id is required")
	}

	return nil
}

// validateTCPConfig validates TCP configuration
func validateTCPConfig(config map[string]interface{}) error {
	if _, ok := config["host"].(string); !ok {
		return fmt.Errorf("TCP host is required")
	}

	if port, ok := config["port"]; ok {
		var portNum int
		switch v := port.(type) {
		case float64:
			portNum = int(v)
		case int:
			portNum = v
		default:
			return fmt.Errorf("invalid port type")
		}

		if portNum < 1 || portNum > 65535 {
			return fmt.Errorf("invalid port number: %d", portNum)
		}
	}

	return nil
}

// validateBluetoothConfig validates Bluetooth configuration
func validateBluetoothConfig(config map[string]interface{}) error {
	// TODO: Implement Bluetooth validation
	return fmt.Errorf("Bluetooth configuration validation not implemented yet")
}
