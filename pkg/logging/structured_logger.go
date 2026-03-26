package logging

import (
	"context"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger wraps zap logger with additional functionality
type Logger struct {
	*zap.Logger
	serviceName string
	version     string
}

// Config holds logging configuration
type Config struct {
	Level          string `yaml:"level"`
	Format         string `yaml:"format"` // json or console
	Output         string `yaml:"output"`
	Filename       string `yaml:"filename"`
	MaxSize        int    `yaml:"max_size"`        // MB
	MaxBackups     int    `yaml:"max_backups"`
	MaxAge         int    `yaml:"max_age"`         // days
	Compress       bool   `yaml:"compress"`
	ServiceName    string `yaml:"service_name"`
	Version        string `yaml:"version"`
	EnableCaller   bool   `yaml:"enable_caller"`
	EnableStacktrace bool `yaml:"enable_stacktrace"`
}

// NewLogger creates a new structured logger
func NewLogger(config Config) (*Logger, error) {
	// Parse log level
	level, err := zapcore.ParseLevel(config.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	// Create encoder
	var encoder zapcore.Encoder
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	if config.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Create writer
	var writer zapcore.WriteSyncer
	if config.Filename != "" {
		writer = zapcore.AddSync(&lumberjack.Logger{
			Filename:   config.Filename,
			MaxSize:    config.MaxSize,
			MaxBackups: config.MaxBackups,
			MaxAge:     config.MaxAge,
			Compress:   config.Compress,
		})
	} else {
		writer = zapcore.AddSync(os.Stdout)
	}

	// Create core
	core := zapcore.NewCore(encoder, writer, level)

	// Create logger options
	options := []zap.Option{}
	
	if config.EnableCaller {
		options = append(options, zap.AddCaller())
	}
	
	if config.EnableStacktrace {
		options = append(options, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	// Add service context
	options = append(options, zap.Fields(
		zap.String("service", config.ServiceName),
		zap.String("version", config.Version),
	))

	// Create logger
	zapLogger := zap.New(core, options...)

	logger := &Logger{
		Logger:       zapLogger,
		serviceName:  config.ServiceName,
		version:      config.Version,
	}

	return logger, nil
}

// WithContext adds context fields to the logger
func (l *Logger) WithContext(ctx context.Context) *zap.Logger {
	fields := []zap.Field{}

	// Add trace ID if present
	if traceID := ctx.Value("trace_id"); traceID != nil {
		fields = append(fields, zap.String("trace_id", traceID.(string)))
	}

	// Add user ID if present
	if userID := ctx.Value("user_id"); userID != nil {
		fields = append(fields, zap.String("user_id", userID.(string)))
	}

	// Add request ID if present
	if requestID := ctx.Value("request_id"); requestID != nil {
		fields = append(fields, zap.String("request_id", requestID.(string)))
	}

	// Add correlation ID if present
	if correlationID := ctx.Value("correlation_id"); correlationID != nil {
		fields = append(fields, zap.String("correlation_id", correlationID.(string)))
	}

	return l.With(fields...)
}

// WithService adds service context to the logger
func (l *Logger) WithService(service string) *zap.Logger {
	return l.With(zap.String("component", service))
}

// WithComponent adds component context to the logger
func (l *Logger) WithComponent(component string) *zap.Logger {
	return l.With(zap.String("component", component))
}

// WithModule adds module context to the logger
func (l *Logger) WithModule(module string) *zap.Logger {
	return l.With(zap.String("module", module))
}

// WithError adds error context to the logger
func (l *Logger) WithError(err error) *zap.Logger {
	if err == nil {
		return l.Logger
	}
	return l.Logger.With(zap.Error(err))
}

// WithDuration adds duration context to the logger
func (l *Logger) WithDuration(duration time.Duration) *zap.Logger {
	return l.With(zap.Duration("duration", duration))
}

// WithFields adds multiple fields to the logger
func (l *Logger) WithFields(fields map[string]interface{}) *zap.Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	
	for key, value := range fields {
		switch v := value.(type) {
		case string:
			zapFields = append(zapFields, zap.String(key, v))
		case int:
			zapFields = append(zapFields, zap.Int(key, v))
		case int64:
			zapFields = append(zapFields, zap.Int64(key, v))
		case float64:
			zapFields = append(zapFields, zap.Float64(key, v))
		case bool:
			zapFields = append(zapFields, zap.Bool(key, v))
		case time.Time:
			zapFields = append(zapFields, zap.Time(key, v))
		case time.Duration:
			zapFields = append(zapFields, zap.Duration(key, v))
		case error:
			zapFields = append(zapFields, zap.NamedError(key, v))
		default:
			zapFields = append(zapFields, zap.Any(key, v))
		}
	}
	
	return l.With(zapFields...)
}

// LogConnection logs connection events
func (l *Logger) LogConnection(event string, userID, deviceID, nodeID string, fields map[string]interface{}) {
	allFields := map[string]interface{}{
		"event":     event,
		"user_id":   userID,
		"device_id": deviceID,
		"node_id":   nodeID,
	}
	
	// Merge additional fields
	for k, v := range fields {
		allFields[k] = v
	}
	
	l.WithFields(allFields).Info("Connection event")
}

// LogMessage logs message events
func (l *Logger) LogMessage(event string, messageID, conversationID, senderID string, fields map[string]interface{}) {
	allFields := map[string]interface{}{
		"event":           event,
		"message_id":      messageID,
		"conversation_id": conversationID,
		"sender_id":       senderID,
	}
	
	// Merge additional fields
	for k, v := range fields {
		allFields[k] = v
	}
	
	l.WithFields(allFields).Info("Message event")
}

// LogError logs error events with context
func (l *Logger) LogError(operation string, err error, fields map[string]interface{}) {
	allFields := map[string]interface{}{
		"operation": operation,
	}
	
	// Merge additional fields
	for k, v := range fields {
		allFields[k] = v
	}
	
	l.WithFields(allFields).Error("Operation failed", zap.Error(err))
}

// LogPerformance logs performance metrics
func (l *Logger) LogPerformance(operation string, duration time.Duration, fields map[string]interface{}) {
	allFields := map[string]interface{}{
		"operation": operation,
		"duration":  duration.Milliseconds(),
	}
	
	// Merge additional fields
	for k, v := range fields {
		allFields[k] = v
	}
	
	// Log as warning if operation is slow
	if duration > 1*time.Second {
		l.WithFields(allFields).Warn("Slow operation detected")
	} else {
		l.WithFields(allFields).Debug("Performance metric")
	}
}

// LogSecurity logs security events
func (l *Logger) LogSecurity(event string, severity string, fields map[string]interface{}) {
	allFields := map[string]interface{}{
		"event":    event,
		"severity": severity,
	}
	
	// Merge additional fields
	for k, v := range fields {
		allFields[k] = v
	}
	
	// Always log security events at warning level or higher
	switch severity {
	case "critical", "high":
		l.WithFields(allFields).Error("Security event")
	case "medium":
		l.WithFields(allFields).Warn("Security event")
	default:
		l.WithFields(allFields).Info("Security event")
	}
}

// LogBusiness logs business events
func (l *Logger) LogBusiness(event string, fields map[string]interface{}) {
	allFields := map[string]interface{}{
		"event": event,
	}
	
	// Merge additional fields
	for k, v := range fields {
		allFields[k] = v
	}
	
	l.WithFields(allFields).Info("Business event")
}

// LogSystem logs system events
func (l *Logger) LogSystem(event string, fields map[string]interface{}) {
	allFields := map[string]interface{}{
		"event": event,
	}
	
	// Merge additional fields
	for k, v := range fields {
		allFields[k] = v
	}
	
	l.WithFields(allFields).Info("System event")
}

// RequestContext holds request context for logging
type RequestContext struct {
	TraceID       string
	RequestID     string
	UserID        string
	CorrelationID string
	Method        string
	Path          string
	RemoteAddr    string
	UserAgent     string
}

// WithRequestContext adds request context to the logger
func (l *Logger) WithRequestContext(req RequestContext) *zap.Logger {
	fields := []zap.Field{}
	
	if req.TraceID != "" {
		fields = append(fields, zap.String("trace_id", req.TraceID))
	}
	
	if req.RequestID != "" {
		fields = append(fields, zap.String("request_id", req.RequestID))
	}
	
	if req.UserID != "" {
		fields = append(fields, zap.String("user_id", req.UserID))
	}
	
	if req.CorrelationID != "" {
		fields = append(fields, zap.String("correlation_id", req.CorrelationID))
	}
	
	if req.Method != "" {
		fields = append(fields, zap.String("method", req.Method))
	}
	
	if req.Path != "" {
		fields = append(fields, zap.String("path", req.Path))
	}
	
	if req.RemoteAddr != "" {
		fields = append(fields, zap.String("remote_addr", req.RemoteAddr))
	}
	
	if req.UserAgent != "" {
		fields = append(fields, zap.String("user_agent", req.UserAgent))
	}
	
	return l.With(fields...)
}

// AuditLogger provides audit logging functionality
type AuditLogger struct {
	logger *Logger
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logger *Logger) *AuditLogger {
	return &AuditLogger{
		logger: logger,
	}
}

// LogUserAction logs user actions for audit purposes
func (a *AuditLogger) LogUserAction(userID, action, resource string, fields map[string]interface{}) {
	allFields := map[string]interface{}{
		"user_id":  userID,
		"action":   action,
		"resource": resource,
		"event":    "user_action",
	}
	
	// Merge additional fields
	for k, v := range fields {
		allFields[k] = v
	}
	
	a.logger.WithFields(allFields).Info("Audit: User action")
}

// LogSystemChange logs system changes for audit purposes
func (a *AuditLogger) LogSystemChange(change, actor string, fields map[string]interface{}) {
	allFields := map[string]interface{}{
		"change": change,
		"actor":  actor,
		"event":  "system_change",
	}
	
	// Merge additional fields
	for k, v := range fields {
		allFields[k] = v
	}
	
	a.logger.WithFields(allFields).Info("Audit: System change")
}

// LogDataAccess logs data access for audit purposes
func (a *AuditLogger) LogDataAccess(userID, dataType, operation string, fields map[string]interface{}) {
	allFields := map[string]interface{}{
		"user_id":    userID,
		"data_type":  dataType,
		"operation":  operation,
		"event":      "data_access",
	}
	
	// Merge additional fields
	for k, v := range fields {
		allFields[k] = v
	}
	
	a.logger.WithFields(allFields).Info("Audit: Data access")
}

// LogSecurityIncident logs security incidents for audit purposes
func (a *AuditLogger) LogSecurityIncident(incident, severity, description string, fields map[string]interface{}) {
	allFields := map[string]interface{}{
		"incident":    incident,
		"severity":    severity,
		"description": description,
		"event":       "security_incident",
	}
	
	// Merge additional fields
	for k, v := range fields {
		allFields[k] = v
	}
	
	a.logger.WithFields(allFields).Error("Audit: Security incident")
}
