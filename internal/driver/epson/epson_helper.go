// internal/driver/epson/helpers.go
package epson

import (
	"context"
	"fmt"
	"time"

	"device-service/internal/model"
	"device-service/pkg/driver"
)

// Helper methods for EPSON driver

// sendCommands sends commands to printer
func (d *EPSONDriver) sendCommands(ctx context.Context, commands [][]byte) error {
	if d.connection == nil {
		return fmt.Errorf("no connection")
	}

	for _, cmd := range commands {
		if err := d.connection.Write(ctx, cmd); err != nil {
			return fmt.Errorf("failed to send command: %w", err)
		}
	}

	return nil
}

// readResponse reads response from printer
func (d *EPSONDriver) readResponse(ctx context.Context, timeout time.Duration) ([]byte, error) {
	if d.connection == nil {
		return nil, fmt.Errorf("no connection")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return d.connection.Read(ctx, 1024)
}

// contentToESCPOS converts print content to ESC/POS commands
func (d *EPSONDriver) contentToESCPOS(content *driver.PrintContent) ([][]byte, error) {
	var commands [][]byte

	// Initialize
	commands = append(commands, ESC_POS_COMMANDS.INITIALIZE)

	switch content.Type {
	case driver.ContentTypeText:
		textCommands, err := d.textToESCPOS(content.Data.(string), content.Options)
		if err != nil {
			return nil, err
		}
		commands = append(commands, textCommands...)

	case driver.ContentTypeESCPOS:
		// Direct ESC/POS commands
		if data, ok := content.Data.([]byte); ok {
			commands = append(commands, data)
		}

	default:
		return nil, fmt.Errorf("unsupported content type: %s", content.Type)
	}

	// Handle options
	if content.Options != nil {
		if cut, ok := content.Options["cut"].(bool); ok && cut {
			commands = append(commands, ESC_POS_COMMANDS.CUT_FULL)
		}

		if openDrawer, ok := content.Options["open_drawer"].(bool); ok && openDrawer {
			if d.config.EnableDrawer {
				commands = append(commands, ESC_POS_COMMANDS.DRAWER_KICK_PIN2)
			}
		}
	}

	return commands, nil
}

// textToESCPOS converts text to ESC/POS commands
func (d *EPSONDriver) textToESCPOS(text string, options map[string]interface{}) ([][]byte, error) {
	var commands [][]byte

	// Text formatting options
	bold := false
	underline := false
	align := "left"

	if options != nil {
		if b, ok := options["bold"].(bool); ok {
			bold = b
		}
		if u, ok := options["underline"].(bool); ok {
			underline = u
		}
		if a, ok := options["align"].(string); ok {
			align = a
		}
	}

	// Set text formatting
	if bold {
		commands = append(commands, ESC_POS_COMMANDS.TEXT_BOLD_ON)
	}
	if underline {
		commands = append(commands, ESC_POS_COMMANDS.TEXT_UNDERLINE_ON)
	}

	// Set alignment
	switch align {
	case "center":
		commands = append(commands, ESC_POS_COMMANDS.ALIGN_CENTER)
	case "right":
		commands = append(commands, ESC_POS_COMMANDS.ALIGN_RIGHT)
	default:
		commands = append(commands, ESC_POS_COMMANDS.ALIGN_LEFT)
	}

	// Add text
	commands = append(commands, []byte(text))

	// Add line feed
	commands = append(commands, ESC_POS_COMMANDS.LINE_FEED)

	// Reset formatting
	if bold || underline {
		commands = append(commands, ESC_POS_COMMANDS.TEXT_RESET)
	}
	commands = append(commands, ESC_POS_COMMANDS.ALIGN_LEFT)

	return commands, nil
}

// Operation handlers
func (d *EPSONDriver) handlePrintOperation(ctx context.Context, operation *model.DeviceOperation) (*driver.OperationResult, error) {
	data := operation.OperationData

	content, ok := data["content"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid content")
	}

	contentType, _ := data["content_type"].(string)
	if contentType == "" {
		contentType = "TEXT"
	}

	copies, _ := data["copies"].(float64)
	if copies <= 0 {
		copies = 1
	}

	cut, _ := data["cut"].(bool)
	openDrawer, _ := data["open_drawer"].(bool)

	printContent := &driver.PrintContent{
		Type: driver.ContentType(contentType),
		Data: content,
		Options: map[string]interface{}{
			"copies":      int(copies),
			"cut":         cut,
			"open_drawer": openDrawer,
		},
	}

	if err := d.Print(ctx, printContent); err != nil {
		return nil, err
	}

	result := &driver.OperationResult{
		Success: true,
		Data: map[string]interface{}{
			"printed": true,
			"copies":  int(copies),
		},
	}

	return result, nil
}

func (d *EPSONDriver) handleCutOperation(ctx context.Context, operation *model.DeviceOperation) (*driver.OperationResult, error) {
	data := operation.OperationData

	cutTypeStr, ok := data["cut_type"].(string)
	if !ok {
		cutTypeStr = "FULL"
	}

	cutType := driver.CutType(cutTypeStr)
	if err := d.Cut(ctx, cutType); err != nil {
		return nil, err
	}

	result := &driver.OperationResult{
		Success: true,
		Data: map[string]interface{}{
			"cut":      true,
			"cut_type": cutTypeStr,
		},
	}

	return result, nil
}

func (d *EPSONDriver) handleDrawerOperation(ctx context.Context, operation *model.DeviceOperation) (*driver.OperationResult, error) {
	data := operation.OperationData

	pin, ok := data["pin"].(float64)
	if !ok {
		pin = float64(d.config.DrawerPin)
	}

	if err := d.OpenDrawer(ctx, int(pin)); err != nil {
		return nil, err
	}

	result := &driver.OperationResult{
		Success: true,
		Data: map[string]interface{}{
			"drawer_opened": true,
			"pin":           int(pin),
		},
	}

	return result, nil
}

func (d *EPSONDriver) handleStatusOperation(ctx context.Context, operation *model.DeviceOperation) (*driver.OperationResult, error) {
	status, err := d.GetStatus()
	if err != nil {
		return nil, err
	}

	paperStatus := d.GetPaperStatus()
	cutterStatus := d.GetCutterStatus()
	drawerStatus := d.GetDrawerStatus()

	result := &driver.OperationResult{
		Success: true,
		Data: map[string]interface{}{
			"device_status": status,
			"paper_status":  paperStatus,
			"cutter_status": cutterStatus,
			"drawer_status": drawerStatus,
		},
	}

	return result, nil
}

// updateHealthMetrics updates device health metrics
func (d *EPSONDriver) updateHealthMetrics(success bool, responseTime time.Duration, err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.healthMetrics.TotalOperations++
	d.healthMetrics.ResponseTime = responseTime

	if success {
		d.healthMetrics.SuccessRate = float64(d.healthMetrics.TotalOperations-d.healthMetrics.ErrorCount) / float64(d.healthMetrics.TotalOperations)
		now := time.Now()
		d.healthMetrics.LastSuccessTime = &now
	} else {
		d.healthMetrics.ErrorCount++
		d.healthMetrics.SuccessRate = float64(d.healthMetrics.TotalOperations-d.healthMetrics.ErrorCount) / float64(d.healthMetrics.TotalOperations)
		now := time.Now()
		d.healthMetrics.LastErrorTime = &now
	}

	// Calculate health score (0-100)
	d.healthMetrics.HealthScore = int(d.healthMetrics.SuccessRate * 100)
	if responseTime > 5*time.Second {
		d.healthMetrics.HealthScore -= 10 // Penalty for slow response
	}
	if d.healthMetrics.HealthScore < 0 {
		d.healthMetrics.HealthScore = 0
	}
}

// notifyEvent notifies event handler
func (d *EPSONDriver) notifyEvent(eventType string, data interface{}) {
	if d.eventHandler != nil {
		switch eventType {
		case "connected":
			d.eventHandler.OnDeviceConnected(d.config.DeviceID)
		case "disconnected":
			d.eventHandler.OnDeviceDisconnected(d.config.DeviceID, data.(string))
		case "operation_completed":
			// Would need operation ID from context
		case "operation_failed":
			if err, ok := data.(error); ok {
				d.eventHandler.OnDeviceError(d.config.DeviceID, err)
			}
		}
	}
}
