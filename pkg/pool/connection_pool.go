package pool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// ConnectionPool manages a pool of connections
type ConnectionPool struct {
	factory     ConnectionFactory
	connections chan Connection
	mu          sync.RWMutex
	closed      bool
	
	// Configuration
	maxOpen     int
	maxIdle     int
	maxLifetime time.Duration
	maxIdleTime time.Duration
	
	// Metrics
	activeConnections   prometheus.Gauge
	idleConnections     prometheus.Gauge
	waitCount           prometheus.Counter
	waitDuration        prometheus.Histogram
	createCount         prometheus.Counter
	destroyCount        prometheus.Counter
	
	logger *zap.Logger
}

// Connection represents a database connection
type Connection interface {
	Close() error
	IsClosed() bool
	LastUsed() time.Time
}

// ConnectionFactory creates new connections
type ConnectionFactory func() (Connection, error)

// Config holds connection pool configuration
type Config struct {
	MaxOpen     int           `yaml:"max_open"`
	MaxIdle     int           `yaml:"max_idle"`
	MaxLifetime time.Duration `yaml:"max_lifetime"`
	MaxIdleTime time.Duration `yaml:"max_idle_time"`
	Namespace   string        `yaml:"namespace"`
	Subsystem   string        `yaml:"subsystem"`
	Service     string        `yaml:"service"`
	Logger      *zap.Logger   `yaml:"-"`
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(factory ConnectionFactory, config Config) (*ConnectionPool, error) {
	if config.MaxOpen <= 0 {
		config.MaxOpen = 10
	}
	if config.MaxIdle <= 0 {
		config.MaxIdle = 5
	}
	if config.MaxIdle > config.MaxOpen {
		config.MaxIdle = config.MaxOpen
	}
	
	pool := &ConnectionPool{
		factory:     factory,
		connections: make(chan Connection, config.MaxIdle),
		maxOpen:     config.MaxOpen,
		maxIdle:     config.MaxIdle,
		maxLifetime: config.MaxLifetime,
		maxIdleTime: config.MaxIdleTime,
		logger:      config.Logger,
	}
	
	// Initialize metrics
	pool.activeConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "active_connections",
		Help:      "Number of active connections",
		ConstLabels: prometheus.Labels{
			"service": config.Service,
		},
	})
	
	pool.idleConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "idle_connections",
		Help:      "Number of idle connections",
		ConstLabels: prometheus.Labels{
			"service": config.Service,
		},
	})
	
	pool.waitCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "wait_count_total",
		Help:      "Total number of times waited for connection",
		ConstLabels: prometheus.Labels{
			"service": config.Service,
		},
	})
	
	pool.waitDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "wait_duration_seconds",
		Help:      "Time spent waiting for connection",
		Buckets:   prometheus.DefBuckets,
		ConstLabels: prometheus.Labels{
			"service": config.Service,
		},
	})
	
	pool.createCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "create_count_total",
		Help:      "Total number of connections created",
		ConstLabels: prometheus.Labels{
			"service": config.Service,
		},
	})
	
	pool.destroyCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "destroy_count_total",
		Help:      "Total number of connections destroyed",
		ConstLabels: prometheus.Labels{
			"service": config.Service,
		},
	})
	
	// Register metrics
	prometheus.MustRegister(
		pool.activeConnections,
		pool.idleConnections,
		pool.waitCount,
		pool.waitDuration,
		pool.createCount,
		pool.destroyCount,
	)
	
	// Start cleanup goroutine
	go pool.cleanup()
	
	config.Logger.Info("Connection pool initialized",
		zap.Int("max_open", config.MaxOpen),
		zap.Int("max_idle", config.MaxIdle),
		zap.Duration("max_lifetime", config.MaxLifetime),
		zap.Duration("max_idle_time", config.MaxIdleTime),
	)
	
	return pool, nil
}

// Get gets a connection from the pool
func (p *ConnectionPool) Get(ctx context.Context) (Connection, error) {
	if p.closed {
		return nil, fmt.Errorf("connection pool is closed")
	}
	
	start := time.Now()
	
	// Try to get idle connection
	select {
	case conn := <-p.connections:
		if conn.IsClosed() {
			p.destroyCount.Inc()
			return p.Get(ctx) // Try again
		}
		p.idleConnections.Dec()
		p.activeConnections.Inc()
		return conn, nil
	default:
		// No idle connections available
	}
	
	// Check if we can create new connection
	p.mu.RLock()
	openCount := len(p.connections) + int(p.activeConnections.Get())
	p.mu.RUnlock()
	
	if openCount < p.maxOpen {
		conn, err := p.factory()
		if err != nil {
			return nil, fmt.Errorf("failed to create connection: %w", err)
		}
		p.createCount.Inc()
		p.activeConnections.Inc()
		return conn, nil
	}
	
	// Wait for a connection to become available
	p.waitCount.Inc()
	
	select {
	case conn := <-p.connections:
		if conn.IsClosed() {
			p.destroyCount.Inc()
			p.activeConnections.Dec()
			return p.Get(ctx) // Try again
		}
		p.idleConnections.Dec()
		p.activeConnections.Inc()
		p.waitDuration.Observe(time.Since(start).Seconds())
		return conn, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Put returns a connection to the pool
func (p *ConnectionPool) Put(conn Connection) error {
	if p.closed || conn.IsClosed() {
		p.destroyCount.Inc()
		p.activeConnections.Dec()
		return nil
	}
	
	p.mu.RLock()
	idleCount := len(p.connections)
	p.mu.RUnlock()
	
	// If pool is full, close the connection
	if idleCount >= p.maxIdle {
		p.destroyCount.Inc()
		p.activeConnections.Dec()
		return conn.Close()
	}
	
	select {
	case p.connections <- conn:
		p.activeConnections.Dec()
		p.idleConnections.Inc()
		return nil
	default:
		// Channel is full (shouldn't happen with maxIdle check)
		p.destroyCount.Inc()
		p.activeConnections.Dec()
		return conn.Close()
	}
}

// Close closes the connection pool
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()
	
	// Close all idle connections
	close(p.connections)
	for conn := range p.connections {
		if err := conn.Close(); err != nil {
			p.logger.Error("Error closing connection",
				zap.Error(err),
			)
		}
		p.destroyCount.Inc()
	}
	
	p.logger.Info("Connection pool closed")
	return nil
}

// cleanup periodically removes old connections
func (p *ConnectionPool) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return
		}
		
		// Check for idle connections that have exceeded max idle time
		if p.maxIdleTime > 0 {
			now := time.Now()
			var toRemove []Connection
			
			// Drain connections temporarily
			temp := make([]Connection, 0, len(p.connections))
			for conn := range p.connections {
				if now.Sub(conn.LastUsed()) > p.maxIdleTime {
					toRemove = append(toRemove, conn)
				} else {
					temp = append(temp, conn)
				}
			}
			
			// Put back valid connections
			for _, conn := range temp {
				p.connections <- conn
			}
			
			// Close old connections
			for _, conn := range toRemove {
				if err := conn.Close(); err != nil {
					p.logger.Error("Error closing old connection",
						zap.Error(err),
					)
				}
				p.destroyCount.Inc()
				p.idleConnections.Dec()
			}
		}
		
		p.mu.Unlock()
	}
}

// Stats returns pool statistics
func (p *ConnectionPool) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return map[string]interface{}{
		"active_connections": int(p.activeConnections.Get()),
		"idle_connections":   int(p.idleConnections.Get()),
		"max_open":          p.maxOpen,
		"max_idle":          p.maxIdle,
		"closed":            p.closed,
	}
}

// RedisConnection implements Connection for Redis
type RedisConnection struct {
	client     interface{}
	lastUsed   time.Time
	created    time.Time
	mu         sync.Mutex
}

// NewRedisConnection creates a new Redis connection wrapper
func NewRedisConnection(client interface{}) *RedisConnection {
	return &RedisConnection{
		client:  client,
		lastUsed: time.Now(),
		created: time.Now(),
	}
}

// Close closes the Redis connection
func (r *RedisConnection) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Type assertion to check if client has Close method
	if closer, ok := r.client.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

// IsClosed returns true if the connection is closed
func (r *RedisConnection) IsClosed() bool {
	// This is a simplified check - in practice, you'd need to implement
	// proper connection state tracking
	return false
}

// LastUsed returns the last time the connection was used
func (r *RedisConnection) LastUsed() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastUsed
}

// UpdateLastUsed updates the last used time
func (r *RedisConnection) UpdateLastUsed() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastUsed = time.Now()
}

// DatabaseConnection implements Connection for database clients
type DatabaseConnection struct {
	client     interface{}
	lastUsed   time.Time
	created    time.Time
	mu         sync.Mutex
}

// NewDatabaseConnection creates a new database connection wrapper
func NewDatabaseConnection(client interface{}) *DatabaseConnection {
	return &DatabaseConnection{
		client:  client,
		lastUsed: time.Now(),
		created: time.Now(),
	}
}

// Close closes the database connection
func (d *DatabaseConnection) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	// Type assertion to check if client has Close method
	if closer, ok := d.client.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

// IsClosed returns true if the connection is closed
func (d *DatabaseConnection) IsClosed() bool {
	// This is a simplified check - in practice, you'd need to implement
	// proper connection state tracking
	return false
}

// LastUsed returns the last time the connection was used
func (d *DatabaseConnection) LastUsed() time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastUsed
}

// UpdateLastUsed updates the last used time
func (d *DatabaseConnection) UpdateLastUsed() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lastUsed = time.Now()
}

// PooledConnection wraps a connection with pool functionality
type PooledConnection struct {
	Connection
	pool *ConnectionPool
}

// NewPooledConnection creates a new pooled connection
func NewPooledConnection(conn Connection, pool *ConnectionPool) *PooledConnection {
	return &PooledConnection{
		Connection: conn,
		pool:       pool,
	}
}

// Close returns the connection to the pool instead of closing it
func (p *PooledConnection) Close() error {
	return p.pool.Put(p.Connection)
}

// ConnectionManager manages multiple connection pools
type ConnectionManager struct {
	pools map[string]*ConnectionPool
	mu    sync.RWMutex
	logger *zap.Logger
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(logger *zap.Logger) *ConnectionManager {
	return &ConnectionManager{
		pools: make(map[string]*ConnectionPool),
		logger: logger,
	}
}

// RegisterPool registers a connection pool
func (cm *ConnectionManager) RegisterPool(name string, pool *ConnectionPool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	cm.pools[name] = pool
	cm.logger.Info("Connection pool registered",
		zap.String("name", name),
	)
}

// GetPool gets a connection pool by name
func (cm *ConnectionManager) GetPool(name string) (*ConnectionPool, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	pool, exists := cm.pools[name]
	if !exists {
		return nil, fmt.Errorf("connection pool '%s' not found", name)
	}
	
	return pool, nil
}

// Close closes all connection pools
func (cm *ConnectionManager) Close() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	var errors []error
	for name, pool := range cm.pools {
		if err := pool.Close(); err != nil {
			cm.logger.Error("Error closing connection pool",
				zap.String("name", name),
				zap.Error(err),
			)
			errors = append(errors, err)
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("errors closing pools: %v", errors)
	}
	
	cm.logger.Info("All connection pools closed")
	return nil
}

// GetStats returns statistics for all pools
func (cm *ConnectionManager) GetStats() map[string]map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	stats := make(map[string]map[string]interface{})
	for name, pool := range cm.pools {
		stats[name] = pool.Stats()
	}
	
	return stats
}
