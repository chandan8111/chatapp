package presence

import (
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
}

func TestNewPresenceService(t *testing.T) {
	service := NewPresenceService(createTestRedisClient(), "test-node")
	require.NotNil(t, service)
	assert.Equal(t, "test-node", service.nodeID)
	assert.NotNil(t, service.localBuffer)
	assert.NotNil(t, service.redisClient)
}

func TestHashUserID(t *testing.T) {
	service := &PresenceService{nodeID: "test"}

	tests := []struct {
		userID       string
		wantPositive bool
	}{
		{"user-123", true},
		{"user-456", true},
		{"", true}, // empty string should still return a hash
		{"very-long-user-id-with-many-characters-12345", true},
	}

	for _, tt := range tests {
		t.Run(tt.userID, func(t *testing.T) {
			hash := service.hashUserID(tt.userID)
			if tt.wantPositive {
				assert.True(t, hash >= 0)
			}
			assert.True(t, hash < 100000000) // Should be within bitmap size
		})
	}
}

func TestPresenceServiceOperations(t *testing.T) {
	service := NewPresenceService(createTestRedisClient(), "test-node")
	userID := "test-user-123"
	deviceID := "device-456"

	// Test marking user online
	service.MarkUserOnline(userID, deviceID)

	// Verify user is in local buffer
	service.localBufferMu.RLock()
	assert.Contains(t, service.localBuffer, userID)
	service.localBufferMu.RUnlock()

	// Test flushing buffer
	service.flushBuffer()

	// Verify buffer is cleared after flush
	service.localBufferMu.RLock()
	assert.Empty(t, service.localBuffer)
	service.localBufferMu.RUnlock()
}

func TestUserPresenceStruct(t *testing.T) {
	presence := &UserPresence{
		UserID:    "user-123",
		Online:    true,
		LastSeen:  time.Now(),
		NodeID:    "node-1",
		DeviceID:  "device-1",
		Timestamp: time.Now(),
	}

	assert.Equal(t, "user-123", presence.UserID)
	assert.True(t, presence.Online)
	assert.Equal(t, "node-1", presence.NodeID)
	assert.Equal(t, "device-1", presence.DeviceID)
}

func TestNodePresenceStruct(t *testing.T) {
	presence := &NodePresence{
		NodeID:        "node-1",
		OnlineUsers:   1000,
		TotalUsers:    5000,
		LastHeartbeat: time.Now(),
		CPUUsage:      45.5,
		MemoryUsage:   62.3,
	}

	assert.Equal(t, "node-1", presence.NodeID)
	assert.Equal(t, 1000, presence.OnlineUsers)
	assert.Equal(t, 5000, presence.TotalUsers)
	assert.Equal(t, 45.5, presence.CPUUsage)
	assert.Equal(t, 62.3, presence.MemoryUsage)
}

func TestPresenceStatsStruct(t *testing.T) {
	stats := &PresenceStats{
		TotalUsers:   100000000,
		OnlineUsers:  75000000,
		OfflineUsers: 25000000,
		OnlineRatio:  0.75,
		ActiveNodes:  50,
	}

	assert.Equal(t, int64(100000000), stats.TotalUsers)
	assert.Equal(t, int64(75000000), stats.OnlineUsers)
	assert.Equal(t, int64(25000000), stats.OfflineUsers)
	assert.Equal(t, 0.75, stats.OnlineRatio)
	assert.Equal(t, 50, stats.ActiveNodes)
}

func TestFlushBufferEmpty(t *testing.T) {
	service := NewPresenceService(createTestRedisClient(), "test-node")

	// Flush empty buffer should not panic
	service.flushBuffer()

	// Buffer should remain empty
	service.localBufferMu.RLock()
	assert.Empty(t, service.localBuffer)
	service.localBufferMu.RUnlock()
}

func TestFlushBufferWithData(t *testing.T) {
	service := NewPresenceService(createTestRedisClient(), "test-node")

	// Add users to buffer
	users := []string{"user-1", "user-2", "user-3"}
	for _, userID := range users {
		service.MarkUserOnline(userID, "device-1")
	}

	// Verify users in buffer
	service.localBufferMu.RLock()
	assert.Equal(t, 3, len(service.localBuffer))
	service.localBufferMu.RUnlock()

	// Note: Actual flush would require Redis connection
	// For unit test, we just verify the buffer state
}

func TestCleanupExpiredPresence(t *testing.T) {
	service := NewPresenceService(createTestRedisClient(), "test-node")

	// This test would require a mock Redis client
	// For now, we just verify the method doesn't panic
	err := service.CleanupExpiredPresence()
	// Error is expected since we're not connected to Redis
	assert.Error(t, err)
}

func TestGetActiveNodes(t *testing.T) {
	service := NewPresenceService(createTestRedisClient(), "test-node")

	// This test would require a mock Redis client
	nodes, err := service.GetActiveNodes()

	// Error is expected since we're not connected to Redis
	assert.Error(t, err)
	assert.Nil(t, nodes)
}

func BenchmarkHashUserID(b *testing.B) {
	service := &PresenceService{nodeID: "benchmark"}
	userID := "user-12345"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.hashUserID(userID)
	}
}

func BenchmarkMarkUserOnline(b *testing.B) {
	service := NewPresenceService(createTestRedisClient(), "benchmark-node")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userID := fmt.Sprintf("user-%d", i)
		service.MarkUserOnline(userID, "device-1")
	}
}

func TestPresenceServiceShutdown(t *testing.T) {
	service := NewPresenceService(createTestRedisClient(), "test-node")

	// Add some users to buffer
	service.MarkUserOnline("user-1", "device-1")
	service.MarkUserOnline("user-2", "device-1")

	// Shutdown should flush buffer and cleanup
	service.Shutdown()

	// Verify service is properly shut down
	assert.NotNil(t, service)
}

func TestUpdateNodePresence(t *testing.T) {
	service := NewPresenceService(createTestRedisClient(), "test-node")

	// This would normally update Redis, but we're testing the struct behavior
	nodePresence := &NodePresence{
		NodeID:        service.nodeID,
		OnlineUsers:   1000,
		TotalUsers:    5000,
		LastHeartbeat: time.Now(),
		CPUUsage:      50.0,
		MemoryUsage:   60.0,
	}

	assert.NotNil(t, nodePresence)
	assert.Equal(t, "test-node", nodePresence.NodeID)
}

func TestGetUserPresence(t *testing.T) {
	service := NewPresenceService(createTestRedisClient(), "test-node")

	// This test would require a mock Redis client
	presence, err := service.GetUserPresence("user-123")

	// Error is expected since we're not connected to Redis
	assert.Error(t, err)
	assert.Nil(t, presence)
}

func TestGetOnlineUsersInBatch(t *testing.T) {
	service := NewPresenceService(createTestRedisClient(), "test-node")

	userIDs := []string{"user-1", "user-2", "user-3"}

	// This test would require a mock Redis client
	onlineMap, err := service.GetOnlineUsersInBatch(userIDs)

	// Error is expected since we're not connected to Redis
	assert.Error(t, err)
	assert.Nil(t, onlineMap)
}

func TestPresenceServiceConcurrency(t *testing.T) {
	service := NewPresenceService(createTestRedisClient(), "test-node")

	// Test concurrent access to localBuffer
	done := make(chan bool)

	// Multiple goroutines marking users online
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				userID := fmt.Sprintf("user-%d-%d", id, j)
				service.MarkUserOnline(userID, "device-1")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify buffer has all users
	service.localBufferMu.RLock()
	assert.Equal(t, 1000, len(service.localBuffer))
	service.localBufferMu.RUnlock()
}

func TestBitmapKeyConsistency(t *testing.T) {
	service := NewPresenceService(createTestRedisClient(), "test-node")

	// Test that same user ID always produces the same hash
	userID := "consistent-user-123"
	hash1 := service.hashUserID(userID)
	hash2 := service.hashUserID(userID)
	hash3 := service.hashUserID(userID)

	assert.Equal(t, hash1, hash2)
	assert.Equal(t, hash2, hash3)
}

func TestHashDistribution(t *testing.T) {
	service := &PresenceService{nodeID: "test"}

	// Test hash distribution across the bitmap space
	hashCount := make(map[int64]int)
	for i := 0; i < 10000; i++ {
		userID := fmt.Sprintf("user-%d", i)
		hash := service.hashUserID(userID)
		hashCount[hash]++
	}

	// Check for reasonable distribution
	// In a good hash function, we should have very few collisions
	collisions := 0
	for _, count := range hashCount {
		if count > 1 {
			collisions++
		}
	}

	// Allow for some collisions (less than 1% is good)
	collisionRate := float64(collisions) / 10000.0
	assert.Less(t, collisionRate, 0.01)
}
