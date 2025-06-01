// internal/config/config.go
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	RabbitMQ RabbitMQConfig `mapstructure:"rabbitmq"`
	Offline  OfflineConfig  `mapstructure:"offline"`
	Security SecurityConfig `mapstructure:"security"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Device   DeviceConfig   `mapstructure:"device"`
	App      AppConfig      `mapstructure:"app"`
}

// ServerConfig represents HTTP server configuration
type ServerConfig struct {
	Host         string        `mapstructure:"host" validate:"required"`
	Port         string        `mapstructure:"port" validate:"required"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	TLS          TLSConfig     `mapstructure:"tls"`
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Host         string        `mapstructure:"host" validate:"required"`
	Port         int           `mapstructure:"port" validate:"required"`
	User         string        `mapstructure:"user" validate:"required"`
	Password     string        `mapstructure:"password" validate:"required"`
	DBName       string        `mapstructure:"dbname" validate:"required"`
	SSLMode      string        `mapstructure:"sslmode"`
	MaxOpenConns int           `mapstructure:"max_open_conns"`
	MaxIdleConns int           `mapstructure:"max_idle_conns"`
	MaxLifetime  time.Duration `mapstructure:"max_lifetime"`
}

// RedisConfig represents Redis configuration
type RedisConfig struct {
	Host     string `mapstructure:"host" validate:"required"`
	Port     int    `mapstructure:"port" validate:"required"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// RabbitMQConfig represents RabbitMQ configuration
type RabbitMQConfig struct {
	Host     string `mapstructure:"host" validate:"required"`
	Port     int    `mapstructure:"port" validate:"required"`
	User     string `mapstructure:"user" validate:"required"`
	Password string `mapstructure:"password" validate:"required"`
	VHost    string `mapstructure:"vhost"`
	Exchange string `mapstructure:"exchange"`
}

// OfflineConfig represents offline mode configuration
type OfflineConfig struct {
	Enabled       bool          `mapstructure:"enabled"`
	LocalDBPath   string        `mapstructure:"local_db_path"`
	SyncInterval  time.Duration `mapstructure:"sync_interval"`
	MaxQueueSize  int           `mapstructure:"max_queue_size"`
	RetryAttempts int           `mapstructure:"retry_attempts"`
	RetryDelay    time.Duration `mapstructure:"retry_delay"`
}

// SecurityConfig represents security configuration
type SecurityConfig struct {
	JWTSecret          string        `mapstructure:"jwt_secret" validate:"required"`
	JWTExpiration      time.Duration `mapstructure:"jwt_expiration"`
	DeviceAuthRequired bool          `mapstructure:"device_auth_required"`
	CertValidation     bool          `mapstructure:"cert_validation"`
	AllowedOrigins     []string      `mapstructure:"allowed_origins"`
	RateLimitEnabled   bool          `mapstructure:"rate_limit_enabled"`
	RateLimitRequests  int           `mapstructure:"rate_limit_requests"`
	RateLimitWindow    time.Duration `mapstructure:"rate_limit_window"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level      string `mapstructure:"level" validate:"required"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

// DeviceConfig represents device-specific configuration
type DeviceConfig struct {
	DiscoveryInterval   time.Duration    `mapstructure:"discovery_interval"`
	HealthCheckInterval time.Duration    `mapstructure:"health_check_interval"`
	PingInterval        time.Duration    `mapstructure:"ping_interval"`
	OperationTimeout    time.Duration    `mapstructure:"operation_timeout"`
	MaxRetryAttempts    int              `mapstructure:"max_retry_attempts"`
	RetryDelay          time.Duration    `mapstructure:"retry_delay"`
	SupportedBrands     []string         `mapstructure:"supported_brands"`
	DefaultPort         DevicePortConfig `mapstructure:"default_ports"`
}

// DevicePortConfig represents default port configurations
type DevicePortConfig struct {
	Serial    SerialPortConfig    `mapstructure:"serial"`
	TCP       TCPPortConfig       `mapstructure:"tcp"`
	USB       USBPortConfig       `mapstructure:"usb"`
	Bluetooth BluetoothPortConfig `mapstructure:"bluetooth"`
}

// SerialPortConfig represents serial port configuration
type SerialPortConfig struct {
	BaudRate int           `mapstructure:"baud_rate"`
	DataBits int           `mapstructure:"data_bits"`
	StopBits int           `mapstructure:"stop_bits"`
	Parity   string        `mapstructure:"parity"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

// TCPPortConfig represents TCP port configuration
type TCPPortConfig struct {
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	KeepAlive      bool          `mapstructure:"keep_alive"`
}

// USBPortConfig represents USB port configuration
type USBPortConfig struct {
	Timeout          time.Duration `mapstructure:"timeout"`
	BulkTransferSize int           `mapstructure:"bulk_transfer_size"`
}

// BluetoothPortConfig represents Bluetooth configuration
type BluetoothPortConfig struct {
	ScanTimeout    time.Duration `mapstructure:"scan_timeout"`
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`
}

// AppConfig represents application metadata
type AppConfig struct {
	Name        string `mapstructure:"name" validate:"required"`
	Version     string `mapstructure:"version" validate:"required"`
	Environment string `mapstructure:"environment" validate:"required"`
	AppID       string `mapstructure:"app_id" validate:"required"`
	Debug       bool   `mapstructure:"debug"`
}

// Load loads configuration from file and environment variables
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("../../internal/config")

	// Environment variable support
	viper.SetEnvPrefix("DEVICE_SERVICE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Set defaults
	setDefaults()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, fmt.Errorf("config file not found: %w", err)
		}
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// Validate configuration
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", "8084")
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.idle_timeout", "120s")
	viper.SetDefault("server.tls.enabled", false)

	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.password", "postgres")
	viper.SetDefault("database.dbname", "device_service")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.max_lifetime", "5m")

	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)

	// RabbitMQ defaults
	viper.SetDefault("rabbitmq.host", "localhost")
	viper.SetDefault("rabbitmq.port", 5672)
	viper.SetDefault("rabbitmq.user", "guest")
	viper.SetDefault("rabbitmq.password", "guest")
	viper.SetDefault("rabbitmq.vhost", "/")
	viper.SetDefault("rabbitmq.exchange", "device.events")

	// Offline defaults
	viper.SetDefault("offline.enabled", true)
	viper.SetDefault("offline.local_db_path", "./data/offline.db")
	viper.SetDefault("offline.sync_interval", "30s")
	viper.SetDefault("offline.max_queue_size", 10000)
	viper.SetDefault("offline.retry_attempts", 3)
	viper.SetDefault("offline.retry_delay", "5s")

	// Security defaults
	viper.SetDefault("security.jwt_expiration", "24h")
	viper.SetDefault("security.device_auth_required", true)
	viper.SetDefault("security.cert_validation", true)
	viper.SetDefault("security.rate_limit_enabled", true)
	viper.SetDefault("security.rate_limit_requests", 100)
	viper.SetDefault("security.rate_limit_window", "1m")

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")
	viper.SetDefault("logging.max_size", 100)
	viper.SetDefault("logging.max_backups", 3)
	viper.SetDefault("logging.max_age", 28)
	viper.SetDefault("logging.compress", true)

	// Device defaults
	viper.SetDefault("device.discovery_interval", "60s")
	viper.SetDefault("device.health_check_interval", "10s")
	viper.SetDefault("device.ping_interval", "5s")
	viper.SetDefault("device.operation_timeout", "30s")
	viper.SetDefault("device.max_retry_attempts", 3)
	viper.SetDefault("device.retry_delay", "2s")
	viper.SetDefault("device.supported_brands", []string{
		"EPSON", "STAR", "INGENICO", "PAX", "CITIZEN", "BIXOLON", "VERIFONE", "GENERIC",
	})

	// Device port defaults
	viper.SetDefault("device.default_ports.serial.baud_rate", 9600)
	viper.SetDefault("device.default_ports.serial.data_bits", 8)
	viper.SetDefault("device.default_ports.serial.stop_bits", 1)
	viper.SetDefault("device.default_ports.serial.parity", "none")
	viper.SetDefault("device.default_ports.serial.timeout", "5s")

	viper.SetDefault("device.default_ports.tcp.connect_timeout", "10s")
	viper.SetDefault("device.default_ports.tcp.read_timeout", "30s")
	viper.SetDefault("device.default_ports.tcp.write_timeout", "30s")
	viper.SetDefault("device.default_ports.tcp.keep_alive", true)

	viper.SetDefault("device.default_ports.usb.timeout", "5s")
	viper.SetDefault("device.default_ports.usb.bulk_transfer_size", 64)

	viper.SetDefault("device.default_ports.bluetooth.scan_timeout", "30s")
	viper.SetDefault("device.default_ports.bluetooth.connect_timeout", "20s")

	// App defaults
	viper.SetDefault("app.name", "device-service")
	viper.SetDefault("app.version", "1.0.0")
	viper.SetDefault("app.environment", "development")
	viper.SetDefault("app.debug", false)
}

// validate validates the configuration
func validate(config *Config) error {
	// Basic validation
	if config.Server.Host == "" {
		return fmt.Errorf("server.host is required")
	}
	if config.Server.Port == "" {
		return fmt.Errorf("server.port is required")
	}
	if config.Database.Host == "" {
		return fmt.Errorf("database.host is required")
	}
	if config.Security.JWTSecret == "" {
		return fmt.Errorf("security.jwt_secret is required")
	}
	if config.App.AppID == "" {
		return fmt.Errorf("app.app_id is required")
	}

	// Validate environment
	validEnvs := []string{"development", "staging", "production", "test"}
	isValidEnv := false
	for _, env := range validEnvs {
		if config.App.Environment == env {
			isValidEnv = true
			break
		}
	}
	if !isValidEnv {
		return fmt.Errorf("app.environment must be one of: %v", validEnvs)
	}

	// Validate logging level
	validLevels := []string{"debug", "info", "warn", "error", "fatal"}
	isValidLevel := false
	for _, level := range validLevels {
		if config.Logging.Level == level {
			isValidLevel = true
			break
		}
	}
	if !isValidLevel {
		return fmt.Errorf("logging.level must be one of: %v", validLevels)
	}

	return nil
}

// GetDatabaseDSN returns the database connection string
func (c *Config) GetDatabaseDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host, c.Database.Port, c.Database.User,
		c.Database.Password, c.Database.DBName, c.Database.SSLMode)
}

// GetRedisAddr returns the Redis address
func (c *Config) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}

// GetRabbitMQURL returns the RabbitMQ connection URL
func (c *Config) GetRabbitMQURL() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d%s",
		c.RabbitMQ.User, c.RabbitMQ.Password,
		c.RabbitMQ.Host, c.RabbitMQ.Port, c.RabbitMQ.VHost)
}

// GetServerAddr returns the server address
func (c *Config) GetServerAddr() string {
	return fmt.Sprintf("%s:%s", c.Server.Host, c.Server.Port)
}

// IsProduction checks if the environment is production
func (c *Config) IsProduction() bool {
	return c.App.Environment == "production"
}

// IsDevelopment checks if the environment is development
func (c *Config) IsDevelopment() bool {
	return c.App.Environment == "development"
}

// IsDebugEnabled checks if debug mode is enabled
func (c *Config) IsDebugEnabled() bool {
	return c.App.Debug || c.IsDevelopment()
}
