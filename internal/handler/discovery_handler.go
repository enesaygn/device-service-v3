// internal/handler/discovery_handler.go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"device-service/internal/service"
	"device-service/internal/utils"
)

// DiscoveryHandler handles device discovery requests
type DiscoveryHandler struct {
	discoveryService *service.DiscoveryService
	logger           *utils.ServiceLogger
}

// NewDiscoveryHandler creates a new discovery handler
func NewDiscoveryHandler(discoveryService *service.DiscoveryService, logger *zap.Logger) *DiscoveryHandler {
	return &DiscoveryHandler{
		discoveryService: discoveryService,
		logger:           utils.NewServiceLogger(logger, "discovery-handler"),
	}
}

// ScanDevices scans for available devices
// @Summary Scan for devices
// @Description Scan for available devices on serial, USB, TCP, or Bluetooth connections
// @Tags Discovery
// @Accept json
// @Produce json
// @Param type query string false "Scan type" Enums(all, serial, usb, tcp, bluetooth) default(all)
// @Param timeout query string false "Scan timeout" default(30s)
// @Success 200 {object} utils.APIResponse{data=object{devices_found=int,devices=[]service.DiscoveredDevice}} "Device scan completed"
// @Failure 500 {object} utils.APIResponse "Scan failed"
// @Router /discovery/scan [get]
func (h *DiscoveryHandler) ScanDevices(c *gin.Context) {
	// Get scan parameters
	scanType := c.DefaultQuery("type", "all") // all, serial, usb, tcp, bluetooth
	timeout := c.DefaultQuery("timeout", "30s")

	req := &service.ScanRequest{
		ScanType: scanType,
		Timeout:  timeout,
	}

	devices, err := h.discoveryService.ScanDevices(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to scan devices", zap.Error(err))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to scan devices", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Device scan completed", gin.H{
		"devices_found": len(devices),
		"devices":       devices,
	})
}

// AutoSetupDevices automatically sets up discovered devices
// @Summary Auto-setup devices
// @Description Automatically register and setup discovered devices
// @Tags Discovery
// @Accept json
// @Produce json
// @Param request body AutoSetupRequest true "Auto-setup request"
// @Success 200 {object} utils.APIResponse{data=service.AutoSetupResult} "Auto-setup completed"
// @Failure 400 {object} utils.APIResponse "Invalid request"
// @Failure 500 {object} utils.APIResponse "Auto-setup failed"
// @Router /discovery/auto-setup [post]
func (h *DiscoveryHandler) AutoSetupDevices(c *gin.Context) {
	var req AutoSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	result, err := h.discoveryService.AutoSetupDevices(c.Request.Context(), &service.AutoSetupRequest{
		BranchID:     req.BranchID,
		DeviceFilter: req.DeviceFilter,
		AutoConnect:  req.AutoConnect,
	})
	if err != nil {
		h.logger.Error("Failed to auto-setup devices", zap.Error(err))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to auto-setup devices", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Auto-setup completed", result)
}

// GetSupportedDevices returns supported device models
// @Summary Get supported devices
// @Description Get list of all supported device brands and models
// @Tags Discovery
// @Accept json
// @Produce json
// @Success 200 {object} utils.APIResponse{data=service.SupportedDevicesResponse} "Supported devices retrieved"
// @Router /discovery/supported [get]
func (h *DiscoveryHandler) GetSupportedDevices(c *gin.Context) {
	supported := h.discoveryService.GetSupportedDevices()
	utils.SuccessResponse(c, http.StatusOK, "Supported devices retrieved", supported)
}

// GetCapabilities returns device capabilities
// @Summary Get device capabilities
// @Description Get capabilities for a specific brand and device type
// @Tags Discovery
// @Accept json
// @Produce json
// @Param brand path string true "Device brand" Enums(EPSON, STAR, INGENICO, PAX, CITIZEN, BIXOLON, VERIFONE, GENERIC)
// @Param type path string true "Device type" Enums(POS, PRINTER, SCANNER, CASH_REGISTER, CASH_DRAWER, DISPLAY)
// @Success 200 {object} utils.APIResponse{data=object{brand=string,device_type=string,capabilities=[]string}} "Capabilities retrieved"
// @Failure 404 {object} utils.APIResponse "Device not supported"
// @Router /discovery/capabilities/{brand}/{type} [get]
func (h *DiscoveryHandler) GetCapabilities(c *gin.Context) {
	brand := c.Param("brand")
	deviceType := c.Param("type")

	capabilities, err := h.discoveryService.GetDeviceCapabilities(brand, deviceType)
	if err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Device not supported", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Capabilities retrieved", gin.H{
		"brand":        brand,
		"device_type":  deviceType,
		"capabilities": capabilities,
	})
}

// AutoSetupRequest represents auto-setup request
type AutoSetupRequest struct {
	BranchID     string            `json:"branch_id" binding:"required"`
	DeviceFilter map[string]string `json:"device_filter,omitempty"`
	AutoConnect  bool              `json:"auto_connect"`
}
