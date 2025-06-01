// internal/repository/offline_repository.go
package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"device-service/internal/database"
	"device-service/internal/model"
)

// offlineRepository implements OfflineRepository interface
type offlineRepository struct {
	db     *database.DB
	logger *zap.Logger
}

// NewOfflineRepository creates a new offline repository
func NewOfflineRepository(db *database.DB, logger *zap.Logger) OfflineRepository {
	return &offlineRepository{
		db:     db,
		logger: logger,
	}
}

// Enqueue adds an operation to the offline queue
func (r *offlineRepository) Enqueue(ctx context.Context, operation *model.OfflineOperation) error {
	query := `
		INSERT INTO offline_operations (
			id, device_id, operation_type, operation_data, priority,
			sync_status, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(ctx, query,
		operation.ID, operation.DeviceID, operation.OperationType,
		operation.OperationData, operation.Priority, operation.SyncStatus,
		operation.ExpiresAt,
	)

	if err != nil {
		r.logger.Error("Failed to enqueue offline operation", zap.Error(err))
		return fmt.Errorf("failed to enqueue offline operation: %w", err)
	}

	return nil
}

// Dequeue retrieves pending operations for a device
func (r *offlineRepository) Dequeue(ctx context.Context, deviceID uuid.UUID, limit int) ([]*model.OfflineOperation, error) {
	query := `
		SELECT id, device_id, operation_type, operation_data, priority,
			   created_at, sync_status, sync_attempts, last_sync_attempt, expires_at
		FROM offline_operations
		WHERE device_id = $1 AND sync_status = 'PENDING'
		ORDER BY priority ASC, created_at ASC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, deviceID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to dequeue offline operations: %w", err)
	}
	defer rows.Close()

	operations := []*model.OfflineOperation{}
	for rows.Next() {
		operation := &model.OfflineOperation{}
		err := rows.Scan(
			&operation.ID, &operation.DeviceID, &operation.OperationType,
			&operation.OperationData, &operation.Priority, &operation.CreatedAt,
			&operation.SyncStatus, &operation.SyncAttempts, &operation.LastSyncAttempt,
			&operation.ExpiresAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan offline operation", zap.Error(err))
			continue
		}
		operations = append(operations, operation)
	}

	return operations, nil
}

// MarkSynced marks an operation as successfully synced
func (r *offlineRepository) MarkSynced(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE offline_operations 
		SET sync_status = 'SYNCED', last_sync_attempt = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark operation as synced: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("offline operation not found with id: %s", id)
	}

	return nil
}

// MarkFailed marks an operation sync attempt as failed
func (r *offlineRepository) MarkFailed(ctx context.Context, id uuid.UUID, attempts int) error {
	query := `
		UPDATE offline_operations 
		SET sync_attempts = $2, last_sync_attempt = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id, attempts)
	if err != nil {
		return fmt.Errorf("failed to mark operation sync as failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("offline operation not found with id: %s", id)
	}

	return nil
}

// GetQueueSize returns the queue size for a device
func (r *offlineRepository) GetQueueSize(ctx context.Context, deviceID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*) FROM offline_operations 
		WHERE device_id = $1 AND sync_status = 'PENDING'
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, deviceID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get queue size: %w", err)
	}

	return count, nil
}

// GetPendingOperations retrieves pending operations for retry
func (r *offlineRepository) GetPendingOperations(ctx context.Context, maxAttempts int) ([]*model.OfflineOperation, error) {
	query := `
		SELECT id, device_id, operation_type, operation_data, priority,
			   created_at, sync_status, sync_attempts, last_sync_attempt, expires_at
		FROM offline_operations
		WHERE sync_status = 'PENDING' AND sync_attempts < $1
		ORDER BY priority ASC, created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, maxAttempts)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending operations: %w", err)
	}
	defer rows.Close()

	operations := []*model.OfflineOperation{}
	for rows.Next() {
		operation := &model.OfflineOperation{}
		err := rows.Scan(
			&operation.ID, &operation.DeviceID, &operation.OperationType,
			&operation.OperationData, &operation.Priority, &operation.CreatedAt,
			&operation.SyncStatus, &operation.SyncAttempts, &operation.LastSyncAttempt,
			&operation.ExpiresAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan offline operation", zap.Error(err))
			continue
		}
		operations = append(operations, operation)
	}

	return operations, nil
}

// DeleteExpired removes expired operations
func (r *offlineRepository) DeleteExpired(ctx context.Context) (int64, error) {
	query := `
		DELETE FROM offline_operations 
		WHERE expires_at IS NOT NULL AND expires_at < CURRENT_TIMESTAMP
	`

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired operations: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected > 0 {
		r.logger.Info("Deleted expired offline operations", zap.Int64("count", rowsAffected))
	}

	return rowsAffected, nil
}

// ClearQueue removes all pending operations for a device
func (r *offlineRepository) ClearQueue(ctx context.Context, deviceID uuid.UUID) error {
	query := `DELETE FROM offline_operations WHERE device_id = $1 AND sync_status = 'PENDING'`

	result, err := r.db.ExecContext(ctx, query, deviceID)
	if err != nil {
		return fmt.Errorf("failed to clear queue: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	r.logger.Info("Cleared offline queue",
		zap.String("device_id", deviceID.String()),
		zap.Int64("operations_removed", rowsAffected),
	)

	return nil
}
