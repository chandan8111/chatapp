package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	WebSocket   WebSocketConfig   `mapstructure:"websocket"`
	Redis       RedisConfig       `mapstructure:"redis"`
	Kafka       KafkaConfig       `mapstructure:"kafka"`
	ScyllaDB    ScyllaDBConfig    `mapstructure:"scylladb"`
	E2EE        E2EEConfig        `mapstructure:"e2ee"`
	Metrics     MetricsConfig     `mapstructure:"metrics"`
	Logging     LoggingConfig     `mapstructure:"logging"`
	Security    SecurityConfig    `mapstructure:"security"`
	Performance PerformanceConfig `mapstructure:"performance"`
}

type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	Host         string        `mapstructure:"host"`
	GracefulShutdownTimeout time.Duration `mapstructure:"graceful_shutdown_timeout"`
}

type WebSocketConfig struct {
	ReadBufferSize  int           `mapstructure:"read_buffer_size"`
	WriteBufferSize int           `mapstructure:"write_buffer_size"`
	PingPeriod      time.Duration `mapstructure:"ping_period"`
	PongWait        time.Duration `mapstructure:"pong_wait"`
	WriteWait       time.Duration `mapstructure:"write_wait"`
	MaxMessageSize  int64         `mapstructure:"max_message_size"`
	MaxConnections  int           `mapstructure:"max_connections"`
	EnableCompression bool        `mapstructure:"enable_compression"`
}

type RedisConfig struct {
	Addr            string        `mapstructure:"addr"`
	Password        string        `mapstructure:"password"`
	DB              int           `mapstructure:"db"`
	MaxRetries      int           `mapstructure:"max_retries"`
	PoolSize        int           `mapstructure:"pool_size"`
	MinIdleConns    int           `mapstructure:"min_idle_conns"`
	MaxConnAge      time.Duration `mapstructure:"max_conn_age"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	PoolTimeout     time.Duration `mapstructure:"pool_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	IdleCheckFrequency time.Duration `mapstructure:"idle_check_frequency"`
	TLS             TLSConfig     `mapstructure:"tls"`
	Cluster         bool          `mapstructure:"cluster"`
	ClusterNodes    []string      `mapstructure:"cluster_nodes"`
}

type KafkaConfig struct {
	Brokers                []string      `mapstructure:"brokers"`
	ConsumerGroup          string        `mapstructure:"consumer_group"`
	ProducerFlushFrequency time.Duration `mapstructure:"producer_flush_frequency"`
	ProducerFlushMessages  int           `mapstructure:"producer_flush_messages"`
	MaxMessageBytes        int           `mapstructure:"max_message_bytes"`
	RequiredAcks           string        `mapstructure:"required_acks"`
	RetryMax               int           `mapstructure:"retry_max"`
	RetryBackoff           time.Duration `mapstructure:"retry_backoff"`
	Compression            string        `mapstructure:"compression"`
	BatchSize              int           `mapstructure:"batch_size"`
	BatchTimeout           time.Duration `mapstructure:"batch_timeout"`
	Topics                 TopicsConfig  `mapstructure:"topics"`
	SASL                   SASLConfig    `mapstructure:"sasl"`
	TLS                    TLSConfig     `mapstructure:"tls"`
}

type TopicsConfig struct {
	ChatMessages      string `mapstructure:"chat_messages"`
	DeliveryReceipts  string `mapstructure:"delivery_receipts"`
	PresenceUpdates   string `mapstructure:"presence_updates"`
	Metrics           string `mapstructure:"metrics"`
}

type SASLConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Mechanism string `mapstructure:"mechanism"`
	Username  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
}

type ScyllaDBConfig struct {
	Hosts              []string      `mapstructure:"hosts"`
	Keyspace           string        `mapstructure:"keyspace"`
	Username           string        `mapstructure:"username"`
	Password           string        `mapstructure:"password"`
	ConnectTimeout     time.Duration `mapstructure:"connect_timeout"`
	Timeout            time.Duration `mapstructure:"timeout"`
	NumConns           int           `mapstructure:"num_conns"`
	Consistency        string        `mapstructure:"consistency"`
	ReplicationFactor  int           `mapstructure:"replication_factor"`
	DC                 string        `mapstructure:"dc"`
	TLS                TLSConfig     `mapstructure:"tls"`
}

type E2EEConfig struct {
	KeyRotationInterval time.Duration `mapstructure:"key_rotation_interval"`
	MaxSkipMessages     int           `mapstructure:"max_skip_messages"`
	KeyDerivationInfo   string        `mapstructure:"key_derivation_info"`
	PreKeyLifetime      time.Duration `mapstructure:"prekey_lifetime"`
	SignedPreKeyLifetime time.Duration `mapstructure:"signed_prekey_lifetime"`
}

type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    int    `mapstructure:"port"`
	Path    string `mapstructure:"path"`
	Namespace string `mapstructure:"namespace"`
	Subsystem string `mapstructure:"subsystem"`
}

type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

type SecurityConfig struct {
	TLSEnabled    bool     `mapstructure:"tls_enabled"`
	MinVersion    string   `mapstructure:"min_version"`
	CertFile      string   `mapstructure:"cert_file"`
	KeyFile       string   `mapstructure:"key_file"`
	CAFile        string   `mapstructure:"ca_file"`
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	RateLimiting  RateLimitConfig `mapstructure:"rate_limiting"`
}

type RateLimitConfig struct {
	Enabled     bool          `mapstructure:"enabled"`
	RequestsPerSecond int     `mapstructure:"requests_per_second"`
	BurstSize   int           `mapstructure:"burst_size"`
	WindowSize  time.Duration `mapstructure:"window_size"`
}

type TLSConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify"`
	CertFile           string `mapstructure:"cert_file"`
	KeyFile            string `mapstructure:"key_file"`
	CAFile             string `mapstructure:"ca_file"`
	ServerName         string `mapstructure:"server_name"`
}

type PerformanceConfig struct {
	GOMAXPROCS    int           `mapstructure:"gomaxprocs"`
	GOGC          string        `mapstructure:"gogc"`
	GOMEMLIMIT    string        `mapstructure:"gomemlimit"`
	MaxGoroutines int           `mapstructure:"max_goroutines"`
	ProfileEnabled bool         `mapstructure:"profile_enabled"`
	ProfilePort   int           `mapstructure:"profile_port"`
	EnableTracing bool          `mapstructure:"enable_tracing"`
	TracingSampleRate float64   `mapstructure:"tracing_sample_rate"`
}

func Load(configPath string) (*Config, error) {
	config := &Config{}
	
	// Set defaults
	setDefaults()
	
	// Load from file
	if configPath != "" {
		viper.SetConfigFile(configPath)
		if err := viper.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}
	
	// Load from environment
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	
	// Unmarshal config
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	
	// Validate config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	
	return config, nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", "15s")
	viper.SetDefault("server.write_timeout", "15s")
	viper.SetDefault("server.idle_timeout", "60s")
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.graceful_shutdown_timeout", "30s")
	
	// WebSocket defaults
	viper.SetDefault("websocket.read_buffer_size", 1024)
	viper.SetDefault("websocket.write_buffer_size", 1024)
	viper.SetDefault("websocket.ping_period", "54s")
	viper.SetDefault("websocket.pong_wait", "60s")
	viper.SetDefault("websocket.write_wait", "10s")
	viper.SetDefault("websocket.max_message_size", 8192)
	viper.SetDefault("websocket.max_connections", 200000)
	viper.SetDefault("websocket.enable_compression", true)
	
	// Redis defaults
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
	
	// Kafka defaults
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
	
	// Kafka topics
	viper.SetDefault("kafka.topics.chat_messages", "chat-messages")
	viper.SetDefault("kafka.topics.delivery_receipts", "delivery-receipts")
	viper.SetDefault("kafka.topics.presence_updates", "presence-updates")
	viper.SetDefault("kafka.topics.metrics", "metrics")
	
	// ScyllaDB defaults
	viper.SetDefault("scylladb.hosts", []string{"localhost:9042"})
	viper.SetDefault("scylladb.keyspace", "chatapp")
	viper.SetDefault("scylladb.connect_timeout", "10s")
	viper.SetDefault("scylladb.timeout", "5s")
	viper.SetDefault("scylladb.num_conns", 4)
	viper.SetDefault("scylladb.consistency", "quorum")
	viper.SetDefault("scylladb.replication_factor", 3)
	
	// E2EE defaults
	viper.SetDefault("e2ee.key_rotation_interval", "24h")
	viper.SetDefault("e2ee.max_skip_messages", 1000)
	viper.SetDefault("e2ee.key_derivation_info", "Double Ratchet Chat")
	viper.SetDefault("e2ee.prekey_lifetime", "30d")
	viper.SetDefault("e2ee.signed_prekey_lifetime", "90d")
	
	// Metrics defaults
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.port", 9090)
	viper.SetDefault("metrics.path", "/metrics")
	viper.SetDefault("metrics.namespace", "chatapp")
	viper.SetDefault("metrics.subsystem", "gateway")
	
	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")
	viper.SetDefault("logging.max_size", 100)
	viper.SetDefault("logging.max_backups", 3)
	viper.SetDefault("logging.max_age", 28)
	viper.SetDefault("logging.compress", true)
	
	// Security defaults
	viper.SetDefault("security.tls_enabled", false)
	viper.SetDefault("security.min_version", "1.2")
	viper.SetDefault("security.allowed_origins", []string{"*"})
	
	// Rate limiting defaults
	viper.SetDefault("security.rate_limiting.enabled", true)
	viper.SetDefault("security.rate_limiting.requests_per_second", 100)
	viper.SetDefault("security.rate_limiting.burst_size", 200)
	viper.SetDefault("security.rate_limiting.window_size", "1m")
	
	// Performance defaults
	viper.SetDefault("performance.gomaxprocs", 0) // Auto-detect
	viper.SetDefault("performance.gogc", "100")
	viper.SetDefault("performance.gomemlimit", "")
	viper.SetDefault("performance.max_goroutines", 1000000)
	viper.SetDefault("performance.profile_enabled", false)
	viper.SetDefault("performance.profile_port", 6060)
	viper.SetDefault("performance.enable_tracing", false)
	viper.SetDefault("performance.tracing_sample_rate", 0.1)
}

func (c *Config) Validate() error {
	// Validate server config
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	
	// Validate WebSocket config
	if c.WebSocket.MaxConnections <= 0 {
		return fmt.Errorf("invalid max_connections: %d", c.WebSocket.MaxConnections)
	}
	
	if c.WebSocket.MaxMessageSize <= 0 {
		return fmt.Errorf("invalid max_message_size: %d", c.WebSocket.MaxMessageSize)
	}
	
	// Validate Redis config
	if c.Redis.Addr == "" {
		return fmt.Errorf("redis address is required")
	}
	
	// Validate Kafka config
	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("at least one kafka broker is required")
	}
	
	// Validate ScyllaDB config
	if len(c.ScyllaDB.Hosts) == 0 {
		return fmt.Errorf("at least one scylladb host is required")
	}
	
	if c.ScyllaDB.Keyspace == "" {
		return fmt.Errorf("scylladb keyspace is required")
	}
	
	// Validate security config
	if c.Security.TLSEnabled {
		if c.Security.CertFile == "" || c.Security.KeyFile == "" {
			return fmt.Errorf("cert_file and key_file are required when TLS is enabled")
		}
	}
	
	return nil
}

func (c *Config) GetLogLevel() (zap.AtomicLevel, error) {
	var level zapcore.Level
	switch strings.ToLower(c.Logging.Level) {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	case "fatal":
		level = zapcore.FatalLevel
	default:
		return zap.NewAtomicLevel(), fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}
	
	return zap.NewAtomicLevelAt(level), nil
}

func (c *Config) IsProduction() bool {
	return strings.ToLower(os.Getenv("ENV")) == "production"
}

func (c *Config) GetNodeID() string {
	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		nodeID = os.Getenv("POD_NAME")
	}
	if nodeID == "" {
		hostname, _ := os.Hostname()
		nodeID = hostname
	}
	return nodeID
}

func (c *Config) GetPodIP() string {
	return os.Getenv("POD_IP")
}

func (c *Config) GetServiceName() string {
	return os.Getenv("SERVICE_NAME")
}

func (c *Config) GetNamespace() string {
	return os.Getenv("NAMESPACE")
}

func (c *Config) GetClusterName() string {
	return os.Getenv("CLUSTER_NAME")
}

func (c *Config) GetRegion() string {
	return os.Getenv("REGION")
}

func (c *Config) GetAvailabilityZone() string {
	return os.Getenv("AVAILABILITY_ZONE")
}

// Helper functions for environment variable parsing
func GetEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func GetEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func GetEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func GetEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
