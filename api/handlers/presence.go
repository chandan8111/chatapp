package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/chatapp/errors"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// PresenceHandler handles presence-related API requests
type PresenceHandler struct {
	presenceService interface{}
	logger          *zap.Logger
}

// NewPresenceHandler creates a new presence handler
func NewPresenceHandler(presenceService interface{}, logger *zap.Logger) *PresenceHandler {
	return &PresenceHandler{
		presenceService: presenceService,
		logger:          logger,
	}
}

// GetPresence handles GET /api/v1/presence/{user_id}
func (h *PresenceHandler) GetPresence(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]
	
	if userID == "" {
		h.respondError(w, errors.ValidationError("user_id is required"))
		return
	}
	
	// Mock response - implementation would call presence service
	response := PresenceResponse{
		UserID:   userID,
		Online:   true,
		LastSeen: "2024-01-15T10:30:00Z",
		Status:   "active",
		Devices: []DeviceInfo{
			{
				DeviceID:   "device-1",
				DeviceType: "mobile",
				LastActive: "2024-01-15T10:30:00Z",
			},
		},
	}
	
	h.respondJSON(w, http.StatusOK, response)
}

// GetPresenceBatch handles POST /api/v1/presence/batch
func (h *PresenceHandler) GetPresenceBatch(w http.ResponseWriter, r *http.Request) {
	var request BatchPresenceRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.respondError(w, errors.InvalidInputError("request body", err))
		return
	}
	
	if len(request.UserIDs) == 0 {
		h.respondError(w, errors.ValidationError("user_ids array is required"))
		return
	}
	
	if len(request.UserIDs) > 1000 {
		h.respondError(w, errors.ValidationError("maximum 1000 user_ids allowed"))
		return
	}
	
	// Mock response - implementation would batch fetch from presence service
	presences := make([]PresenceResponse, 0, len(request.UserIDs))
	for _, userID := range request.UserIDs {
		presences = append(presences, PresenceResponse{
			UserID:   userID,
			Online:   true,
			LastSeen: "2024-01-15T10:30:00Z",
			Status:   "active",
		})
	}
	
	response := BatchPresenceResponse{
		Presences: presences,
		Count:     len(presences),
	}
	
	h.respondJSON(w, http.StatusOK, response)
}

// GetOnlineUsers handles GET /api/v1/presence/online
func (h *PresenceHandler) GetOnlineUsers(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 1000 {
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
	
	// Mock response - implementation would query presence service
	response := OnlineUsersResponse{
		Total:   1000000,
		Count:   limit,
		Offset:  offset,
		UserIDs: generateMockUserIDs(limit, offset),
	}
	
	h.respondJSON(w, http.StatusOK, response)
}

// Helper methods

func (h *PresenceHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *PresenceHandler) respondError(w http.ResponseWriter, err *errors.AppError) {
	h.logger.Error("Presence API error",
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

func generateMockUserIDs(count, offset int) []string {
	userIDs := make([]string, count)
	for i := 0; i < count; i++ {
		userIDs[i] = fmt.Sprintf("user-%d", offset+i+1)
	}
	return userIDs
}

// Request/Response types

type BatchPresenceRequest struct {
	UserIDs []string `json:"user_ids"`
}

type BatchPresenceResponse struct {
	Presences []PresenceResponse `json:"presences"`
	Count     int                `json:"count"`
}

type PresenceResponse struct {
	UserID   string       `json:"user_id"`
	Online   bool         `json:"online"`
	LastSeen string       `json:"last_seen"`
	Status   string       `json:"status"`
	Devices  []DeviceInfo `json:"devices,omitempty"`
}

type DeviceInfo struct {
	DeviceID   string `json:"device_id"`
	DeviceType string `json:"device_type"`
	LastActive string `json:"last_active"`
}

type OnlineUsersResponse struct {
	Total   int      `json:"total"`
	Count   int      `json:"count"`
	Offset  int      `json:"offset"`
	UserIDs []string `json:"user_ids"`
}

type ErrorResponse struct {
	Error     string                 `json:"error"`
	Message   string                 `json:"message"`
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
}
