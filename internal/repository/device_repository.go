// internal/repository/device_repository.go
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

// deviceRepository implements DeviceRepository interface
type deviceRepository struct {
	db     *database.DB
	logger *zap.Logger
}

// NewDeviceRepository creates a new device repository
func NewDeviceRepository(db *database.DB, logger *zap.Logger) DeviceRepository {
	return &deviceRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new device
func (r *deviceRepository) Create(ctx context.Context, device *model.Device) error {
	query := `
		INSERT INTO devices (
			id, device_id, device_type, brand, model, firmware_version,
			connection_type, connection_config, capabilities, branch_id,
			location, status, error_info, performance_metrics
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err := r.db.ExecContext(ctx, query,
		device.ID, device.DeviceID, device.DeviceType, device.Brand,
		device.Model, device.FirmwareVersion, device.ConnectionType,
		device.ConnectionConfig, device.Capabilities, device.BranchID,
		device.Location, device.Status, device.ErrorInfo, device.PerformanceMetrics,
	)

	if err != nil {
		r.logger.Error("Failed to create device", zap.Error(err), zap.String("device_id", device.DeviceID))
		return fmt.Errorf("failed to create device: %w", err)
	}

	r.logger.Info("Device created successfully", zap.String("device_id", device.DeviceID))
	return nil
}

// GetByID retrieves a device by its UUID
func (r *deviceRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Device, error) {
	query := `
		SELECT id, device_id, device_type, brand, model, firmware_version,
			   connection_type, connection_config, capabilities, branch_id,
			   location, status, last_ping, error_info, performance_metrics,
			   created_at, updated_at
		FROM devices WHERE id = $1
	`

	device := &model.Device{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&device.ID, &device.DeviceID, &device.DeviceType, &device.Brand,
		&device.Model, &device.FirmwareVersion, &device.ConnectionType,
		&device.ConnectionConfig, &device.Capabilities, &device.BranchID,
		&device.Location, &device.Status, &device.LastPing, &device.ErrorInfo,
		&device.PerformanceMetrics, &device.CreatedAt, &device.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("device not found with id: %s", id)
		}
		r.logger.Error("Failed to get device by ID", zap.Error(err), zap.String("id", id.String()))
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	return device, nil
}

// GetByDeviceID retrieves a device by its device ID
func (r *deviceRepository) GetByDeviceID(ctx context.Context, deviceID string) (*model.Device, error) {
	query := `
		SELECT id, device_id, device_type, brand, model, firmware_version,
			   connection_type, connection_config, capabilities, branch_id,
			   location, status, last_ping, error_info, performance_metrics,
			   created_at, updated_at
		FROM devices WHERE device_id = $1
	`

	device := &model.Device{}
	err := r.db.QueryRowContext(ctx, query, deviceID).Scan(
		&device.ID, &device.DeviceID, &device.DeviceType, &device.Brand,
		&device.Model, &device.FirmwareVersion, &device.ConnectionType,
		&device.ConnectionConfig, &device.Capabilities, &device.BranchID,
		&device.Location, &device.Status, &device.LastPing, &device.ErrorInfo,
		&device.PerformanceMetrics, &device.CreatedAt, &device.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("device not found with device_id: %s", deviceID)
		}
		r.logger.Error("Failed to get device by device_id", zap.Error(err), zap.String("device_id", deviceID))
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	return device, nil
}

// Update updates an existing device
func (r *deviceRepository) Update(ctx context.Context, device *model.Device) error {
	query := `
		UPDATE devices SET
			device_type = $2, brand = $3, model = $4, firmware_version = $5,
			connection_type = $6, connection_config = $7, capabilities = $8,
			branch_id = $9, location = $10, status = $11, last_ping = $12,
			error_info = $13, performance_metrics = $14, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		device.ID, device.DeviceType, device.Brand, device.Model,
		device.FirmwareVersion, device.ConnectionType, device.ConnectionConfig,
		device.Capabilities, device.BranchID, device.Location, device.Status,
		device.LastPing, device.ErrorInfo, device.PerformanceMetrics,
	)

	if err != nil {
		r.logger.Error("Failed to update device", zap.Error(err), zap.String("device_id", device.DeviceID))
		return fmt.Errorf("failed to update device: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("device not found with id: %s", device.ID)
	}

	r.logger.Debug("Device updated successfully", zap.String("device_id", device.DeviceID))
	return nil
}

// UpdateStatus updates device status
func (r *deviceRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.DeviceStatus) error {
	query := `
		UPDATE devices SET status = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id, status)
	if err != nil {
		r.logger.Error("Failed to update device status", zap.Error(err), zap.String("id", id.String()))
		return fmt.Errorf("failed to update device status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("device not found with id: %s", id)
	}

	return nil
}

// Delete removes a device
func (r *deviceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM devices WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete device", zap.Error(err), zap.String("id", id.String()))
		return fmt.Errorf("failed to delete device: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("device not found with id: %s", id)
	}

	r.logger.Info("Device deleted successfully", zap.String("id", id.String()))
	return nil
}

// List retrieves devices with filtering and pagination
func (r *deviceRepository) List(ctx context.Context, filter *DeviceFilter) ([]*model.Device, int, error) {
	// Build WHERE clause
	whereConditions := []string{}
	args := []interface{}{}
	argIndex := 1

	if filter.BranchID != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("branch_id = $%d", argIndex))
		args = append(args, *filter.BranchID)
		argIndex++
	}

	if filter.DeviceType != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("device_type = $%d", argIndex))
		args = append(args, *filter.DeviceType)
		argIndex++
	}

	if filter.Brand != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("brand = $%d", argIndex))
		args = append(args, *filter.Brand)
		argIndex++
	}

	if filter.Status != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, *filter.Status)
		argIndex++
	}

	if filter.Location != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("location ILIKE $%d", argIndex))
		args = append(args, "%"+*filter.Location+"%")
		argIndex++
	}

	if filter.SearchTerm != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("(device_id ILIKE $%d OR model ILIKE $%d)", argIndex, argIndex))
		args = append(args, "%"+*filter.SearchTerm+"%")
		argIndex++
	}

	// Build WHERE clause
	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + strings.Join(whereConditions, " AND ")
	}

	// Count total records
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM devices %s", whereClause)
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count devices: %w", err)
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
		SELECT id, device_id, device_type, brand, model, firmware_version,
			   connection_type, connection_config, capabilities, branch_id,
			   location, status, last_ping, error_info, performance_metrics,
			   created_at, updated_at
		FROM devices %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderBy, argIndex, argIndex+1)

	args = append(args, filter.PerPage, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to list devices", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to list devices: %w", err)
	}
	defer rows.Close()

	devices := []*model.Device{}
	for rows.Next() {
		device := &model.Device{}
		err := rows.Scan(
			&device.ID, &device.DeviceID, &device.DeviceType, &device.Brand,
			&device.Model, &device.FirmwareVersion, &device.ConnectionType,
			&device.ConnectionConfig, &device.Capabilities, &device.BranchID,
			&device.Location, &device.Status, &device.LastPing, &device.ErrorInfo,
			&device.PerformanceMetrics, &device.CreatedAt, &device.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan device row", zap.Error(err))
			continue
		}
		devices = append(devices, device)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate device rows: %w", err)
	}

	return devices, total, nil
}

// ListByBranch retrieves devices by branch
func (r *deviceRepository) ListByBranch(ctx context.Context, branchID uuid.UUID) ([]*model.Device, error) {
	query := `
		SELECT id, device_id, device_type, brand, model, firmware_version,
			   connection_type, connection_config, capabilities, branch_id,
			   location, status, last_ping, error_info, performance_metrics,
			   created_at, updated_at
		FROM devices 
		WHERE branch_id = $1
		ORDER BY device_type, device_id
	`

	rows, err := r.db.QueryContext(ctx, query, branchID)
	if err != nil {
		r.logger.Error("Failed to list devices by branch", zap.Error(err))
		return nil, fmt.Errorf("failed to list devices by branch: %w", err)
	}
	defer rows.Close()

	devices := []*model.Device{}
	for rows.Next() {
		device := &model.Device{}
		err := rows.Scan(
			&device.ID, &device.DeviceID, &device.DeviceType, &device.Brand,
			&device.Model, &device.FirmwareVersion, &device.ConnectionType,
			&device.ConnectionConfig, &device.Capabilities, &device.BranchID,
			&device.Location, &device.Status, &device.LastPing, &device.ErrorInfo,
			&device.PerformanceMetrics, &device.CreatedAt, &device.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan device row", zap.Error(err))
			continue
		}
		devices = append(devices, device)
	}

	return devices, nil
}

// ListByStatus retrieves devices by status
func (r *deviceRepository) ListByStatus(ctx context.Context, status model.DeviceStatus) ([]*model.Device, error) {
	query := `
		SELECT id, device_id, device_type, brand, model, firmware_version,
			   connection_type, connection_config, capabilities, branch_id,
			   location, status, last_ping, error_info, performance_metrics,
			   created_at, updated_at
		FROM devices 
		WHERE status = $1
		ORDER BY last_ping DESC
	`

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		r.logger.Error("Failed to list devices by status", zap.Error(err))
		return nil, fmt.Errorf("failed to list devices by status: %w", err)
	}
	defer rows.Close()

	devices := []*model.Device{}
	for rows.Next() {
		device := &model.Device{}
		err := rows.Scan(
			&device.ID, &device.DeviceID, &device.DeviceType, &device.Brand,
			&device.Model, &device.FirmwareVersion, &device.ConnectionType,
			&device.ConnectionConfig, &device.Capabilities, &device.BranchID,
			&device.Location, &device.Status, &device.LastPing, &device.ErrorInfo,
			&device.PerformanceMetrics, &device.CreatedAt, &device.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan device row", zap.Error(err))
			continue
		}
		devices = append(devices, device)
	}

	return devices, nil
}

// UpdateLastPing updates device last ping time
func (r *deviceRepository) UpdateLastPing(ctx context.Context, id uuid.UUID, pingTime time.Time) error {
	query := `
		UPDATE devices SET last_ping = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query, id, pingTime)
	if err != nil {
		r.logger.Error("Failed to update last ping", zap.Error(err))
		return fmt.Errorf("failed to update last ping: %w", err)
	}

	return nil
}

// GetHealthLogs retrieves device health logs
func (r *deviceRepository) GetHealthLogs(ctx context.Context, deviceID uuid.UUID, limit int) ([]*model.DeviceHealth, error) {
	query := `
		SELECT device_id, health_score, metrics, recorded_at
		FROM device_health_logs
		WHERE device_id = $1
		ORDER BY recorded_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, deviceID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get health logs: %w", err)
	}
	defer rows.Close()

	logs := []*model.DeviceHealth{}
	for rows.Next() {
		log := &model.DeviceHealth{}
		var metrics model.JSONObject
		err := rows.Scan(&log.DeviceID, &log.HealthScore, &metrics, &log.RecordedAt)
		if err != nil {
			r.logger.Error("Failed to scan health log", zap.Error(err))
			continue
		}

		// Extract metrics
		if responseTime, ok := metrics["response_time"]; ok {
			if rt, ok := responseTime.(float64); ok {
				rtInt := int(rt)
				log.ResponseTime = &rtInt
			}
		}
		if errorRate, ok := metrics["error_rate"]; ok {
			if er, ok := errorRate.(float64); ok {
				log.ErrorRate = &er
			}
		}
		if uptime, ok := metrics["uptime"]; ok {
			if ut, ok := uptime.(float64); ok {
				log.Uptime = &ut
			}
		}

		logs = append(logs, log)
	}

	return logs, nil
}

// CreateHealthLog creates a device health log
func (r *deviceRepository) CreateHealthLog(ctx context.Context, health *model.DeviceHealth) error {
	query := `
		INSERT INTO device_health_logs (device_id, health_score, metrics)
		VALUES ($1, $2, $3)
	`

	metrics := model.JSONObject{}
	if health.ResponseTime != nil {
		metrics["response_time"] = *health.ResponseTime
	}
	if health.ErrorRate != nil {
		metrics["error_rate"] = *health.ErrorRate
	}
	if health.Uptime != nil {
		metrics["uptime"] = *health.Uptime
	}

	_, err := r.db.ExecContext(ctx, query, health.DeviceID, health.HealthScore, metrics)
	if err != nil {
		r.logger.Error("Failed to create health log", zap.Error(err))
		return fmt.Errorf("failed to create health log: %w", err)
	}

	return nil
}

// UpdateMultipleStatus updates status for multiple devices
func (r *deviceRepository) UpdateMultipleStatus(ctx context.Context, deviceIDs []uuid.UUID, status model.DeviceStatus) error {
	if len(deviceIDs) == 0 {
		return nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(deviceIDs))
	args := make([]interface{}, len(deviceIDs)+1)

	for i, id := range deviceIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	args[len(deviceIDs)] = status

	query := fmt.Sprintf(`
		UPDATE devices SET status = $%d, updated_at = CURRENT_TIMESTAMP
		WHERE id IN (%s)
	`, len(deviceIDs)+1, strings.Join(placeholders, ","))

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to update multiple device status", zap.Error(err))
		return fmt.Errorf("failed to update multiple device status: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	r.logger.Info("Updated multiple device status",
		zap.Int64("rows_affected", rowsAffected),
		zap.String("status", string(status)),
	)

	return nil
}

// GetDeviceStats retrieves device statistics
func (r *deviceRepository) GetDeviceStats(ctx context.Context, branchID *uuid.UUID) (*DeviceStats, error) {
	whereClause := ""
	args := []interface{}{}
	if branchID != nil {
		whereClause = "WHERE branch_id = $1"
		args = append(args, *branchID)
	}

	query := fmt.Sprintf(`
		SELECT 
			COUNT(*) as total,
			COUNT(CASE WHEN status = 'ONLINE' THEN 1 END) as online,
			COUNT(CASE WHEN status = 'OFFLINE' THEN 1 END) as offline,
			COUNT(CASE WHEN status = 'ERROR' THEN 1 END) as error,
			device_type,
			brand,
			status
		FROM devices %s
		GROUP BY ROLLUP(device_type, brand, status)
	`, whereClause)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get device stats: %w", err)
	}
	defer rows.Close()

	stats := &DeviceStats{
		ByType:   make(map[model.DeviceType]int),
		ByBrand:  make(map[model.DeviceBrand]int),
		ByStatus: make(map[model.DeviceStatus]int),
	}

	for rows.Next() {
		var total, online, offline, errorCount int
		var deviceType, brand, status sql.NullString

		err := rows.Scan(&total, &online, &offline, &errorCount, &deviceType, &brand, &status)
		if err != nil {
			continue
		}

		// This is a simplified version - actual implementation would be more complex
		if !deviceType.Valid && !brand.Valid && !status.Valid {
			stats.TotalDevices = total
			stats.OnlineDevices = online
			stats.OfflineDevices = offline
			stats.ErrorDevices = errorCount
		}
	}

	return stats, nil
}
