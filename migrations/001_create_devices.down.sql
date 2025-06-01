// migrations/001_create_devices.up.sql
-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create devices table
CREATE TABLE IF NOT EXISTS devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id VARCHAR(255) UNIQUE NOT NULL,
    device_type VARCHAR(50) NOT NULL CHECK (device_type IN ('POS', 'PRINTER', 'SCANNER', 'CASH_REGISTER', 'CASH_DRAWER', 'DISPLAY')),
    brand VARCHAR(100) NOT NULL CHECK (brand IN ('EPSON', 'STAR', 'INGENICO', 'PAX', 'CITIZEN', 'BIXOLON', 'VERIFONE', 'GENERIC')),
    model VARCHAR(100) NOT NULL,
    firmware_version VARCHAR(50),
    connection_type VARCHAR(50) NOT NULL CHECK (connection_type IN ('SERIAL', 'USB', 'TCP', 'BLUETOOTH')),
    connection_config JSONB NOT NULL DEFAULT '{}',
    capabilities JSONB NOT NULL DEFAULT '[]',
    branch_id UUID NOT NULL,
    location VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'OFFLINE' CHECK (status IN ('ONLINE', 'OFFLINE', 'ERROR', 'MAINTENANCE', 'CONNECTING')),
    last_ping TIMESTAMP WITH TIME ZONE,
    error_info JSONB DEFAULT '{}',
    performance_metrics JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for devices table
CREATE INDEX idx_devices_device_id ON devices(device_id);
CREATE INDEX idx_devices_branch_id ON devices(branch_id);
CREATE INDEX idx_devices_status ON devices(status);
CREATE INDEX idx_devices_device_type ON devices(device_type);
CREATE INDEX idx_devices_brand ON devices(brand);
CREATE INDEX idx_devices_connection_type ON devices(connection_type);
CREATE INDEX idx_devices_created_at ON devices(created_at);
CREATE INDEX idx_devices_last_ping ON devices(last_ping);

-- Create composite indexes for common queries
CREATE INDEX idx_devices_branch_status ON devices(branch_id, status);
CREATE INDEX idx_devices_type_status ON devices(device_type, status);

-- migrations/002_create_operations.up.sql
-- Create device operations table
CREATE TABLE IF NOT EXISTS device_operations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    operation_type VARCHAR(50) NOT NULL CHECK (operation_type IN ('PRINT', 'PAYMENT', 'SCAN', 'STATUS_CHECK', 'OPEN_DRAWER', 'DISPLAY_TEXT', 'BEEP', 'REFUND', 'CUT')),
    operation_data JSONB NOT NULL DEFAULT '{}',
    priority INTEGER NOT NULL DEFAULT 3 CHECK (priority >= 1 AND priority <= 5),
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PROCESSING', 'SUCCESS', 'FAILED', 'TIMEOUT', 'CANCELLED')),
    started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    duration_ms INTEGER,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    correlation_id UUID,
    result JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for operations table
CREATE INDEX idx_operations_device_id ON device_operations(device_id);
CREATE INDEX idx_operations_status ON device_operations(status);
CREATE INDEX idx_operations_operation_type ON device_operations(operation_type);
CREATE INDEX idx_operations_priority ON device_operations(priority);
CREATE INDEX idx_operations_created_at ON device_operations(created_at);
CREATE INDEX idx_operations_correlation_id ON device_operations(correlation_id);

-- Create composite indexes for performance
CREATE INDEX idx_operations_device_status ON device_operations(device_id, status);
CREATE INDEX idx_operations_device_type ON device_operations(device_id, operation_type);
CREATE INDEX idx_operations_status_priority ON device_operations(status, priority);
CREATE INDEX idx_operations_created_date ON device_operations(DATE(created_at));

-- migrations/003_create_offline_operations.up.sql
-- Create offline operations queue table
CREATE TABLE IF NOT EXISTS offline_operations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID NOT NULL,
    operation_type VARCHAR(50) NOT NULL CHECK (operation_type IN ('PRINT', 'PAYMENT', 'SCAN', 'STATUS_CHECK', 'OPEN_DRAWER', 'DISPLAY_TEXT', 'BEEP', 'REFUND', 'CUT')),
    operation_data JSONB NOT NULL DEFAULT '{}',
    priority INTEGER NOT NULL DEFAULT 3 CHECK (priority >= 1 AND priority <= 5),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    sync_status VARCHAR(20) DEFAULT 'PENDING' CHECK (sync_status IN ('PENDING', 'SYNCED', 'CONFLICT', 'EXPIRED')),
    sync_attempts INTEGER DEFAULT 0,
    last_sync_attempt TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes for offline operations
CREATE INDEX idx_offline_ops_device_id ON offline_operations(device_id);
CREATE INDEX idx_offline_ops_sync_status ON offline_operations(sync_status);
CREATE INDEX idx_offline_ops_priority ON offline_operations(priority);
CREATE INDEX idx_offline_ops_created_at ON offline_operations(created_at);
CREATE INDEX idx_offline_ops_expires_at ON offline_operations(expires_at);

-- Create composite indexes
CREATE INDEX idx_offline_ops_device_sync ON offline_operations(device_id, sync_status);
CREATE INDEX idx_offline_ops_sync_attempts ON offline_operations(sync_status, sync_attempts);

-- migrations/004_create_device_health.up.sql
-- Create device health logs table
CREATE TABLE IF NOT EXISTS device_health_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    health_score INTEGER NOT NULL CHECK (health_score >= 0 AND health_score <= 100),
    metrics JSONB NOT NULL DEFAULT '{}',
    alerts JSONB DEFAULT '[]',
    recorded_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for health logs
CREATE INDEX idx_health_logs_device_id ON device_health_logs(device_id);
CREATE INDEX idx_health_logs_recorded_at ON device_health_logs(recorded_at);
CREATE INDEX idx_health_logs_health_score ON device_health_logs(health_score);

-- Create composite indexes
CREATE INDEX idx_health_logs_device_date ON device_health_logs(device_id, recorded_at);
CREATE INDEX idx_health_logs_device_score ON device_health_logs(device_id, health_score);

-- migrations/005_create_triggers.up.sql
-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create triggers for updated_at
CREATE TRIGGER trigger_devices_updated_at
    BEFORE UPDATE ON devices
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Function to calculate operation duration
CREATE OR REPLACE FUNCTION calculate_operation_duration()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.completed_at IS NOT NULL AND NEW.started_at IS NOT NULL THEN
        NEW.duration_ms = EXTRACT(EPOCH FROM (NEW.completed_at - NEW.started_at)) * 1000;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for operation duration calculation
CREATE TRIGGER trigger_calculate_duration
    BEFORE UPDATE ON device_operations
    FOR EACH ROW
    EXECUTE FUNCTION calculate_operation_duration();

-- Function to cleanup old records
CREATE OR REPLACE FUNCTION cleanup_old_records()
RETURNS VOID AS $$
BEGIN
    -- Delete operations older than 30 days
    DELETE FROM device_operations 
    WHERE created_at < CURRENT_TIMESTAMP - INTERVAL '30 days';
    
    -- Delete offline operations older than 7 days
    DELETE FROM offline_operations 
    WHERE created_at < CURRENT_TIMESTAMP - INTERVAL '7 days';
    
    -- Delete health logs older than 90 days
    DELETE FROM device_health_logs 
    WHERE recorded_at < CURRENT_TIMESTAMP - INTERVAL '90 days';
    
    -- Delete expired offline operations
    DELETE FROM offline_operations 
    WHERE expires_at IS NOT NULL AND expires_at < CURRENT_TIMESTAMP;
END;
$$ LANGUAGE plpgsql;

-- migrations/001_create_devices.down.sql
DROP TRIGGER IF EXISTS trigger_devices_updated_at ON devices;
DROP INDEX IF EXISTS idx_devices_device_id;
DROP INDEX IF EXISTS idx_devices_branch_id;
DROP INDEX IF EXISTS idx_devices_status;
DROP INDEX IF EXISTS idx_devices_device_type;
DROP INDEX IF EXISTS idx_devices_brand;
DROP INDEX IF EXISTS idx_devices_connection_type;
DROP INDEX IF EXISTS idx_devices_created_at;
DROP INDEX IF EXISTS idx_devices_last_ping;
DROP INDEX IF EXISTS idx_devices_branch_status;
DROP INDEX IF EXISTS idx_devices_type_status;
DROP TABLE IF EXISTS devices;

-- migrations/002_create_operations.down.sql
DROP INDEX IF EXISTS idx_operations_device_id;
DROP INDEX IF EXISTS idx_operations_status;
DROP INDEX IF EXISTS idx_operations_operation_type;
DROP INDEX IF EXISTS idx_operations_priority;
DROP INDEX IF EXISTS idx_operations_created_at;
DROP INDEX IF EXISTS idx_operations_correlation_id;
DROP INDEX IF EXISTS idx_operations_device_status;
DROP INDEX IF EXISTS idx_operations_device_type;
DROP INDEX IF EXISTS idx_operations_status_priority;
DROP INDEX IF EXISTS idx_operations_created_date;
DROP TABLE IF EXISTS device_operations;

-- migrations/003_create_offline_operations.down.sql
DROP INDEX IF EXISTS idx_offline_ops_device_id;
DROP INDEX IF EXISTS idx_offline_ops_sync_status;
DROP INDEX IF EXISTS idx_offline_ops_priority;
DROP INDEX IF EXISTS idx_offline_ops_created_at;
DROP INDEX IF EXISTS idx_offline_ops_expires_at;
DROP INDEX IF EXISTS idx_offline_ops_device_sync;
DROP INDEX IF EXISTS idx_offline_ops_sync_attempts;
DROP TABLE IF EXISTS offline_operations;

-- migrations/004_create_device_health.down.sql
DROP INDEX IF EXISTS idx_health_logs_device_id;
DROP INDEX IF EXISTS idx_health_logs_recorded_at;
DROP INDEX IF EXISTS idx_health_logs_health_score;
DROP INDEX IF EXISTS idx_health_logs_device_date;
DROP INDEX IF EXISTS idx_health_logs_device_score;
DROP TABLE IF EXISTS device_health_logs;

-- migrations/005_create_triggers.down.sql
DROP TRIGGER IF EXISTS trigger_calculate_duration ON device_operations;
DROP FUNCTION IF EXISTS calculate_operation_duration();
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP FUNCTION IF EXISTS cleanup_old_records();