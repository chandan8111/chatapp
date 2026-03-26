package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/chatapp/pkg/resilience"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ResilientRedisClient wraps Redis client with circuit breaker and retry logic
type ResilientRedisClient struct {
	client         *redis.Client
	circuitBreaker *resilience.CircuitBreaker
	bulkhead       *resilience.Bulkhead
	logger         *zap.Logger
}

// ResilientRedisConfig holds configuration for resilient Redis client
type ResilientRedisConfig struct {
	RedisAddr         string
	MaxRetries        int
	RetryDelay        time.Duration
	CircuitBreaker    resilience.CircuitBreakerConfig
	Bulkhead          resilience.BulkheadConfig
	Logger            *zap.Logger
}

// NewResilientRedisClient creates a new resilient Redis client
func NewResilientRedisClient(config ResilientRedisConfig) (*ResilientRedisClient, error) {
	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:            config.RedisAddr,
		MaxRetries:      config.MaxRetries,
		PoolSize:        100,
		MinIdleConns:    10,
		MaxConnAge:      time.Hour,
		ReadTimeout:     100 * time.Millisecond,
		WriteTimeout:    100 * time.Millisecond,
		PoolTimeout:     30 * time.Second,
		IdleTimeout:     5 * time.Minute,
		IdleCheckFrequency: 1 * time.Minute,
	})

	// Create circuit breaker
	circuitBreaker := resilience.NewCircuitBreaker(config.CircuitBreaker)

	// Create bulkhead
	bulkhead := resilience.NewBulkhead(config.Bulkhead)

	resilientClient := &ResilientRedisClient{
		client:         client,
		circuitBreaker: circuitBreaker,
		bulkhead:       bulkhead,
		logger:         config.Logger,
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := resilientClient.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	config.Logger.Info("Resilient Redis client initialized",
		zap.String("addr", config.RedisAddr),
	)

	return resilientClient, nil
}

// Ping tests Redis connection
func (r *ResilientRedisClient) Ping(ctx context.Context) error {
	return r.circuitBreaker.Execute(ctx, func() error {
		return r.bulkhead.Execute(ctx, func() error {
			return resilience.Retry(ctx, resilience.RetryConfig{
				MaxRetries:    3,
				InitialDelay:  10 * time.Millisecond,
				MaxDelay:      100 * time.Millisecond,
				BackoffFactor: 2.0,
			}, func() error {
				return r.client.Ping(ctx).Err()
			}, r.logger)
		})
	})
}

// SetBit sets a bit in a Redis bitmap
func (r *ResilientRedisClient) SetBit(ctx context.Context, key string, offset int64, value int) (int64, error) {
	var result int64
	err := r.circuitBreaker.Execute(ctx, func() error {
		return r.bulkhead.Execute(ctx, func() error {
			return resilience.Retry(ctx, resilience.RetryConfig{
				MaxRetries:    3,
				InitialDelay:  10 * time.Millisecond,
				MaxDelay:      100 * time.Millisecond,
				BackoffFactor: 2.0,
			}, func() error {
				val, err := r.client.SetBit(ctx, key, offset, value).Result()
				if err != nil {
					return err
				}
				result = val
				return nil
			}, r.logger)
		})
	})
	
	if err != nil {
		r.logger.Error("Failed to set Redis bit",
			zap.String("key", key),
			zap.Int64("offset", offset),
			zap.Int("value", value),
			zap.Error(err),
		)
	}
	
	return result, err
}

// Pipeline creates a Redis pipeline
func (r *ResilientRedisClient) Pipeline() redis.Pipeliner {
	return r.client.Pipeline()
}

// Exec executes a Redis pipeline
func (r *ResilientRedisClient) ExecPipeline(ctx context.Context, pipe redis.Pipeliner) ([]redis.Cmder, error) {
	var result []redis.Cmder
	err := r.circuitBreaker.Execute(ctx, func() error {
		return r.bulkhead.Execute(ctx, func() error {
			return resilience.Retry(ctx, resilience.RetryConfig{
				MaxRetries:    3,
				InitialDelay:  10 * time.Millisecond,
				MaxDelay:      100 * time.Millisecond,
				BackoffFactor: 2.0,
			}, func() error {
				val, err := pipe.Exec(ctx)
				if err != nil {
					return err
				}
				result = val
				return nil
			}, r.logger)
		})
	})
	
	if err != nil {
		r.logger.Error("Failed to execute Redis pipeline",
			zap.Error(err),
		)
	}
	
	return result, err
}

// Get retrieves a value from Redis
func (r *ResilientRedisClient) Get(ctx context.Context, key string) (string, error) {
	var result string
	err := r.circuitBreaker.Execute(ctx, func() error {
		return r.bulkhead.Execute(ctx, func() error {
			return resilience.Retry(ctx, resilience.RetryConfig{
				MaxRetries:    3,
				InitialDelay:  10 * time.Millisecond,
				MaxDelay:      100 * time.Millisecond,
				BackoffFactor: 2.0,
			}, func() error {
				val, err := r.client.Get(ctx, key).Result()
				if err != nil {
					return err
				}
				result = val
				return nil
			}, r.logger)
		})
	})
	
	return result, err
}

// Set sets a value in Redis
func (r *ResilientRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	err := r.circuitBreaker.Execute(ctx, func() error {
		return r.bulkhead.Execute(ctx, func() error {
			return resilience.Retry(ctx, resilience.RetryConfig{
				MaxRetries:    3,
				InitialDelay:  10 * time.Millisecond,
				MaxDelay:      100 * time.Millisecond,
				BackoffFactor: 2.0,
			}, func() error {
				return r.client.Set(ctx, key, value, expiration).Err()
			}, r.logger)
		})
	})
	
	if err != nil {
		r.logger.Error("Failed to set Redis value",
			zap.String("key", key),
			zap.Error(err),
		)
	}
	
	return err
}

// Close closes the Redis client
func (r *ResilientRedisClient) Close() error {
	return r.client.Close()
}

// GetMetrics returns Prometheus metrics for the resilient Redis client
func (r *ResilientRedisClient) GetMetrics() []interface{} {
	metrics := []interface{}{}
	
	// Add circuit breaker metrics
	for _, metric := range r.circuitBreaker.GetMetrics() {
		metrics = append(metrics, metric)
	}
	
	// Add bulkhead metrics
	for _, metric := range r.bulkhead.GetMetrics() {
		metrics = append(metrics, metric)
	}
	
	return metrics
}
