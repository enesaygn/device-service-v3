// internal/driver/epson/commands.go
package epson

// ESC_POS_COMMANDS contains all ESC/POS command definitions for EPSON printers
var ESC_POS_COMMANDS = struct {
	// Basic commands
	INITIALIZE      []byte
	STATUS_REQUEST  []byte
	GET_DEVICE_INFO []byte

	// Text formatting
	TEXT_BOLD_ON       []byte
	TEXT_BOLD_OFF      []byte
	TEXT_UNDERLINE_ON  []byte
	TEXT_UNDERLINE_OFF []byte
	TEXT_RESET         []byte

	// Text size
	TEXT_SIZE_NORMAL        []byte
	TEXT_SIZE_DOUBLE_WIDTH  []byte
	TEXT_SIZE_DOUBLE_HEIGHT []byte
	TEXT_SIZE_DOUBLE_BOTH   []byte

	// Text alignment
	ALIGN_LEFT   []byte
	ALIGN_CENTER []byte
	ALIGN_RIGHT  []byte

	// Character sets
	SELECT_CHARSET_PC437 []byte
	SELECT_CHARSET_PC850 []byte
	SELECT_CHARSET_PC852 []byte
	SELECT_CHARSET_PC858 []byte

	// Paper handling
	LINE_FEED      []byte
	FORM_FEED      []byte
	FEED_LINES     []byte // + line count byte
	SET_WIDTH_58MM []byte
	SET_WIDTH_80MM []byte

	// Cutting
	CUT_FULL    []byte
	CUT_PARTIAL []byte

	// Cash drawer
	DRAWER_KICK_PIN2 []byte // Pin 2 (most common)
	DRAWER_KICK_PIN5 []byte // Pin 5

	// Graphics and barcodes
	PRINT_LOGO      []byte
	BARCODE_CODE128 []byte
	BARCODE_CODE39  []byte
	QR_CODE_START   []byte
}{
	// Basic commands
	INITIALIZE:      []byte{0x1B, 0x40},       // ESC @
	STATUS_REQUEST:  []byte{0x10, 0x04, 0x01}, // DLE EOT 1
	GET_DEVICE_INFO: []byte{0x1D, 0x49, 0x01}, // GS I 1

	// Text formatting
	TEXT_BOLD_ON:       []byte{0x1B, 0x45, 0x01}, // ESC E 1
	TEXT_BOLD_OFF:      []byte{0x1B, 0x45, 0x00}, // ESC E 0
	TEXT_UNDERLINE_ON:  []byte{0x1B, 0x2D, 0x01}, // ESC - 1
	TEXT_UNDERLINE_OFF: []byte{0x1B, 0x2D, 0x00}, // ESC - 0
	TEXT_RESET:         []byte{0x1B, 0x21, 0x00}, // ESC ! 0

	// Text size
	TEXT_SIZE_NORMAL:        []byte{0x1D, 0x21, 0x00}, // GS ! 0
	TEXT_SIZE_DOUBLE_WIDTH:  []byte{0x1D, 0x21, 0x20}, // GS ! 32
	TEXT_SIZE_DOUBLE_HEIGHT: []byte{0x1D, 0x21, 0x10}, // GS ! 16
	TEXT_SIZE_DOUBLE_BOTH:   []byte{0x1D, 0x21, 0x30}, // GS ! 48

	// Text alignment
	ALIGN_LEFT:   []byte{0x1B, 0x61, 0x00}, // ESC a 0
	ALIGN_CENTER: []byte{0x1B, 0x61, 0x01}, // ESC a 1
	ALIGN_RIGHT:  []byte{0x1B, 0x61, 0x02}, // ESC a 2

	// Character sets
	SELECT_CHARSET_PC437: []byte{0x1B, 0x74, 0x00}, // ESC t 0
	SELECT_CHARSET_PC850: []byte{0x1B, 0x74, 0x02}, // ESC t 2
	SELECT_CHARSET_PC852: []byte{0x1B, 0x74, 0x12}, // ESC t 18
	SELECT_CHARSET_PC858: []byte{0x1B, 0x74, 0x13}, // ESC t 19

	// Paper handling
	LINE_FEED:      []byte{0x0A},                   // LF
	FORM_FEED:      []byte{0x0C},                   // FF
	FEED_LINES:     []byte{0x1B, 0x64},             // ESC d + n
	SET_WIDTH_58MM: []byte{0x1D, 0x57, 0x40, 0x01}, // GS W 320
	SET_WIDTH_80MM: []byte{0x1D, 0x57, 0x00, 0x02}, // GS W 512

	// Cutting
	CUT_FULL:    []byte{0x1D, 0x56, 0x00}, // GS V 0
	CUT_PARTIAL: []byte{0x1D, 0x56, 0x01}, // GS V 1

	// Cash drawer
	DRAWER_KICK_PIN2: []byte{0x1B, 0x70, 0x00, 0x19, 0x19}, // ESC p 0 25 25
	DRAWER_KICK_PIN5: []byte{0x1B, 0x70, 0x01, 0x19, 0x19}, // ESC p 1 25 25

	// Graphics and barcodes
	PRINT_LOGO:      []byte{0x1D, 0x2F, 0x00},             // GS / 0
	BARCODE_CODE128: []byte{0x1D, 0x6B, 0x49},             // GS k I
	BARCODE_CODE39:  []byte{0x1D, 0x6B, 0x04},             // GS k 4
	QR_CODE_START:   []byte{0x1D, 0x28, 0x6B, 0x04, 0x00}, // GS ( k
}
