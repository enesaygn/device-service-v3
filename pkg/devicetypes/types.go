// pkg/devicetypes/types.go
package devicetypes

// Common device type definitions that can be used across the application

// ConnectionInfo represents device connection information
type ConnectionInfo struct {
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config"`
}

// SerialConnectionInfo represents serial connection configuration
type SerialConnectionInfo struct {
	Port     string `json:"port"`
	BaudRate int    `json:"baud_rate"`
	DataBits int    `json:"data_bits"`
	StopBits int    `json:"stop_bits"`
	Parity   string `json:"parity"`
}

// USBConnectionInfo represents USB connection configuration
type USBConnectionInfo struct {
	VendorID  string `json:"vendor_id"`
	ProductID string `json:"product_id"`
	Interface int    `json:"interface"`
}

// TCPConnectionInfo represents TCP connection configuration
type TCPConnectionInfo struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	SSL  bool   `json:"ssl"`
}

// BluetoothConnectionInfo represents Bluetooth connection configuration
type BluetoothConnectionInfo struct {
	MACAddress string `json:"mac_address"`
	PIN        string `json:"pin,omitempty"`
}

// DeviceCapabilities defines standard device capabilities
var DeviceCapabilities = map[string][]string{
	"POS": {
		"PAYMENT", "DISPLAY", "BEEP", "STATUS",
	},
	"PRINTER": {
		"PRINT", "CUT", "DRAWER", "STATUS", "BEEP", "LOGO",
	},
	"SCANNER": {
		"SCAN", "BEEP", "STATUS",
	},
	"CASH_REGISTER": {
		"PAYMENT", "PRINT", "DRAWER", "DISPLAY", "STATUS",
	},
	"CASH_DRAWER": {
		"DRAWER", "STATUS",
	},
	"DISPLAY": {
		"DISPLAY", "STATUS",
	},
}

// BrandModels defines supported models for each brand
var BrandModels = map[string][]string{
	"EPSON": {
		"TM-T88VI", "TM-T88V", "TM-T20III", "TM-T82III", "TM-M30",
		"TM-P20", "TM-P80", "TM-H6000V",
	},
	"STAR": {
		"TSP143III", "TSP143IIIU", "TSP143IIIW", "TSP143IIILAN",
		"TSP654II", "TSP700II", "TSP800II", "mC-Print2", "mC-Print3",
	},
	"INGENICO": {
		"iCT220", "iCT250", "iPP320", "iPP350", "iWL220", "iWL250",
		"Desk/3500", "Move/5000",
	},
	"PAX": {
		"A920", "A920Pro", "A80", "A35", "IM30", "S80", "S90",
	},
	"CITIZEN": {
		"CT-S310II", "CT-S4000", "CT-E351", "CT-E601", "CT-D150",
	},
	"BIXOLON": {
		"SRP-330II", "SRP-350III", "SRP-Q300", "SRP-S300",
	},
	"VERIFONE": {
		"VX520", "VX680", "VX805", "VX820", "e280", "e355",
	},
}

// ErrorCodes defines standard error codes
var ErrorCodes = map[string]string{
	"CONNECTION_FAILED":     "Failed to connect to device",
	"OPERATION_TIMEOUT":     "Operation timed out",
	"DEVICE_BUSY":           "Device is busy",
	"PAPER_OUT":             "Printer is out of paper",
	"PAPER_JAM":             "Paper jam detected",
	"CUTTER_ERROR":          "Cutter error",
	"DRAWER_OPEN":           "Cash drawer is open",
	"CARD_READ_ERROR":       "Failed to read card",
	"PAYMENT_DECLINED":      "Payment was declined",
	"INVALID_AMOUNT":        "Invalid payment amount",
	"UNSUPPORTED_OPERATION": "Operation not supported",
	"HARDWARE_ERROR":        "Hardware error",
	"FIRMWARE_ERROR":        "Firmware error",
	"CONFIGURATION_ERROR":   "Configuration error",
}

// Standard timeouts for different operations
var DefaultTimeouts = map[string]int{
	"CONNECT":      30, // seconds
	"PRINT":        10,
	"PAYMENT":      60,
	"SCAN":         30,
	"STATUS_CHECK": 5,
	"DRAWER_OPEN":  3,
	"DISPLAY":      2,
}

// Health score calculation weights
var HealthWeights = map[string]float64{
	"response_time": 0.3,
	"success_rate":  0.4,
	"uptime":        0.2,
	"error_rate":    0.1,
}
