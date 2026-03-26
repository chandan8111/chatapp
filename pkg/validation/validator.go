package validation

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Validator provides input validation functionality
type Validator struct {
	logger *zap.Logger
}

// NewValidator creates a new validator
func NewValidator(logger *zap.Logger) *Validator {
	return &Validator{
		logger: logger,
	}
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

func (v ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", v.Field, v.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return "no validation errors"
	}
	
	var messages []string
	for _, err := range ve {
		messages = append(messages, err.Error())
	}
	
	return strings.Join(messages, "; ")
}

// ValidationRules holds validation rules
type ValidationRules struct {
	MinLength      int           `json:"min_length"`
	MaxLength      int           `json:"max_length"`
	Pattern        string        `json:"pattern"`
	Required       bool          `json:"required"`
	AllowedValues  []string      `json:"allowed_values"`
	MinValue       interface{}   `json:"min_value"`
	MaxValue       interface{}   `json:"max_value"`
	AllowEmpty     bool          `json:"allow_empty"`
	TrimWhitespace bool          `json:"trim_whitespace"`
	CustomFunc     func(string) error `json:"-"`
}

// ValidateUserID validates a user ID
func (v *Validator) ValidateUserID(userID string) error {
	if userID == "" {
		return ValidationError{
			Field:   "user_id",
			Message: "user ID is required",
			Code:    "REQUIRED",
		}
	}
	
	// Check if it's a valid UUID
	if _, err := uuid.Parse(userID); err != nil {
		return ValidationError{
			Field:   "user_id",
			Message: "user ID must be a valid UUID",
			Code:    "INVALID_FORMAT",
		}
	}
	
	return nil
}

// ValidateDeviceID validates a device ID
func (v *Validator) ValidateDeviceID(deviceID string) error {
	if deviceID == "" {
		return ValidationError{
			Field:   "device_id",
			Message: "device ID is required",
			Code:    "REQUIRED",
		}
	}
	
	// Check length
	if len(deviceID) < 3 {
		return ValidationError{
			Field:   "device_id",
			Message: "device ID must be at least 3 characters long",
			Code:    "MIN_LENGTH",
		}
	}
	
	if len(deviceID) > 100 {
		return ValidationError{
			Field:   "device_id",
			Message: "device ID must be less than 100 characters",
			Code:    "MAX_LENGTH",
		}
	}
	
	// Check for valid characters (alphanumeric, hyphen, underscore)
	pattern := regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)
	if !pattern.MatchString(deviceID) {
		return ValidationError{
			Field:   "device_id",
			Message: "device ID can only contain alphanumeric characters, hyphens, and underscores",
			Code:    "INVALID_CHARACTERS",
		}
	}
	
	return nil
}

// ValidateNodeID validates a node ID
func (v *Validator) ValidateNodeID(nodeID string) error {
	if nodeID == "" {
		return ValidationError{
			Field:   "node_id",
			Message: "node ID is required",
			Code:    "REQUIRED",
		}
	}
	
	// Check length
	if len(nodeID) < 3 {
		return ValidationError{
			Field:   "node_id",
			Message: "node ID must be at least 3 characters long",
			Code:    "MIN_LENGTH",
		}
	}
	
	if len(nodeID) > 100 {
		return ValidationError{
			Field:   "node_id",
			Message: "node ID must be less than 100 characters",
			Code:    "MAX_LENGTH",
		}
	}
	
	// Check for valid characters
	pattern := regexp.MustCompile(`^[a-zA-Z0-9\-_.]+$`)
	if !pattern.MatchString(nodeID) {
		return ValidationError{
			Field:   "node_id",
			Message: "node ID can only contain alphanumeric characters, hyphens, underscores, and dots",
			Code:    "INVALID_CHARACTERS",
		}
	}
	
	return nil
}

// ValidateConversationID validates a conversation ID
func (v *Validator) ValidateConversationID(conversationID string) error {
	if conversationID == "" {
		return ValidationError{
			Field:   "conversation_id",
			Message: "conversation ID is required",
			Code:    "REQUIRED",
		}
	}
	
	// Check if it's a valid UUID
	if _, err := uuid.Parse(conversationID); err != nil {
		return ValidationError{
			Field:   "conversation_id",
			Message: "conversation ID must be a valid UUID",
			Code:    "INVALID_FORMAT",
		}
	}
	
	return nil
}

// ValidateMessageContent validates message content
func (v *Validator) ValidateMessageContent(content string, messageType int) error {
	if content == "" {
		return ValidationError{
			Field:   "content",
			Message: "message content cannot be empty",
			Code:    "REQUIRED",
		}
	}
	
	// Check maximum length based on message type
	maxLength := 4000 // Default for text messages
	switch messageType {
	case 0: // text
		maxLength = 4000
	case 1: // image
		maxLength = 1000000 // 1MB for base64 encoded images
	case 2: // file
		maxLength = 50000000 // 50MB for files
	case 3: // voice
		maxLength = 10000000 // 10MB for voice messages
	}
	
	if len(content) > maxLength {
		return ValidationError{
			Field:   "content",
			Message: fmt.Sprintf("message content exceeds maximum length of %d bytes", maxLength),
			Code:    "MAX_LENGTH",
		}
	}
	
	// For text messages, check for potentially malicious content
	if messageType == 0 {
		if err := v.validateTextContent(content); err != nil {
			return ValidationError{
				Field:   "content",
				Message: "message content contains invalid characters or patterns",
				Code:    "INVALID_CONTENT",
			}
		}
	}
	
	return nil
}

// validateTextContent validates text content for security
func (v *Validator) validateTextContent(content string) error {
	// Check for null bytes
	if strings.Contains(content, "\x00") {
		return errors.New("content contains null bytes")
	}
	
	// Check for control characters (except newline, tab, carriage return)
	for _, r := range content {
		if unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r' {
			return errors.New("content contains invalid control characters")
		}
	}
	
	// Check for script injection patterns
	dangerousPatterns := []string{
		"<script",
		"javascript:",
		"vbscript:",
		"onload=",
		"onerror=",
		"onclick=",
		"onmouseover=",
		"onfocus=",
		"onblur=",
		"onchange=",
		"onsubmit=",
	}
	
	lowerContent := strings.ToLower(content)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerContent, pattern) {
			return errors.New("content contains potentially dangerous patterns")
		}
	}
	
	return nil
}

// ValidateMessageType validates message type
func (v *Validator) ValidateMessageType(messageType int) error {
	validTypes := []int{0, 1, 2, 3} // 0: text, 1: image, 2: file, 3: voice
	
	isValid := false
	for _, validType := range validTypes {
		if messageType == validType {
			isValid = true
			break
		}
	}
	
	if !isValid {
		return ValidationError{
			Field:   "message_type",
			Message: "message type must be 0 (text), 1 (image), 2 (file), or 3 (voice)",
			Code:    "INVALID_VALUE",
		}
	}
	
	return nil
}

// ValidateTimestamp validates a timestamp
func (v *Validator) ValidateTimestamp(timestamp int64) error {
	if timestamp <= 0 {
		return ValidationError{
			Field:   "timestamp",
			Message: "timestamp must be positive",
			Code:    "INVALID_VALUE",
		}
	}
	
	// Check if timestamp is not too far in the future (more than 5 minutes)
	now := time.Now().UnixNano()
	futureLimit := now + (5 * time.Minute.Nanoseconds())
	
	if timestamp > futureLimit {
		return ValidationError{
			Field:   "timestamp",
			Message: "timestamp is too far in the future",
			Code:    "FUTURE_TIMESTAMP",
		}
	}
	
	// Check if timestamp is not too far in the past (more than 1 day)
	pastLimit := now - (24 * time.Hour.Nanoseconds())
	
	if timestamp < pastLimit {
		return ValidationError{
			Field:   "timestamp",
			Message: "timestamp is too far in the past",
			Code:    "PAST_TIMESTAMP",
		}
	}
	
	return nil
}

// ValidateMessage validates a complete message
func (v *Validator) ValidateMessage(message *Message) error {
	var errors ValidationErrors
	
	// Validate message ID
	if err := v.ValidateUserID(message.MessageID); err != nil {
		errors = append(errors, err.(ValidationError))
	}
	
	// Validate conversation ID
	if err := v.ValidateConversationID(message.ConversationID); err != nil {
		errors = append(errors, err.(ValidationError))
	}
	
	// Validate sender ID
	if err := v.ValidateUserID(message.SenderID); err != nil {
		errors = append(errors, err.(ValidationError))
	}
	
	// Validate message type
	if err := v.ValidateMessageType(message.MessageType); err != nil {
		errors = append(errors, err.(ValidationError))
	}
	
	// Validate timestamp
	if err := v.ValidateTimestamp(message.Timestamp); err != nil {
		errors = append(errors, err.(ValidationError))
	}
	
	// Validate content
	if err := v.ValidateMessageContent(string(message.Ciphertext), message.MessageType); err != nil {
		errors = append(errors, err.(ValidationError))
	}
	
	if len(errors) > 0 {
		return errors
	}
	
	return nil
}

// ValidateConnectionRequest validates a WebSocket connection request
func (v *Validator) ValidateConnectionRequest(userID, deviceID, nodeID string) error {
	var errors ValidationErrors
	
	// Validate user ID
	if err := v.ValidateUserID(userID); err != nil {
		errors = append(errors, err.(ValidationError))
	}
	
	// Validate device ID
	if err := v.ValidateDeviceID(deviceID); err != nil {
		errors = append(errors, err.(ValidationError))
	}
	
	// Validate node ID
	if err := v.ValidateNodeID(nodeID); err != nil {
		errors = append(errors, err.(ValidationError))
	}
	
	if len(errors) > 0 {
		return errors
	}
	
	return nil
}

// ValidateAPIRequest validates an API request
func (v *Validator) ValidateAPIRequest(method, path string, headers map[string]string, body interface{}) error {
	// Validate HTTP method
	validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
	isValidMethod := false
	for _, validMethod := range validMethods {
		if method == validMethod {
			isValidMethod = true
			break
		}
	}
	
	if !isValidMethod {
		return ValidationError{
			Field:   "method",
			Message: "invalid HTTP method",
			Code:    "INVALID_METHOD",
		}
	}
	
	// Validate path
	if path == "" {
		return ValidationError{
			Field:   "path",
			Message: "path is required",
			Code:    "REQUIRED",
		}
	}
	
	// Check path length
	if len(path) > 1000 {
		return ValidationError{
			Field:   "path",
			Message: "path is too long",
			Code:    "MAX_LENGTH",
		}
	}
	
	// Validate headers
	for key, value := range headers {
		if err := v.validateHeader(key, value); err != nil {
			return ValidationError{
				Field:   fmt.Sprintf("header_%s", key),
				Message: fmt.Sprintf("invalid header value: %v", err),
				Code:    "INVALID_HEADER",
			}
		}
	}
	
	return nil
}

// validateHeader validates a header key and value
func (v *Validator) validateHeader(key, value string) error {
	// Check header key length
	if len(key) > 100 {
		return errors.New("header key too long")
	}
	
	// Check header value length
	if len(value) > 1000 {
		return errors.New("header value too long")
	}
	
	// Check for dangerous characters in header values
	if strings.Contains(value, "\n") || strings.Contains(value, "\r") {
		return errors.New("header value contains line breaks")
	}
	
	return nil
}

// ValidateLimit validates pagination limit
func (v *Validator) ValidateLimit(limit int) error {
	if limit <= 0 {
		return ValidationError{
			Field:   "limit",
			Message: "limit must be positive",
			Code:    "INVALID_VALUE",
		}
	}
	
	if limit > 1000 {
		return ValidationError{
			Field:   "limit",
			Message: "limit cannot exceed 1000",
			Code:    "MAX_VALUE",
		}
	}
	
	return nil
}

// ValidateOffset validates pagination offset
func (v *Validator) ValidateOffset(offset int) error {
	if offset < 0 {
		return ValidationError{
			Field:   "offset",
			Message: "offset cannot be negative",
			Code:    "INVALID_VALUE",
		}
	}
	
	if offset > 100000 {
		return ValidationError{
			Field:   "offset",
			Message: "offset cannot exceed 100000",
			Code:    "MAX_VALUE",
		}
	}
	
	return nil
}

// ValidateSearchQuery validates a search query
func (v *Validator) ValidateSearchQuery(query string) error {
	// Check length
	if len(query) > 500 {
		return ValidationError{
			Field:   "query",
			Message: "search query is too long",
			Code:    "MAX_LENGTH",
		}
	}
	
	// Check for SQL injection patterns
	sqlPatterns := []string{
		"SELECT ",
		"INSERT ",
		"UPDATE ",
		"DELETE ",
		"DROP ",
		"UNION ",
		"EXEC ",
		"SCRIPT ",
		"'",
		"\"",
		";",
		"--",
		"/*",
		"*/",
	}
	
	upperQuery := strings.ToUpper(query)
	for _, pattern := range sqlPatterns {
		if strings.Contains(upperQuery, pattern) {
			return ValidationError{
				Field:   "query",
				Message: "search query contains invalid characters",
				Code:    "INVALID_CONTENT",
			}
		}
	}
	
	return nil
}

// Message represents a message for validation
type Message struct {
	MessageID      string
	ConversationID string
	SenderID       string
	MessageType    int
	Timestamp      int64
	Ciphertext     []byte
}

// SanitizeInput sanitizes user input
func (v *Validator) SanitizeInput(input string) string {
	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")
	
	// Trim whitespace
	input = strings.TrimSpace(input)
	
	// Remove control characters (except newline, tab, carriage return)
	var result []rune
	for _, r := range input {
		if !unicode.IsControl(r) || r == '\n' || r == '\t' || r == '\r' {
			result = append(result, r)
		}
	}
	
	return string(result)
}

// ValidateAndSanitize validates and sanitizes input
func (v *Validator) ValidateAndSanitize(input string, rules ValidationRules) (string, error) {
	// Trim whitespace if required
	if rules.TrimWhitespace {
		input = strings.TrimSpace(input)
	}
	
	// Check if required
	if rules.Required && input == "" {
		return "", ValidationError{
			Field:   "input",
			Message: "input is required",
			Code:    "REQUIRED",
		}
	}
	
	// Allow empty check
	if !rules.AllowEmpty && input == "" && !rules.Required {
		return "", nil
	}
	
	// Check length
	if rules.MinLength > 0 && len(input) < rules.MinLength {
		return "", ValidationError{
			Field:   "input",
			Message: fmt.Sprintf("input must be at least %d characters long", rules.MinLength),
			Code:    "MIN_LENGTH",
		}
	}
	
	if rules.MaxLength > 0 && len(input) > rules.MaxLength {
		return "", ValidationError{
			Field:   "input",
			Message: fmt.Sprintf("input must be less than %d characters", rules.MaxLength),
			Code:    "MAX_LENGTH",
		}
	}
	
	// Check pattern
	if rules.Pattern != "" {
		pattern := regexp.MustCompile(rules.Pattern)
		if !pattern.MatchString(input) {
			return "", ValidationError{
				Field:   "input",
				Message: "input does not match required pattern",
				Code:    "INVALID_FORMAT",
			}
		}
	}
	
	// Check allowed values
	if len(rules.AllowedValues) > 0 {
		isAllowed := false
		for _, allowed := range rules.AllowedValues {
			if input == allowed {
				isAllowed = true
				break
			}
		}
		
		if !isAllowed {
			return "", ValidationError{
				Field:   "input",
				Message: "input value is not allowed",
				Code:    "INVALID_VALUE",
			}
		}
	}
	
	// Custom validation
	if rules.CustomFunc != nil {
		if err := rules.CustomFunc(input); err != nil {
			return "", ValidationError{
				Field:   "input",
				Message: err.Error(),
				Code:    "CUSTOM_VALIDATION",
			}
		}
	}
	
	// Sanitize input
	return v.SanitizeInput(input), nil
}
