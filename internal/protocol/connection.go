// internal/protocol/connection.go
package protocol

import "time"

// SerialConfig represents serial connection configuration
type SerialConfig struct {
	Port     string        `json:"port"`
	BaudRate int           `json:"baud_rate"`
	DataBits int           `json:"data_bits"`
	StopBits int           `json:"stop_bits"`
	Parity   string        `json:"parity"`
	Timeout  time.Duration `json:"timeout"`
}

// USBConfig represents USB connection configuration
type USBConfig struct {
	VendorID     string        `json:"vendor_id"`
	ProductID    string        `json:"product_id"`
	Interface    int           `json:"interface"`
	Endpoint     int           `json:"endpoint"`
	SerialNumber string        `json:"serial_number"`
	Timeout      time.Duration `json:"timeout"`
}

// TCPConfig represents TCP connection configuration
type TCPConfig struct {
	Host         string        `json:"host"`
	Port         int           `json:"port"`
	SSL          bool          `json:"ssl"`
	KeepAlive    bool          `json:"keep_alive"`
	BufferSize   int           `json:"buffer_size"`
	Timeout      time.Duration `json:"timeout"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
}
