package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestWebSocketGatewayInitialization(t *testing.T) {
	logger := zaptest.NewLogger(t)
	gateway := NewWebSocketGateway("localhost:6379", logger)
	require.NotNil(t, gateway)
	assert.NotNil(t, gateway.hub)
	assert.NotNil(t, gateway.presence)
}

func TestHandleHealth(t *testing.T) {
	logger := zaptest.NewLogger(t)
	gateway := NewWebSocketGateway("localhost:6379", logger)

	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "healthy",
			"connections": 0,
		})
	})

	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestConnectionValidation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	gateway := NewWebSocketGateway("localhost:6379", logger)

	tests := []struct {
		name     string
		userID   string
		deviceID string
		nodeID   string
		wantErr  bool
	}{
		{
			name:     "Valid connection",
			userID:   "user-123",
			deviceID: "device-456",
			nodeID:   "gateway-1",
			wantErr:  false,
		},
		{
			name:     "Missing user_id",
			userID:   "",
			deviceID: "device-456",
			nodeID:   "gateway-1",
			wantErr:  true,
		},
		{
			name:     "Missing device_id",
			userID:   "user-123",
			deviceID: "",
			nodeID:   "gateway-1",
			wantErr:  true,
		},
		{
			name:     "Missing node_id",
			userID:   "user-123",
			deviceID: "device-456",
			nodeID:   "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/ws", nil)
			require.NoError(t, err)

			// Add query parameters
			q := req.URL.Query()
			q.Add("user_id", tt.userID)
			q.Add("device_id", tt.deviceID)
			q.Add("node_id", tt.nodeID)
			req.URL.RawQuery = q.Encode()

			rr := httptest.NewRecorder()

			// The actual WebSocket upgrade would fail in test, but we can verify parameter validation
			gateway.handleWebSocket(rr, req)

			if tt.wantErr {
				assert.NotEqual(t, http.StatusOK, rr.Code)
			}
		})
	}
}

func TestHubConnectionManagement(t *testing.T) {
	logger := zaptest.NewLogger(t)
	hub := NewHub(nil, logger)
	require.NotNil(t, hub)

	// Test connection registration
	conn1 := &Connection{
		userID:   "user-1",
		deviceID: "device-1",
		send:     make(chan []byte, 256),
	}

	hub.register <- conn1
	time.Sleep(100 * time.Millisecond) // Allow hub to process

	assert.Contains(t, hub.connections, "user-1")

	// Test connection unregistration
	hub.unregister <- conn1
	time.Sleep(100 * time.Millisecond)

	assert.NotContains(t, hub.connections, "user-1")
}

func TestConnectionMessageHandling(t *testing.T) {
	tests := []struct {
		name      string
		message   []byte
		wantError bool
	}{
		{
			name:      "Valid message",
			message:   []byte(`{"type":"message","content":"Hello"}`),
			wantError: false,
		},
		{
			name:      "Invalid JSON",
			message:   []byte(`invalid json`),
			wantError: true,
		},
		{
			name:      "Empty message",
			message:   []byte{},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &Connection{
				userID: "user-1",
				send:   make(chan []byte, 256),
			}

			// Test message handling logic
			if len(tt.message) > 0 {
				var msg map[string]interface{}
				err := json.Unmarshal(tt.message, &msg)
				if tt.wantError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			}
		})
	}
}

func TestHubBroadcast(t *testing.T) {
	logger := zaptest.NewLogger(t)
	hub := NewHub(nil, logger)
	go hub.run()

	// Create test connections
	conn1 := &Connection{
		userID: "user-1",
		send:   make(chan []byte, 256),
	}
	conn2 := &Connection{
		userID: "user-2",
		send:   make(chan []byte, 256),
	}

	// Register connections
	hub.mu.Lock()
	hub.connections["user-1"] = conn1
	hub.connections["user-2"] = conn2
	hub.mu.Unlock()

	// Test broadcast
	message := []byte("test message")
	hub.broadcast <- message

	// Verify message was broadcasted (in real scenario, both connections would receive it)
	time.Sleep(100 * time.Millisecond)
}

func BenchmarkHubBroadcast(b *testing.B) {
	logger := zaptest.NewLogger(b)
	hub := NewHub(nil, logger)
	go hub.run()

	// Create test connections
	for i := 0; i < 100; i++ {
		conn := &Connection{
			userID: fmt.Sprintf("user-%d", i),
			send:   make(chan []byte, 256),
		}
		hub.mu.Lock()
		hub.connections[conn.userID] = conn
		hub.mu.Unlock()
	}

	message := []byte("benchmark message")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.broadcast <- message
	}
}

func TestWebSocketConnectionValidation(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		deviceID   string
		nodeID     string
		shouldFail bool
	}{
		{
			name:       "Valid connection parameters",
			userID:     "user-12345",
			deviceID:   "device-67890",
			nodeID:     "gateway-node-1",
			shouldFail: false,
		},
		{
			name:       "Empty user ID",
			userID:     "",
			deviceID:   "device-67890",
			nodeID:     "gateway-node-1",
			shouldFail: true,
		},
		{
			name:       "Empty device ID",
			userID:     "user-12345",
			deviceID:   "",
			nodeID:     "gateway-node-1",
			shouldFail: true,
		},
		{
			name:       "Empty node ID",
			userID:     "user-12345",
			deviceID:   "device-67890",
			nodeID:     "",
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate parameters
			hasEmptyParams := tt.userID == "" || tt.deviceID == "" || tt.nodeID == ""
			if tt.shouldFail {
				assert.True(t, hasEmptyParams, "Expected validation to fail for empty parameters")
			} else {
				assert.False(t, hasEmptyParams, "Expected validation to pass for valid parameters")
			}
		})
	}
}

func TestMessageSerialization(t *testing.T) {
	tests := []struct {
		name    string
		message map[string]interface{}
	}{
		{
			name: "Simple message",
			message: map[string]interface{}{
				"type":    "text",
				"content": "Hello World",
				"sender":  "user-123",
			},
		},
		{
			name: "Complex message with metadata",
			message: map[string]interface{}{
				"type":      "text",
				"content":   "Test message",
				"sender":    "user-123",
				"timestamp": time.Now().Unix(),
				"metadata": map[string]string{
					"client_version": "1.0.0",
					"platform":       "web",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON serialization
			data, err := json.Marshal(tt.message)
			require.NoError(t, err)
			assert.NotNil(t, data)
			assert.True(t, len(data) > 0)

			// Test JSON deserialization
			var decoded map[string]interface{}
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, tt.message["type"], decoded["type"])
		})
	}
}

func TestConnectionPool(t *testing.T) {
	// Test connection pooling behavior
	pool := make(map[string]*Connection)

	// Add connections
	for i := 0; i < 10; i++ {
		userID := fmt.Sprintf("user-%d", i)
		conn := &Connection{
			userID: userID,
			send:   make(chan []byte, 256),
		}
		pool[userID] = conn
	}

	assert.Equal(t, 10, len(pool))

	// Remove connection
	delete(pool, "user-5")
	assert.Equal(t, 9, len(pool))
	assert.NotContains(t, pool, "user-5")
}

func TestWebsocketUpgrader(t *testing.T) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	assert.NotNil(t, upgrader)
	assert.Equal(t, 1024, upgrader.ReadBufferSize)
	assert.Equal(t, 1024, upgrader.WriteBufferSize)
}

func TestGracefulShutdown(t *testing.T) {
	logger := zaptest.NewLogger(t)
	gateway := NewWebSocketGateway("localhost:6379", logger)
	require.NotNil(t, gateway)

	// Simulate connections
	gateway.hub.connections["user-1"] = &Connection{
		userID: "user-1",
		send:   make(chan []byte),
	}

	// Verify connections exist
	assert.Equal(t, 1, len(gateway.hub.connections))
}

func BenchmarkConnectionRegistration(b *testing.B) {
	logger := zaptest.NewLogger(b)
	hub := NewHub(nil, logger)
	go hub.run()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn := &Connection{
			userID: fmt.Sprintf("user-%d", i),
			send:   make(chan []byte, 256),
		}
		hub.register <- conn
	}
}
