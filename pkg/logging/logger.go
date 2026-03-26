package logging

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.Logger with additional functionality
type Logger struct {
	*zap.Logger
	service    string
	version    string
	environment string
}

// LogConfig represents logging configuration
type LogConfig struct {
	Level      string `json:"level"`
	Format     string `json:"format"`
	Output     string `json:"output"`
	Filename   string `json:"filename"`
	MaxSize    int    `json:"max_size"`
	MaxBackups int    `json:"max_backups"`
	MaxAge     int    `json:"max_age"`
	Compress   bool   `json:"compress"`
	Service    string `json:"service"`
	Version    string `json:"version"`
	Environment string `json:"environment"`
}

// NewLogger creates a new logger with the given configuration
func NewLogger(config LogConfig) (*Logger, error) {
	// Parse log level
	level, err := parseLogLevel(config.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	// Create zap config
	var zapConfig zap.Config
	if config.Format == "json" {
		zapConfig = zap.NewProductionConfig()
	} else {
		zapConfig = zap.NewDevelopmentConfig()
	}

	zapConfig.Level = zap.NewAtomicLevelAt(level)
	zapConfig.OutputPaths = []string{config.Output}
	if config.Filename != "" {
		zapConfig.OutputPaths = append(zapConfig.OutputPaths, config.Filename)
	}

	// Configure encoder
	zapConfig.EncoderConfig = configureEncoder(config.Format)

	// Build logger
	logger, err := zapConfig.Build(zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	return &Logger{
		Logger:     logger,
		service:    config.Service,
		version:    config.Version,
		environment: config.Environment,
	}, nil
}

// WithContext adds context information to the logger
func (l *Logger) WithContext(ctx context.Context) *Logger {
	fields := []zap.Field{}

	// Add trace information if available
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		fields = append(fields,
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.String("span_id", span.SpanContext().SpanID().String()),
		)
	}

	// Add request ID from context if available
	if requestID := ctx.Value("request_id"); requestID != nil {
		if id, ok := requestID.(string); ok {
			fields = append(fields, zap.String("request_id", id))
		}
	}

	// Add user ID from context if available
	if userID := ctx.Value("user_id"); userID != nil {
		if id, ok := userID.(string); ok {
			fields = append(fields, zap.String("user_id", id))
		}
	}

	// Add correlation ID from context if available
	if correlationID := ctx.Value("correlation_id"); correlationID != nil {
		if id, ok := correlationID.(string); ok {
			fields = append(fields, zap.String("correlation_id", id))
		}
	}

	return &Logger{
		Logger: l.Logger.With(fields...),
		service: l.service,
		version: l.version,
		environment: l.environment,
	}
}

// WithService adds service information to the logger
func (l *Logger) WithService(service string) *Logger {
	return &Logger{
		Logger: l.Logger.With(zap.String("service", service)),
		service: service,
		version: l.version,
		environment: l.environment,
	}
}

// WithComponent adds component information to the logger
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger: l.Logger.With(zap.String("component", component)),
		service: l.service,
		version: l.version,
		environment: l.environment,
	}
}

// WithVersion adds version information to the logger
func (l *Logger) WithVersion(version string) *Logger {
	return &Logger{
		Logger: l.Logger.With(zap.String("version", version)),
		service: l.service,
		version: version,
		environment: l.environment,
	}
}

// WithFields adds multiple fields to the logger
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}
	
	return &Logger{
		Logger: l.Logger.With(zapFields...),
		service: l.service,
		version: l.version,
		environment: l.environment,
	}
}

// WithField adds a single field to the logger
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{
		Logger: l.Logger.With(zap.Any(key, value)),
		service: l.service,
		version: l.version,
		environment: l.environment,
	}
}

// LogConnection logs connection-related events
func (l *Logger) LogConnection(event string, userID, connectionID string, fields map[string]interface{}) {
	logFields := []zap.Field{
		zap.String("event", event),
		zap.String("user_id", userID),
		zap.String("connection_id", connectionID),
		zap.Time("timestamp", time.Now()),
	}

	for k, v := range fields {
		logFields = append(logFields, zap.Any(k, v))
	}

	l.Info("Connection event", logFields...)
}

// LogMessage logs message-related events
func (l *Logger) LogMessage(event string, messageID, conversationID, senderID string, fields map[string]interface{}) {
	logFields := []zap.Field{
		zap.String("event", event),
		zap.String("message_id", messageID),
		zap.String("conversation_id", conversationID),
		zap.String("sender_id", senderID),
		zap.Time("timestamp", time.Now()),
	}

	for k, v := range fields {
		logFields = append(logFields, zap.Any(k, v))
	}

	l.Info("Message event", logFields...)
}

// LogPerformance logs performance-related events
func (l *Logger) LogPerformance(operation string, duration time.Duration, fields map[string]interface{}) {
	logFields := []zap.Field{
		zap.String("operation", operation),
		zap.Duration("duration", duration),
		zap.Time("timestamp", time.Now()),
	}

	for k, v := range fields {
		logFields = append(logFields, zap.Any(k, v))
	}

	// Log performance metrics with appropriate level
	if duration > 5*time.Second {
		l.Error("Slow operation detected", logFields...)
	} else if duration > 1*time.Second {
		l.Warn("Operation took longer than expected", logFields...)
	} else {
		l.Debug("Performance metric", logFields...)
	}
}

// LogError logs errors with context
func (l *Logger) LogError(err error, message string, fields map[string]interface{}) {
	logFields := []zap.Field{
		zap.NamedError("error", err),
		zap.String("message", message),
		zap.Time("timestamp", time.Now()),
		zap.String("stack_trace", getStackTrace()),
	}

	for k, v := range fields {
		logFields = append(logFields, zap.Any(k, v))
	}

	l.Error("Error occurred", logFields...)
}

// LogSecurity logs security-related events
func (l *Logger) LogSecurity(event string, userID, ip string, fields map[string]interface{}) {
	logFields := []zap.Field{
		zap.String("event", event),
		zap.String("user_id", userID),
		zap.String("ip_address", ip),
		zap.Time("timestamp", time.Now()),
	}

	for k, v := range fields {
		logFields = append(logFields, zap.Any(k, v))
	}

	// Security events are always logged at WARN level or higher
	l.Warn("Security event", logFields...)
}

// LogMetrics logs application metrics
func (l *Logger) LogMetrics(metrics map[string]interface{}) {
	logFields := []zap.Field{
		zap.Time("timestamp", time.Now()),
	}

	for k, v := range metrics {
		logFields = append(logFields, zap.Any(k, v))
	}

	l.Info("Metrics", logFields...)
}

// LogStartup logs application startup information
func (l *Logger) LogStartup(fields map[string]interface{}) {
	logFields := []zap.Field{
		zap.String("event", "startup"),
		zap.Time("timestamp", time.Now()),
		zap.String("go_version", runtime.Version()),
		zap.Int("goroutines", runtime.NumGoroutine()),
		zap.String("service", l.service),
		zap.String("version", l.version),
		zap.String("environment", l.environment),
	}

	for k, v := range fields {
		logFields = append(logFields, zap.Any(k, v))
	}

	l.Info("Application starting", logFields...)
}

// LogShutdown logs application shutdown information
func (l *Logger) LogShutdown(reason string, fields map[string]interface{}) {
	logFields := []zap.Field{
		zap.String("event", "shutdown"),
		zap.String("reason", reason),
		zap.Time("timestamp", time.Now()),
	}

	for k, v := range fields {
		logFields = append(logFields, zap.Any(k, v))
	}

	l.Info("Application shutting down", logFields...)
}

// LogHealth logs health check information
func (l *Logger) LogHealth(status string, checks map[string]bool, fields map[string]interface{}) {
	logFields := []zap.Field{
		zap.String("event", "health_check"),
		zap.String("status", status),
		zap.Time("timestamp", time.Now()),
	}

	for name, healthy := range checks {
		logFields = append(logFields, zap.Bool(fmt.Sprintf("check_%s", name), healthy))
	}

	for k, v := range fields {
		logFields = append(logFields, zap.Any(k, v))
	}

	if status == "healthy" {
		l.Info("Health check", logFields...)
	} else {
		l.Warn("Health check failed", logFields...)
	}
}

// LogRateLimit logs rate limiting events
func (l *Logger) LogRateLimit(userID, endpoint string, limit int, window time.Duration, fields map[string]interface{}) {
	logFields := []zap.Field{
		zap.String("event", "rate_limit"),
		zap.String("user_id", userID),
		zap.String("endpoint", endpoint),
		zap.Int("limit", limit),
		zap.Duration("window", window),
		zap.Time("timestamp", time.Now()),
	}

	for k, v := range fields {
		logFields = append(logFields, zap.Any(k, v))
	}

	l.Warn("Rate limit exceeded", logFields...)
}

// LogCache logs cache-related events
func (l *Logger) LogCache(operation, key string, hit bool, fields map[string]interface{}) {
	logFields := []zap.Field{
		zap.String("event", "cache_operation"),
		zap.String("operation", operation),
		zap.String("key", key),
		zap.Bool("hit", hit),
		zap.Time("timestamp", time.Now()),
	}

	for k, v := range fields {
		logFields = append(logFields, zap.Any(k, v))
	}

	l.Debug("Cache operation", logFields...)
}

// LogDatabase logs database-related events
func (l *Logger) LogDatabase(operation, table string, duration time.Duration, fields map[string]interface{}) {
	logFields := []zap.Field{
		zap.String("event", "database_operation"),
		zap.String("operation", operation),
		zap.String("table", table),
		zap.Duration("duration", duration),
		zap.Time("timestamp", time.Now()),
	}

	for k, v := range fields {
		logFields = append(logFields, zap.Any(k, v))
	}

	// Slow database queries are logged at WARN level
	if duration > 1*time.Second {
		l.Warn("Slow database query", logFields...)
	} else {
		l.Debug("Database operation", logFields...)
	}
}

// LogKafka logs Kafka-related events
func (l *Logger) LogKafka(operation, topic string, fields map[string]interface{}) {
	logFields := []zap.Field{
		zap.String("event", "kafka_operation"),
		zap.String("operation", operation),
		zap.String("topic", topic),
		zap.Time("timestamp", time.Now()),
	}

	for k, v := range fields {
		logFields = append(logFields, zap.Any(k, v))
	}

	l.Info("Kafka operation", logFields...)
}

// LogRedis logs Redis-related events
func (l *Logger) LogRedis(operation string, duration time.Duration, fields map[string]interface{}) {
	logFields := []zap.Field{
		zap.String("event", "redis_operation"),
		zap.String("operation", operation),
		zap.Duration("duration", duration),
		zap.Time("timestamp", time.Now()),
	}

	for k, v := range fields {
		logFields = append(logFields, zap.Any(k, v))
	}

	// Slow Redis operations are logged at WARN level
	if duration > 100*time.Millisecond {
		l.Warn("Slow Redis operation", logFields...)
	} else {
		l.Debug("Redis operation", logFields...)
	}
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

// parseLogLevel parses string log level to zapcore.Level
func parseLogLevel(level string) (zapcore.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "fatal":
		return zapcore.FatalLevel, nil
	case "panic":
		return zapcore.PanicLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("unknown log level: %s", level)
	}
}

// configureEncoder configures the encoder based on format
func configureEncoder(format string) zapcore.EncoderConfig {
	if format == "json" {
		return zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "message",
			StacktraceKey:  "stack_trace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}
	}

	return zapcore.EncoderConfig{
		TimeKey:        "T",
		LevelKey:       "L",
		NameKey:        "N",
		CallerKey:      "C",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

// getStackTrace captures the current stack trace
func getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// ContextKey is used for context values
type ContextKey string

const (
	RequestIDKey    ContextKey = "request_id"
	UserIDKey       ContextKey = "user_id"
	CorrelationIDKey ContextKey = "correlation_id"
	TraceIDKey      ContextKey = "trace_id"
	SpanIDKey       ContextKey = "span_id"
)

// WithRequestID adds request ID to context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithUserID adds user ID to context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// WithCorrelationID adds correlation ID to context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, correlationID)
}

// GetRequestID gets request ID from context
func GetRequestID(ctx context.Context) string {
	if requestID := ctx.Value(RequestIDKey); requestID != nil {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}

// GetUserID gets user ID from context
func GetUserID(ctx context.Context) string {
	if userID := ctx.Value(UserIDKey); userID != nil {
		if id, ok := userID.(string); ok {
			return id
		}
	}
	return ""
}

// GetCorrelationID gets correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if correlationID := ctx.Value(CorrelationIDKey); correlationID != nil {
		if id, ok := correlationID.(string); ok {
			return id
		}
	}
	return ""
}

// Global logger instance
var defaultLogger *Logger

// InitDefaultLogger initializes the default logger
func InitDefaultLogger(config LogConfig) error {
	logger, err := NewLogger(config)
	if err != nil {
		return err
	}
	defaultLogger = logger
	return nil
}

// GetDefaultLogger returns the default logger
func GetDefaultLogger() *Logger {
	if defaultLogger == nil {
		// Create a basic logger if none is initialized
		logger, _ := NewLogger(LogConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		})
		defaultLogger = logger
	}
	return defaultLogger
}

// Convenience functions using the default logger
func Info(message string, fields ...zap.Field) {
	GetDefaultLogger().Info(message, fields...)
}

func Error(message string, fields ...zap.Field) {
	GetDefaultLogger().Error(message, fields...)
}

func Debug(message string, fields ...zap.Field) {
	GetDefaultLogger().Debug(message, fields...)
}

func Warn(message string, fields ...zap.Field) {
	GetDefaultLogger().Warn(message, fields...)
}

func Fatal(message string, fields ...zap.Field) {
	GetDefaultLogger().Fatal(message, fields...)
}

func Panic(message string, fields ...zap.Field) {
	GetDefaultLogger().Panic(message, fields...)
}
