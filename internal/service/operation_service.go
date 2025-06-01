// internal/service/operation_service.go
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"device-service/internal/config"
	"device-service/internal/driver"
	"device-service/internal/model"
	"device-service/internal/repository"
	"device-service/internal/utils"
)

// OperationService handles device operation business logic
type OperationService struct {
	operationRepo  repository.OperationRepository
	deviceRepo     repository.DeviceRepository
	driverRegistry *driver.Registry
	config         *config.Config
	logger         *utils.ServiceLogger
	auditLogger    *utils.AuditLogger
}

// NewOperationService creates a new operation service instance
func NewOperationService(
	operationRepo repository.OperationRepository,
	deviceRepo repository.DeviceRepository,
	driverRegistry *driver.Registry,
	config *config.Config,
	logger *zap.Logger,
) *OperationService {
	return &OperationService{
		operationRepo:  operationRepo,
		deviceRepo:     deviceRepo,
		driverRegistry: driverRegistry,
		config:         config,
		logger:         utils.NewServiceLogger(logger, "operation-service"),
		auditLogger:    utils.NewAuditLogger(logger),
	}
}

// ExecuteOperation executes an operation on a device
func (os *OperationService) ExecuteOperation(ctx context.Context, req *OperationRequest) (*OperationResponse, error) {
	// Create operation record
	operation := &model.DeviceOperation{
		ID:            uuid.New(),
		DeviceID:      req.DeviceID,
		OperationType: req.OperationType,
		OperationData: model.JSONObject(req.Data),
		Priority:      req.Priority,
		Status:        model.OperationStatusPending,
		StartedAt:     time.Now(),
		CorrelationID: req.CorrelationID,
		CreatedAt:     time.Now(),
	}

	// Save operation to database
	if err := os.operationRepo.Create(ctx, operation); err != nil {
		return nil, fmt.Errorf("failed to create operation: %w", err)
	}

	// Create operation logger
	opLogger := utils.NewOperationLogger(os.logger.Logger, string(req.OperationType), operation.ID.String())
	opLogger.Start(zap.String("device_id", req.DeviceID.String()))

	// Get device
	device, err := os.deviceRepo.GetByID(ctx, req.DeviceID)
	if err != nil {
		os.updateOperationError(ctx, operation, err)
		opLogger.Error(err)
		return nil, fmt.Errorf("device not found: %w", err)
	}

	// Check if device is online
	if device.Status != model.DeviceStatusOnline {
		err := fmt.Errorf("device is not online: %s", device.Status)
		os.updateOperationError(ctx, operation, err)
		opLogger.Error(err)
		return nil, err
	}

	// Create driver instance
	driverInstance, err := os.driverRegistry.CreateDriver(device, device.ConnectionConfig)
	if err != nil {
		os.updateOperationError(ctx, operation, err)
		opLogger.Error(err)
		return nil, fmt.Errorf("failed to create driver: %w", err)
	}

	// Update operation status to processing
	operation.Status = model.OperationStatusProcessing
	if err := os.operationRepo.UpdateStatus(ctx, operation.ID, operation.Status); err != nil {
		os.logger.Error("Failed to update operation status", zap.Error(err))
	}

	// Execute operation with timeout
	timeout := os.getOperationTimeout(req.OperationType)
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := driverInstance.ExecuteOperation(execCtx, operation)
	if err != nil {
		os.updateOperationError(ctx, operation, err)
		opLogger.Error(err)
		return nil, fmt.Errorf("operation execution failed: %w", err)
	}

	// Update operation as completed
	completedAt := time.Now()
	operation.Status = model.OperationStatusSuccess
	operation.CompletedAt = &completedAt
	operation.Result = model.JSONObject(result.Data)

	if err := os.operationRepo.Update(ctx, operation); err != nil {
		os.logger.Error("Failed to update operation", zap.Error(err))
	}
	duration, err := time.ParseDuration(result.Duration)
	if err != nil {
		panic(err)
	}
	opLogger.Success(
		zap.Duration("duration", duration),
		zap.Any("result", result.Data),
	)

	// Audit log for sensitive operations
	if req.OperationType == model.OperationTypePayment {
		os.auditLogger.LogPaymentTransaction(
			device.DeviceID,
			operation.ID.String(),
			0, // Amount would be extracted from operation data
			"TRY",
			"SUCCESS",
		)
	}

	return &OperationResponse{
		OperationID: operation.ID,
		Success:     true,
		Result:      result.Data,
		Duration:    result.Duration,
	}, nil
}

// GetOperation retrieves operation details
func (os *OperationService) GetOperation(ctx context.Context, operationID uuid.UUID) (*model.DeviceOperation, error) {
	operation, err := os.operationRepo.GetByID(ctx, operationID)
	if err != nil {
		return nil, fmt.Errorf("operation not found: %w", err)
	}
	return operation, nil
}

// ListOperations lists operations with filtering
func (os *OperationService) ListOperations(ctx context.Context, filter *OperationFilter) ([]*model.DeviceOperation, *PaginationResult, error) {
	operations, total, err := os.operationRepo.List(ctx, filter.toRepoFilter())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list operations: %w", err)
	}

	pagination := &PaginationResult{
		Total:      total,
		Page:       filter.Page,
		PerPage:    filter.PerPage,
		TotalPages: (total + filter.PerPage - 1) / filter.PerPage,
	}

	return operations, pagination, nil
}

// CancelOperation cancels a pending operation
func (os *OperationService) CancelOperation(ctx context.Context, operationID uuid.UUID, reason string) error {
	operation, err := os.operationRepo.GetByID(ctx, operationID)
	if err != nil {
		return fmt.Errorf("operation not found: %w", err)
	}

	if operation.Status != model.OperationStatusPending && operation.Status != model.OperationStatusProcessing {
		return fmt.Errorf("cannot cancel operation in status: %s", operation.Status)
	}

	completedAt := time.Now()
	operation.Status = model.OperationStatusCancelled
	operation.CompletedAt = &completedAt
	operation.ErrorMessage = &reason

	if err := os.operationRepo.Update(ctx, operation); err != nil {
		return fmt.Errorf("failed to cancel operation: %w", err)
	}

	os.logger.Info("Operation cancelled",
		zap.String("operation_id", operationID.String()),
		zap.String("reason", reason),
	)

	return nil
}

// Helper methods

// updateOperationError updates operation with error
func (os *OperationService) updateOperationError(ctx context.Context, operation *model.DeviceOperation, err error) {
	completedAt := time.Now()
	operation.Status = model.OperationStatusFailed
	operation.CompletedAt = &completedAt
	errorMsg := err.Error()
	operation.ErrorMessage = &errorMsg

	if updateErr := os.operationRepo.Update(ctx, operation); updateErr != nil {
		os.logger.Error("Failed to update operation error", zap.Error(updateErr))
	}
}

// getOperationTimeout returns timeout for operation type
func (os *OperationService) getOperationTimeout(operationType model.OperationType) time.Duration {
	switch operationType {
	case model.OperationTypePayment:
		return 60 * time.Second
	case model.OperationTypePrint:
		return 30 * time.Second
	case model.OperationTypeScan:
		return 30 * time.Second
	default:
		return os.config.Device.OperationTimeout
	}
}

// DTOs for Operation Service

// OperationRequest represents operation execution request
type OperationRequest struct {
	DeviceID      uuid.UUID               `json:"device_id"`
	OperationType model.OperationType     `json:"operation_type"`
	Data          map[string]interface{}  `json:"data"`
	Priority      model.OperationPriority `json:"priority"`
	CorrelationID *uuid.UUID              `json:"correlation_id,omitempty"`
}

// OperationResponse represents operation execution response
type OperationResponse struct {
	OperationID  uuid.UUID              `json:"operation_id"`
	Success      bool                   `json:"success"`
	Result       map[string]interface{} `json:"result,omitempty"`
	Duration     string                 `json:"duration"`
	ErrorMessage string                 `json:"error_message,omitempty"`
}

// OperationFilter represents operation listing filters
type OperationFilter struct {
	DeviceID      *uuid.UUID               `json:"device_id,omitempty"`
	OperationType *model.OperationType     `json:"operation_type,omitempty"`
	Status        *model.OperationStatus   `json:"status,omitempty"`
	Priority      *model.OperationPriority `json:"priority,omitempty"`
	CorrelationID *uuid.UUID               `json:"correlation_id,omitempty"`
	StartDate     *time.Time               `json:"start_date,omitempty"`
	EndDate       *time.Time               `json:"end_date,omitempty"`
	Page          int                      `json:"page"`
	PerPage       int                      `json:"per_page"`
	SortBy        string                   `json:"sort_by"`
	SortOrder     string                   `json:"sort_order"`
}

// toRepoFilter converts to repository filter
func (of *OperationFilter) toRepoFilter() *repository.OperationFilter {
	return &repository.OperationFilter{
		DeviceID:      of.DeviceID,
		OperationType: of.OperationType,
		Status:        of.Status,
		Priority:      of.Priority,
		CorrelationID: of.CorrelationID,
		StartDate:     of.StartDate,
		EndDate:       of.EndDate,
		Page:          of.Page,
		PerPage:       of.PerPage,
		SortBy:        of.SortBy,
		SortOrder:     of.SortOrder,
	}
}
