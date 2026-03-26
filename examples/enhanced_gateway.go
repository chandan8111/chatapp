package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chatapp/pkg/logging"
	"github.com/chatapp/pkg/monitoring"
	"github.com/chatapp/pkg/pool"
	"github.com/chatapp/pkg/ratelimit"
	"github.com/chatapp/pkg/resilience"
	"github.com/chatapp/pkg/shutdown"
	"github.com/chatapp/pkg/validation"
	"github.com/chatapp/storage"
	"github.com/gocql/gocql"
	"github.com/chatapp/kafka"
	"go.uber.org/zap"
)

// EnhancedGateway demonstrates the improved WebSocket gateway with all recommendations
type EnhancedGateway struct {
	// Core components
	logger         *logging.Logger
	metrics        *monitoring.Metrics
	shutdownManager *shutdown.ShutdownManager
	validator      *validation.Validator
	rateLimiter    *ratelimit.RateLimiter
	
	// External dependencies with resilience
	redisClient    *storage.ResilientRedisClient
	scyllaClient   *storage.ResilientScyllaDBClient
	kafkaProducer  *kafka.ResilientKafkaProducer
	kafkaConsumer  *kafka.ResilientKafkaConsumer
	
	// Connection pools
	connectionManager *pool.ConnectionManager
	
	// HTTP server with middleware
	server         *http.Server
	middleware     *monitoring.Middleware
	rateLimitMW    *ratelimit.Middleware
}

// Config holds the enhanced gateway configuration
type Config struct {
	ServiceName    string `yaml:"service_name"`
	Version        string `yaml:"version"`
	Port          int    `yaml:"port"`
	MetricsPort   int    `yaml:"metrics_port"`
	
	// Logging
	LogLevel      string `yaml:"log_level"`
	LogFormat     string `yaml:"log_format"`
	LogFile       string `yaml:"log_file"`
	
	// Rate limiting
	RedisAddr     string `yaml:"redis_addr"`
	
	// External dependencies
	RedisConfig   storage.ResilientRedisConfig    `yaml:"redis"`
	ScyllaConfig  storage.ResilientScyllaConfig   `yaml:"scylla"`
	KafkaConfig   kafka.ResilientKafkaConfig      `yaml:"kafka"`
	
	// Shutdown
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// NewEnhancedGateway creates a new enhanced WebSocket gateway
func NewEnhancedGateway(config Config) (*EnhancedGateway, error) {
	// Initialize structured logger
	loggerConfig := logging.Config{
		Level:              config.LogLevel,
		Format:             config.LogFormat,
		Filename:           config.LogFile,
		ServiceName:        config.ServiceName,
		Version:            config.Version,
		EnableCaller:       true,
		EnableStacktrace:   true,
	}
	
	logger, err := logging.NewLogger(loggerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}
	
	logger.Info("Starting enhanced WebSocket gateway",
		zap.String("version", config.Version),
		zap.Int("port", config.Port),
	)
	
	// Initialize metrics
	metricsConfig := monitoring.MetricsConfig{
		Namespace:   "chatapp",
		Subsystem:   "gateway",
		ServiceName: config.ServiceName,
		Port:        config.MetricsPort,
		Logger:      logger.Logger,
	}
	
	metrics := monitoring.NewMetrics(metricsConfig)
	
	// Start metrics server
	go func() {
		if err := metrics.StartMetricsServer(context.Background(), config.MetricsPort); err != nil {
			logger.Error("Metrics server failed", zap.Error(err))
		}
	}()
	
	// Initialize shutdown manager
	shutdownConfig := shutdown.Config{
		ShutdownTimeout: config.ShutdownTimeout,
		Logger:          logger.Logger,
	}
	
	shutdownManager := shutdown.NewShutdownManager(shutdownConfig)
	
	// Initialize validator
	validator := validation.NewValidator(logger.Logger)
	
	// Initialize rate limiter
	rateLimitConfig := ratelimit.RateLimitConfig{
		RedisAddr: config.RedisAddr,
		DefaultLimits: map[string]ratelimit.Limit{
			"connection": {Requests: 10, Window: time.Minute, Burst: 5},
			"message":    {Requests: 100, Window: time.Minute, Burst: 20},
			"api":        {Requests: 1000, Window: time.Hour, Burst: 100},
		},
		Logger: logger.Logger,
	}
	
	rateLimiter, err := ratelimit.NewRateLimiter(rateLimitConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create rate limiter: %w", err)
	}
	
	// Initialize connection manager
	connectionManager := pool.NewConnectionManager(logger.Logger)
	
	// Initialize Redis client with resilience
	redisConfig := config.RedisConfig
	redisConfig.CircuitBreaker = resilience.CircuitBreakerConfig{
		Name:         "redis",
		MaxFailures:  5,
		Timeout:      1 * time.Second,
		ResetTimeout: 30 * time.Second,
		Logger:       logger.Logger,
	}
	redisConfig.Bulkhead = resilience.BulkheadConfig{
		Name:           "redis",
		MaxConcurrent:  50,
		Logger:         logger.Logger,
	}
	redisConfig.Logger = logger.Logger
	
	redisClient, err := storage.NewResilientRedisClient(redisConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis client: %w", err)
	}
	
	// Initialize ScyllaDB client with resilience
	scyllaConfig := config.ScyllaConfig
	scyllaConfig.CircuitBreaker = resilience.CircuitBreakerConfig{
		Name:         "scylla",
		MaxFailures:  5,
		Timeout:      2 * time.Second,
		ResetTimeout: 60 * time.Second,
		Logger:       logger.Logger,
	}
	scyllaConfig.Bulkhead = resilience.BulkheadConfig{
		Name:           "scylla",
		MaxConcurrent:  30,
		Logger:         logger.Logger,
	}
	scyllaConfig.Logger = logger.Logger
	
	scyllaClient, err := storage.NewResilientScyllaDBClient(scyllaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create ScyllaDB client: %w", err)
	}
	
	// Initialize Kafka producer with resilience
	kafkaConfig := config.KafkaConfig
	kafkaConfig.CircuitBreaker = resilience.CircuitBreakerConfig{
		Name:         "kafka_producer",
		MaxFailures:  5,
		Timeout:      5 * time.Second,
		ResetTimeout: 60 * time.Second,
		Logger:       logger.Logger,
	}
	kafkaConfig.Bulkhead = resilience.BulkheadConfig{
		Name:           "kafka_producer",
		MaxConcurrent:  20,
		Logger:         logger.Logger,
	}
	kafkaConfig.Logger = logger.Logger
	
	kafkaProducer, err := kafka.NewResilientKafkaProducer(kafkaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}
	
	// Initialize Kafka consumer with resilience
	kafkaConsumerConfig := kafkaConfig
	kafkaConsumerConfig.CircuitBreaker.Name = "kafka_consumer"
	kafkaConsumerConfig.Bulkhead.Name = "kafka_consumer"
	
	kafkaConsumer, err := kafka.NewResilientKafkaConsumer(kafkaConsumerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka consumer: %w", err)
	}
	
	// Initialize middleware
	middleware := monitoring.NewMiddleware(metrics, logger.Logger)
	rateLimitMW := ratelimit.NewMiddleware(rateLimiter, logger.Logger)
	
	// Create HTTP server with middleware
	mux := http.NewServeMux()
	
	// Apply middleware chain
	handler := middleware.TracingMiddleware(
		middleware.SecurityMiddleware(
			rateLimitMW.RateLimitMiddleware(
				middleware.HTTPMiddleware(mux),
			),
		),
	)
	
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.Port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	// Register routes
	gateway := &EnhancedGateway{
		logger:          logger,
		metrics:         metrics,
		shutdownManager: shutdownManager,
		validator:       validator,
		rateLimiter:     rateLimiter,
		redisClient:     redisClient,
		scyllaClient:    scyllaClient,
		kafkaProducer:   kafkaProducer,
		kafkaConsumer:   kafkaConsumer,
		connectionManager: connectionManager,
		server:          server,
		middleware:      middleware,
		rateLimitMW:     rateLimitMW,
	}
	
	gateway.registerRoutes()
	
	// Register services for shutdown
	shutdownManager.Register(shutdown.NewHTTPServerService(server, "http-server"))
	shutdownManager.Register(shutdown.NewCustomService(func(ctx context.Context) error {
		return kafkaProducer.Close()
	}, "kafka-producer"))
	shutdownManager.Register(shutdown.NewCustomService(func(ctx context.Context) error {
		return kafkaConsumer.Close()
	}, "kafka-consumer"))
	shutdownManager.Register(shutdown.NewCustomService(func(ctx context.Context) error {
		return redisClient.Close()
	}, "redis-client"))
	shutdownManager.Register(shutdown.NewCustomService(func(ctx context.Context) error {
		scyllaClient.Close()
		return nil
	}, "scylla-client"))
	shutdownManager.Register(shutdown.NewCustomService(func(ctx context.Context) error {
		return rateLimiter.Close()
	}, "rate-limiter"))
	shutdownManager.Register(shutdown.NewCustomService(func(ctx context.Context) error {
		return connectionManager.Close()
	}, "connection-manager"))
	
	logger.Info("Enhanced WebSocket gateway initialized successfully")
	return gateway, nil
}

// registerRoutes registers HTTP routes
func (g *EnhancedGateway) registerRoutes() {
	// Health check
	http.HandleFunc("/health", g.handleHealth)
	
	// Metrics (handled by metrics server)
	
	// WebSocket endpoint
	http.HandleFunc("/ws", g.handleWebSocket)
	
	// API endpoints
	http.HandleFunc("/api/v1/messages", g.handleMessages)
	http.HandleFunc("/api/v1/presence", g.handlePresence)
}

// handleHealth handles health check requests
func (g *EnhancedGateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		g.metrics.RecordHTTPRequest(r.Method, r.URL.Path, "200", time.Since(start))
	}()
	
	// Check health of all dependencies
	ctx := r.Context()
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   g.logger.serviceName,
		"version":   g.logger.version,
	}
	
	// Check Redis
	if err := g.redisClient.Ping(ctx); err != nil {
		health["redis"] = "unhealthy"
		health["status"] = "degraded"
	} else {
		health["redis"] = "healthy"
	}
	
	// Check ScyllaDB
	if err := g.scyllaClient.Ping(); err != nil {
		health["scylla"] = "unhealthy"
		health["status"] = "degraded"
	} else {
		health["scylla"] = "healthy"
	}
	
	// Add metrics
	health["metrics"] = map[string]interface{}{
		"active_connections": g.metrics.ConnectionsActive.Get(),
		"total_connections":  g.metrics.ConnectionsTotal.Get(),
		"messages_total":     g.metrics.MessagesTotal.Get(),
	}
	
	// Add rate limiter stats
	health["rate_limiter"] = g.rateLimiter.GetStats()
	
	// Add connection pool stats
	health["connection_pools"] = g.connectionManager.GetStats()
	
	w.Header().Set("Content-Type", "application/json")
	if health["status"] == "healthy" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	
	if err := json.NewEncoder(w).Encode(health); err != nil {
		g.logger.Error("Failed to encode health response", zap.Error(err))
	}
}

// handleWebSocket handles WebSocket connection requests
func (g *EnhancedGateway) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	
	// Extract and validate parameters
	userID := r.URL.Query().Get("user_id")
	deviceID := r.URL.Query().Get("device_id")
	nodeID := r.URL.Query().Get("node_id")
	
	// Validate input
	if err := g.validator.ValidateConnectionRequest(userID, deviceID, nodeID); err != nil {
		g.logger.LogSecurity("invalid_connection_params", "medium", map[string]interface{}{
			"user_id":   userID,
			"device_id": deviceID,
			"node_id":   nodeID,
			"errors":    err.Error(),
			"remote_addr": r.RemoteAddr,
		})
		
		http.Error(w, "Invalid parameters", http.StatusBadRequest)
		g.metrics.RecordHTTPRequest(r.Method, r.URL.Path, "400", time.Since(start))
		return
	}
	
	// Check rate limits
	clientIP := getClientIP(r)
	allowed, err := g.rateLimiter.AllowConnection(r.Context(), clientIP)
	if err != nil {
		g.logger.LogError("rate_limit_check", err, map[string]interface{}{
			"client_ip": clientIP,
			"user_id":   userID,
		})
	}
	
	if !allowed {
		g.logger.LogSecurity("connection_rate_limit", "medium", map[string]interface{}{
			"client_ip": clientIP,
			"user_id":   userID,
		})
		
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		g.metrics.RecordHTTPRequest(r.Method, r.URL.Path, "429", time.Since(start))
		return
	}
	
	// Log connection attempt
	g.logger.LogConnection("connection_attempt", userID, deviceID, nodeID, map[string]interface{}{
		"client_ip": clientIP,
		"user_agent": r.UserAgent(),
	})
	
	// TODO: Implement actual WebSocket upgrade logic
	// This would include the WebSocket connection handling with all the improvements
	
	w.WriteHeader(http.StatusOK)
	g.metrics.RecordHTTPRequest(r.Method, r.URL.Path, "200", time.Since(start))
}

// handleMessages handles message API requests
func (g *EnhancedGateway) handleMessages(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	
	// Validate API request
	if err := g.validator.ValidateAPIRequest(r.Method, r.URL.Path, getHeaders(r), nil); err != nil {
		g.logger.LogSecurity("invalid_api_request", "medium", map[string]interface{}{
			"method":     r.Method,
			"path":       r.URL.Path,
			"errors":     err.Error(),
			"remote_addr": r.RemoteAddr,
		})
		
		http.Error(w, "Invalid request", http.StatusBadRequest)
		g.metrics.RecordHTTPRequest(r.Method, r.URL.Path, "400", time.Since(start))
		return
	}
	
	// Check rate limits
	clientIP := getClientIP(r)
	allowed, err := g.rateLimiter.AllowAPIRequest(r.Context(), clientIP, "messages")
	if err != nil {
		g.logger.LogError("rate_limit_check", err, map[string]interface{}{
			"client_ip": clientIP,
			"endpoint":  "messages",
		})
	}
	
	if !allowed {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		g.metrics.RecordHTTPRequest(r.Method, r.URL.Path, "429", time.Since(start))
		return
	}
	
	// TODO: Implement actual message handling logic
	
	w.WriteHeader(http.StatusOK)
	g.metrics.RecordHTTPRequest(r.Method, r.URL.Path, "200", time.Since(start))
}

// handlePresence handles presence API requests
func (g *EnhancedGateway) handlePresence(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	
	// Similar validation and rate limiting as handleMessages
	
	// TODO: Implement actual presence handling logic
	
	w.WriteHeader(http.StatusOK)
	g.metrics.RecordHTTPRequest(r.Method, r.URL.Path, "200", time.Since(start))
}

// Start starts the enhanced gateway
func (g *EnhancedGateway) Start(ctx context.Context) error {
	g.logger.Info("Starting enhanced WebSocket gateway")
	
	// Start shutdown manager in background
	go g.shutdownManager.Start(ctx)
	
	// Start HTTP server
	port := 8080 // Default port, should come from config
	g.logger.Info("Starting HTTP server", zap.Int("port", port))
	if err := g.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server failed: %w", err)
	}
	
	return nil
}

// getClientIP extracts the real client IP from request
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	return r.RemoteAddr
}

// getHeaders extracts headers from request
func getHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	return headers
}

// main demonstrates how to use the enhanced gateway
func main() {
	config := Config{
		ServiceName:    "chatapp-gateway",
		Version:        "2.0.0",
		Port:          8080,
		MetricsPort:   9090,
		LogLevel:      "info",
		LogFormat:     "json",
		LogFile:       "/var/log/chatapp/gateway.log",
		RedisAddr:     "localhost:6379",
		ShutdownTimeout: 30 * time.Second,
		
		RedisConfig: storage.ResilientRedisConfig{
			RedisAddr:      "localhost:6379",
			MaxRetries:     3,
			RetryDelay:     100 * time.Millisecond,
		},
		
		ScyllaConfig: storage.ResilientScyllaConfig{
			Hosts:          []string{"localhost:9042"},
			Keyspace:       "chatapp",
			ConnectTimeout: 5 * time.Second,
			Timeout:        2 * time.Second,
			NumConns:       10,
			Consistency:    gocql.Quorum,
		},
		
		KafkaConfig: kafka.ResilientKafkaConfig{
			Brokers:       []string{"localhost:9092"},
			ConsumerGroup: "gateway-group",
			MaxRetries:    3,
			RetryDelay:    100 * time.Millisecond,
		},
	}
	
	gateway, err := NewEnhancedGateway(config)
	if err != nil {
		fmt.Printf("Failed to create enhanced gateway: %v\n", err)
		os.Exit(1)
	}
	
	ctx := context.Background()
	if err := gateway.Start(ctx); err != nil {
		fmt.Printf("Failed to start enhanced gateway: %v\n", err)
		os.Exit(1)
	}
}
