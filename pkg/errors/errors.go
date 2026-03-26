package errors

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ErrorCode represents different types of errors in the system
type ErrorCode string

const (
	// Validation errors
	ErrCodeValidation     ErrorCode = "VALIDATION_ERROR"
	ErrCodeInvalidInput   ErrorCode = "INVALID_INPUT"
	ErrCodeMissingField   ErrorCode = "MISSING_FIELD"
	ErrCodeInvalidFormat  ErrorCode = "INVALID_FORMAT"

	// Authentication/Authorization errors
	ErrCodeUnauthorized    ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden       ErrorCode = "FORBIDDEN"
	ErrCodeInvalidToken    ErrorCode = "INVALID_TOKEN"
	ErrCodeExpiredToken    ErrorCode = "EXPIRED_TOKEN"
	ErrCodeInvalidAuth     ErrorCode = "INVALID_AUTH"

	// Business logic errors
	ErrCodeUserNotFound     ErrorCode = "USER_NOT_FOUND"
	ErrCodeConversationNotFound ErrorCode = "CONVERSATION_NOT_FOUND"
	ErrCodeMessageNotFound   ErrorCode = "MESSAGE_NOT_FOUND"
	ErrCodeDuplicateUser     ErrorCode = "DUPLICATE_USER"
	ErrCodeDuplicateMessage  ErrorCode = "DUPLICATE_MESSAGE"
	ErrCodeInvalidOperation  ErrorCode = "INVALID_OPERATION"

	// System errors
	ErrCodeInternalError     ErrorCode = "INTERNAL_ERROR"
	ErrCodeDatabaseError     ErrorCode = "DATABASE_ERROR"
	ErrCodeNetworkError      ErrorCode = "NETWORK_ERROR"
	ErrCodeTimeoutError      ErrorCode = "TIMEOUT_ERROR"
	ErrCodeRateLimitExceeded ErrorCode = "RATE_LIMIT_EXCEEDED"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"

	// Connection errors
	ErrCodeConnectionFailed  ErrorCode = "CONNECTION_FAILED"
	ErrCodeConnectionLimit   ErrorCode = "CONNECTION_LIMIT"
	ErrCodeConnectionLost    ErrorCode = "CONNECTION_LOST"
	ErrCodeInvalidMessage    ErrorCode = "INVALID_MESSAGE"

	// Security errors
	ErrCodeEncryptionFailed  ErrorCode = "ENCRYPTION_FAILED"
	ErrCodeDecryptionFailed  ErrorCode = "DECRYPTION_FAILED"
	ErrCodeInvalidSignature  ErrorCode = "INVALID_SIGNATURE"
	ErrCodeKeyExchangeFailed ErrorCode = "KEY_EXCHANGE_FAILED"
)

// AppError represents a structured application error
type AppError struct {
	ID          string                 `json:"id"`
	Code        ErrorCode              `json:"code"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Cause       error                  `json:"-"`
	HTTPStatus  int                    `json:"-"`
	Retryable   bool                   `json:"retryable"`
	Timestamp   time.Time              `json:"timestamp"`
	StackTrace  string                 `json:"stack_trace,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
	Service     string                 `json:"service,omitempty"`
	Component   string                 `json:"component,omitempty"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause
func (e *AppError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches the target
func (e *AppError) Is(target error) bool {
	if t, ok := target.(*AppError); ok {
		return e.Code == t.Code
	}
	return false
}

// NewAppError creates a new application error
func NewAppError(code ErrorCode, message string) *AppError {
	return &AppError{
		ID:         uuid.New().String(),
		Code:       code,
		Message:    message,
		HTTPStatus: getHTTPStatus(code),
		Retryable:  isRetryable(code),
		Timestamp:  time.Now(),
		StackTrace: getStackTrace(),
	}
}

// WithCause adds a cause to the error
func (e *AppError) WithCause(cause error) *AppError {
	e.Cause = cause
	return e
}

// WithDetails adds details to the error
func (e *AppError) WithDetails(key string, value interface{}) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithUserID adds user ID to the error
func (e *AppError) WithUserID(userID string) *AppError {
	e.UserID = userID
	return e
}

// WithRequestID adds request ID to the error
func (e *AppError) WithRequestID(requestID string) *AppError {
	e.RequestID = requestID
	return e
}

// WithService adds service name to the error
func (e *AppError) WithService(service string) *AppError {
	e.Service = service
	return e
}

// WithComponent adds component name to the error
func (e *AppError) WithComponent(component string) *AppError {
	e.Component = component
	return e
}

// WithHTTPStatus overrides the default HTTP status
func (e *AppError) WithHTTPStatus(status int) *AppError {
	e.HTTPStatus = status
	return e
}

// WithRetryable sets whether the error is retryable
func (e *AppError) WithRetryable(retryable bool) *AppError {
	e.Retryable = retryable
	return e
}

// ValidationError creates a validation error
func ValidationError(message string) *AppError {
	return NewAppError(ErrCodeValidation, message)
}

// InvalidInputError creates an invalid input error
func InvalidInputError(field string, value interface{}) *AppError {
	return NewAppError(ErrCodeInvalidInput, fmt.Sprintf("Invalid input for field %s", field)).
		WithDetails("field", field).
		WithDetails("value", value)
}

// UnauthorizedError creates an unauthorized error
func UnauthorizedError(message string) *AppError {
	return NewAppError(ErrCodeUnauthorized, message)
}

// ForbiddenError creates a forbidden error
func ForbiddenError(message string) *AppError {
	return NewAppError(ErrCodeForbidden, message)
}

// UserNotFoundError creates a user not found error
func UserNotFoundError(userID string) *AppError {
	return NewAppError(ErrCodeUserNotFound, "User not found").
		WithDetails("user_id", userID)
}

// ConversationNotFoundError creates a conversation not found error
func ConversationNotFoundError(conversationID string) *AppError {
	return NewAppError(ErrCodeConversationNotFound, "Conversation not found").
		WithDetails("conversation_id", conversationID)
}

// MessageNotFoundError creates a message not found error
func MessageNotFoundError(messageID string) *AppError {
	return NewAppError(ErrCodeMessageNotFound, "Message not found").
		WithDetails("message_id", messageID)
}

// InternalError creates an internal server error
func InternalError(message string, cause error) *AppError {
	return NewAppError(ErrCodeInternalError, message).WithCause(cause)
}

// DatabaseError creates a database error
func DatabaseError(message string, cause error) *AppError {
	return NewAppError(ErrCodeDatabaseError, message).WithCause(cause)
}

// NetworkError creates a network error
func NetworkError(message string, cause error) *AppError {
	return NewAppError(ErrCodeNetworkError, message).WithCause(cause)
}

// TimeoutError creates a timeout error
func TimeoutError(operation string, timeout time.Duration) *AppError {
	return NewAppError(ErrCodeTimeoutError, fmt.Sprintf("Operation %s timed out after %v", operation, timeout)).
		WithDetails("operation", operation).
		WithDetails("timeout", timeout.String())
}

// RateLimitExceededError creates a rate limit exceeded error
func RateLimitExceededError(limit int, window time.Duration) *AppError {
	return NewAppError(ErrCodeRateLimitExceeded, "Rate limit exceeded").
		WithDetails("limit", limit).
		WithDetails("window", window.String())
}

// ConnectionFailedError creates a connection failed error
func ConnectionFailedError(service string, cause error) *AppError {
	return NewAppError(ErrCodeConnectionFailed, fmt.Sprintf("Failed to connect to %s", service)).
		WithCause(cause).
		WithDetails("service", service)
}

// ConnectionLimitError creates a connection limit error
func ConnectionLimitError(current, max int) *AppError {
	return NewAppError(ErrCodeConnectionLimit, "Connection limit exceeded").
		WithDetails("current", current).
		WithDetails("max", max)
}

// EncryptionFailedError creates an encryption failed error
func EncryptionFailedError(cause error) *AppError {
	return NewAppError(ErrCodeEncryptionFailed, "Encryption failed").WithCause(cause)
}

// DecryptionFailedError creates a decryption failed error
func DecryptionFailedError(cause error) *AppError {
	return NewAppError(ErrCodeDecryptionFailed, "Decryption failed").WithCause(cause)
}

// getHTTPStatus returns the appropriate HTTP status code for an error code
func getHTTPStatus(code ErrorCode) int {
	switch code {
	case ErrCodeValidation, ErrCodeInvalidInput, ErrCodeMissingField, ErrCodeInvalidFormat:
		return http.StatusBadRequest
	case ErrCodeUnauthorized, ErrCodeInvalidToken, ErrCodeExpiredToken, ErrCodeInvalidAuth:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeUserNotFound, ErrCodeConversationNotFound, ErrCodeMessageNotFound:
		return http.StatusNotFound
	case ErrCodeDuplicateUser, ErrCodeDuplicateMessage, ErrCodeInvalidOperation:
		return http.StatusConflict
	case ErrCodeRateLimitExceeded:
		return http.StatusTooManyRequests
	case ErrCodeServiceUnavailable:
		return http.StatusServiceUnavailable
	case ErrCodeConnectionLimit:
		return http.StatusServiceUnavailable
	case ErrCodeTimeoutError:
		return http.StatusRequestTimeout
	default:
		return http.StatusInternalServerError
	}
}

// isRetryable returns whether an error is retryable
func isRetryable(code ErrorCode) bool {
	switch code {
	case ErrCodeDatabaseError, ErrCodeNetworkError, ErrCodeTimeoutError, ErrCodeServiceUnavailable:
		return true
	case ErrRateLimitExceeded, ErrCodeConnectionFailed:
		return true
	default:
		return false
	}
}

// getStackTrace captures the current stack trace
func getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// ErrorHandler handles errors in a consistent way
type ErrorHandler struct {
	logger *zap.Logger
	service string
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(logger *zap.Logger, service string) *ErrorHandler {
	return &ErrorHandler{
		logger:  logger,
		service: service,
	}
}

// Handle logs the error and returns appropriate response
func (h *ErrorHandler) Handle(err error, requestID, userID string) *AppError {
	var appErr *AppError
	
	// Convert to AppError if needed
	if errAsAppErr, ok := err.(*AppError); ok {
		appErr = errAsAppErr
	} else {
		appErr = InternalError("Internal server error", err)
	}

	// Add context
	appErr = appErr.
		WithRequestID(requestID).
		WithUserID(userID).
		WithService(h.service)

	// Log the error
	h.logError(appErr)

	return appErr
}

// HandleWithRetry handles errors with retry logic
func (h *ErrorHandler) HandleWithRetry(err error, requestID, userID string, maxRetries int) (*AppError, bool) {
	appErr := h.Handle(err, requestID, userID)
	
	if appErr.Retryable && maxRetries > 0 {
		appErr = appErr.WithDetails("retry_attempts", maxRetries)
		return appErr, true
	}
	
	return appErr, false
}

// logError logs the error with appropriate level
func (h *ErrorHandler) logError(err *AppError) {
	fields := []zap.Field{
		zap.String("error_id", err.ID),
		zap.String("error_code", string(err.Code)),
		zap.String("message", err.Message),
		zap.Time("timestamp", err.Timestamp),
		zap.Bool("retryable", err.Retryable),
	}

	if err.RequestID != "" {
		fields = append(fields, zap.String("request_id", err.RequestID))
	}
	if err.UserID != "" {
		fields = append(fields, zap.String("user_id", err.UserID))
	}
	if err.Service != "" {
		fields = append(fields, zap.String("service", err.Service))
	}
	if err.Component != "" {
		fields = append(fields, zap.String("component", err.Component))
	}
	if err.Cause != nil {
		fields = append(fields, zap.NamedError("cause", err.Cause))
	}
	if len(err.Details) > 0 {
		fields = append(fields, zap.Any("details", err.Details))
	}

	// Log with appropriate level based on error type
	switch err.Code {
	case ErrCodeValidation, ErrCodeInvalidInput, ErrCodeMissingField, ErrCodeInvalidFormat,
		 ErrCodeUnauthorized, ErrCodeForbidden, ErrCodeUserNotFound, ErrCodeConversationNotFound,
		 ErrCodeMessageNotFound, ErrCodeDuplicateUser, ErrCodeDuplicateMessage:
		h.logger.Warn("Application error", fields...)
	case ErrCodeRateLimitExceeded, ErrCodeConnectionLimit:
		h.logger.Info("Rate limit or connection limit exceeded", fields...)
	default:
		if err.HTTPStatus >= 500 {
			h.logger.Error("Server error", fields...)
		} else {
			h.logger.Warn("Client error", fields...)
		}
	}
}

// RecoveryMiddleware recovers from panics and converts them to errors
func (h *ErrorHandler) RecoveryMiddleware() func(interface{}) *AppError {
	return func(panicValue interface{}) *AppError {
		var err error
		if panicErr, ok := panicValue.(error); ok {
			err = panicErr
		} else {
			err = fmt.Errorf("panic: %v", panicValue)
		}

		appErr := InternalError("Internal server error", err).
			WithDetails("panic", true).
			WithDetails("panic_value", panicValue)

		h.logger.Error("Panic recovered",
			zap.String("error_id", appErr.ID),
			zap.NamedError("error", err),
			zap.String("stack_trace", getStackTrace()),
		)

		return appErr
	}
}

// ErrorMetrics tracks error metrics
type ErrorMetrics struct {
	errorCounts map[ErrorCode]int64
	logger      *zap.Logger
}

// NewErrorMetrics creates a new error metrics tracker
func NewErrorMetrics(logger *zap.Logger) *ErrorMetrics {
	return &ErrorMetrics{
		errorCounts: make(map[ErrorCode]int64),
		logger:      logger,
	}
}

// Record records an error
func (m *ErrorMetrics) Record(err *AppError) {
	m.errorCounts[err.Code]++
	
	// Log metrics every 100 errors
	if m.errorCounts[err.Code]%100 == 0 {
		m.logger.Info("Error metrics",
			zap.String("error_code", string(err.Code)),
			zap.Int64("count", m.errorCounts[err.Code]),
		)
	}
}

// GetCounts returns error counts
func (m *ErrorMetrics) GetCounts() map[ErrorCode]int64 {
	return m.errorCounts
}

// Reset resets all counts
func (m *ErrorMetrics) Reset() {
	m.errorCounts = make(map[ErrorCode]int64)
}

// WrapError wraps any error into an AppError with context
func WrapError(err error, code ErrorCode, message string) *AppError {
	if err == nil {
		return nil
	}
	
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	
	return NewAppError(code, message).WithCause(err)
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Retryable
	}
	return false
}

// GetErrorCode extracts the error code from an error
func GetErrorCode(err error) ErrorCode {
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	return ErrCodeInternalError
}

// GetHTTPStatus extracts the HTTP status from an error
func GetHTTPStatus(err error) int {
	if appErr, ok := err.(*AppError); ok {
		return appErr.HTTPStatus
	}
	return http.StatusInternalServerError
}
