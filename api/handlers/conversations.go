package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/chatapp/errors"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// ConversationHandler handles conversation-related API requests
type ConversationHandler struct {
	conversationService interface{}
	logger              *zap.Logger
}

// NewConversationHandler creates a new conversation handler
func NewConversationHandler(conversationService interface{}, logger *zap.Logger) *ConversationHandler {
	return &ConversationHandler{
		conversationService: conversationService,
		logger:              logger,
	}
}

// CreateConversation handles POST /api/v1/conversations
func (h *ConversationHandler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	var request CreateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.respondError(w, errors.InvalidInputError("request body", err))
		return
	}
	
	// Validate request
	if request.Name == "" {
		h.respondError(w, errors.ValidationError("name is required"))
		return
	}
	
	if len(request.Participants) == 0 {
		h.respondError(w, errors.ValidationError("at least one participant is required"))
		return
	}
	
	if len(request.Participants) > 100 {
		h.respondError(w, errors.ValidationError("maximum 100 participants allowed"))
		return
	}
	
	// Mock response - implementation would create through conversation service
	response := ConversationResponse{
		ConversationID: "conv-" + generateConversationID(),
		Name:           request.Name,
		Description:    request.Description,
		Type:           request.Type,
		CreatedBy:      request.CreatedBy,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		Participants:   request.Participants,
		MessageCount:   0,
		IsPublic:       request.IsPublic,
		Metadata:       request.Metadata,
	}
	
	h.logger.Info("Conversation created",
		zap.String("conversation_id", response.ConversationID),
		zap.String("name", response.Name),
		zap.Int("participants", len(request.Participants)),
	)
	
	h.respondJSON(w, http.StatusCreated, response)
}

// GetConversation handles GET /api/v1/conversations/{conversation_id}
func (h *ConversationHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	conversationID := vars["conversation_id"]
	
	if conversationID == "" {
		h.respondError(w, errors.ValidationError("conversation_id is required"))
		return
	}
	
	// Mock response - implementation would fetch from storage
	response := ConversationResponse{
		ConversationID: conversationID,
		Name:           "Test Conversation",
		Description:    "A test conversation",
		Type:           "group",
		CreatedBy:      "user-1",
		CreatedAt:      time.Now().Add(-7 * 24 * time.Hour).UTC().Format(time.RFC3339),
		UpdatedAt:      time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339),
		Participants: []ParticipantInfo{
			{UserID: "user-1", Role: "owner", JoinedAt: time.Now().Add(-7 * 24 * time.Hour).UTC().Format(time.RFC3339)},
			{UserID: "user-2", Role: "member", JoinedAt: time.Now().Add(-6 * 24 * time.Hour).UTC().Format(time.RFC3339)},
			{UserID: "user-3", Role: "member", JoinedAt: time.Now().Add(-5 * 24 * time.Hour).UTC().Format(time.RFC3339)},
		},
		MessageCount: 1250,
		LastMessage: &LastMessageInfo{
			MessageID: "msg-123",
			SenderID:  "user-2",
			Content:   "Hello everyone!",
			Timestamp: time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339),
		},
		IsPublic: false,
	}
	
	h.respondJSON(w, http.StatusOK, response)
}

// UpdateConversation handles PUT /api/v1/conversations/{conversation_id}
func (h *ConversationHandler) UpdateConversation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	conversationID := vars["conversation_id"]
	
	if conversationID == "" {
		h.respondError(w, errors.ValidationError("conversation_id is required"))
		return
	}
	
	var request UpdateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.respondError(w, errors.InvalidInputError("request body", err))
		return
	}
	
	// Mock response - implementation would update through conversation service
	response := ConversationResponse{
		ConversationID: conversationID,
		Name:           request.Name,
		Description:    request.Description,
		Type:           "group",
		UpdatedBy:      request.UpdatedBy,
		UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	
	h.respondJSON(w, http.StatusOK, response)
}

// AddParticipant handles POST /api/v1/conversations/{conversation_id}/participants
func (h *ConversationHandler) AddParticipant(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	conversationID := vars["conversation_id"]
	
	if conversationID == "" {
		h.respondError(w, errors.ValidationError("conversation_id is required"))
		return
	}
	
	var request AddParticipantRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.respondError(w, errors.InvalidInputError("request body", err))
		return
	}
	
	if request.UserID == "" {
		h.respondError(w, errors.ValidationError("user_id is required"))
		return
	}
	
	// Mock response
	response := ParticipantResponse{
		UserID:         request.UserID,
		Role:           request.Role,
		JoinedAt:       time.Now().UTC().Format(time.RFC3339),
		ConversationID: conversationID,
		AddedBy:        request.AddedBy,
	}
	
	h.respondJSON(w, http.StatusCreated, response)
}

// RemoveParticipant handles DELETE /api/v1/conversations/{conversation_id}/participants/{user_id}
func (h *ConversationHandler) RemoveParticipant(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	conversationID := vars["conversation_id"]
	userID := vars["user_id"]
	
	if conversationID == "" || userID == "" {
		h.respondError(w, errors.ValidationError("conversation_id and user_id are required"))
		return
	}
	
	// Mock response
	response := map[string]string{
		"status":          "removed",
		"user_id":         userID,
		"conversation_id": conversationID,
		"removed_at":      time.Now().UTC().Format(time.RFC3339),
	}
	
	h.respondJSON(w, http.StatusOK, response)
}

// SendTypingIndicator handles POST /api/v1/conversations/{conversation_id}/typing
func (h *ConversationHandler) SendTypingIndicator(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	conversationID := vars["conversation_id"]
	
	if conversationID == "" {
		h.respondError(w, errors.ValidationError("conversation_id is required"))
		return
	}
	
	var request TypingIndicatorRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.respondError(w, errors.InvalidInputError("request body", err))
		return
	}
	
	if request.UserID == "" {
		h.respondError(w, errors.ValidationError("user_id is required"))
		return
	}
	
	// Mock response
	response := map[string]interface{}{
		"conversation_id": conversationID,
		"user_id":         request.UserID,
		"is_typing":       request.IsTyping,
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
	}
	
	h.respondJSON(w, http.StatusOK, response)
}

// Helper methods

func (h *ConversationHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *ConversationHandler) respondError(w http.ResponseWriter, err *errors.AppError) {
	h.logger.Error("Conversation API error",
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

func generateConversationID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Request/Response types

type CreateConversationRequest struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	Type         string                 `json:"type,omitempty"`
	CreatedBy    string                 `json:"created_by"`
	Participants []ParticipantInfo      `json:"participants"`
	IsPublic     bool                   `json:"is_public,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type UpdateConversationRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	UpdatedBy   string `json:"updated_by"`
}

type AddParticipantRequest struct {
	UserID  string `json:"user_id"`
	Role    string `json:"role,omitempty"`
	AddedBy string `json:"added_by"`
}

type TypingIndicatorRequest struct {
	UserID    string `json:"user_id"`
	IsTyping  bool   `json:"is_typing"`
	DeviceID  string `json:"device_id,omitempty"`
}

type ConversationResponse struct {
	ConversationID string                 `json:"conversation_id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description,omitempty"`
	Type           string                 `json:"type"`
	CreatedBy      string                 `json:"created_by"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedBy      string                 `json:"updated_by,omitempty"`
	UpdatedAt      string                 `json:"updated_at,omitempty"`
	Participants   []ParticipantInfo      `json:"participants"`
	MessageCount   int                    `json:"message_count"`
	LastMessage    *LastMessageInfo       `json:"last_message,omitempty"`
	IsPublic       bool                   `json:"is_public"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type ParticipantInfo struct {
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	JoinedAt  string `json:"joined_at,omitempty"`
	LeftAt    string `json:"left_at,omitempty"`
	AddedBy   string `json:"added_by,omitempty"`
	IsActive  bool   `json:"is_active,omitempty"`
}

type LastMessageInfo struct {
	MessageID string `json:"message_id"`
	SenderID  string `json:"sender_id"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

type ParticipantResponse struct {
	UserID         string `json:"user_id"`
	Role           string `json:"role"`
	JoinedAt       string `json:"joined_at"`
	ConversationID string `json:"conversation_id"`
	AddedBy        string `json:"added_by"`
}
