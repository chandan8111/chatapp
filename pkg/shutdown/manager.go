package shutdown

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// ShutdownManager manages graceful shutdown of services
type ShutdownManager struct {
	logger         *zap.Logger
	services       []Service
	shutdownTimeout time.Duration
	mu             sync.RWMutex
	shuttingDown   bool
	done           chan struct{}
}

// Service represents a service that can be shut down
type Service interface {
	Shutdown(ctx context.Context) error
	Name() string
}

// Config holds shutdown configuration
type Config struct {
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	Logger          *zap.Logger   `yaml:"-"`
}

// NewShutdownManager creates a new shutdown manager
func NewShutdownManager(config Config) *ShutdownManager {
	return &ShutdownManager{
		logger:         config.Logger,
		services:       make([]Service, 0),
		shutdownTimeout: config.ShutdownTimeout,
		done:           make(chan struct{}),
	}
}

// Register registers a service for shutdown
func (sm *ShutdownManager) Register(service Service) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.services = append(sm.services, service)
	sm.logger.Info("Service registered for shutdown",
		zap.String("service", service.Name()),
		zap.Int("total_services", len(sm.services)),
	)
}

// Start starts the shutdown manager
func (sm *ShutdownManager) Start(ctx context.Context) {
	// Wait for shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	select {
	case sig := <-sigChan:
		sm.logger.Info("Received shutdown signal",
			zap.String("signal", sig.String()),
		)
		sm.shutdown()
	case <-ctx.Done():
		sm.logger.Info("Context cancelled, initiating shutdown")
		sm.shutdown()
	}
}

// shutdown performs graceful shutdown
func (sm *ShutdownManager) shutdown() {
	sm.mu.Lock()
	if sm.shuttingDown {
		sm.mu.Unlock()
		return
	}
	sm.shuttingDown = true
	sm.mu.Unlock()
	
	sm.logger.Info("Starting graceful shutdown",
		zap.Duration("timeout", sm.shutdownTimeout),
		zap.Int("services_count", len(sm.services)),
	)
	
	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), sm.shutdownTimeout)
	defer cancel()
	
	// Shutdown all services concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, len(sm.services))
	
	for _, service := range sm.services {
		wg.Add(1)
		go func(s Service) {
			defer wg.Done()
			
			sm.logger.Info("Shutting down service",
				zap.String("service", s.Name()),
			)
			
			if err := s.Shutdown(shutdownCtx); err != nil {
				sm.logger.Error("Service shutdown failed",
					zap.String("service", s.Name()),
					zap.Error(err),
				)
				errChan <- err
			} else {
				sm.logger.Info("Service shutdown completed",
					zap.String("service", s.Name()),
				)
			}
		}(service)
	}
	
	// Wait for all services to shutdown or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		sm.logger.Info("All services shutdown successfully")
	case <-shutdownCtx.Done():
		sm.logger.Error("Shutdown timeout exceeded",
			zap.Duration("timeout", sm.shutdownTimeout),
		)
	}
	
	// Check for errors
	close(errChan)
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}
	
	if len(errors) > 0 {
		sm.logger.Error("Shutdown completed with errors",
			zap.Int("error_count", len(errors)),
		)
	}
	
	close(sm.done)
}

// Wait waits for shutdown to complete
func (sm *ShutdownManager) Wait() {
	<-sm.done
}

// IsShuttingDown returns true if shutdown is in progress
func (sm *ShutdownManager) IsShuttingDown() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.shuttingDown
}

// HTTPServerService wraps an HTTP server for shutdown
type HTTPServerService struct {
	server *http.Server
	name   string
}

// NewHTTPServerService creates a new HTTP server service
func NewHTTPServerService(server *http.Server, name string) *HTTPServerService {
	return &HTTPServerService{
		server: server,
		name:   name,
	}
}

// Shutdown shuts down the HTTP server
func (h *HTTPServerService) Shutdown(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}

// Name returns the service name
func (h *HTTPServerService) Name() string {
	return h.name
}

// WebSocketGatewayService wraps a WebSocket gateway for shutdown
type WebSocketGatewayService struct {
	gateway interface {
		Shutdown(ctx context.Context) error
	}
	name string
}

// NewWebSocketGatewayService creates a new WebSocket gateway service
func NewWebSocketGatewayService(gateway interface{}, name string) *WebSocketGatewayService {
	return &WebSocketGatewayService{
		gateway: gateway,
		name:    name,
	}
}

// Shutdown shuts down the WebSocket gateway
func (w *WebSocketGatewayService) Shutdown(ctx context.Context) error {
	// Type assertion to check if gateway has Shutdown method
	if gateway, ok := w.gateway.(interface{ Shutdown(ctx context.Context) error }); ok {
		return gateway.Shutdown(ctx)
	}
	return nil
}

// Name returns the service name
func (w *WebSocketGatewayService) Name() string {
	return w.name
}

// KafkaProducerService wraps a Kafka producer for shutdown
type KafkaProducerService struct {
	producer interface {
		Close() error
	}
	name string
}

// NewKafkaProducerService creates a new Kafka producer service
func NewKafkaProducerService(producer interface{}, name string) *KafkaProducerService {
	return &KafkaProducerService{
		producer: producer,
		name:     name,
	}
}

// Shutdown shuts down the Kafka producer
func (k *KafkaProducerService) Shutdown(ctx context.Context) error {
	// Kafka producer Close() doesn't accept context, so we run it in a goroutine
	done := make(chan error, 1)
	go func() {
		if producer, ok := k.producer.(interface{ Close() error }); ok {
			done <- producer.Close()
		} else {
			done <- nil
		}
	}()
	
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Name returns the service name
func (k *KafkaProducerService) Name() string {
	return k.name
}

// KafkaConsumerService wraps a Kafka consumer for shutdown
type KafkaConsumerService struct {
	consumer interface {
		Close() error
	}
	name string
}

// NewKafkaConsumerService creates a new Kafka consumer service
func NewKafkaConsumerService(consumer interface{}, name string) *KafkaConsumerService {
	return &KafkaConsumerService{
		consumer: consumer,
		name:     name,
	}
}

// Shutdown shuts down the Kafka consumer
func (k *KafkaConsumerService) Shutdown(ctx context.Context) error {
	// Kafka consumer Close() doesn't accept context, so we run it in a goroutine
	done := make(chan error, 1)
	go func() {
		if consumer, ok := k.consumer.(interface{ Close() error }); ok {
			done <- consumer.Close()
		} else {
			done <- nil
		}
	}()
	
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Name returns the service name
func (k *KafkaConsumerService) Name() string {
	return k.name
}

// DatabaseService wraps a database client for shutdown
type DatabaseService struct {
	client interface {
		Close() error
	}
	name string
}

// NewDatabaseService creates a new database service
func NewDatabaseService(client interface{}, name string) *DatabaseService {
	return &DatabaseService{
		client: client,
		name:   name,
	}
}

// Shutdown shuts down the database client
func (d *DatabaseService) Shutdown(ctx context.Context) error {
	// Database Close() doesn't accept context, so we run it in a goroutine
	done := make(chan error, 1)
	go func() {
		if client, ok := d.client.(interface{ Close() error }); ok {
			done <- client.Close()
		} else {
			done <- nil
		}
	}()
	
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Name returns the service name
func (d *DatabaseService) Name() string {
	return d.name
}

// RedisService wraps a Redis client for shutdown
type RedisService struct {
	client interface {
		Close() error
	}
	name string
}

// NewRedisService creates a new Redis service
func NewRedisService(client interface{}, name string) *RedisService {
	return &RedisService{
		client: client,
		name:   name,
	}
}

// Shutdown shuts down the Redis client
func (r *RedisService) Shutdown(ctx context.Context) error {
	// Redis Close() doesn't accept context, so we run it in a goroutine
	done := make(chan error, 1)
	go func() {
		if client, ok := r.client.(interface{ Close() error }); ok {
			done <- client.Close()
		} else {
			done <- nil
		}
	}()
	
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Name returns the service name
func (r *RedisService) Name() string {
	return r.name
}

// CustomService wraps a custom shutdown function
type CustomService struct {
	shutdownFunc func(ctx context.Context) error
	name         string
}

// NewCustomService creates a new custom service
func NewCustomService(shutdownFunc func(ctx context.Context) error, name string) *CustomService {
	return &CustomService{
		shutdownFunc: shutdownFunc,
		name:         name,
	}
}

// Shutdown calls the custom shutdown function
func (c *CustomService) Shutdown(ctx context.Context) error {
	return c.shutdownFunc(ctx)
}

// Name returns the service name
func (c *CustomService) Name() string {
	return c.name
}

// HealthChecker provides health check functionality during shutdown
type HealthChecker struct {
	services map[string]func() error
	logger   *zap.Logger
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(logger *zap.Logger) *HealthChecker {
	return &HealthChecker{
		services: make(map[string]func() error),
		logger:   logger,
	}
}

// AddService adds a health check for a service
func (h *HealthChecker) AddService(name string, checkFunc func() error) {
	h.services[name] = checkFunc
}

// CheckHealth checks the health of all services
func (h *HealthChecker) CheckHealth() map[string]error {
	results := make(map[string]error)
	
	for name, checkFunc := range h.services {
		if err := checkFunc(); err != nil {
			results[name] = err
			h.logger.Error("Health check failed",
				zap.String("service", name),
				zap.Error(err),
			)
		} else {
			h.logger.Debug("Health check passed",
				zap.String("service", name),
			)
		}
	}
	
	return results
}

// WaitForHealthy waits for all services to be healthy
func (h *HealthChecker) WaitForHealthy(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	
	for {
		results := h.CheckHealth()
		if len(results) == 0 {
			return nil
		}
		
		if time.Now().After(deadline) {
			return fmt.Errorf("services not healthy after timeout: %v", results)
		}
		
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			// Continue checking
		}
	}
}
