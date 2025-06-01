// internal/utils/logger.go
package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"device-service/internal/config"
)

// LoggerManager manages application logging
type LoggerManager struct {
	logger *zap.Logger
	config *config.LoggingConfig
}

// NewLogger creates a new logger instance based on configuration
func NewLogger(cfg *config.LoggingConfig) (*zap.Logger, error) {
	manager := &LoggerManager{
		config: cfg,
	}

	logger, err := manager.createLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	manager.logger = logger
	return logger, nil
}

// createLogger creates the zap logger with proper configuration
func (lm *LoggerManager) createLogger() (*zap.Logger, error) {
	// Create encoder configuration
	encoderConfig := lm.getEncoderConfig()

	// Create encoder
	var encoder zapcore.Encoder
	switch lm.config.Format {
	case "json":
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	case "console":
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	default:
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// Create write syncer
	writeSyncer, err := lm.getWriteSyncer()
	if err != nil {
		return nil, fmt.Errorf("failed to create write syncer: %w", err)
	}

	// Get log level
	level, err := lm.getLogLevel()
	if err != nil {
		return nil, fmt.Errorf("failed to parse log level: %w", err)
	}

	// Create core
	core := zapcore.NewCore(encoder, writeSyncer, level)

	// Create logger with options
	logger := zap.New(core, lm.getLoggerOptions()...)

	return logger, nil
}

// getEncoderConfig returns encoder configuration based on format
func (lm *LoggerManager) getEncoderConfig() zapcore.EncoderConfig {
	config := zap.NewProductionEncoderConfig()

	// Customize time format
	config.TimeKey = "timestamp"
	config.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339)

	// Customize level format
	config.LevelKey = "level"
	config.EncodeLevel = zapcore.LowercaseLevelEncoder

	// Customize caller format
	config.CallerKey = "caller"
	config.EncodeCaller = zapcore.ShortCallerEncoder

	// Message key
	config.MessageKey = "message"

	// Stack trace key
	config.StacktraceKey = "stacktrace"

	// Console format customizations
	if lm.config.Format == "console" {
		config.EncodeLevel = zapcore.CapitalColorLevelEncoder
		config.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05")
	}

	return config
}

// getWriteSyncer returns write syncer based on output configuration
func (lm *LoggerManager) getWriteSyncer() (zapcore.WriteSyncer, error) {
	switch lm.config.Output {
	case "stdout":
		return zapcore.AddSync(os.Stdout), nil
	case "stderr":
		return zapcore.AddSync(os.Stderr), nil
	default:
		// File output with rotation
		if lm.config.Output == "" {
			lm.config.Output = "./logs/device-service.log"
		}

		// Ensure log directory exists
		logDir := filepath.Dir(lm.config.Output)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		// Create lumberjack logger for rotation
		lumber := &lumberjack.Logger{
			Filename:   lm.config.Output,
			MaxSize:    lm.config.MaxSize, // MB
			MaxBackups: lm.config.MaxBackups,
			MaxAge:     lm.config.MaxAge, // days
			Compress:   lm.config.Compress,
		}

		return zapcore.AddSync(lumber), nil
	}
}

// getLogLevel parses and returns log level
func (lm *LoggerManager) getLogLevel() (zapcore.Level, error) {
	switch lm.config.Level {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "fatal":
		return zapcore.FatalLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("invalid log level: %s", lm.config.Level)
	}
}

// getLoggerOptions returns logger options
func (lm *LoggerManager) getLoggerOptions() []zap.Option {
	options := []zap.Option{
		zap.AddCaller(),
		zap.AddCallerSkip(1),
	}

	// Add stack trace for error level and above
	options = append(options, zap.AddStacktrace(zapcore.ErrorLevel))

	return options
}

// DeviceLogger wraps zap.Logger with device-specific functionality
type DeviceLogger struct {
	*zap.Logger
	deviceID   string
	deviceType string
	brand      string
}

// NewDeviceLogger creates a device-specific logger
func NewDeviceLogger(baseLogger *zap.Logger, deviceID, deviceType, brand string) *DeviceLogger {
	logger := baseLogger.With(
		zap.String("device_id", deviceID),
		zap.String("device_type", deviceType),
		zap.String("brand", brand),
		zap.String("component", "device"),
	)

	return &DeviceLogger{
		Logger:     logger,
		deviceID:   deviceID,
		deviceType: deviceType,
		brand:      brand,
	}
}

// LogOperation logs device operation with context
func (dl *DeviceLogger) LogOperation(operationType, operationID string, duration time.Duration, success bool, err error) {
	fields := []zap.Field{
		zap.String("operation_type", operationType),
		zap.String("operation_id", operationID),
		zap.Duration("duration", duration),
		zap.Bool("success", success),
	}

	if err != nil {
		fields = append(fields, zap.Error(err))
		dl.Error("Device operation failed", fields...)
	} else {
		dl.Info("Device operation completed", fields...)
	}
}

// LogConnection logs connection events
func (dl *DeviceLogger) LogConnection(action string, success bool, err error) {
	fields := []zap.Field{
		zap.String("action", action),
		zap.Bool("success", success),
	}

	if err != nil {
		fields = append(fields, zap.Error(err))
		dl.Error("Device connection event", fields...)
	} else {
		dl.Info("Device connection event", fields...)
	}
}

// LogHealth logs health metrics
func (dl *DeviceLogger) LogHealth(healthScore int, responseTime time.Duration, errorRate float64) {
	dl.Info("Device health metrics",
		zap.Int("health_score", healthScore),
		zap.Duration("response_time", responseTime),
		zap.Float64("error_rate", errorRate),
	)
}

// LogPayment logs payment operations (without sensitive data)
func (dl *DeviceLogger) LogPayment(amount float64, currency, paymentMethod, transactionID string, success bool, err error) {
	fields := []zap.Field{
		zap.Float64("amount", amount),
		zap.String("currency", currency),
		zap.String("payment_method", paymentMethod),
		zap.String("transaction_id", transactionID),
		zap.Bool("success", success),
	}

	if err != nil {
		fields = append(fields, zap.Error(err))
		dl.Error("Payment operation", fields...)
	} else {
		dl.Info("Payment operation", fields...)
	}
}

// OperationLogger provides structured logging for operations
type OperationLogger struct {
	logger      *zap.Logger
	operationID string
	startTime   time.Time
}

// NewOperationLogger creates an operation-specific logger
func NewOperationLogger(baseLogger *zap.Logger, operationType, operationID string) *OperationLogger {
	logger := baseLogger.With(
		zap.String("operation_type", operationType),
		zap.String("operation_id", operationID),
		zap.String("component", "operation"),
	)

	return &OperationLogger{
		logger:      logger,
		operationID: operationID,
		startTime:   time.Now(),
	}
}

// Start logs operation start
func (ol *OperationLogger) Start(fields ...zap.Field) {
	allFields := append([]zap.Field{
		zap.Time("start_time", ol.startTime),
	}, fields...)

	ol.logger.Info("Operation started", allFields...)
}

// Success logs successful operation completion
func (ol *OperationLogger) Success(fields ...zap.Field) {
	duration := time.Since(ol.startTime)
	allFields := append([]zap.Field{
		zap.Duration("duration", duration),
		zap.Bool("success", true),
	}, fields...)

	ol.logger.Info("Operation completed successfully", allFields...)
}

// Error logs operation failure
func (ol *OperationLogger) Error(err error, fields ...zap.Field) {
	duration := time.Since(ol.startTime)
	allFields := append([]zap.Field{
		zap.Duration("duration", duration),
		zap.Bool("success", false),
		zap.Error(err),
	}, fields...)

	ol.logger.Error("Operation failed", allFields...)
}

// Progress logs operation progress
func (ol *OperationLogger) Progress(message string, progress float64, fields ...zap.Field) {
	allFields := append([]zap.Field{
		zap.Float64("progress", progress),
		zap.Duration("elapsed", time.Since(ol.startTime)),
	}, fields...)

	ol.logger.Info(message, allFields...)
}

// ServiceLogger provides service-level logging functionality
type ServiceLogger struct {
	*zap.Logger
	serviceName string
}

// NewServiceLogger creates a service-specific logger
func NewServiceLogger(baseLogger *zap.Logger, serviceName string) *ServiceLogger {
	logger := baseLogger.With(
		zap.String("service", serviceName),
		zap.String("component", "service"),
	)

	return &ServiceLogger{
		Logger:      logger,
		serviceName: serviceName,
	}
}

// LogServiceStart logs service startup
func (sl *ServiceLogger) LogServiceStart(version string, config interface{}) {
	sl.Info("Service starting",
		zap.String("version", version),
		zap.Any("config", config),
	)
}

// LogServiceStop logs service shutdown
func (sl *ServiceLogger) LogServiceStop(reason string) {
	sl.Info("Service stopping",
		zap.String("reason", reason),
	)
}

// LogAPIRequest logs HTTP API requests
func (sl *ServiceLogger) LogAPIRequest(method, path, userAgent, clientIP string, statusCode int, duration time.Duration) {
	level := zapcore.InfoLevel
	if statusCode >= 400 {
		level = zapcore.WarnLevel
	}
	if statusCode >= 500 {
		level = zapcore.ErrorLevel
	}

	if ce := sl.Check(level, "API request"); ce != nil {
		ce.Write(
			zap.String("method", method),
			zap.String("path", path),
			zap.String("user_agent", userAgent),
			zap.String("client_ip", clientIP),
			zap.Int("status_code", statusCode),
			zap.Duration("duration", duration),
		)
	}
}

// LogDatabaseQuery logs database queries (for debugging)
func (sl *ServiceLogger) LogDatabaseQuery(query string, args []interface{}, duration time.Duration, err error) {
	fields := []zap.Field{
		zap.String("query", query),
		zap.Any("args", args),
		zap.Duration("duration", duration),
	}

	if err != nil {
		fields = append(fields, zap.Error(err))
		sl.Error("Database query failed", fields...)
	} else {
		sl.Debug("Database query executed", fields...)
	}
}

// AuditLogger provides audit logging functionality
type AuditLogger struct {
	logger *zap.Logger
}

// NewAuditLogger creates an audit-specific logger
func NewAuditLogger(baseLogger *zap.Logger) *AuditLogger {
	logger := baseLogger.With(
		zap.String("component", "audit"),
	)

	return &AuditLogger{
		logger: logger,
	}
}

// LogDeviceRegistration logs device registration events
func (al *AuditLogger) LogDeviceRegistration(deviceID, deviceType, brand, userID string, success bool) {
	al.logger.Info("Device registration",
		zap.String("device_id", deviceID),
		zap.String("device_type", deviceType),
		zap.String("brand", brand),
		zap.String("user_id", userID),
		zap.Bool("success", success),
		zap.String("action", "register_device"),
	)
}

// LogDeviceConfiguration logs device configuration changes
func (al *AuditLogger) LogDeviceConfiguration(deviceID, userID string, oldConfig, newConfig interface{}) {
	al.logger.Info("Device configuration changed",
		zap.String("device_id", deviceID),
		zap.String("user_id", userID),
		zap.Any("old_config", oldConfig),
		zap.Any("new_config", newConfig),
		zap.String("action", "configure_device"),
	)
}

// LogPaymentTransaction logs payment transactions (audit trail)
func (al *AuditLogger) LogPaymentTransaction(deviceID, transactionID string, amount float64, currency, status string) {
	al.logger.Info("Payment transaction",
		zap.String("device_id", deviceID),
		zap.String("transaction_id", transactionID),
		zap.Float64("amount", amount),
		zap.String("currency", currency),
		zap.String("status", status),
		zap.String("action", "payment_transaction"),
	)
}

// SecurityLogger provides security-related logging
type SecurityLogger struct {
	logger *zap.Logger
}

// NewSecurityLogger creates a security-specific logger
func NewSecurityLogger(baseLogger *zap.Logger) *SecurityLogger {
	logger := baseLogger.With(
		zap.String("component", "security"),
	)

	return &SecurityLogger{
		logger: logger,
	}
}

// LogAuthAttempt logs authentication attempts
func (sl *SecurityLogger) LogAuthAttempt(userID, clientIP, userAgent string, success bool, reason string) {
	level := zapcore.InfoLevel
	if !success {
		level = zapcore.WarnLevel
	}

	if ce := sl.logger.Check(level, "Authentication attempt"); ce != nil {
		ce.Write(
			zap.String("user_id", userID),
			zap.String("client_ip", clientIP),
			zap.String("user_agent", userAgent),
			zap.Bool("success", success),
			zap.String("reason", reason),
			zap.String("action", "auth_attempt"),
		)
	}
}

// LogSuspiciousActivity logs suspicious security events
func (sl *SecurityLogger) LogSuspiciousActivity(description, clientIP, userAgent string, severity string) {
	sl.logger.Warn("Suspicious activity detected",
		zap.String("description", description),
		zap.String("client_ip", clientIP),
		zap.String("user_agent", userAgent),
		zap.String("severity", severity),
		zap.String("action", "suspicious_activity"),
	)
}

// LogRateLimitViolation logs rate limit violations
func (sl *SecurityLogger) LogRateLimitViolation(clientIP, endpoint string, requestCount int, timeWindow string) {
	sl.logger.Warn("Rate limit violation",
		zap.String("client_ip", clientIP),
		zap.String("endpoint", endpoint),
		zap.Int("request_count", requestCount),
		zap.String("time_window", timeWindow),
		zap.String("action", "rate_limit_violation"),
	)
}

// Helper functions for common logging patterns

// LoggerWithRequestID adds request ID to logger
func LoggerWithRequestID(logger *zap.Logger, requestID string) *zap.Logger {
	return logger.With(zap.String("request_id", requestID))
}

// LoggerWithUserID adds user ID to logger
func LoggerWithUserID(logger *zap.Logger, userID string) *zap.Logger {
	return logger.With(zap.String("user_id", userID))
}

// LoggerWithTraceID adds trace ID for distributed tracing
func LoggerWithTraceID(logger *zap.Logger, traceID string) *zap.Logger {
	return logger.With(zap.String("trace_id", traceID))
}

// LogError is a helper function for consistent error logging
func LogError(logger *zap.Logger, message string, err error, fields ...zap.Field) {
	allFields := append([]zap.Field{zap.Error(err)}, fields...)
	logger.Error(message, allFields...)
}

// LogPanic logs and recovers from panics
func LogPanic(logger *zap.Logger) {
	if r := recover(); r != nil {
		logger.Fatal("Application panic",
			zap.Any("panic", r),
			zap.Stack("stacktrace"),
		)
	}
}
func CloseLogger(logger *zap.Logger) error {
	return logger.Sync()
}
