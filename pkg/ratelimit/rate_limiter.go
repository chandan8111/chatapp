package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	redis  *redis.Client
	logger *zap.Logger
	
	// Metrics
	requestsTotal    prometheus.Counter
	requestsBlocked  prometheus.Counter
	requestsAllowed  prometheus.Counter
	
	// Local cache for rate limits
	localLimits map[string]*LocalLimiter
	localMu     sync.RWMutex
}

// LocalLimiter provides in-memory rate limiting
type LocalLimiter struct {
	tokens    int
	capacity  int
	lastRefill time.Time
	mu        sync.Mutex
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	RedisAddr     string        `yaml:"redis_addr"`
	DefaultLimits map[string]Limit `yaml:"default_limits"`
	WindowSizes   map[string]time.Duration `yaml:"window_sizes"`
	Logger        *zap.Logger
}

// Limit defines a rate limit
type Limit struct {
	Requests int           `yaml:"requests"`
	Window   time.Duration `yaml:"window"`
	Burst    int           `yaml:"burst"`
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config RateLimitConfig) (*RateLimiter, error) {
	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:            config.RedisAddr,
		MaxRetries:      3,
		PoolSize:        50,
		MinIdleConns:    5,
		MaxConnAge:      time.Hour,
		ReadTimeout:     100 * time.Millisecond,
		WriteTimeout:    100 * time.Millisecond,
		PoolTimeout:     30 * time.Second,
		IdleTimeout:     5 * time.Minute,
		IdleCheckFrequency: 1 * time.Minute,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := rdb.Ping(ctx).Err(); err != nil {
		config.Logger.Warn("Redis not available, using local rate limiting only", zap.Error(err))
		rdb = nil
	}

	// Initialize metrics
	requestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "rate_limit_requests_total",
		Help: "Total number of rate limit requests",
	}, []string{"type", "key"})

	requestsBlocked := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "rate_limit_requests_blocked_total",
		Help: "Total number of blocked requests",
	}, []string{"type", "key"})

	requestsAllowed := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "rate_limit_requests_allowed_total",
		Help: "Total number of allowed requests",
	}, []string{"type", "key"})

	rl := &RateLimiter{
		redis:       rdb,
		logger:      config.Logger,
		requestsTotal: requestsTotal,
		requestsBlocked: requestsBlocked,
		requestsAllowed: requestsAllowed,
		localLimits: make(map[string]*LocalLimiter),
	}

	// Register metrics
	prometheus.MustRegister(requestsTotal, requestsBlocked, requestsAllowed)

	config.Logger.Info("Rate limiter initialized",
		zap.Bool("redis_enabled", rdb != nil),
	)

	return rl, nil
}

// Allow checks if a request is allowed based on rate limits
func (rl *RateLimiter) Allow(ctx context.Context, key string, limit Limit) (bool, error) {
	rl.requestsTotal.WithLabelValues("check", key).Inc()
	
	// Try Redis first if available
	if rl.redis != nil {
		allowed, err := rl.allowRedis(ctx, key, limit)
		if err == nil {
			if allowed {
				rl.requestsAllowed.WithLabelValues("redis", key).Inc()
			} else {
				rl.requestsBlocked.WithLabelValues("redis", key).Inc()
			}
			return allowed, nil
		}
		
		rl.logger.Warn("Redis rate limiting failed, falling back to local", 
			zap.String("key", key), 
			zap.Error(err))
	}
	
	// Fall back to local rate limiting
	allowed := rl.allowLocal(key, limit)
	if allowed {
		rl.requestsAllowed.WithLabelValues("local", key).Inc()
	} else {
		rl.requestsBlocked.WithLabelValues("local", key).Inc()
	}
	
	return allowed, nil
}

// allowRedis implements Redis-based rate limiting using sliding window
func (rl *RateLimiter) allowRedis(ctx context.Context, key string, limit Limit) (bool, error) {
	now := time.Now().UnixNano()
	windowStart := now - limit.Window.Nanoseconds()
	
	redisKey := fmt.Sprintf("rate_limit:%s", key)
	
	// Use Redis pipeline for atomic operations
	pipe := rl.redis.Pipeline()
	
	// Remove old entries
	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart))
	
	// Count current entries
	countCmd := pipe.ZCard(ctx, redisKey)
	
	// Add current request
	pipe.ZAdd(ctx, redisKey, redis.Z{
		Score:  float64(now),
		Member: now,
	})
	
	// Set expiration
	pipe.Expire(ctx, redisKey, limit.Window)
	
	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}
	
	currentCount := countCmd.Val()
	
	// Check if under limit
	return currentCount < limit.Requests, nil
}

// allowLocal implements local in-memory rate limiting using token bucket
func (rl *RateLimiter) allowLocal(key string, limit Limit) bool {
	rl.localMu.Lock()
	defer rl.localMu.Unlock()
	
	// Get or create local limiter
	limiter, exists := rl.localLimits[key]
	if !exists {
		limiter = &LocalLimiter{
			tokens:    limit.Requests,
			capacity:  limit.Requests,
			lastRefill: time.Now(),
		}
		rl.localLimits[key] = limiter
	}
	
	return limiter.allow(limit)
}

// allow implements token bucket algorithm
func (ll *LocalLimiter) allow(limit Limit) bool {
	ll.mu.Lock()
	defer ll.mu.Unlock()
	
	now := time.Now()
	
	// Refill tokens based on time elapsed
	elapsed := now.Sub(ll.lastRefill)
	tokensToAdd := int(elapsed / limit.Window * time.Duration(limit.Requests))
	
	if tokensToAdd > 0 {
		ll.tokens += tokensToAdd
		if ll.tokens > ll.capacity {
			ll.tokens = ll.capacity
		}
		ll.lastRefill = now
	}
	
	// Check if token available
	if ll.tokens > 0 {
		ll.tokens--
		return true
	}
	
	return false
}

// AllowConnection checks if a new WebSocket connection is allowed
func (rl *RateLimiter) AllowConnection(ctx context.Context, clientIP string) (bool, error) {
	// Rate limit connections per IP
	limit := Limit{
		Requests: 10,
		Window:   time.Minute,
		Burst:    5,
	}
	
	key := fmt.Sprintf("connection:%s", clientIP)
	return rl.Allow(ctx, key, limit)
}

// AllowMessage checks if a message is allowed
func (rl *RateLimiter) AllowMessage(ctx context.Context, userID string) (bool, error) {
	// Rate limit messages per user
	limit := Limit{
		Requests: 100,
		Window:   time.Minute,
		Burst:    20,
	}
	
	key := fmt.Sprintf("message:%s", userID)
	return rl.Allow(ctx, key, limit)
}

// AllowAPIRequest checks if an API request is allowed
func (rl *RateLimiter) AllowAPIRequest(ctx context.Context, clientIP, endpoint string) (bool, error) {
	// Rate limit API requests per IP and endpoint
	limit := Limit{
		Requests: 1000,
		Window:   time.Hour,
		Burst:    100,
	}
	
	key := fmt.Sprintf("api:%s:%s", clientIP, endpoint)
	return rl.Allow(ctx, key, limit)
}

// AllowPresenceUpdate checks if a presence update is allowed
func (rl *RateLimiter) AllowPresenceUpdate(ctx context.Context, userID string) (bool, error) {
	// Rate limit presence updates per user
	limit := Limit{
		Requests: 60,
		Window:   time.Minute,
		Burst:    10,
	}
	
	key := fmt.Sprintf("presence:%s", userID)
	return rl.Allow(ctx, key, limit)
}

// AllowSearch checks if a search request is allowed
func (rl *RateLimiter) AllowSearch(ctx context.Context, userID string) (bool, error) {
	// Rate limit search requests per user
	limit := Limit{
		Requests: 30,
		Window:   time.Minute,
		Burst:    5,
	}
	
	key := fmt.Sprintf("search:%s", userID)
	return rl.Allow(ctx, key, limit)
}

// GetRemainingRequests returns the number of remaining requests for a key
func (rl *RateLimiter) GetRemainingRequests(ctx context.Context, key string, limit Limit) (int, error) {
	if rl.redis != nil {
		return rl.getRemainingRedis(ctx, key, limit)
	}
	
	return rl.getRemainingLocal(key, limit), nil
}

// getRemainingRedis gets remaining requests from Redis
func (rl *RateLimiter) getRemainingRedis(ctx context.Context, key string, limit Limit) (int, error) {
	now := time.Now().UnixNano()
	windowStart := now - limit.Window.Nanoseconds()
	
	redisKey := fmt.Sprintf("rate_limit:%s", key)
	
	// Remove old entries and count current
	pipe := rl.redis.Pipeline()
	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart))
	countCmd := pipe.ZCard(ctx, redisKey)
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	
	currentCount := countCmd.Val()
	remaining := limit.Requests - currentCount
	if remaining < 0 {
		remaining = 0
	}
	
	return remaining, nil
}

// getRemainingLocal gets remaining requests from local limiter
func (rl *RateLimiter) getRemainingLocal(key string, limit Limit) int {
	rl.localMu.RLock()
	defer rl.localMu.RUnlock()
	
	limiter, exists := rl.localLimits[key]
	if !exists {
		return limit.Requests
	}
	
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	
	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(limiter.lastRefill)
	tokensToAdd := int(elapsed / limit.Window * time.Duration(limit.Requests))
	
	if tokensToAdd > 0 {
		limiter.tokens += tokensToAdd
		if limiter.tokens > limiter.capacity {
			limiter.tokens = limiter.capacity
		}
		limiter.lastRefill = now
	}
	
	return limiter.tokens
}

// Reset resets the rate limit for a key
func (rl *RateLimiter) Reset(ctx context.Context, key string) error {
	if rl.redis != nil {
		redisKey := fmt.Sprintf("rate_limit:%s", key)
		return rl.redis.Del(ctx, redisKey).Err()
	}
	
	rl.localMu.Lock()
	defer rl.localMu.Unlock()
	
	delete(rl.localLimits, key)
	return nil
}

// CleanupExpiredLimits cleans up expired local limits
func (rl *RateLimiter) CleanupExpiredLimits() {
	rl.localMu.Lock()
	defer rl.localMu.Unlock()
	
	now := time.Now()
	for key, limiter := range rl.localLimits {
		limiter.mu.Lock()
		
		// Remove if not used for 1 hour
		if now.Sub(limiter.lastRefill) > time.Hour {
			delete(rl.localLimits, key)
		}
		
		limiter.mu.Unlock()
	}
}

// GetStats returns rate limiting statistics
func (rl *RateLimiter) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})
	
	rl.localMu.RLock()
	stats["local_limits_count"] = len(rl.localLimits)
	rl.localMu.RUnlock()
	
	stats["redis_enabled"] = rl.redis != nil
	
	return stats
}

// Close closes the rate limiter
func (rl *RateLimiter) Close() error {
	if rl.redis != nil {
		return rl.redis.Close()
	}
	return nil
}

// Middleware provides HTTP rate limiting middleware
type Middleware struct {
	rateLimiter *RateLimiter
	logger      *zap.Logger
}

// NewMiddleware creates a new rate limiting middleware
func NewMiddleware(rateLimiter *RateLimiter, logger *zap.Logger) *Middleware {
	return &Middleware{
		rateLimiter: rateLimiter,
		logger:      logger,
	}
}

// RateLimitMiddleware returns an HTTP middleware for rate limiting
func (m *Middleware) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)
		endpoint := r.URL.Path
		
		// Check rate limit
		allowed, err := m.rateLimiter.AllowAPIRequest(r.Context(), clientIP, endpoint)
		if err != nil {
			m.logger.Error("Rate limiting check failed",
				zap.String("client_ip", clientIP),
				zap.String("endpoint", endpoint),
				zap.Error(err),
			)
			// Allow request on error
			next.ServeHTTP(w, r)
			return
		}
		
		if !allowed {
			m.logger.Warn("Rate limit exceeded",
				zap.String("client_ip", clientIP),
				zap.String("endpoint", endpoint),
				zap.String("method", r.Method),
				zap.String("user_agent", r.UserAgent()),
			)
			
			w.Header().Set("Retry-After", "60")
			w.Header().Set("X-RateLimit-Limit", "1000")
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Minute).Unix()))
			
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		
		// Add rate limit headers
		remaining, _ := m.rateLimiter.GetRemainingRequests(r.Context(), 
			fmt.Sprintf("api:%s:%s", clientIP, endpoint), 
			Limit{Requests: 1000, Window: time.Hour})
		
		w.Header().Set("X-RateLimit-Limit", "1000")
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix()))
		
		next.ServeHTTP(w, r)
	})
}

// getClientIP extracts the real client IP from request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// Fall back to RemoteAddr
	return r.RemoteAddr
}
