package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/chatapp/proto"
)

// MockRedisClient is a mock for Redis client
type MockRedisClient struct {
	mock.Mock
}

func (m *MockRedisClient) SetBit(ctx context.Context, key string, offset int64, value int) *redis.IntCmd {
	args := m.Called(ctx, key, offset, value)
	return args.Get(0).(*redis.IntCmd)
}

func (m *MockRedisClient) Pipeline() redis.Pipeliner {
	args := m.Called()
	return args.Get(0).(redis.Pipeliner)
}

// MockWebSocket is a mock for WebSocket connection
type MockWebSocket struct {
	mock.Mock
	messages [][]byte
	closed   bool
}

func (m *MockWebSocket) ReadMessage() (messageType int, p []byte, err error) {
	args := m.Called()
	return args.Int(0), args.Get(1).([]byte), args.Error(2)
}

func (m *MockWebSocket) WriteMessage(messageType int, data []byte) error {
	args := m.Called(messageType, data)
	if !m.closed {
		m.messages = append(m.messages, data)
	}
	return args.Error(0)
}

func (m *MockWebSocket) Close() error {
	m.closed = true
	args := m.Called()
	return args.Error(0)
}

func (m *MockWebSocket) SetReadDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *MockWebSocket) SetWriteDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *MockWebSocket) SetPongHandler(h func(appData string) error) {
	_ = m.Called(h)
}

func (m *MockWebSocket) NextWriter(messageType int) (websocket.NextWriter, error) {
	args := m.Called(messageType)
	return args.Get(0).(websocket.NextWriter), args.Error(1)
}

// TestConnectionMessageHandling tests the message handling logic
func TestConnectionMessageHandling(t *testing.T) {
	logger := zaptest.NewLogger(t)
	
	tests := []struct {
		name          string
		message       *proto.ChatMessage
		senderID      string
		expectError   bool
		expectForward bool
	}{
		{
			name: "Valid text message",
			message: &proto.ChatMessage{
				MessageId:      uuid.New().String(),
				ConversationId: uuid.New().String(),
				SenderId:       "user-123",
				Timestamp:      time.Now().UnixNano(),
				Ciphertext:     []byte("encrypted content"),
				MessageType:    0, // text
			},
			senderID:      "user-123",
			expectError:   false,
			expectForward: true,
		},
		{
			name: "Sender ID mismatch",
			message: &proto.ChatMessage{
				MessageId:      uuid.New().String(),
				ConversationId: uuid.New().String(),
				SenderId:       "user-456",
				Timestamp:      time.Now().UnixNano(),
				Ciphertext:     []byte("encrypted content"),
				MessageType:    0,
			},
			senderID:      "user-123",
			expectError:   true,
			expectForward: false,
		},
		{
			name: "Empty message ID",
			message: &proto.ChatMessage{
				ConversationId: uuid.New().String(),
				SenderId:       "user-123",
				Timestamp:      time.Now().UnixNano(),
				Ciphertext:     []byte("encrypted content"),
				MessageType:    0,
			},
			senderID:      "user-123",
			expectError:   true, // Will fail validation
			expectForward: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create connection
			conn := &Connection{
				userID: tt.senderID,
				send:   make(chan []byte, 256),
			}

			// Serialize message
			messageBytes, err := proto.Marshal(tt.message)
			require.NoError(t, err)

			// Create hub with mock
			mockRedis := new(MockRedisClient)
			hub := NewHub(mockRedis, logger)
			
			receivedBroadcast := false
			go func() {
				select {
				case <-hub.broadcast:
					receivedBroadcast = true
				case <-time.After(100 * time.Millisecond):
				}
			}()

			// Handle message
			conn.handleMessage(messageBytes, hub)

			// Verify results
			if tt.expectForward {
				assert.True(t, receivedBroadcast, "Message should be forwarded to broadcast")
			} else {
				assert.False(t, receivedBroadcast, "Message should not be forwarded")
			}
		})
	}
}

// TestConnectionLifecycle tests the complete connection lifecycle
func TestConnectionLifecycle(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockRedis := new(MockRedisClient)
	hub := NewHub(mockRedis, logger)
	
	// Start hub in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.run()

	// Test connection registration
	conn := &Connection{
		userID:   "user-123",
		deviceID: "device-456",
		nodeID:   "node-1",
		send:     make(chan []byte, 256),
	}

	// Register connection
	hub.register <- conn
	time.Sleep(50 * time.Millisecond)

	// Verify connection is registered
	hub.mu.RLock()
	assert.Contains(t, hub.connections, "user-123")
	assert.Equal(t, int64(1), hub.connectionCount)
	hub.mu.RUnlock()

	// Test message sending to connection
	testMessage := []byte("test message")
	select {
	case conn.send <- testMessage:
		// Message sent successfully
	default:
		t.Error("Failed to send message to connection")
	}

	// Test connection unregistration
	hub.unregister <- conn
	time.Sleep(50 * time.Millisecond)

	// Verify connection is unregistered
	hub.mu.RLock()
	assert.NotContains(t, hub.connections, "user-123")
	assert.Equal(t, int64(0), hub.connectionCount)
	hub.mu.RUnlock()
}

// TestConcurrentConnectionHandling tests concurrent connection operations
func TestConcurrentConnectionHandling(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockRedis := new(MockRedisClient)
	hub := NewHub(mockRedis, logger)
	
	// Start hub in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.run()

	numConnections := 100
	var wg sync.WaitGroup

	// Test concurrent connections
	for i := 0; i < numConnections; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			conn := &Connection{
				userID:   fmt.Sprintf("user-%d", id),
				deviceID: fmt.Sprintf("device-%d", id),
				nodeID:   "node-1",
				send:     make(chan []byte, 256),
			}

			// Register connection
			hub.register <- conn
			time.Sleep(10 * time.Millisecond)

			// Unregister connection
			hub.unregister <- conn
		}(i)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	// Verify all connections are cleaned up
	hub.mu.RLock()
	assert.Equal(t, 0, len(hub.connections))
	assert.Equal(t, int64(0), hub.connectionCount)
	hub.mu.RUnlock()
}

// TestHeartbeatBatching tests the heartbeat batching functionality
func TestHeartbeatBatching(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockRedis := new(MockRedisClient)
	mockPipeline := new(MockRedisClient)
	
	// Setup mock expectations
	mockRedis.On("Pipeline").Return(mockPipeline)
	mockPipeline.On("SetBit", mock.Anything, "presence:online", mock.AnythingOfType("int64"), 1).Return(&redis.IntCmd{})
	mockPipeline.On("Expire", mock.Anything, "presence:online", mock.AnythingOfType("time.Duration")).Return(&redis.BoolCmd{})
	mockPipeline.On("Exec", mock.Anything).Return([]redis.Cmder{}, nil)

	hub := NewHub(mockRedis, logger)
	
	// Add users to heartbeat batch
	userIDs := []string{"user-1", "user-2", "user-3"}
	for _, userID := range userIDs {
		hub.markUserOnline(userID)
	}

	// Verify users are in batch
	hub.heartbeatMu.Lock()
	assert.Equal(t, 3, len(hub.heartbeatBatch))
	for _, userID := range userIDs {
		assert.Contains(t, hub.heartbeatBatch, userID)
	}
	hub.heartbeatMu.Unlock()

	// Flush batch
	hub.flushHeartbeatBatch()

	// Verify batch is cleared
	hub.heartbeatMu.Lock()
	assert.Equal(t, 0, len(hub.heartbeatBatch))
	hub.heartbeatMu.Unlock()

	// Verify mock expectations
	mockRedis.AssertExpectations(t)
	mockPipeline.AssertExpectations(t)
}

// TestMessageValidation tests message validation logic
func TestMessageValidation(t *testing.T) {
	tests := []struct {
		name        string
		message     *proto.ChatMessage
		expectValid bool
	}{
		{
			name: "Valid message",
			message: &proto.ChatMessage{
				MessageId:      uuid.New().String(),
				ConversationId: uuid.New().String(),
				SenderId:       "user-123",
				Timestamp:      time.Now().UnixNano(),
				Ciphertext:     []byte("encrypted content"),
				MessageType:    0,
			},
			expectValid: true,
		},
		{
			name: "Empty message ID",
			message: &proto.ChatMessage{
				ConversationId: uuid.New().String(),
				SenderId:       "user-123",
				Timestamp:      time.Now().UnixNano(),
				Ciphertext:     []byte("encrypted content"),
				MessageType:    0,
			},
			expectValid: false,
		},
		{
			name: "Empty conversation ID",
			message: &proto.ChatMessage{
				MessageId:  uuid.New().String(),
				SenderId:   "user-123",
				Timestamp:  time.Now().UnixNano(),
				Ciphertext: []byte("encrypted content"),
				MessageType: 0,
			},
			expectValid: false,
		},
		{
			name: "Empty sender ID",
			message: &proto.ChatMessage{
				MessageId:      uuid.New().String(),
				ConversationId: uuid.New().String(),
				Timestamp:      time.Now().UnixNano(),
				Ciphertext:     []byte("encrypted content"),
				MessageType:    0,
			},
			expectValid: false,
		},
		{
			name: "Invalid message type",
			message: &proto.ChatMessage{
				MessageId:      uuid.New().String(),
				ConversationId: uuid.New().String(),
				SenderId:       "user-123",
				Timestamp:      time.Now().UnixNano(),
				Ciphertext:     []byte("encrypted content"),
				MessageType:    99, // invalid type
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validateChatMessage(tt.message)
			assert.Equal(t, tt.expectValid, isValid)
		})
	}
}

// validateChatMessage validates a chat message
func validateChatMessage(msg *proto.ChatMessage) bool {
	if msg.MessageId == "" || msg.ConversationId == "" || msg.SenderId == "" {
		return false
	}
	if msg.Timestamp <= 0 {
		return false
	}
	if msg.MessageType < 0 || msg.MessageType > 3 {
		return false
	}
	return true
}

// TestConnectionLimit tests connection limit enforcement
func TestConnectionLimit(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockRedis := new(MockRedisClient)
	
	// Create gateway with low connection limit for testing
	gateway := &WebSocketGateway{
		hub: NewHub(mockRedis, logger),
	}
	
	// Set connection limit to 5 for testing
	originalLimit := maxConnectionsPerNode
	maxConnectionsPerNode = 5
	defer func() { maxConnectionsPerNode = originalLimit }()

	// Add connections up to limit
	for i := 0; i < 5; i++ {
		gateway.hub.connections[fmt.Sprintf("user-%d", i)] = &Connection{
			userID: fmt.Sprintf("user-%d", i),
			send:   make(chan []byte, 256),
		}
	}
	gateway.hub.connectionCount = 5

	// Test connection limit enforcement
	req, err := http.NewRequest("GET", "/ws?user_id=user-6&device_id=device-6&node_id=node-1", nil)
	require.NoError(t, err)
	
	rr := httptest.NewRecorder()
	gateway.handleWebSocket(rr, req)
	
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

// BenchmarkMessageHandling benchmarks message handling performance
func BenchmarkMessageHandling(b *testing.B) {
	logger := zaptest.NewLogger(b)
	mockRedis := new(MockRedisClient)
	hub := NewHub(mockRedis, logger)
	
	message := &proto.ChatMessage{
		MessageId:      uuid.New().String(),
		ConversationId: uuid.New().String(),
		SenderId:       "user-123",
		Timestamp:      time.Now().UnixNano(),
		Ciphertext:     make([]byte, 1024), // 1KB message
		MessageType:    0,
	}
	
	messageBytes, err := proto.Marshal(message)
	require.NoError(b, err)
	
	conn := &Connection{
		userID: "user-123",
		send:   make(chan []byte, 256),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn.handleMessage(messageBytes, hub)
	}
}

// BenchmarkConnectionRegistration benchmarks connection registration performance
func BenchmarkConnectionRegistration(b *testing.B) {
	logger := zaptest.NewLogger(b)
	mockRedis := new(MockRedisClient)
	hub := NewHub(mockRedis, logger)
	go hub.run()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn := &Connection{
			userID:   fmt.Sprintf("user-%d", i),
			deviceID: fmt.Sprintf("device-%d", i),
			nodeID:   "node-1",
			send:     make(chan []byte, 256),
		}
		hub.register <- conn
	}
}
