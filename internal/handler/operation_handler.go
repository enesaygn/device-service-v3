// internal/handler/operation_handler.go
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"device-service/internal/model"
	"device-service/internal/service"
	"device-service/internal/utils"
)

// OperationHandler handles operation-related HTTP requests
type OperationHandler struct {
	operationService *service.OperationService
	logger           *utils.ServiceLogger
}

// NewOperationHandler creates a new operation handler
func NewOperationHandler(operationService *service.OperationService, logger *zap.Logger) *OperationHandler {
	return &OperationHandler{
		operationService: operationService,
		logger:           utils.NewServiceLogger(logger, "operation-handler"),
	}
}

// RegisterRoutes registers operation-related routes
func (h *OperationHandler) RegisterRoutes(router *gin.RouterGroup) {
	// Genel operation routes
	operations := router.Group("/operations")
	{
		operations.POST("", h.ExecuteOperation)
		operations.GET("", h.ListOperations)
		operations.GET("/:id", h.GetOperation)
		operations.PUT("/:id/cancel", h.CancelOperation)
	}
}

// Device-specific operation routes için ayrı method
func (h *OperationHandler) RegisterDeviceRoutes(router *gin.RouterGroup) {
	// Device-specific operation routes - /device-ops prefix kullanarak çakışmayı önlüyoruz
	deviceOps := router.Group("/device-ops/:device_id")
	{
		deviceOps.POST("/operations", h.ExecuteDeviceOperation)
		deviceOps.GET("/operations", h.ListDeviceOperations)
		deviceOps.POST("/print", h.PrintOperation)
		deviceOps.POST("/payment", h.PaymentOperation)
		deviceOps.POST("/scan", h.ScanOperation)
		deviceOps.POST("/open-drawer", h.OpenDrawerOperation)
		deviceOps.POST("/display", h.DisplayOperation)
	}
}

// ExecuteOperation handles general operation execution
func (h *OperationHandler) ExecuteOperation(c *gin.Context) {
	var req service.OperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	response, err := h.operationService.ExecuteOperation(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to execute operation", zap.Error(err))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to execute operation", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Operation executed successfully", response)
}

// ExecuteDeviceOperation handles device-specific operation execution
func (h *OperationHandler) ExecuteDeviceOperation(c *gin.Context) {
	deviceIDStr := c.Param("device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid device ID", err)
		return
	}

	var req DeviceOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	operationReq := &service.OperationRequest{
		DeviceID:      deviceID,
		OperationType: req.OperationType,
		Data:          req.Data,
		Priority:      req.Priority,
	}

	if req.CorrelationID != nil {
		correlationID, err := uuid.Parse(*req.CorrelationID)
		if err == nil {
			operationReq.CorrelationID = &correlationID
		}
	}

	response, err := h.operationService.ExecuteOperation(c.Request.Context(), operationReq)
	if err != nil {
		h.logger.Error("Failed to execute device operation", zap.Error(err))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to execute operation", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Operation executed successfully", response)
}

// PrintOperation executes print operation
// @Summary Print operation
// @Description Execute a print operation on a device
// @Tags Operations
// @Accept json
// @Produce json
// @Param device_id path string true "Device ID"
// @Param request body PrintRequest true "Print request"
// @Success 200 {object} utils.APIResponse{data=service.OperationResponse} "Print operation completed"
// @Failure 400 {object} utils.APIResponse "Invalid request"
// @Failure 500 {object} utils.APIResponse "Print operation failed"
// @Router /devices/{device_id}/print [post]
func (h *OperationHandler) PrintOperation(c *gin.Context) {
	deviceIDStr := c.Param("device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid device ID", err)
		return
	}

	var req PrintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Convert print request to operation data
	operationData := map[string]interface{}{
		"content":      req.Content,
		"content_type": req.ContentType,
		"copies":       req.Copies,
		"cut":          req.Cut,
		"open_drawer":  req.OpenDrawer,
	}

	operationReq := &service.OperationRequest{
		DeviceID:      deviceID,
		OperationType: model.OperationTypePrint,
		Data:          operationData,
		Priority:      model.PriorityHigh,
	}

	response, err := h.operationService.ExecuteOperation(c.Request.Context(), operationReq)
	if err != nil {
		h.logger.Error("Failed to execute print operation", zap.Error(err))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to print", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Print operation completed", response)
}

// PaymentOperation executes payment operation
// @Summary Payment operation
// @Description Execute a payment operation on a device
// @Tags Operations
// @Accept json
// @Produce json
// @Param device_id path string true "Device ID"
// @Param request body PaymentRequest true "Payment request"
// @Success 200 {object} utils.APIResponse{data=service.OperationResponse} "Payment operation completed"
// @Failure 400 {object} utils.APIResponse "Invalid request"
// @Failure 500 {object} utils.APIResponse "Payment operation failed"
// @Router /devices/{device_id}/payment [post]
func (h *OperationHandler) PaymentOperation(c *gin.Context) {
	deviceIDStr := c.Param("device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid device ID", err)
		return
	}

	var req PaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	operationData := map[string]interface{}{
		"amount":         req.Amount,
		"currency":       req.Currency,
		"payment_method": req.PaymentMethod,
		"reference":      req.Reference,
		"timeout":        req.Timeout,
	}

	correlationID := uuid.New()
	operationReq := &service.OperationRequest{
		DeviceID:      deviceID,
		OperationType: model.OperationTypePayment,
		Data:          operationData,
		Priority:      model.PriorityUltraCritical,
		CorrelationID: &correlationID,
	}

	response, err := h.operationService.ExecuteOperation(c.Request.Context(), operationReq)
	if err != nil {
		h.logger.Error("Failed to execute payment operation", zap.Error(err))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to process payment", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Payment operation completed", response)
}

// ScanOperation executes scan operation
// @Summary Scan operation
// @Description Execute a scan operation on a device
// @Tags Operations
// @Accept json
// @Produce json
// @Param device_id path string true "Device ID"
// @Param request body ScanRequest true "Scan request"
// @Success 200 {object} utils.APIResponse{data=service.OperationResponse} "Scan operation completed"
// @Failure 400 {object} utils.APIResponse "Invalid request"
// @Failure 500 {object} utils.APIResponse "Scan operation failed"
// @Router /devices/{device_id}/scan [post]
func (h *OperationHandler) ScanOperation(c *gin.Context) {
	deviceIDStr := c.Param("device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid device ID", err)
		return
	}

	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	operationData := map[string]interface{}{
		"scan_type": req.ScanType,
		"timeout":   req.Timeout,
	}

	operationReq := &service.OperationRequest{
		DeviceID:      deviceID,
		OperationType: model.OperationTypeScan,
		Data:          operationData,
		Priority:      model.PriorityNormal,
	}

	response, err := h.operationService.ExecuteOperation(c.Request.Context(), operationReq)
	if err != nil {
		h.logger.Error("Failed to execute scan operation", zap.Error(err))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to scan", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Scan operation completed", response)
}

// OpenDrawerOperation executes cash drawer operation
// @Summary Open cash drawer
// @Description Open cash drawer on a device
// @Tags Operations
// @Accept json
// @Produce json
// @Param device_id path string true "Device ID"
// @Success 200 {object} utils.APIResponse{data=service.OperationResponse} "Drawer opened successfully"
// @Failure 400 {object} utils.APIResponse "Invalid device ID"
// @Failure 500 {object} utils.APIResponse "Drawer operation failed"
// @Router /devices/{device_id}/open-drawer [post]
func (h *OperationHandler) OpenDrawerOperation(c *gin.Context) {
	deviceIDStr := c.Param("device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid device ID", err)
		return
	}

	operationData := map[string]interface{}{
		"pin": 0, // Default drawer pin
	}

	operationReq := &service.OperationRequest{
		DeviceID:      deviceID,
		OperationType: model.OperationTypeOpenDrawer,
		Data:          operationData,
		Priority:      model.PriorityHigh,
	}

	response, err := h.operationService.ExecuteOperation(c.Request.Context(), operationReq)
	if err != nil {
		h.logger.Error("Failed to execute drawer operation", zap.Error(err))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to open drawer", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Drawer opened successfully", response)
}

// DisplayOperation executes display operation
// @Summary Display text
// @Description Display text on customer display
// @Tags Operations
// @Accept json
// @Produce json
// @Param device_id path string true "Device ID"
// @Param request body DisplayRequest true "Display request"
// @Success 200 {object} utils.APIResponse{data=service.OperationResponse} "Display operation completed"
// @Failure 400 {object} utils.APIResponse "Invalid request"
// @Failure 500 {object} utils.APIResponse "Display operation failed"
// @Router /devices/{device_id}/display [post]
func (h *OperationHandler) DisplayOperation(c *gin.Context) {
	deviceIDStr := c.Param("device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid device ID", err)
		return
	}

	var req DisplayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	operationData := map[string]interface{}{
		"line1":    req.Line1,
		"line2":    req.Line2,
		"duration": req.Duration,
		"clear":    req.Clear,
	}

	operationReq := &service.OperationRequest{
		DeviceID:      deviceID,
		OperationType: model.OperationTypeDisplayText,
		Data:          operationData,
		Priority:      model.PriorityNormal,
	}

	response, err := h.operationService.ExecuteOperation(c.Request.Context(), operationReq)
	if err != nil {
		h.logger.Error("Failed to execute display operation", zap.Error(err))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to display text", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Display operation completed", response)
}

// GetOperation retrieves operation by ID
// @Summary Get operation details
// @Description Get operation details and status by operation ID
// @Tags Operations
// @Accept json
// @Produce json
// @Param id path string true "Operation ID"
// @Success 200 {object} utils.APIResponse{data=model.DeviceOperation} "Operation retrieved successfully"
// @Failure 400 {object} utils.APIResponse "Invalid operation ID"
// @Failure 404 {object} utils.APIResponse "Operation not found"
// @Router /operations/{id} [get]
func (h *OperationHandler) GetOperation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid operation ID", err)
		return
	}

	operation, err := h.operationService.GetOperation(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get operation", zap.Error(err))
		utils.ErrorResponse(c, http.StatusNotFound, "Operation not found", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Operation retrieved successfully", operation)
}

// ListOperations lists operations with filtering
// @Summary List operations
// @Description Get list of operations with filtering and pagination
// @Tags Operations
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Param device_id query string false "Filter by device ID"
// @Param operation_type query string false "Filter by operation type" Enums(PRINT, PAYMENT, SCAN, STATUS_CHECK, OPEN_DRAWER, DISPLAY_TEXT, BEEP, REFUND, CUT)
// @Param status query string false "Filter by status" Enums(PENDING, PROCESSING, SUCCESS, FAILED, TIMEOUT, CANCELLED)
// @Param start_date query string false "Start date filter (RFC3339)"
// @Param end_date query string false "End date filter (RFC3339)"
// @Success 200 {object} utils.APIResponse{data=object{operations=[]model.DeviceOperation,pagination=service.PaginationResult}} "Operations retrieved successfully"
// @Failure 500 {object} utils.APIResponse "Internal server error"
// @Router /operations [get]
func (h *OperationHandler) ListOperations(c *gin.Context) {
	filter := &service.OperationFilter{
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
	if deviceID := c.Query("device_id"); deviceID != "" {
		if id, err := uuid.Parse(deviceID); err == nil {
			filter.DeviceID = &id
		}
	}
	if operationType := c.Query("operation_type"); operationType != "" {
		ot := model.OperationType(operationType)
		filter.OperationType = &ot
	}
	if status := c.Query("status"); status != "" {
		s := model.OperationStatus(status)
		filter.Status = &s
	}
	if startDate := c.Query("start_date"); startDate != "" {
		if date, err := time.Parse(time.RFC3339, startDate); err == nil {
			filter.StartDate = &date
		}
	}
	if endDate := c.Query("end_date"); endDate != "" {
		if date, err := time.Parse(time.RFC3339, endDate); err == nil {
			filter.EndDate = &date
		}
	}

	operations, pagination, err := h.operationService.ListOperations(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list operations", zap.Error(err))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to list operations", err)
		return
	}

	response := gin.H{
		"operations": operations,
		"pagination": pagination,
	}

	utils.SuccessResponse(c, http.StatusOK, "Operations retrieved successfully", response)
}

// ListDeviceOperations handles device-specific operation listing
func (h *OperationHandler) ListDeviceOperations(c *gin.Context) {
	deviceIDStr := c.Param("device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid device ID", err)
		return
	}

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	filter := &service.OperationFilter{
		DeviceID:  &deviceID,
		Page:      1,
		PerPage:   limit,
		SortBy:    "created_at",
		SortOrder: "desc",
	}

	operations, pagination, err := h.operationService.ListOperations(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list device operations", zap.Error(err))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to list operations", err)
		return
	}

	response := gin.H{
		"operations": operations,
		"pagination": pagination,
	}

	utils.SuccessResponse(c, http.StatusOK, "Device operations retrieved successfully", response)
}

// CancelOperation cancels an operation
// @Summary Cancel operation
// @Description Cancel a pending or processing operation
// @Tags Operations
// @Accept json
// @Produce json
// @Param id path string true "Operation ID"
// @Param request body CancelOperationRequest true "Cancel operation request"
// @Success 200 {object} utils.APIResponse "Operation cancelled successfully"
// @Failure 400 {object} utils.APIResponse "Invalid request"
// @Failure 500 {object} utils.APIResponse "Cancel failed"
// @Router /operations/{id}/cancel [put]
func (h *OperationHandler) CancelOperation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid operation ID", err)
		return
	}

	var req CancelOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if err := h.operationService.CancelOperation(c.Request.Context(), id, req.Reason); err != nil {
		h.logger.Error("Failed to cancel operation", zap.Error(err))
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to cancel operation", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Operation cancelled successfully", gin.H{"operation_id": id})
}

// Request DTOs for operations

// DeviceOperationRequest represents a device operation request
type DeviceOperationRequest struct {
	OperationType model.OperationType     `json:"operation_type" binding:"required"`
	Data          map[string]interface{}  `json:"data" binding:"required"`
	Priority      model.OperationPriority `json:"priority"`
	CorrelationID *string                 `json:"correlation_id,omitempty"`
}

// PrintRequest represents a print operation request
type PrintRequest struct {
	Content     string `json:"content" binding:"required"`
	ContentType string `json:"content_type"`
	Copies      int    `json:"copies"`
	Cut         bool   `json:"cut"`
	OpenDrawer  bool   `json:"open_drawer"`
}

// PaymentRequest represents a payment operation request
type PaymentRequest struct {
	Amount        float64 `json:"amount" binding:"required"`
	Currency      string  `json:"currency"`
	PaymentMethod string  `json:"payment_method" binding:"required"`
	Reference     string  `json:"reference"`
	Timeout       int     `json:"timeout"`
}

// ScanRequest represents a scan operation request
type ScanRequest struct {
	ScanType string `json:"scan_type" binding:"required"`
	Timeout  int    `json:"timeout"`
}

// DisplayRequest represents a display operation request
type DisplayRequest struct {
	Line1    string `json:"line1" binding:"required"`
	Line2    string `json:"line2"`
	Duration int    `json:"duration"`
	Clear    bool   `json:"clear"`
}

// CancelOperationRequest represents an operation cancellation request
type CancelOperationRequest struct {
	Reason string `json:"reason" binding:"required"`
}
