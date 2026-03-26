package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Middleware provides HTTP monitoring middleware
type Middleware struct {
	metrics *Metrics
	logger  *zap.Logger
}

// NewMiddleware creates a new monitoring middleware
func NewMiddleware(metrics *Metrics, logger *zap.Logger) *Middleware {
	return &Middleware{
		metrics: metrics,
		logger:  logger,
	}
}

// HTTPMiddleware returns an HTTP middleware for monitoring
func (m *Middleware) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create response writer wrapper to capture status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // Default status code
		}
		
		// Process request
		next.ServeHTTP(wrapped, r)
		
		// Record metrics
		duration := time.Since(start)
		status := strconv.Itoa(wrapped.statusCode)
		
		m.metrics.RecordHTTPRequest(r.Method, r.URL.Path, status, duration)
		
		// Log slow requests
		if duration > 1*time.Second {
			m.logger.Warn("Slow HTTP request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("status", status),
				zap.Duration("duration", duration),
				zap.String("user_agent", r.UserAgent()),
				zap.String("remote_addr", r.RemoteAddr),
			)
		}
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// TracingMiddleware adds distributed tracing to HTTP requests
func (m *Middleware) TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract trace ID from headers or generate new one
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			traceID = generateTraceID()
		}
		
		// Add trace ID to context
		ctx := context.WithValue(r.Context(), "trace_id", traceID)
		r = r.WithContext(ctx)
		
		// Add trace ID to response headers
		w.Header().Set("X-Trace-ID", traceID)
		
		// Log request with trace ID
		m.logger.Debug("HTTP request",
			zap.String("trace_id", traceID),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
		)
		
		next.ServeHTTP(w, r)
	})
}

// SecurityMiddleware adds security headers and logging
func (m *Middleware) SecurityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		
		// Log suspicious requests
		if isSuspiciousRequest(r) {
			m.logger.Warn("Suspicious HTTP request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
				zap.Header("headers", r.Header),
			)
		}
		
		next.ServeHTTP(w, r)
	})
}

// RateLimitMiddleware adds rate limiting
func (m *Middleware) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simple rate limiting based on IP
		// In production, use a proper rate limiting library like go-redis-rate-limit
		clientIP := getClientIP(r)
		
		// Check rate limit (this is a placeholder implementation)
		if isRateLimited(clientIP) {
			m.logger.Warn("Rate limit exceeded",
				zap.String("client_ip", clientIP),
				zap.String("path", r.URL.Path),
			)
			
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// HealthCheckMiddleware provides health check endpoint
func (m *Middleware) HealthCheckMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			// Return health status
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			
			health := map[string]interface{}{
				"status":    "healthy",
				"timestamp": time.Now().Unix(),
				"service":   "chatapp-gateway",
			}
			
			// Add metrics to health check
			if m.metrics != nil {
				health["metrics"] = map[string]interface{}{
					"active_connections": m.metrics.ConnectionsActive.Get(),
					"total_connections":  m.metrics.ConnectionsTotal.Get(),
					"messages_total":     m.metrics.MessagesTotal.Get(),
				}
			}
			
			// Write JSON response
			if err := json.NewEncoder(w).Encode(health); err != nil {
				m.logger.Error("Failed to encode health response", zap.Error(err))
			}
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// generateTraceID generates a unique trace ID
func generateTraceID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

// isSuspiciousRequest checks if a request is suspicious
func isSuspiciousRequest(r *http.Request) bool {
	// Check for common attack patterns
	suspiciousPatterns := []string{
		"<script",
		"javascript:",
		"vbscript:",
		"onload=",
		"onerror=",
		"onclick=",
		"../",
		"..\\",
		"SELECT ",
		"INSERT ",
		"UPDATE ",
		"DELETE ",
		"DROP ",
		"UNION ",
		"EXEC ",
		"SCRIPT ",
	}
	
	userAgent := r.UserAgent()
	path := r.URL.Path
	
	for _, pattern := range suspiciousPatterns {
		if contains(path, pattern) || contains(userAgent, pattern) {
			return true
		}
	}
	
	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
			(s[:len(substr)] == substr || 
			 s[len(s)-len(substr):] == substr ||
			 findSubstring(s, substr))))
}

// findSubstring performs case-insensitive substring search
func findSubstring(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}

// getClientIP extracts the real client IP from request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
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

// isRateLimited checks if a client IP is rate limited
func isRateLimited(clientIP string) bool {
	// This is a placeholder implementation
	// In production, use a proper rate limiting solution
	return false
}

// RequestMetrics holds detailed request metrics
type RequestMetrics struct {
	Method       string        `json:"method"`
	Path         string        `json:"path"`
	StatusCode   int           `json:"status_code"`
	Duration     time.Duration `json:"duration"`
	TraceID      string        `json:"trace_id"`
	ClientIP     string        `json:"client_ip"`
	UserAgent    string        `json:"user_agent"`
	Timestamp    time.Time     `json:"timestamp"`
}

// MetricsCollector collects and exports detailed metrics
type MetricsCollector struct {
	metrics *Metrics
	logger  *zap.Logger
	buffer  []RequestMetrics
	mu      sync.Mutex
 maxSize int
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(metrics *Metrics, logger *zap.Logger, maxSize int) *MetricsCollector {
	return &MetricsCollector{
		metrics: metrics,
		logger:  logger,
		buffer:  make([]RequestMetrics, 0, maxSize),
		maxSize: maxSize,
	}
}

// CollectRequestMetrics collects request metrics
func (mc *MetricsCollector) CollectRequestMetrics(metrics RequestMetrics) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	// Add to buffer
	mc.buffer = append(mc.buffer, metrics)
	
	// Trim buffer if it exceeds max size
	if len(mc.buffer) > mc.maxSize {
		mc.buffer = mc.buffer[1:]
	}
	
	// Log detailed metrics for slow requests
	if metrics.Duration > 1*time.Second {
		mc.logger.Warn("Detailed slow request metrics",
			zap.Any("metrics", metrics),
		)
	}
}

// GetRecentMetrics returns recent metrics
func (mc *MetricsCollector) GetRecentMetrics(count int) []RequestMetrics {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	if count > len(mc.buffer) {
		count = len(mc.buffer)
	}
	
	start := len(mc.buffer) - count
	result := make([]RequestMetrics, count)
	copy(result, mc.buffer[start:])
	
	return result
}

// ClearMetrics clears the metrics buffer
func (mc *MetricsCollector) ClearMetrics() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.buffer = mc.buffer[:0]
}
