// internal/repository/operation_repository.go
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"device-service/internal/database"
	"device-service/internal/model"
)

// operationRepository implements OperationRepository interface
type operationRepository struct {
	db     *database.DB
	logger *zap.Logger
}

// NewOperationRepository creates a new operation repository
func NewOperationRepository(db *database.DB, logger *zap.Logger) OperationRepository {
	return &operationRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new operation
func (r *operationRepository) Create(ctx context.Context, operation *model.DeviceOperation) error {
	query := `
		INSERT INTO device_operations (
			id, device_id, operation_type, operation_data, priority,
			status, started_at, correlation_id, result
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.ExecContext(ctx, query,
		operation.ID, operation.DeviceID, operation.OperationType,
		operation.OperationData, operation.Priority, operation.Status,
		operation.StartedAt, operation.CorrelationID, operation.Result,
	)

	if err != nil {
		r.logger.Error("Failed to create operation", zap.Error(err))
		return fmt.Errorf("failed to create operation: %w", err)
	}

	return nil
}

// GetByID retrieves an operation by ID
func (r *operationRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.DeviceOperation, error) {
	query := `
		SELECT id, device_id, operation_type, operation_data, priority,
			   status, started_at, completed_at, duration_ms, error_message,
			   retry_count, correlation_id, result, created_at
		FROM device_operations WHERE id = $1
	`

	operation := &model.DeviceOperation{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&operation.ID, &operation.DeviceID, &operation.OperationType,
		&operation.OperationData, &operation.Priority, &operation.Status,
		&operation.StartedAt, &operation.CompletedAt, &operation.DurationMs,
		&operation.ErrorMessage, &operation.RetryCount, &operation.CorrelationID,
		&operation.Result, &operation.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("operation not found with id: %s", id)
		}
		return nil, fmt.Errorf("failed to get operation: %w", err)
	}

	return operation, nil
}

// Update updates an existing operation
func (r *operationRepository) Update(ctx context.Context, operation *model.DeviceOperation) error {
	query := `
		UPDATE device_operations SET
			status = $2, completed_at = $3, duration_ms = $4,
			error_message = $5, retry_count = $6, result = $7
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		operation.ID, operation.Status, operation.CompletedAt,
		operation.DurationMs, operation.ErrorMessage, operation.RetryCount,
		operation.Result,
	)

	if err != nil {
		return fmt.Errorf("failed to update operation: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("operation not found with id: %s", operation.ID)
	}

	return nil
}

// UpdateStatus updates operation status
func (r *operationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.OperationStatus) error {
	query := `UPDATE device_operations SET status = $2 WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("failed to update operation status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("operation not found with id: %s", id)
	}

	return nil
}

// Delete removes an operation
func (r *operationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM device_operations WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete operation: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("operation not found with id: %s", id)
	}

	return nil
}

// List retrieves operations with filtering and pagination
func (r *operationRepository) List(ctx context.Context, filter *OperationFilter) ([]*model.DeviceOperation, int, error) {
	// Build WHERE clause
	whereConditions := []string{}
	args := []interface{}{}
	argIndex := 1

	if filter.DeviceID != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("device_id = $%d", argIndex))
		args = append(args, *filter.DeviceID)
		argIndex++
	}

	if filter.OperationType != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("operation_type = $%d", argIndex))
		args = append(args, *filter.OperationType)
		argIndex++
	}

	if filter.Status != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, *filter.Status)
		argIndex++
	}

	if filter.Priority != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("priority = $%d", argIndex))
		args = append(args, *filter.Priority)
		argIndex++
	}

	if filter.CorrelationID != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("correlation_id = $%d", argIndex))
		args = append(args, *filter.CorrelationID)
		argIndex++
	}

	if filter.StartDate != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *filter.StartDate)
		argIndex++
	}

	if filter.EndDate != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, *filter.EndDate)
		argIndex++
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + strings.Join(whereConditions, " AND ")
	}

	// Count total records
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM device_operations %s", whereClause)
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count operations: %w", err)
	}

	// Build ORDER BY clause
	orderBy := "created_at DESC"
	if filter.SortBy != "" {
		order := "ASC"
		if filter.SortOrder == "desc" {
			order = "DESC"
		}
		orderBy = fmt.Sprintf("%s %s", filter.SortBy, order)
	}

	// Build main query with pagination
	offset := (filter.Page - 1) * filter.PerPage
	query := fmt.Sprintf(`
		SELECT id, device_id, operation_type, operation_data, priority,
			   status, started_at, completed_at, duration_ms, error_message,
			   retry_count, correlation_id, result, created_at
		FROM device_operations %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderBy, argIndex, argIndex+1)

	args = append(args, filter.PerPage, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list operations: %w", err)
	}
	defer rows.Close()

	operations := []*model.DeviceOperation{}
	for rows.Next() {
		operation := &model.DeviceOperation{}
		err := rows.Scan(
			&operation.ID, &operation.DeviceID, &operation.OperationType,
			&operation.OperationData, &operation.Priority, &operation.Status,
			&operation.StartedAt, &operation.CompletedAt, &operation.DurationMs,
			&operation.ErrorMessage, &operation.RetryCount, &operation.CorrelationID,
			&operation.Result, &operation.CreatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan operation row", zap.Error(err))
			continue
		}
		operations = append(operations, operation)
	}

	return operations, total, nil
}

// ListByDevice retrieves operations for a specific device
func (r *operationRepository) ListByDevice(ctx context.Context, deviceID uuid.UUID, limit int) ([]*model.DeviceOperation, error) {
	query := `
		SELECT id, device_id, operation_type, operation_data, priority,
			   status, started_at, completed_at, duration_ms, error_message,
			   retry_count, correlation_id, result, created_at
		FROM device_operations 
		WHERE device_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, deviceID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list operations by device: %w", err)
	}
	defer rows.Close()

	operations := []*model.DeviceOperation{}
	for rows.Next() {
		operation := &model.DeviceOperation{}
		err := rows.Scan(
			&operation.ID, &operation.DeviceID, &operation.OperationType,
			&operation.OperationData, &operation.Priority, &operation.Status,
			&operation.StartedAt, &operation.CompletedAt, &operation.DurationMs,
			&operation.ErrorMessage, &operation.RetryCount, &operation.CorrelationID,
			&operation.Result, &operation.CreatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan operation row", zap.Error(err))
			continue
		}
		operations = append(operations, operation)
	}

	return operations, nil
}

// ListByCorrelation retrieves operations by correlation ID
func (r *operationRepository) ListByCorrelation(ctx context.Context, correlationID uuid.UUID) ([]*model.DeviceOperation, error) {
	query := `
		SELECT id, device_id, operation_type, operation_data, priority,
			   status, started_at, completed_at, duration_ms, error_message,
			   retry_count, correlation_id, result, created_at
		FROM device_operations 
		WHERE correlation_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, correlationID)
	if err != nil {
		return nil, fmt.Errorf("failed to list operations by correlation: %w", err)
	}
	defer rows.Close()

	operations := []*model.DeviceOperation{}
	for rows.Next() {
		operation := &model.DeviceOperation{}
		err := rows.Scan(
			&operation.ID, &operation.DeviceID, &operation.OperationType,
			&operation.OperationData, &operation.Priority, &operation.Status,
			&operation.StartedAt, &operation.CompletedAt, &operation.DurationMs,
			&operation.ErrorMessage, &operation.RetryCount, &operation.CorrelationID,
			&operation.Result, &operation.CreatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan operation row", zap.Error(err))
			continue
		}
		operations = append(operations, operation)
	}

	return operations, nil
}

// ListPending retrieves pending operations
func (r *operationRepository) ListPending(ctx context.Context, priority *model.OperationPriority) ([]*model.DeviceOperation, error) {
	whereClause := "WHERE status = 'PENDING'"
	args := []interface{}{}

	if priority != nil {
		whereClause += " AND priority = $1"
		args = append(args, *priority)
	}

	query := fmt.Sprintf(`
		SELECT id, device_id, operation_type, operation_data, priority,
			   status, started_at, completed_at, duration_ms, error_message,
			   retry_count, correlation_id, result, created_at
		FROM device_operations %s
		ORDER BY priority ASC, created_at ASC
	`, whereClause)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending operations: %w", err)
	}
	defer rows.Close()

	operations := []*model.DeviceOperation{}
	for rows.Next() {
		operation := &model.DeviceOperation{}
		err := rows.Scan(
			&operation.ID, &operation.DeviceID, &operation.OperationType,
			&operation.OperationData, &operation.Priority, &operation.Status,
			&operation.StartedAt, &operation.CompletedAt, &operation.DurationMs,
			&operation.ErrorMessage, &operation.RetryCount, &operation.CorrelationID,
			&operation.Result, &operation.CreatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan operation row", zap.Error(err))
			continue
		}
		operations = append(operations, operation)
	}

	return operations, nil
}

// GetOperationStats retrieves operation statistics
func (r *operationRepository) GetOperationStats(ctx context.Context, filter *OperationStatsFilter) (*OperationStats, error) {
	whereConditions := []string{}
	args := []interface{}{}
	argIndex := 1

	if filter.DeviceID != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("device_id = $%d", argIndex))
		args = append(args, *filter.DeviceID)
		argIndex++
	}

	if filter.StartDate != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *filter.StartDate)
		argIndex++
	}

	if filter.EndDate != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, *filter.EndDate)
		argIndex++
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + strings.Join(whereConditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT 
			COUNT(*) as total_operations,
			COUNT(CASE WHEN status = 'SUCCESS' THEN 1 END) as successful_ops,
			COUNT(CASE WHEN status = 'FAILED' THEN 1 END) as failed_ops,
			COUNT(CASE WHEN status = 'PENDING' THEN 1 END) as pending_ops,
			AVG(duration_ms) as avg_duration_ms
		FROM device_operations %s
	`, whereClause)

	stats := &OperationStats{
		ByType:     make(map[model.OperationType]int),
		ByStatus:   make(map[model.OperationStatus]int),
		ByPriority: make(map[model.OperationPriority]int),
	}

	var avgDurationMs sql.NullFloat64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.TotalOperations,
		&stats.SuccessfulOps,
		&stats.FailedOps,
		&stats.PendingOps,
		&avgDurationMs,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get operation stats: %w", err)
	}

	if avgDurationMs.Valid {
		stats.AvgDuration = time.Duration(avgDurationMs.Float64) * time.Millisecond
	}

	return stats, nil
}

// GetDeviceOperationSummary retrieves operation summary for a device
func (r *operationRepository) GetDeviceOperationSummary(ctx context.Context, deviceID uuid.UUID, period time.Duration) (*OperationSummary, error) {
	since := time.Now().Add(-period)

	query := `
		SELECT 
			COUNT(*) as total_ops,
			COUNT(CASE WHEN status = 'SUCCESS' THEN 1 END) as successful_ops,
			COUNT(CASE WHEN status = 'FAILED' THEN 1 END) as error_count,
			AVG(duration_ms) as avg_response_time_ms,
			MAX(created_at) as last_operation
		FROM device_operations
		WHERE device_id = $1 AND created_at >= $2
	`

	summary := &OperationSummary{
		DeviceID: deviceID,
		Period:   period,
	}

	var successfulOps, avgResponseTimeMs sql.NullFloat64
	var lastOp sql.NullTime

	err := r.db.QueryRowContext(ctx, query, deviceID, since).Scan(
		&summary.TotalOps,
		&successfulOps,
		&summary.ErrorCount,
		&avgResponseTimeMs,
		&lastOp,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get operation summary: %w", err)
	}

	if summary.TotalOps > 0 {
		summary.SuccessRate = successfulOps.Float64 / float64(summary.TotalOps)
	}

	if avgResponseTimeMs.Valid {
		summary.AvgResponseTime = time.Duration(avgResponseTimeMs.Float64) * time.Millisecond
	}

	if lastOp.Valid {
		summary.LastOperation = &lastOp.Time
	}

	return summary, nil
}

// DeleteOldOperations removes old operation records
func (r *operationRepository) DeleteOldOperations(ctx context.Context, olderThan time.Time) (int64, error) {
	query := `DELETE FROM device_operations WHERE created_at < $1`

	result, err := r.db.ExecContext(ctx, query, olderThan)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old operations: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	r.logger.Info("Deleted old operations",
		zap.Int64("rows_deleted", rowsAffected),
		zap.Time("older_than", olderThan),
	)

	return rowsAffected, nil
}
