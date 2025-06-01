// internal/repository/interfaces.go
package repository

import (
	"context"
	"time"

	"device-service/internal/model"

	"github.com/google/uuid"
)

// DeviceRepository defines device data access operations
type DeviceRepository interface {
	// CRUD operations
	Create(ctx context.Context, device *model.Device) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Device, error)
	GetByDeviceID(ctx context.Context, deviceID string) (*model.Device, error)
	Update(ctx context.Context, device *model.Device) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.DeviceStatus) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Listing and filtering
	List(ctx context.Context, filter *DeviceFilter) ([]*model.Device, int, error)
	ListByBranch(ctx context.Context, branchID uuid.UUID) ([]*model.Device, error)
	ListByStatus(ctx context.Context, status model.DeviceStatus) ([]*model.Device, error)

	// Health and monitoring
	UpdateLastPing(ctx context.Context, id uuid.UUID, pingTime time.Time) error
	GetHealthLogs(ctx context.Context, deviceID uuid.UUID, limit int) ([]*model.DeviceHealth, error)
	CreateHealthLog(ctx context.Context, health *model.DeviceHealth) error

	// Batch operations
	UpdateMultipleStatus(ctx context.Context, deviceIDs []uuid.UUID, status model.DeviceStatus) error
	GetDeviceStats(ctx context.Context, branchID *uuid.UUID) (*DeviceStats, error)
}

// OperationRepository defines operation data access operations
type OperationRepository interface {
	// CRUD operations
	Create(ctx context.Context, operation *model.DeviceOperation) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.DeviceOperation, error)
	Update(ctx context.Context, operation *model.DeviceOperation) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.OperationStatus) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Listing and filtering
	List(ctx context.Context, filter *OperationFilter) ([]*model.DeviceOperation, int, error)
	ListByDevice(ctx context.Context, deviceID uuid.UUID, limit int) ([]*model.DeviceOperation, error)
	ListByCorrelation(ctx context.Context, correlationID uuid.UUID) ([]*model.DeviceOperation, error)
	ListPending(ctx context.Context, priority *model.OperationPriority) ([]*model.DeviceOperation, error)

	// Analytics and reporting
	GetOperationStats(ctx context.Context, filter *OperationStatsFilter) (*OperationStats, error)
	GetDeviceOperationSummary(ctx context.Context, deviceID uuid.UUID, period time.Duration) (*OperationSummary, error)

	// Cleanup
	DeleteOldOperations(ctx context.Context, olderThan time.Time) (int64, error)
}

// OfflineRepository defines offline operation data access operations
type OfflineRepository interface {
	// Queue operations
	Enqueue(ctx context.Context, operation *model.OfflineOperation) error
	Dequeue(ctx context.Context, deviceID uuid.UUID, limit int) ([]*model.OfflineOperation, error)
	MarkSynced(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, attempts int) error

	// Queue management
	GetQueueSize(ctx context.Context, deviceID uuid.UUID) (int, error)
	GetPendingOperations(ctx context.Context, maxAttempts int) ([]*model.OfflineOperation, error)
	DeleteExpired(ctx context.Context) (int64, error)
	ClearQueue(ctx context.Context, deviceID uuid.UUID) error
}

// Filter structures

// DeviceFilter represents device listing filters
type DeviceFilter struct {
	BranchID   *uuid.UUID          `json:"branch_id,omitempty"`
	DeviceType *model.DeviceType   `json:"device_type,omitempty"`
	Brand      *model.DeviceBrand  `json:"brand,omitempty"`
	Status     *model.DeviceStatus `json:"status,omitempty"`
	Location   *string             `json:"location,omitempty"`
	SearchTerm *string             `json:"search_term,omitempty"`
	Page       int                 `json:"page"`
	PerPage    int                 `json:"per_page"`
	SortBy     string              `json:"sort_by"`
	SortOrder  string              `json:"sort_order"`
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

// OperationStatsFilter represents operation statistics filters
type OperationStatsFilter struct {
	DeviceID  *uuid.UUID `json:"device_id,omitempty"`
	BranchID  *uuid.UUID `json:"branch_id,omitempty"`
	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
}

// Statistics structures

// DeviceStats represents device statistics
type DeviceStats struct {
	TotalDevices   int                        `json:"total_devices"`
	OnlineDevices  int                        `json:"online_devices"`
	OfflineDevices int                        `json:"offline_devices"`
	ErrorDevices   int                        `json:"error_devices"`
	ByType         map[model.DeviceType]int   `json:"by_type"`
	ByBrand        map[model.DeviceBrand]int  `json:"by_brand"`
	ByStatus       map[model.DeviceStatus]int `json:"by_status"`
}

// OperationStats represents operation statistics
type OperationStats struct {
	TotalOperations int                             `json:"total_operations"`
	SuccessfulOps   int                             `json:"successful_operations"`
	FailedOps       int                             `json:"failed_operations"`
	PendingOps      int                             `json:"pending_operations"`
	AvgDuration     time.Duration                   `json:"average_duration"`
	ByType          map[model.OperationType]int     `json:"by_type"`
	ByStatus        map[model.OperationStatus]int   `json:"by_status"`
	ByPriority      map[model.OperationPriority]int `json:"by_priority"`
}

// OperationSummary represents operation summary for a device
type OperationSummary struct {
	DeviceID        uuid.UUID     `json:"device_id"`
	Period          time.Duration `json:"period"`
	TotalOps        int           `json:"total_operations"`
	SuccessRate     float64       `json:"success_rate"`
	AvgResponseTime time.Duration `json:"average_response_time"`
	ErrorCount      int           `json:"error_count"`
	LastOperation   *time.Time    `json:"last_operation,omitempty"`
}
