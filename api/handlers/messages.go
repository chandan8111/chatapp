package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/chatapp/errors"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// MessageHandler handles message-related API requests
type MessageHandler struct {
	messageService interface{}
	logger         *zap.Logger
}

// NewMessageHandler creates a new message handler
func NewMessageHandler(messageService interface{}, logger *zap.Logger) *MessageHandler {
	return &MessageHandler{
		messageService: messageService,
		logger:         logger,
	}
}

// SendMessage handles POST /api/v1/messages
func (h *MessageHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	var request SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.respondError(w, errors.InvalidInputError("request body", err))
		return
	}
	
	// Validate request
	if request.ConversationID == "" {
		h.respondError(w, errors.ValidationError("conversation_id is required"))
		return
	}
	
	if request.SenderID == "" {
		h.respondError(w, errors.ValidationError("sender_id is required"))
		return
	}
	
	if len(request.Content) == 0 {
		h.respondError(w, errors.ValidationError("content is required"))
		return
	}
	
	if len(request.Content) > 8192 {
		h.respondError(w, errors.ValidationError("content exceeds maximum length of 8192 bytes"))
		return
	}
	
	// Mock response - implementation would process through message service
	response := MessageResponse{
		MessageID:      "msg-" + generateMessageID(),
		ConversationID: request.ConversationID,
		SenderID:       request.SenderID,
		Content:        request.Content,
		MessageType:    request.MessageType,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Status:         "sent",
		Metadata:       request.Metadata,
	}
	
	h.logger.Info("Message sent",
		zap.String("message_id", response.MessageID),
		zap.String("conversation_id", response.ConversationID),
		zap.String("sender_id", response.SenderID),
	)
	
	h.respondJSON(w, http.StatusCreated, response)
}

// GetMessage handles GET /api/v1/messages/{message_id}
func (h *MessageHandler) GetMessage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	messageID := vars["message_id"]
	
	if messageID == "" {
		h.respondError(w, errors.ValidationError("message_id is required"))
		return
	}
	
	// Mock response - implementation would fetch from storage
	response := MessageResponse{
		MessageID:      messageID,
		ConversationID: "conv-123",
		SenderID:       "user-456",
		Content:        "Hello, this is a test message",
		MessageType:    "text",
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Status:         "delivered",
		ReadBy: []ReadReceipt{
			{
				UserID:    "user-789",
				ReadAt:    time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339),
			},
		},
	}
	
	h.respondJSON(w, http.StatusOK, response)
}

// UpdateMessageStatus handles PUT /api/v1/messages/{message_id}/status
func (h *MessageHandler) UpdateMessageStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	messageID := vars["message_id"]
	
	if messageID == "" {
		h.respondError(w, errors.ValidationError("message_id is required"))
		return
	}
	
	var request UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.respondError(w, errors.InvalidInputError("request body", err))
		return
	}
	
	// Validate status
	validStatuses := map[string]bool{"sent": true, "delivered": true, "read": true}
	if !validStatuses[request.Status] {
		h.respondError(w, errors.ValidationError("invalid status, must be one of: sent, delivered, read"))
		return
	}
	
	// Mock response - implementation would update through message service
	response := StatusUpdateResponse{
		MessageID: messageID,
		Status:    request.Status,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		UpdatedBy: request.UserID,
	}
	
	h.respondJSON(w, http.StatusOK, response)
}

// GetConversationMessages handles GET /api/v1/conversations/{conversation_id}/messages
func (h *MessageHandler) GetConversationMessages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	conversationID := vars["conversation_id"]
	
	if conversationID == "" {
		h.respondError(w, errors.ValidationError("conversation_id is required"))
		return
	}
	
	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	
	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	
	before := r.URL.Query().Get("before")
	after := r.URL.Query().Get("after")
	
	// Mock response - implementation would fetch from storage with pagination
	messages := make([]MessageResponse, 0, limit)
	for i := 0; i < limit; i++ {
		messages = append(messages, MessageResponse{
			MessageID:      fmt.Sprintf("msg-%d", offset+i+1),
			ConversationID: conversationID,
			SenderID:       fmt.Sprintf("user-%d", (i%5)+1),
			Content:        fmt.Sprintf("Message content %d", offset+i+1),
			MessageType:    "text",
			Timestamp:      time.Now().Add(-time.Duration(i) * time.Minute).UTC().Format(time.RFC3339),
			Status:         "delivered",
		})
	}
	
	response := ConversationMessagesResponse{
		Messages:       messages,
		Count:          len(messages),
		Total:          1000,
		Offset:         offset,
		Limit:          limit,
		HasMore:        offset+limit < 1000,
		ConversationID: conversationID,
	}
	
	h.respondJSON(w, http.StatusOK, response)
}

// SendConversationMessage handles POST /api/v1/conversations/{conversation_id}/messages
func (h *MessageHandler) SendConversationMessage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	conversationID := vars["conversation_id"]
	
	if conversationID == "" {
		h.respondError(w, errors.ValidationError("conversation_id is required"))
		return
	}
	
	var request SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.respondError(w, errors.InvalidInputError("request body", err))
		return
	}
	
	// Set conversation ID from URL
	request.ConversationID = conversationID
	
	// Validate request
	if request.SenderID == "" {
		h.respondError(w, errors.ValidationError("sender_id is required"))
		return
	}
	
	if len(request.Content) == 0 {
		h.respondError(w, errors.ValidationError("content is required"))
		return
	}
	
	// Mock response - implementation would process through message service
	response := MessageResponse{
		MessageID:      "msg-" + generateMessageID(),
		ConversationID: conversationID,
		SenderID:       request.SenderID,
		Content:        request.Content,
		MessageType:    request.MessageType,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Status:         "sent",
		Metadata:       request.Metadata,
	}
	
	h.respondJSON(w, http.StatusCreated, response)
}

// Helper methods

func (h *MessageHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *MessageHandler) respondError(w http.ResponseWriter, err *errors.AppError) {
	h.logger.Error("Message API error",
		zap.String("error_id", err.ID),
		zap.String("code", string(err.Code)),
		zap.String("message", err.Message),
	)
	
	h.respondJSON(w, err.HTTPStatus, ErrorResponse{
		Error:     err.Code,
		Message:   err.Message,
		ID:        err.ID,
		Timestamp: err.Timestamp,
		Details:   err.Details,
	})
}

func generateMessageID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Request/Response types

type SendMessageRequest struct {
	ConversationID string                 `json:"conversation_id,omitempty"`
	SenderID       string                 `json:"sender_id"`
	Content        string                 `json:"content"`
	MessageType    string                 `json:"message_type,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	ReplyTo        string                 `json:"reply_to,omitempty"`
}

type MessageResponse struct {
	MessageID      string                 `json:"message_id"`
	ConversationID string                 `json:"conversation_id"`
	SenderID       string                 `json:"sender_id"`
	Content        string                 `json:"content"`
	MessageType    string                 `json:"message_type"`
	Timestamp      string                 `json:"timestamp"`
	Status         string                 `json:"status"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	ReadBy         []ReadReceipt          `json:"read_by,omitempty"`
	ReplyTo        string                 `json:"reply_to,omitempty"`
}

type ReadReceipt struct {
	UserID string `json:"user_id"`
	ReadAt string `json:"read_at"`
}

type UpdateStatusRequest struct {
	Status string `json:"status"`
	UserID string `json:"user_id"`
}

type StatusUpdateResponse struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
	UpdatedBy string `json:"updated_by"`
}

type ConversationMessagesResponse struct {
	Messages       []MessageResponse `json:"messages"`
	Count          int               `json:"count"`
	Total          int               `json:"total"`
	Offset         int               `json:"offset"`
	Limit          int               `json:"limit"`
	HasMore        bool              `json:"has_more"`
	ConversationID string            `json:"conversation_id"`
}
