package monitoring

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Metrics holds all application metrics
type Metrics struct {
	// Connection metrics
	ConnectionsActive      prometheus.Gauge
	ConnectionsTotal       prometheus.Counter
	ConnectionsDuration    prometheus.Histogram
	ConnectionErrors       prometheus.Counter
	
	// Message metrics
	MessagesTotal          prometheus.Counter
	MessagesDuration       prometheus.Histogram
	MessageSize            prometheus.Histogram
	MessageErrors          prometheus.Counter
	
	// Presence metrics
	PresenceUpdates        prometheus.Counter
	PresenceOnlineUsers    prometheus.Gauge
	PresenceErrors         prometheus.Counter
	
	// Kafka metrics
	KafkaMessagesProduced  prometheus.Counter
	KafkaMessagesConsumed  prometheus.Counter
	KafkaErrors            prometheus.Counter
	KafkaLag               prometheus.Gauge
	
	// Redis metrics
	RedisOperations        prometheus.Counter
	RedisErrors            prometheus.Counter
	RedisDuration          prometheus.Histogram
	
	// ScyllaDB metrics
	ScyllaOperations       prometheus.Counter
	ScyllaErrors           prometheus.Counter
	ScyllaDuration         prometheus.Histogram
	
	// HTTP metrics
	HTTPRequests           prometheus.Counter
	HTTPDuration           prometheus.Histogram
	HTTPErrors             prometheus.Counter
	
	// System metrics
	Goroutines             prometheus.Gauge
	MemoryUsage            prometheus.Gauge
	GCDuration             prometheus.Histogram
	
	logger *zap.Logger
}

// MetricsConfig holds configuration for metrics
type MetricsConfig struct {
	Namespace   string
	Subsystem   string
	ServiceName string
	Port        int
	Logger      *zap.Logger
}

// NewMetrics creates a new metrics instance
func NewMetrics(config MetricsConfig) *Metrics {
	m := &Metrics{
		logger: config.Logger,
	}
	
	// Initialize connection metrics
	m.ConnectionsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "connections_active",
		Help:      "Number of active WebSocket connections",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	m.ConnectionsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "connections_total",
		Help:      "Total number of WebSocket connections",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	m.ConnectionsDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "connections_duration_seconds",
		Help:      "Duration of WebSocket connections",
		Buckets:   prometheus.DefBuckets,
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	m.ConnectionErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "connection_errors_total",
		Help:      "Total number of connection errors",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	// Initialize message metrics
	m.MessagesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "messages_total",
		Help:      "Total number of messages processed",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	m.MessagesDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "messages_duration_seconds",
		Help:      "Duration of message processing",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	m.MessageSize = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "messages_size_bytes",
		Help:      "Size of messages in bytes",
		Buckets:   []float64{100, 500, 1000, 5000, 10000, 50000, 100000, 500000, 1000000},
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	m.MessageErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "message_errors_total",
		Help:      "Total number of message processing errors",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	// Initialize presence metrics
	m.PresenceUpdates = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "presence_updates_total",
		Help:      "Total number of presence updates",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	m.PresenceOnlineUsers = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "presence_online_users",
		Help:      "Number of currently online users",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	m.PresenceErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "presence_errors_total",
		Help:      "Total number of presence update errors",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	// Initialize Kafka metrics
	m.KafkaMessagesProduced = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "kafka_messages_produced_total",
		Help:      "Total number of Kafka messages produced",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	m.KafkaMessagesConsumed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "kafka_messages_consumed_total",
		Help:      "Total number of Kafka messages consumed",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	m.KafkaErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "kafka_errors_total",
		Help:      "Total number of Kafka errors",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	m.KafkaLag = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "kafka_lag",
		Help:      "Kafka consumer lag",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	// Initialize Redis metrics
	m.RedisOperations = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "redis_operations_total",
		Help:      "Total number of Redis operations",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	m.RedisErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "redis_errors_total",
		Help:      "Total number of Redis errors",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	m.RedisDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "redis_duration_seconds",
		Help:      "Duration of Redis operations",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	// Initialize ScyllaDB metrics
	m.ScyllaOperations = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "scylla_operations_total",
		Help:      "Total number of ScyllaDB operations",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	m.ScyllaErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "scylla_errors_total",
		Help:      "Total number of ScyllaDB errors",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	m.ScyllaDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "scylla_duration_seconds",
		Help:      "Duration of ScyllaDB operations",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	// Initialize HTTP metrics
	m.HTTPRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "http_requests_total",
		Help:      "Total number of HTTP requests",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	}, []string{"method", "endpoint", "status"})
	
	m.HTTPDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "http_duration_seconds",
		Help:      "Duration of HTTP requests",
		Buckets:   prometheus.DefBuckets,
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	}, []string{"method", "endpoint"})
	
	m.HTTPErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "http_errors_total",
		Help:      "Total number of HTTP errors",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	// Initialize system metrics
	m.Goroutines = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "goroutines",
		Help:      "Number of goroutines",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	m.MemoryUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "memory_usage_bytes",
		Help:      "Memory usage in bytes",
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	m.GCDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: config.Namespace,
		Subsystem: config.Subsystem,
		Name:      "gc_duration_seconds",
		Help:      "Garbage collection duration",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
		ConstLabels: prometheus.Labels{
			"service": config.ServiceName,
		},
	})
	
	// Register all metrics
	m.registerMetrics()
	
	return m
}

// registerMetrics registers all metrics with Prometheus
func (m *Metrics) registerMetrics() {
	prometheus.MustRegister(
		m.ConnectionsActive,
		m.ConnectionsTotal,
		m.ConnectionsDuration,
		m.ConnectionErrors,
		m.MessagesTotal,
		m.MessagesDuration,
		m.MessageSize,
		m.MessageErrors,
		m.PresenceUpdates,
		m.PresenceOnlineUsers,
		m.PresenceErrors,
		m.KafkaMessagesProduced,
		m.KafkaMessagesConsumed,
		m.KafkaErrors,
		m.KafkaLag,
		m.RedisOperations,
		m.RedisErrors,
		m.RedisDuration,
		m.ScyllaOperations,
		m.ScyllaErrors,
		m.ScyllaDuration,
		m.HTTPRequests,
		m.HTTPDuration,
		m.HTTPErrors,
		m.Goroutines,
		m.MemoryUsage,
		m.GCDuration,
	)
}

// StartMetricsServer starts the Prometheus metrics server
func (m *Metrics) StartMetricsServer(ctx context.Context, port int) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	
	m.logger.Info("Starting metrics server", zap.Int("port", port))
	
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			m.logger.Error("Metrics server failed", zap.Error(err))
		}
	}()
	
	// Wait for context cancellation
	<-ctx.Done()
	
	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := server.Shutdown(shutdownCtx); err != nil {
		m.logger.Error("Failed to shutdown metrics server", zap.Error(err))
		return err
	}
	
	m.logger.Info("Metrics server stopped")
	return nil
}

// RecordConnection records connection metrics
func (m *Metrics) RecordConnection(duration time.Duration, err error) {
	m.ConnectionsTotal.Inc()
	m.ConnectionsDuration.Observe(duration.Seconds())
	
	if err != nil {
		m.ConnectionErrors.Inc()
	}
}

// UpdateActiveConnections updates the active connections gauge
func (m *Metrics) UpdateActiveConnections(count int64) {
	m.ConnectionsActive.Set(float64(count))
}

// RecordMessage records message metrics
func (m *Metrics) RecordMessage(size int, duration time.Duration, err error) {
	m.MessagesTotal.Inc()
	m.MessagesDuration.Observe(duration.Seconds())
	m.MessageSize.Observe(float64(size))
	
	if err != nil {
		m.MessageErrors.Inc()
	}
}

// RecordPresenceUpdate records presence update metrics
func (m *Metrics) RecordPresenceUpdate(err error) {
	m.PresenceUpdates.Inc()
	
	if err != nil {
		m.PresenceErrors.Inc()
	}
}

// UpdateOnlineUsers updates the online users gauge
func (m *Metrics) UpdateOnlineUsers(count int64) {
	m.PresenceOnlineUsers.Set(float64(count))
}

// RecordKafkaMessage records Kafka message metrics
func (m *Metrics) RecordKafkaMessage(produced bool, err error) {
	if produced {
		m.KafkaMessagesProduced.Inc()
	} else {
		m.KafkaMessagesConsumed.Inc()
	}
	
	if err != nil {
		m.KafkaErrors.Inc()
	}
}

// UpdateKafkaLag updates the Kafka lag gauge
func (m *Metrics) UpdateKafkaLag(lag int64) {
	m.KafkaLag.Set(float64(lag))
}

// RecordRedisOperation records Redis operation metrics
func (m *Metrics) RecordRedisOperation(duration time.Duration, err error) {
	m.RedisOperations.Inc()
	m.RedisDuration.Observe(duration.Seconds())
	
	if err != nil {
		m.RedisErrors.Inc()
	}
}

// RecordScyllaOperation records ScyllaDB operation metrics
func (m *Metrics) RecordScyllaOperation(duration time.Duration, err error) {
	m.ScyllaOperations.Inc()
	m.ScyllaDuration.Observe(duration.Seconds())
	
	if err != nil {
		m.ScyllaErrors.Inc()
	}
}

// RecordHTTPRequest records HTTP request metrics
func (m *Metrics) RecordHTTPRequest(method, endpoint, status string, duration time.Duration) {
	m.HTTPRequests.WithLabelValues(method, endpoint, status).Inc()
	m.HTTPDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
	
	if status[0] == '4' || status[0] == '5' {
		m.HTTPErrors.Inc()
	}
}

// UpdateSystemMetrics updates system-level metrics
func (m *Metrics) UpdateSystemMetrics(goroutines, memoryBytes int) {
	m.Goroutines.Set(float64(goroutines))
	m.MemoryUsage.Set(float64(memoryBytes))
}

// RecordGCDuration records garbage collection duration
func (m *Metrics) RecordGCDuration(duration time.Duration) {
	m.GCDuration.Observe(duration.Seconds())
}

// GetCollectors returns all Prometheus collectors
func (m *Metrics) GetCollectors() []prometheus.Collector {
	return []prometheus.Collector{
		m.ConnectionsActive,
		m.ConnectionsTotal,
		m.ConnectionsDuration,
		m.ConnectionErrors,
		m.MessagesTotal,
		m.MessagesDuration,
		m.MessageSize,
		m.MessageErrors,
		m.PresenceUpdates,
		m.PresenceOnlineUsers,
		m.PresenceErrors,
		m.KafkaMessagesProduced,
		m.KafkaMessagesConsumed,
		m.KafkaErrors,
		m.KafkaLag,
		m.RedisOperations,
		m.RedisErrors,
		m.RedisDuration,
		m.ScyllaOperations,
		m.ScyllaErrors,
		m.ScyllaDuration,
		m.HTTPRequests,
		m.HTTPDuration,
		m.HTTPErrors,
		m.Goroutines,
		m.MemoryUsage,
		m.GCDuration,
	}
}
