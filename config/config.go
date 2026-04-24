// Package config provides configuration management for the chat application.
// It handles loading configuration from files, environment variables, and provides
// validation and default values for all application components.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper" // Viper is a configuration management library that handles various config sources
	"go.uber.org/zap"        // Zap is a structured logging library
	"go.uber.org/zap/zapcore"
)

// Config is the main configuration structure that holds all application settings.
// The `mapstructure` tags tell Viper how to map configuration keys (from files/env vars)
// to these struct fields. For example, a config key "server.port" maps to Server.Port.
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`      // HTTP server configuration
	WebSocket   WebSocketConfig   `mapstructure:"websocket"`   // WebSocket connection settings
	Redis       RedisConfig       `mapstructure:"redis"`       // Redis cache/session storage
	Kafka       KafkaConfig       `mapstructure:"kafka"`       // Message queue for chat events
	ScyllaDB    ScyllaDBConfig    `mapstructure:"scylladb"`    // NoSQL database for persistent storage
	E2EE        E2EEConfig        `mapstructure:"e2ee"`        // End-to-end encryption settings
	Metrics     MetricsConfig     `mapstructure:"metrics"`     // Prometheus metrics configuration
	Logging     LoggingConfig     `mapstructure:"logging"`     // Logging configuration
	Security    SecurityConfig    `mapstructure:"security"`    // Security and CORS settings
	Performance PerformanceConfig `mapstructure:"performance"` // Performance tuning settings
}

// ServerConfig contains HTTP server settings for the REST API
// These settings control how the HTTP server handles incoming requests and connections.
type ServerConfig struct {
	Port                    int           `mapstructure:"port"`                      // Port number for the HTTP server (default: 8080)
	ReadTimeout             time.Duration `mapstructure:"read_timeout"`              // Max time to read a request (default: 15s)
	WriteTimeout            time.Duration `mapstructure:"write_timeout"`             // Max time to write a response (default: 15s)
	IdleTimeout             time.Duration `mapstructure:"idle_timeout"`              // Max time to wait for next request (default: 60s)
	Host                    string        `mapstructure:"host"`                      // Host address to bind to (default: 0.0.0.0)
	GracefulShutdownTimeout time.Duration `mapstructure:"graceful_shutdown_timeout"` // Time to wait for graceful shutdown (default: 30s)
}

// WebSocketConfig contains settings for real-time WebSocket connections
// These settings optimize performance and reliability of the chat's real-time features.
type WebSocketConfig struct {
	ReadBufferSize    int           `mapstructure:"read_buffer_size"`   // Buffer size for reading WebSocket messages (default: 1024 bytes)
	WriteBufferSize   int           `mapstructure:"write_buffer_size"`  // Buffer size for writing WebSocket messages (default: 1024 bytes)
	PingPeriod        time.Duration `mapstructure:"ping_period"`        // Interval to send ping messages (default: 54s)
	PongWait          time.Duration `mapstructure:"pong_wait"`          // Time to wait for pong response (default: 60s)
	WriteWait         time.Duration `mapstructure:"write_wait"`         // Time to wait when writing messages (default: 10s)
	MaxMessageSize    int64         `mapstructure:"max_message_size"`   // Maximum message size in bytes (default: 8192)
	MaxConnections    int           `mapstructure:"max_connections"`    // Maximum concurrent WebSocket connections (default: 200000)
	EnableCompression bool          `mapstructure:"enable_compression"` // Enable WebSocket compression (default: true)
}

type RedisConfig struct {
	Addr               string        `mapstructure:"addr"`                 // Redis server address (default: localhost:6379)
	Password           string        `mapstructure:"password"`             // Redis password (empty if no auth)
	DB                 int           `mapstructure:"db"`                   // Redis database number (default: 0)
	MaxRetries         int           `mapstructure:"max_retries"`          // Maximum connection retries (default: 3)
	PoolSize           int           `mapstructure:"pool_size"`            // Maximum connections in pool (default: 100)
	MinIdleConns       int           `mapstructure:"min_idle_conns"`       // Minimum idle connections (default: 10)
	MaxConnAge         time.Duration `mapstructure:"max_conn_age"`         // Maximum connection age (default: 1h)
	ReadTimeout        time.Duration `mapstructure:"read_timeout"`         // Read operation timeout (default: 100ms)
	WriteTimeout       time.Duration `mapstructure:"write_timeout"`        // Write operation timeout (default: 100ms)
	PoolTimeout        time.Duration `mapstructure:"pool_timeout"`         // Time to wait for connection from pool (default: 30s)
	IdleTimeout        time.Duration `mapstructure:"idle_timeout"`         // Idle connection timeout (default: 5m)
	IdleCheckFrequency time.Duration `mapstructure:"idle_check_frequency"` // Frequency to check idle connections (default: 1m)
	TLS                TLSConfig     `mapstructure:"tls"`                  // TLS configuration for Redis
	Cluster            bool          `mapstructure:"cluster"`              // Enable Redis cluster mode (default: false)
	ClusterNodes       []string      `mapstructure:"cluster_nodes"`        // List of cluster node addresses
}

// KafkaConfig contains Apache Kafka settings for message streaming
// Kafka is used for distributing chat messages, receipts, and presence updates across services.
type KafkaConfig struct {
	Brokers                []string      `mapstructure:"brokers"`                  // Kafka broker addresses (default: [localhost:9092])
	ConsumerGroup          string        `mapstructure:"consumer_group"`           // Consumer group ID (default: chatapp)
	ProducerFlushFrequency time.Duration `mapstructure:"producer_flush_frequency"` // How often to flush messages (default: 100ms)
	ProducerFlushMessages  int           `mapstructure:"producer_flush_messages"`  // Messages to batch before flush (default: 100)
	MaxMessageBytes        int           `mapstructure:"max_message_bytes"`        // Maximum message size (default: 10MB)
	RequiredAcks           string        `mapstructure:"required_acks"`            // Acknowledgment level (default: all)
	RetryMax               int           `mapstructure:"retry_max"`                // Maximum retry attempts (default: 5)
	RetryBackoff           time.Duration `mapstructure:"retry_backoff"`            // Delay between retries (default: 100ms)
	Compression            string        `mapstructure:"compression"`              // Compression type (default: snappy)
	BatchSize              int           `mapstructure:"batch_size"`               // Messages per batch (default: 50)
	BatchTimeout           time.Duration `mapstructure:"batch_timeout"`            // Time to wait for batch (default: 10ms)
	Topics                 TopicsConfig  `mapstructure:"topics"`                   // Kafka topic names
	SASL                   SASLConfig    `mapstructure:"sasl"`                     // SASL authentication settings
	TLS                    TLSConfig     `mapstructure:"tls"`                      // TLS encryption settings
}

type TopicsConfig struct {
	ChatMessages     string `mapstructure:"chat_messages"`     // Topic for chat messages (default: chat-messages)
	DeliveryReceipts string `mapstructure:"delivery_receipts"` // Topic for message delivery receipts (default: delivery-receipts)
	PresenceUpdates  string `mapstructure:"presence_updates"`  // Topic for user online/offline status (default: presence-updates)
	Metrics          string `mapstructure:"metrics"`           // Topic for application metrics (default: metrics)
}

// SASLConfig contains SASL authentication settings for Kafka
// SASL provides secure authentication when connecting to Kafka brokers.
type SASLConfig struct {
	Enabled   bool   `mapstructure:"enabled"`   // Enable SASL authentication
	Mechanism string `mapstructure:"mechanism"` // SASL mechanism (PLAIN, SCRAM-SHA-256, etc.)
	Username  string `mapstructure:"username"`  // SASL username
	Password  string `mapstructure:"password"`  // SASL password
}

// ScyllaDBConfig contains ScyllaDB (Cassandra-compatible) database settings
// ScyllaDB is used for persistent storage of chat messages, user data, and conversations.
type ScyllaDBConfig struct {
	Hosts             []string      `mapstructure:"hosts"`              // Database host addresses (default: [localhost:9042])
	Keyspace          string        `mapstructure:"keyspace"`           // Database keyspace name (default: chatapp)
	Username          string        `mapstructure:"username"`           // Database username
	Password          string        `mapstructure:"password"`           // Database password
	ConnectTimeout    time.Duration `mapstructure:"connect_timeout"`    // Connection timeout (default: 10s)
	Timeout           time.Duration `mapstructure:"timeout"`            // Query timeout (default: 5s)
	NumConns          int           `mapstructure:"num_conns"`          // Number of connections per host (default: 4)
	Consistency       string        `mapstructure:"consistency"`        // Consistency level (default: quorum)
	ReplicationFactor int           `mapstructure:"replication_factor"` // Data replication factor (default: 3)
	DC                string        `mapstructure:"dc"`                 // Data center name for multi-DC setups
	TLS               TLSConfig     `mapstructure:"tls"`                // TLS configuration for database
}

// E2EEConfig contains end-to-end encryption settings
// These settings implement the Double Ratchet algorithm for secure messaging.
type E2EEConfig struct {
	KeyRotationInterval  time.Duration `mapstructure:"key_rotation_interval"`  // How often to rotate encryption keys (default: 24h)
	MaxSkipMessages      int           `mapstructure:"max_skip_messages"`      // Max messages to skip in key chain (default: 1000)
	KeyDerivationInfo    string        `mapstructure:"key_derivation_info"`    // Info string for key derivation (default: "Double Ratchet Chat")
	PreKeyLifetime       time.Duration `mapstructure:"prekey_lifetime"`        // Lifetime of pre-keys (default: 30d)
	SignedPreKeyLifetime time.Duration `mapstructure:"signed_prekey_lifetime"` // Lifetime of signed pre-keys (default: 90d)
}

// MetricsConfig contains Prometheus metrics configuration
// These settings enable monitoring and observability of the chat application.
type MetricsConfig struct {
	Enabled   bool   `mapstructure:"enabled"`   // Enable metrics collection (default: true)
	Port      int    `mapstructure:"port"`      // Metrics server port (default: 9090)
	Path      string `mapstructure:"path"`      // Metrics endpoint path (default: /metrics)
	Namespace string `mapstructure:"namespace"` // Metrics namespace (default: chatapp)
	Subsystem string `mapstructure:"subsystem"` // Metrics subsystem (default: gateway)
}

// LoggingConfig contains application logging settings
// These settings control how logs are formatted, written, and rotated.
type LoggingConfig struct {
	Level      string `mapstructure:"level"`       // Log level (debug, info, warn, error, fatal) (default: info)
	Format     string `mapstructure:"format"`      // Log format (json, console) (default: json)
	Output     string `mapstructure:"output"`      // Log output (stdout, stderr, file) (default: stdout)
	Filename   string `mapstructure:"filename"`    // Log file path when output is file
	MaxSize    int    `mapstructure:"max_size"`    // Max log file size in MB before rotation (default: 100)
	MaxBackups int    `mapstructure:"max_backups"` // Max number of old log files to keep (default: 3)
	MaxAge     int    `mapstructure:"max_age"`     // Max age of log files in days (default: 28)
	Compress   bool   `mapstructure:"compress"`    // Compress rotated log files (default: true)
}

type SecurityConfig struct {
	TLSEnabled     bool            `mapstructure:"tls_enabled"`     // Enable TLS/HTTPS (default: false)
	MinVersion     string          `mapstructure:"min_version"`     // Minimum TLS version (default: 1.2)
	CertFile       string          `mapstructure:"cert_file"`       // TLS certificate file path
	KeyFile        string          `mapstructure:"key_file"`        // TLS private key file path
	CAFile         string          `mapstructure:"ca_file"`         // CA certificate file path
	AllowedOrigins []string        `mapstructure:"allowed_origins"` // CORS allowed origins (default: [*])
	RateLimiting   RateLimitConfig `mapstructure:"rate_limiting"`   // Rate limiting configuration
}

type RateLimitConfig struct {
	Enabled           bool          `mapstructure:"enabled"`             // Enable rate limiting (default: true)
	RequestsPerSecond int           `mapstructure:"requests_per_second"` // Max requests per second (default: 100)
	BurstSize         int           `mapstructure:"burst_size"`          // Max burst size (default: 200)
	WindowSize        time.Duration `mapstructure:"window_size"`         // Time window for rate limiting (default: 1m)
}

// TLSConfig contains TLS encryption settings for external services
// This is used for Redis, Kafka, and database connections.
type TLSConfig struct {
	Enabled            bool   `mapstructure:"enabled"`              // Enable TLS (default: false)
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify"` // Skip certificate verification (default: false)
	CertFile           string `mapstructure:"cert_file"`            // Client certificate file path
	KeyFile            string `mapstructure:"key_file"`             // Client private key file path
	CAFile             string `mapstructure:"ca_file"`              // CA certificate file path
	ServerName         string `mapstructure:"server_name"`          // Expected server name for certificate verification
}

type PerformanceConfig struct {
	GOMAXPROCS        int     `mapstructure:"gomaxprocs"`          // Number of CPU cores to use (0 = auto-detect) (default: 0)
	GOGC              string  `mapstructure:"gogc"`                // GC target percentage (default: 100)
	GOMEMLIMIT        string  `mapstructure:"gomemlimit"`          // Memory limit (e.g., 1GiB) (default: empty)
	MaxGoroutines     int     `mapstructure:"max_goroutines"`      // Maximum concurrent goroutines (default: 1000000)
	ProfileEnabled    bool    `mapstructure:"profile_enabled"`     // Enable pprof profiling (default: false)
	ProfilePort       int     `mapstructure:"profile_port"`        // Profiling server port (default: 6060)
	EnableTracing     bool    `mapstructure:"enable_tracing"`      // Enable distributed tracing (default: false)
	TracingSampleRate float64 `mapstructure:"tracing_sample_rate"` // Tracing sample rate (0.0-1.0) (default: 0.1)
}

// Load loads configuration from multiple sources in priority order:
// 1. Default values (set in setDefaults())
// 2. Configuration file (if configPath is provided)
// 3. Environment variables (override file settings)
// 4. Validation (ensures all required settings are valid)
func Load(configPath string) (*Config, error) {
	config := &Config{}

	// Step 1: Set default values for all configuration options
	setDefaults()

	// Step 2: Load configuration from file if provided
	// Supports various formats: JSON, YAML, TOML, HCL, etc.
	if configPath != "" {
		viper.SetConfigFile(configPath)
		if err := viper.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Step 3: Load configuration from environment variables
	// Environment variables override file settings.
	// Nested config keys use underscore separation (e.g., SERVER_PORT maps to server.port)
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Step 4: Unmarshal all configuration into the Config struct
	// Viper maps the configuration data to our struct fields using mapstructure tags
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Step 5: Validate the final configuration
	// Ensures all required fields are present and values are within acceptable ranges
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return config, nil
}

// setDefaults defines sensible default values for all configuration options
// These values are used when no configuration file or environment variable is provided
func setDefaults() {
	// Server defaults - HTTP server configuration
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", "15s")
	viper.SetDefault("server.write_timeout", "15s")
	viper.SetDefault("server.idle_timeout", "60s")
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.graceful_shutdown_timeout", "30s")

	// WebSocket defaults - Real-time connection settings
	viper.SetDefault("websocket.read_buffer_size", 1024)
	viper.SetDefault("websocket.write_buffer_size", 1024)
	viper.SetDefault("websocket.ping_period", "54s")
	viper.SetDefault("websocket.pong_wait", "60s")
	viper.SetDefault("websocket.write_wait", "10s")
	viper.SetDefault("websocket.max_message_size", 8192)
	viper.SetDefault("websocket.max_connections", 200000)
	viper.SetDefault("websocket.enable_compression", true)

	// Redis defaults - Cache and session storage
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.max_retries", 3)
	viper.SetDefault("redis.pool_size", 100)
	viper.SetDefault("redis.min_idle_conns", 10)
	viper.SetDefault("redis.max_conn_age", "1h")
	viper.SetDefault("redis.read_timeout", "100ms")
	viper.SetDefault("redis.write_timeout", "100ms")
	viper.SetDefault("redis.pool_timeout", "30s")
	viper.SetDefault("redis.idle_timeout", "5m")
	viper.SetDefault("redis.idle_check_frequency", "1m")
	viper.SetDefault("redis.cluster", false)

	// Kafka defaults - Message streaming system
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.consumer_group", "chatapp")
	viper.SetDefault("kafka.producer_flush_frequency", "100ms")
	viper.SetDefault("kafka.producer_flush_messages", 100)
	viper.SetDefault("kafka.max_message_bytes", 10485760) // 10MB
	viper.SetDefault("kafka.required_acks", "all")
	viper.SetDefault("kafka.retry_max", 5)
	viper.SetDefault("kafka.retry_backoff", "100ms")
	viper.SetDefault("kafka.compression", "snappy")
	viper.SetDefault("kafka.batch_size", 50)
	viper.SetDefault("kafka.batch_timeout", "10ms")

	// Kafka topics - Event streaming channels
	viper.SetDefault("kafka.topics.chat_messages", "chat-messages")
	viper.SetDefault("kafka.topics.delivery_receipts", "delivery-receipts")
	viper.SetDefault("kafka.topics.presence_updates", "presence-updates")
	viper.SetDefault("kafka.topics.metrics", "metrics")

	// ScyllaDB defaults - Primary database for persistent storage
	viper.SetDefault("scylladb.hosts", []string{"localhost:9042"})
	viper.SetDefault("scylladb.keyspace", "chatapp")
	viper.SetDefault("scylladb.connect_timeout", "10s")
	viper.SetDefault("scylladb.timeout", "5s")
	viper.SetDefault("scylladb.num_conns", 4)
	viper.SetDefault("scylladb.consistency", "quorum")
	viper.SetDefault("scylladb.replication_factor", 3)

	// E2EE defaults - End-to-end encryption settings
	viper.SetDefault("e2ee.key_rotation_interval", "24h")
	viper.SetDefault("e2ee.max_skip_messages", 1000)
	viper.SetDefault("e2ee.key_derivation_info", "Double Ratchet Chat")
	viper.SetDefault("e2ee.prekey_lifetime", "30d")
	viper.SetDefault("e2ee.signed_prekey_lifetime", "90d")

	// Metrics defaults - Prometheus monitoring
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.port", 9090)
	viper.SetDefault("metrics.path", "/metrics")
	viper.SetDefault("metrics.namespace", "chatapp")
	viper.SetDefault("metrics.subsystem", "gateway")

	// Logging defaults - Application logging configuration
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")
	viper.SetDefault("logging.max_size", 100)
	viper.SetDefault("logging.max_backups", 3)
	viper.SetDefault("logging.max_age", 28)
	viper.SetDefault("logging.compress", true)

	// Security defaults - TLS and CORS settings
	viper.SetDefault("security.tls_enabled", false)
	viper.SetDefault("security.min_version", "1.2")
	viper.SetDefault("security.allowed_origins", []string{"*"})

	// Rate limiting defaults - Prevent abuse and DoS attacks
	viper.SetDefault("security.rate_limiting.enabled", true)
	viper.SetDefault("security.rate_limiting.requests_per_second", 100)
	viper.SetDefault("security.rate_limiting.burst_size", 200)
	viper.SetDefault("security.rate_limiting.window_size", "1m")

	// Performance defaults - Go runtime optimization
	viper.SetDefault("performance.gomaxprocs", 0) // Auto-detect
	viper.SetDefault("performance.gogc", "100")
	viper.SetDefault("performance.gomemlimit", "")
	viper.SetDefault("performance.max_goroutines", 1000000)
	viper.SetDefault("performance.profile_enabled", false)
	viper.SetDefault("performance.profile_port", 6060)
	viper.SetDefault("performance.enable_tracing", false)
	viper.SetDefault("performance.tracing_sample_rate", 0.1)
}

// Validate performs comprehensive validation of all configuration settings
// It ensures that critical settings have valid values and required fields are present
func (c *Config) Validate() error {
	// Validate server configuration
	// Port must be in valid range (1-65535)
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	// Validate WebSocket configuration
	// Must allow at least one connection
	if c.WebSocket.MaxConnections <= 0 {
		return fmt.Errorf("invalid max_connections: %d", c.WebSocket.MaxConnections)
	}

	// Messages must have positive size limit
	if c.WebSocket.MaxMessageSize <= 0 {
		return fmt.Errorf("invalid max_message_size: %d", c.WebSocket.MaxMessageSize)
	}

	// Validate Redis configuration
	// Redis address is required for cache/session functionality
	if c.Redis.Addr == "" {
		return fmt.Errorf("redis address is required")
	}

	// Validate Kafka configuration
	// At least one broker is needed for message streaming
	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("at least one kafka broker is required")
	}

	// Validate ScyllaDB configuration
	// Database hosts are required for persistent storage
	if len(c.ScyllaDB.Hosts) == 0 {
		return fmt.Errorf("at least one scylladb host is required")
	}

	// Keyspace is required for database organization
	if c.ScyllaDB.Keyspace == "" {
		return fmt.Errorf("scylladb keyspace is required")
	}

	// Validate security configuration
	// TLS requires both certificate and private key files
	if c.Security.TLSEnabled {
		if c.Security.CertFile == "" || c.Security.KeyFile == "" {
			return fmt.Errorf("cert_file and key_file are required when TLS is enabled")
		}
	}

	return nil
}

// GetLogLevel converts the string log level to Zap's AtomicLevel
// This allows dynamic log level changes at runtime
func (c *Config) GetLogLevel() (zap.AtomicLevel, error) {
	var level zapcore.Level
	switch strings.ToLower(c.Logging.Level) {
	case "debug":
		level = zapcore.DebugLevel // Most verbose - includes all debug information
	case "info":
		level = zapcore.InfoLevel // General information about application flow
	case "warn":
		level = zapcore.WarnLevel // Warning messages for potentially problematic situations
	case "error":
		level = zapcore.ErrorLevel // Error messages for failures that don't stop the application
	case "fatal":
		level = zapcore.FatalLevel // Critical errors that cause the application to exit
	default:
		return zap.NewAtomicLevel(), fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	return zap.NewAtomicLevelAt(level), nil
}

// IsProduction checks if the application is running in production mode
// Used to enable/disable development-specific features
func (c *Config) IsProduction() bool {
	return strings.ToLower(os.Getenv("ENV")) == "production"
}

// GetNodeID returns a unique identifier for this application instance
// Priority order: NODE_ID > POD_NAME (Kubernetes) > hostname
func (c *Config) GetNodeID() string {
	nodeID := os.Getenv("NODE_ID") // Explicit node ID
	if nodeID == "" {
		nodeID = os.Getenv("POD_NAME") // Kubernetes pod name
	}
	if nodeID == "" {
		hostname, _ := os.Hostname() // System hostname as fallback
		nodeID = hostname
	}
	return nodeID
}

// GetPodIP returns the IP address of the pod (Kubernetes environment)
func (c *Config) GetPodIP() string {
	return os.Getenv("POD_IP")
}

// GetServiceName returns the name of the service (Kubernetes service discovery)
func (c *Config) GetServiceName() string {
	return os.Getenv("SERVICE_NAME")
}

// GetNamespace returns the Kubernetes namespace the pod is running in
func (c *Config) GetNamespace() string {
	return os.Getenv("NAMESPACE")
}

// GetClusterName returns the name of the cluster (for multi-cluster deployments)
func (c *Config) GetClusterName() string {
	return os.Getenv("CLUSTER_NAME")
}

// GetRegion returns the geographic region (for multi-region deployments)
func (c *Config) GetRegion() string {
	return os.Getenv("REGION")
}

// GetAvailabilityZone returns the availability zone (for high availability setups)
func (c *Config) GetAvailabilityZone() string {
	return os.Getenv("AVAILABILITY_ZONE")
}

// Helper functions for environment variable parsing
// These provide convenient ways to read environment variables with type conversion and defaults

// GetEnvInt reads an environment variable as an integer with a default value
func GetEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// GetEnvDuration reads an environment variable as a time.Duration with a default value
// Accepts Go duration format (e.g., "30s", "5m", "1h")
func GetEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// GetEnvBool reads an environment variable as a boolean with a default value
// Accepts: "1", "t", "T", "TRUE", "true", "True", "0", "f", "F", "FALSE", "false", "False"
func GetEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// GetEnvStringSlice reads an environment variable as a string slice with a default value
// Splits the environment variable value by commas
func GetEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
