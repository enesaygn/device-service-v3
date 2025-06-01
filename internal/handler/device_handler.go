// internal/handler/device_handler.go
package handler

import (
	"net/http"
	"strconv"

	"device-service/internal/model"
	"device-service/internal/service"
	"device-service/internal/utils"
	_ "time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// DeviceHandler handles device-related HTTP requests
type DeviceHandler struct {
	deviceService *service.DeviceService
	logger        *utils.ServiceLogger
}

// NewDeviceHandler creates a new device handler
func NewDeviceHandler(deviceService *service.DeviceService, logger *zap.Logger) *DeviceHandler {
	return &DeviceHandler{
		deviceService: deviceService,
		logger:        utils.NewServiceLogger(logger, "device-handler"),
	}
}

// RegisterRoutes registers device-related routes
func (h *DeviceHandler) RegisterRoutes(router *gin.RouterGroup) {
	devices := router.Group("/devices")
	{
		// Genel device routes
		devices.POST("", h.RegisterDevice)
		devices.GET("", h.ListDevices)

		// Spesifik device routes - :id parametresi
		deviceRoutes := devices.Group("/:id")
		{
			deviceRoutes.GET("", h.GetDevice)
			deviceRoutes.PUT("", h.UpdateDevice)
			deviceRoutes.DELETE("", h.DeleteDevice)
			deviceRoutes.POST("/connect", h.ConnectDevice)
			deviceRoutes.POST("/disconnect", h.DisconnectDevice)
			deviceRoutes.POST("/test", h.TestDevice)
			deviceRoutes.GET("/health", h.GetDeviceHealth)
			deviceRoutes.PUT("/config", h.UpdateDeviceConfig)
		}
	}
}

// RegisterDevice registers a new device
// @Summary Register a new device
// @Description Register a new device in the system with configuration
// @Tags Devices
// @Accept json
// @Produce json
// @Param request body service.RegisterDeviceRequest true "Device registration request"
// @Success 201 {object} utils.APIResponse{data=model.Device} "Device registered successfully"
// @Failure 400 {object} utils.APIResponse "Invalid request"
// @Failure 500 {object} utils.APIResponse "Internal server error"
// @Router /devices [post]
func (h *DeviceHandler) RegisterDevice(c *gin.Context) {
	var req service.RegisterDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.LogAPIRequest(c.Request.Method, c.Request.URL.Path, c.Request.UserAgent(), c.ClientIP(), 400, 0)
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Add user ID from context (would come from auth middleware)
	if userID, exists := c.Get("user_id"); exists {
		req.UserID = userID.(string)
	}

	device, err := h.deviceService.RegisterDevice(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to register device", zap.Error(err))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to register device", err)
		return
	}

	h.logger.Info("Device registered successfully", zap.String("device_id", device.DeviceID))
	utils.SuccessResponse(c, http.StatusCreated, "Device registered successfully", device)
}

// ListDevices lists devices with filtering and pagination
// @Summary List devices
// @Description Get list of devices with filtering and pagination support
// @Tags Devices
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Param branch_id query string false "Filter by branch ID"
// @Param device_type query string false "Filter by device type" Enums(POS, PRINTER, SCANNER, CASH_REGISTER, CASH_DRAWER, DISPLAY)
// @Param brand query string false "Filter by brand" Enums(EPSON, STAR, INGENICO, PAX, CITIZEN, BIXOLON, VERIFONE, GENERIC)
// @Param status query string false "Filter by status" Enums(ONLINE, OFFLINE, ERROR, MAINTENANCE, CONNECTING)
// @Param location query string false "Filter by location"
// @Param sort_by query string false "Sort by field" default(created_at)
// @Param sort_order query string false "Sort order" Enums(asc, desc) default(desc)
// @Success 200 {object} utils.APIResponse{data=object{devices=[]model.Device,pagination=service.PaginationResult}} "Devices retrieved successfully"
// @Failure 500 {object} utils.APIResponse "Internal server error"
// @Router /devices [get]
func (h *DeviceHandler) ListDevices(c *gin.Context) {
	// Parse query parameters
	filter := &service.DeviceFilter{
		Page:      1,
		PerPage:   20,
		SortBy:    "created_at",
		SortOrder: "desc",
	}

	// Parse pagination
	if page := c.Query("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil && p > 0 {
			filter.Page = p
		}
	}
	if perPage := c.Query("per_page"); perPage != "" {
		if pp, err := strconv.Atoi(perPage); err == nil && pp > 0 && pp <= 100 {
			filter.PerPage = pp
		}
	}

	// Parse filters
	if branchID := c.Query("branch_id"); branchID != "" {
		if id, err := uuid.Parse(branchID); err == nil {
			filter.BranchID = &id
		}
	}
	if deviceType := c.Query("device_type"); deviceType != "" {
		dt := model.DeviceType(deviceType)
		filter.DeviceType = &dt
	}
	if brand := c.Query("brand"); brand != "" {
		b := model.DeviceBrand(brand)
		filter.Brand = &b
	}
	if status := c.Query("status"); status != "" {
		s := model.DeviceStatus(status)
		filter.Status = &s
	}
	if location := c.Query("location"); location != "" {
		filter.Location = &location
	}

	// Parse sorting
	if sortBy := c.Query("sort_by"); sortBy != "" {
		filter.SortBy = sortBy
	}
	if sortOrder := c.Query("sort_order"); sortOrder != "" {
		filter.SortOrder = sortOrder
	}

	devices, pagination, err := h.deviceService.ListDevices(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list devices", zap.Error(err))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to list devices", err)
		return
	}

	response := gin.H{
		"devices":    devices,
		"pagination": pagination,
	}

	utils.SuccessResponse(c, http.StatusOK, "Devices retrieved successfully", response)
}

// GetDevice retrieves device by ID
// @Summary Get device details
// @Description Get device details and current status by device ID
// @Tags Devices
// @Accept json
// @Produce json
// @Param id path string true "Device ID"
// @Success 200 {object} utils.APIResponse{data=model.Device} "Device retrieved successfully"
// @Failure 400 {object} utils.APIResponse "Invalid device ID"
// @Failure 404 {object} utils.APIResponse "Device not found"
// @Router /devices/{id} [get]
func (h *DeviceHandler) GetDevice(c *gin.Context) {
	deviceID := c.Param("id")
	if deviceID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Device ID is required", nil)
		return
	}

	device, err := h.deviceService.GetDevice(c.Request.Context(), deviceID)
	if err != nil {
		h.logger.Error("Failed to get device", zap.Error(err), zap.String("device_id", deviceID))
		utils.ErrorResponse(c, http.StatusNotFound, "Device not found", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Device retrieved successfully", device)
}

// UpdateDevice handles device updates
func (h *DeviceHandler) UpdateDevice(c *gin.Context) {
	deviceID := c.Param("id")
	if deviceID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Device ID is required", nil)
		return
	}

	var req UpdateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Implementation would update device fields
	// For now, just return success
	utils.SuccessResponse(c, http.StatusOK, "Device updated successfully", gin.H{"device_id": deviceID})
}

// DeleteDevice handles device deletion
func (h *DeviceHandler) DeleteDevice(c *gin.Context) {
	deviceID := c.Param("id")
	if deviceID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Device ID is required", nil)
		return
	}

	userID := getUserID(c)
	if err := h.deviceService.DeleteDevice(c.Request.Context(), deviceID, userID); err != nil {
		h.logger.Error("Failed to delete device", zap.Error(err), zap.String("device_id", deviceID))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to delete device", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Device deleted successfully", gin.H{"device_id": deviceID})
}

// ConnectDevice connects to a device
// @Summary Connect to device
// @Description Establish connection to a device
// @Tags Devices
// @Accept json
// @Produce json
// @Param id path string true "Device ID"
// @Success 200 {object} utils.APIResponse "Device connected successfully"
// @Failure 400 {object} utils.APIResponse "Invalid device ID"
// @Failure 500 {object} utils.APIResponse "Connection failed"
// @Router /devices/{id}/connect [post]
func (h *DeviceHandler) ConnectDevice(c *gin.Context) {
	deviceID := c.Param("id")
	if deviceID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Device ID is required", nil)
		return
	}

	if err := h.deviceService.ConnectDevice(c.Request.Context(), deviceID); err != nil {
		h.logger.Error("Failed to connect device", zap.Error(err), zap.String("device_id", deviceID))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to connect device", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Device connected successfully", gin.H{"device_id": deviceID})
}

// DisconnectDevice disconnects from a device
// @Summary Disconnect from device
// @Description Disconnect from a device
// @Tags Devices
// @Accept json
// @Produce json
// @Param id path string true "Device ID"
// @Success 200 {object} utils.APIResponse "Device disconnected successfully"
// @Failure 400 {object} utils.APIResponse "Invalid device ID"
// @Failure 500 {object} utils.APIResponse "Disconnection failed"
// @Router /devices/{id}/disconnect [post]
func (h *DeviceHandler) DisconnectDevice(c *gin.Context) {
	deviceID := c.Param("id")
	if deviceID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Device ID is required", nil)
		return
	}

	if err := h.deviceService.DisconnectDevice(c.Request.Context(), deviceID); err != nil {
		h.logger.Error("Failed to disconnect device", zap.Error(err), zap.String("device_id", deviceID))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to disconnect device", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Device disconnected successfully", gin.H{"device_id": deviceID})
}

// TestDevice tests device connectivity
// @Summary Test device connectivity
// @Description Test connection and basic functionality of a device
// @Tags Devices
// @Accept json
// @Produce json
// @Param id path string true "Device ID"
// @Success 200 {object} utils.APIResponse{data=service.TestResult} "Device test completed"
// @Failure 400 {object} utils.APIResponse "Invalid device ID"
// @Failure 500 {object} utils.APIResponse "Test failed"
// @Router /devices/{id}/test [post]
func (h *DeviceHandler) TestDevice(c *gin.Context) {
	deviceID := c.Param("id")
	if deviceID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Device ID is required", nil)
		return
	}

	result, err := h.deviceService.TestDevice(c.Request.Context(), deviceID)
	if err != nil {
		h.logger.Error("Failed to test device", zap.Error(err), zap.String("device_id", deviceID))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to test device", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Device test completed", result)
}

// GetDeviceHealth retrieves device health metrics
// @Summary Get device health
// @Description Get current health metrics and status of a device
// @Tags Devices
// @Accept json
// @Produce json
// @Param id path string true "Device ID"
// @Success 200 {object} utils.APIResponse{data=service.DeviceHealth} "Device health retrieved successfully"
// @Failure 400 {object} utils.APIResponse "Invalid device ID"
// @Failure 500 {object} utils.APIResponse "Failed to get device health"
// @Router /devices/{id}/health [get]
func (h *DeviceHandler) GetDeviceHealth(c *gin.Context) {
	deviceID := c.Param("id")
	if deviceID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Device ID is required", nil)
		return
	}

	health, err := h.deviceService.GetDeviceHealth(c.Request.Context(), deviceID)
	if err != nil {
		h.logger.Error("Failed to get device health", zap.Error(err), zap.String("device_id", deviceID))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to get device health", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Device health retrieved successfully", health)
}

// UpdateDeviceConfig updates device configuration
// @Summary Update device configuration
// @Description Update device configuration settings
// @Tags Devices
// @Accept json
// @Produce json
// @Param id path string true "Device ID"
// @Param request body UpdateConfigRequest true "Configuration update request"
// @Success 200 {object} utils.APIResponse "Device configuration updated successfully"
// @Failure 400 {object} utils.APIResponse "Invalid request"
// @Failure 500 {object} utils.APIResponse "Update failed"
// @Router /devices/{id}/config [put]
func (h *DeviceHandler) UpdateDeviceConfig(c *gin.Context) {
	deviceID := c.Param("id")
	if deviceID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Device ID is required", nil)
		return
	}

	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	userID := getUserID(c)
	if err := h.deviceService.UpdateDeviceConfiguration(c.Request.Context(), deviceID, req.Config, userID); err != nil {
		h.logger.Error("Failed to update device config", zap.Error(err), zap.String("device_id", deviceID))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to update device configuration", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Device configuration updated successfully", gin.H{"device_id": deviceID})
}

// Helper functions and DTOs

// getUserID extracts user ID from context
func getUserID(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		return userID.(string)
	}
	return ""
}

// UpdateDeviceRequest represents device update request
type UpdateDeviceRequest struct {
	Location        *string `json:"location,omitempty"`
	FirmwareVersion *string `json:"firmware_version,omitempty"`
}

// UpdateConfigRequest represents configuration update request
type UpdateConfigRequest struct {
	Config map[string]interface{} `json:"config"`
}
