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

// UserHandler handles user-related API requests
type UserHandler struct {
	userService interface{}
	logger      *zap.Logger
}

// NewUserHandler creates a new user handler
func NewUserHandler(userService interface{}, logger *zap.Logger) *UserHandler {
	return &UserHandler{
		userService: userService,
		logger:      logger,
	}
}

// GetUser handles GET /api/v1/users/{user_id}
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]
	
	if userID == "" {
		h.respondError(w, errors.ValidationError("user_id is required"))
		return
	}
	
	// Mock response - implementation would fetch from user service
	response := UserResponse{
		UserID:      userID,
		Username:    "testuser",
		DisplayName: "Test User",
		Email:       "test@example.com",
		AvatarURL:   "https://example.com/avatar.jpg",
		Status:      "active",
		CreatedAt:   time.Now().Add(-365 * 24 * time.Hour).UTC().Format(time.RFC3339),
		LastSeen:    time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339),
		IsOnline:    true,
		Metadata: map[string]interface{}{
			"timezone": "UTC",
			"language": "en",
		},
	}
	
	h.respondJSON(w, http.StatusOK, response)
}

// GetUserConversations handles GET /api/v1/users/{user_id}/conversations
func (h *UserHandler) GetUserConversations(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]
	
	if userID == "" {
		h.respondError(w, errors.ValidationError("user_id is required"))
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
	
	archived := r.URL.Query().Get("archived") == "true"
	
	// Mock response - implementation would fetch from user service
	conversations := make([]UserConversationInfo, 0, limit)
	for i := 0; i < limit; i++ {
		conversations = append(conversations, UserConversationInfo{
			ConversationID:   fmt.Sprintf("conv-%d", offset+i+1),
			Name:             fmt.Sprintf("Conversation %d", offset+i+1),
			Type:             "group",
			LastMessageAt:    time.Now().Add(-time.Duration(i) * time.Hour).UTC().Format(time.RFC3339),
			UnreadCount:      i % 5,
			IsArchived:       archived,
			IsMuted:          i%3 == 0,
			ParticipantCount: 10,
		})
	}
	
	response := UserConversationsResponse{
		UserID:        userID,
		Conversations: conversations,
		Count:         len(conversations),
		Total:         1000,
		Offset:        offset,
		Limit:         limit,
		HasMore:       offset+limit < 1000,
	}
	
	h.respondJSON(w, http.StatusOK, response)
}

// UpdateUserPresence handles PUT /api/v1/users/{user_id}/presence
func (h *UserHandler) UpdateUserPresence(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]
	
	if userID == "" {
		h.respondError(w, errors.ValidationError("user_id is required"))
		return
	}
	
	var request UpdateUserPresenceRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.respondError(w, errors.InvalidInputError("request body", err))
		return
	}
	
	// Mock response - implementation would update through presence service
	response := UserPresenceResponse{
		UserID:    userID,
		Online:    request.Online,
		Status:    request.Status,
		LastSeen:  time.Now().UTC().Format(time.RFC3339),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	
	h.respondJSON(w, http.StatusOK, response)
}

// GetUserSessions handles GET /api/v1/users/{user_id}/sessions
func (h *UserHandler) GetUserSessions(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]
	
	if userID == "" {
		h.respondError(w, errors.ValidationError("user_id is required"))
		return
	}
	
	// Mock response - implementation would fetch from session service
	sessions := []UserSessionInfo{
		{
			SessionID:  "session-1",
			DeviceType: "mobile",
			DeviceName: "iPhone 14",
			IP:         "192.168.1.100",
			Location:   "New York, USA",
			LastActive: time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339),
			IsActive:   true,
		},
		{
			SessionID:  "session-2",
			DeviceType: "web",
			DeviceName: "Chrome on Windows",
			IP:         "192.168.1.101",
			Location:   "New York, USA",
			LastActive: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
			IsActive:   false,
		},
	}
	
	response := UserSessionsResponse{
		UserID:   userID,
		Sessions: sessions,
		Count:    len(sessions),
	}
	
	h.respondJSON(w, http.StatusOK, response)
}

// Helper methods

func (h *UserHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *UserHandler) respondError(w http.ResponseWriter, err *errors.AppError) {
	h.logger.Error("User API error",
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

// Request/Response types

type UserResponse struct {
	UserID      string                 `json:"user_id"`
	Username    string                 `json:"username"`
	DisplayName string                 `json:"display_name"`
	Email       string                 `json:"email"`
	AvatarURL   string                 `json:"avatar_url,omitempty"`
	Status      string                 `json:"status"`
	CreatedAt   string                 `json:"created_at"`
	LastSeen    string                 `json:"last_seen"`
	IsOnline    bool                   `json:"is_online"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type UserConversationInfo struct {
	ConversationID   string `json:"conversation_id"`
	Name             string `json:"name"`
	Type             string `json:"type"`
	LastMessageAt    string `json:"last_message_at"`
	UnreadCount      int    `json:"unread_count"`
	IsArchived       bool   `json:"is_archived"`
	IsMuted          bool   `json:"is_muted"`
	ParticipantCount int    `json:"participant_count"`
}

type UserConversationsResponse struct {
	UserID        string                 `json:"user_id"`
	Conversations []UserConversationInfo `json:"conversations"`
	Count         int                    `json:"count"`
	Total         int                    `json:"total"`
	Offset        int                    `json:"offset"`
	Limit         int                    `json:"limit"`
	HasMore       bool                   `json:"has_more"`
}

type UpdateUserPresenceRequest struct {
	Online bool   `json:"online"`
	Status string `json:"status,omitempty"`
}

type UserPresenceResponse struct {
	UserID    string `json:"user_id"`
	Online    bool   `json:"online"`
	Status    string `json:"status,omitempty"`
	LastSeen  string `json:"last_seen"`
	UpdatedAt string `json:"updated_at"`
}

type UserSessionInfo struct {
	SessionID  string `json:"session_id"`
	DeviceType string `json:"device_type"`
	DeviceName string `json:"device_name"`
	IP         string `json:"ip_address"`
	Location   string `json:"location,omitempty"`
	LastActive string `json:"last_active"`
	IsActive   bool   `json:"is_active"`
}

type UserSessionsResponse struct {
	UserID   string            `json:"user_id"`
	Sessions []UserSessionInfo `json:"sessions"`
	Count    int               `json:"count"`
}
