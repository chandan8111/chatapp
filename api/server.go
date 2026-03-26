package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/chatapp/errors"
	"github.com/chatapp/logging"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// APIServer represents the REST API server
type APIServer struct {
	router     *mux.Router
	logger     *logging.Logger
	errorHandler *errors.ErrorHandler
	port       int
	
	// Service handlers
	presenceHandler  *PresenceHandler
	messageHandler   *MessageHandler
	userHandler      *UserHandler
	conversationHandler *ConversationHandler
	analyticsHandler *AnalyticsHandler
}

// NewAPIServer creates a new API server
func NewAPIServer(port int, logger *logging.Logger) *APIServer {
	router := mux.NewRouter()
	
	server := &APIServer{
		router:         router,
		logger:         logger,
		errorHandler: errors.NewErrorHandler(logger.Logger, "api-server"),
		port:           port,
	}
	
	server.setupRoutes()
	server.setupMiddleware()
	
	return server
}

// setupRoutes configures all API routes
func (s *APIServer) setupRoutes() {
	// Health and metrics
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")
	s.router.HandleFunc("/ready", s.handleReadiness).Methods("GET")
	s.router.HandleFunc("/metrics", s.handleMetrics).Methods("GET")
	
	// API v1 routes
	api := s.router.PathPrefix("/api/v1").Subrouter()
	
	// Presence routes
	api.HandleFunc("/presence/{user_id}", s.presenceHandler.GetPresence).Methods("GET")\tapi.HandleFunc("/presence/batch", s.presenceHandler.GetPresenceBatch).Methods("POST")
	api.HandleFunc("/presence/online", s.presenceHandler.GetOnlineUsers).Methods("GET")
	
	// Message routes
	api.HandleFunc("/messages", s.messageHandler.SendMessage).Methods("POST")\tapi.HandleFunc("/messages/{message_id}", s.messageHandler.GetMessage).Methods("GET")
	api.HandleFunc("/messages/{message_id}/status", s.messageHandler.UpdateMessageStatus).Methods("PUT")
	api.HandleFunc("/conversations/{conversation_id}/messages", s.messageHandler.GetConversationMessages).Methods("GET")
	api.HandleFunc("/conversations/{conversation_id}/messages", s.messageHandler.SendConversationMessage).Methods("POST")
	
	// User routes
	api.HandleFunc("/users/{user_id}", s.userHandler.GetUser).Methods("GET")
	api.HandleFunc("/users/{user_id}/conversations", s.userHandler.GetUserConversations).Methods("GET")
	api.HandleFunc("/users/{user_id}/presence", s.userHandler.UpdateUserPresence).Methods("PUT")
	api.HandleFunc("/users/{user_id}/sessions", s.userHandler.GetUserSessions).Methods("GET")
	
	// Conversation routes
	api.HandleFunc("/conversations", s.conversationHandler.CreateConversation).Methods("POST")
	api.HandleFunc("/conversations/{conversation_id}", s.conversationHandler.GetConversation).Methods("GET")
	api.HandleFunc("/conversations/{conversation_id}", s.conversationHandler.UpdateConversation).Methods("PUT")
	api.HandleFunc("/conversations/{conversation_id}/participants", s.conversationHandler.AddParticipant).Methods("POST")
	api.HandleFunc("/conversations/{conversation_id}/participants/{user_id}", s.conversationHandler.RemoveParticipant).Methods("DELETE")
	api.HandleFunc("/conversations/{conversation_id}/typing", s.conversationHandler.SendTypingIndicator).Methods("POST")
	
	// Analytics routes
	api.HandleFunc("/analytics/metrics", s.analyticsHandler.GetMetrics).Methods("GET")
	api.HandleFunc("/analytics/health", s.analyticsHandler.GetHealthStatus).Methods("GET")
	api.HandleFunc("/analytics/performance", s.analyticsHandler.GetPerformanceMetrics).Methods("GET")
}

// setupMiddleware configures middleware
func (s *APIServer) setupMiddleware() {
	// CORS
	s.router.Use(s.corsMiddleware)
	
	// Request logging
	s.router.Use(s.loggingMiddleware)
	
	// Rate limiting
	s.router.Use(s.rateLimitMiddleware)
	
	// Authentication
	s.router.Use(s.authMiddleware)
	
	// Request ID
	s.router.Use(s.requestIDMiddleware)
	
	// Recovery
	s.router.Use(s.recoveryMiddleware)
}

// Start starts the API server
func (s *APIServer) Start() error {
	s.logger.Info("Starting API server", zap.Int("port", s.port))
	
	server := &http.Server{
		Addr:         ":" + strconv.Itoa(s.port),
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	return server.ListenAndServe()
}

// Stop stops the API server
func (s *APIServer) Stop(ctx context.Context) error {
	s.logger.Info("Stopping API server")
	// Implementation would gracefully shutdown the server
	return nil
}

// handleHealth handles health check requests
func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Version:   "1.0.0",
		Services: map[string]string{
			"websocket": "healthy",
			"kafka":     "healthy",
			"redis":     "healthy",
			"scylladb":  "healthy",
		},
	}
	
	s.respondJSON(w, http.StatusOK, health)
}

// handleReadiness handles readiness check requests
func (s *APIServer) handleReadiness(w http.ResponseWriter, r *http.Request) {
	// Check all dependencies
	ready := true
	checks := make(map[string]bool)
	
	// Implementation would check actual service health
	checks["websocket"] = true
	checks["kafka"] = true
	checks["redis"] = true
	checks["scylladb"] = true
	
	for _, healthy := range checks {
		if !healthy {
			ready = false
			break
		}
	}
	
	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	
	s.respondJSON(w, status, ReadinessResponse{
		Ready:  ready,
		Checks: checks,
	})
}

// handleMetrics handles metrics requests (Prometheus format)
func (s *APIServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	// This would typically serve Prometheus metrics
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("# Metrics endpoint - implement with Prometheus client"))
}

// respondJSON sends a JSON response
func (s *APIServer) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError sends an error response
func (s *APIServer) respondError(w http.ResponseWriter, err *errors.AppError) {
	s.logger.Error("API error", 
		zap.String("error_id", err.ID),
		zap.String("code", string(err.Code)),
		zap.String("message", err.Message),
	)
	
	s.respondJSON(w, err.HTTPStatus, ErrorResponse{
		Error:     err.Code,
		Message:   err.Message,
		ID:        err.ID,
		Timestamp: err.Timestamp,
		Details:   err.Details,
	})
}

// Middleware implementations

func (s *APIServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

func (s *APIServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create response wrapper to capture status code
		wrapper := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(wrapper, r)
		
		duration := time.Since(start)
		
		s.logger.Info("HTTP request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", wrapper.statusCode),
			zap.Duration("duration", duration),
			zap.String("ip", r.RemoteAddr),
			zap.String("user_agent", r.UserAgent()),
		)
	})
}

func (s *APIServer) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Implementation would check rate limits
		// For now, just pass through
		next.ServeHTTP(w, r)
	})
}

func (s *APIServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health endpoints
		if r.URL.Path == "/health" || r.URL.Path == "/ready" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		
		// Implementation would validate JWT tokens
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.respondError(w, errors.UnauthorizedError("Authorization header required"))
			return
		}
		
		// Extract and validate token
		// Set user context
		ctx := context.WithValue(r.Context(), "user_id", "extracted_user_id")
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *APIServer) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), "request_id", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *APIServer) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				s.logger.Error("Panic recovered",
					zap.Any("error", err),
					zap.String("path", r.URL.Path),
				)
				s.respondError(w, errors.InternalError("Internal server error", nil))
			}
		}()
		
		next.ServeHTTP(w, r)
	})
}

// Response types

type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Version   string            `json:"version"`
	Services  map[string]string `json:"services"`
}

type ReadinessResponse struct {
	Ready  bool            `json:"ready"`
	Checks map[string]bool `json:"checks"`
}

type ErrorResponse struct {
	Error     string                 `json:"error"`
	Message   string                 `json:"message"`
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
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

func generateRequestID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
