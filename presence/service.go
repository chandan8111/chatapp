package presence

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

const (
	bitmapKey       = "presence:online"
	userPresenceKey = "presence:user:%s"
	nodePresenceKey = "presence:node:%s"
	batchSize       = 1000
	flushInterval   = 5 * time.Second
	expiryDuration  = 24 * time.Hour
)

type PresenceService struct {
	redisClient    *redis.Client
	localBuffer    map[string]bool
	localBufferMu  sync.RWMutex
	flushTicker    *time.Ticker
	nodeID         string
	shutdown       chan struct{}
	wg             sync.WaitGroup
}

type UserPresence struct {
	UserID    string    `json:"user_id"`
	Online    bool      `json:"online"`
	LastSeen  time.Time `json:"last_seen"`
	NodeID    string    `json:"node_id"`
	DeviceID  string    `json:"device_id"`
	Timestamp time.Time `json:"timestamp"`
}

type NodePresence struct {
	NodeID       string    `json:"node_id"`
	OnlineUsers  int       `json:"online_users"`
	TotalUsers   int       `json:"total_users"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	CPUUsage     float64   `json:"cpu_usage"`
	MemoryUsage  float64   `json:"memory_usage"`
}

type PresenceStats struct {
	TotalUsers     int64   `json:"total_users"`
	OnlineUsers    int64   `json:"online_users"`
	OfflineUsers   int64   `json:"offline_users"`
	OnlineRatio    float64 `json:"online_ratio"`
	ActiveNodes    int     `json:"active_nodes"`
}

func NewPresenceService(redisClient *redis.Client, nodeID string) *PresenceService {
	ps := &PresenceService{
		redisClient: redisClient,
		localBuffer: make(map[string]bool),
		nodeID:      nodeID,
		shutdown:    make(chan struct{}),
	}

	ps.wg.Add(1)
	go ps.flushLoop()

	return ps
}

func (ps *PresenceService) MarkUserOnline(userID, deviceID string) {
	ps.localBufferMu.Lock()
	ps.localBuffer[userID] = true
	ps.localBufferMu.Unlock()

	// Update detailed user presence in separate hash
	ctx := context.Background()
	presence := &UserPresence{
		UserID:    userID,
		Online:    true,
		LastSeen:  time.Now(),
		NodeID:    ps.nodeID,
		DeviceID:  deviceID,
		Timestamp: time.Now(),
	}

	presenceJSON, _ := json.Marshal(presence)
	key := fmt.Sprintf(userPresenceKey, userID)
	
	pipe := ps.redisClient.Pipeline()
	pipe.HSet(ctx, key, presenceJSON)
	pipe.Expire(ctx, key, expiryDuration)
	pipe.Exec(ctx)
}

func (ps *PresenceService) MarkUserOffline(userID string) {
	ctx := context.Background()
	userIDInt := ps.hashUserID(userID)

	// Update bitmap
	pipe := ps.redisClient.Pipeline()
	pipe.SetBit(ctx, bitmapKey, userIDInt, 0)
	
	// Update detailed presence
	key := fmt.Sprintf(userPresenceKey, userID)
	presence := &UserPresence{
		UserID:    userID,
		Online:    false,
		LastSeen:  time.Now(),
		NodeID:    ps.nodeID,
		Timestamp: time.Now(),
	}

	presenceJSON, _ := json.Marshal(presence)
	pipe.HSet(ctx, key, presenceJSON)
	pipe.Expire(ctx, key, expiryDuration)
	
	pipe.Exec(ctx)
}

func (ps *PresenceService) IsUserOnline(userID string) (bool, error) {
	ctx := context.Background()
	userIDInt := ps.hashUserID(userID)
	
	result, err := ps.redisClient.GetBit(ctx, bitmapKey, userIDInt).Result()
	if err != nil {
		return false, err
	}
	
	return result == 1, nil
}

func (ps *PresenceService) GetUserPresence(userID string) (*UserPresence, error) {
	ctx := context.Background()
	key := fmt.Sprintf(userPresenceKey, userID)
	
	result, err := ps.redisClient.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	
	if len(result) == 0 {
		return &UserPresence{
			UserID:   userID,
			Online:   false,
			LastSeen: time.Time{},
		}, nil
	}
	
	var presence UserPresence
	// Find the first field (which contains the JSON)
	for _, value := range result {
		if err := json.Unmarshal([]byte(value), &presence); err == nil {
			break
		}
	}
	
	return &presence, nil
}

func (ps *PresenceService) GetOnlineUsersInBatch(userIDs []string) (map[string]bool, error) {
	ctx := context.Background()
	pipe := ps.redisClient.Pipeline()
	
	cmds := make([]*redis.IntCmd, len(userIDs))
	for i, userID := range userIDs {
		userIDInt := ps.hashUserID(userID)
		cmds[i] = pipe.GetBit(ctx, bitmapKey, userIDInt)
	}
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}
	
	result := make(map[string]bool)
	for i, userID := range userIDs {
		online, _ := cmds[i].Result()
		result[userID] = online == 1
	}
	
	return result, nil
}

func (ps *PresenceService) GetPresenceStats() (*PresenceStats, error) {
	ctx := context.Background()
	
	// Count online users using BITCOUNT
	onlineCount, err := ps.redisClient.BitCount(ctx, bitmapKey, nil).Result()
	if err != nil {
		return nil, err
	}
	
	// Get active nodes
	nodeKeys, err := ps.redisClient.Keys(ctx, "presence:node:*").Result()
	if err != nil {
		return nil, err
	}
	
	stats := &PresenceStats{
		TotalUsers:   100000000, // 100M total users
		OnlineUsers:  onlineCount,
		OfflineUsers: 100000000 - onlineCount,
		ActiveNodes:  len(nodeKeys),
	}
	
	if stats.TotalUsers > 0 {
		stats.OnlineRatio = float64(stats.OnlineUsers) / float64(stats.TotalUsers)
	}
	
	return stats, nil
}

func (ps *PresenceService) UpdateNodePresence(onlineUsers, totalUsers int, cpuUsage, memoryUsage float64) {
	ctx := context.Background()
	key := fmt.Sprintf(nodePresenceKey, ps.nodeID)
	
	nodePresence := &NodePresence{
		NodeID:        ps.nodeID,
		OnlineUsers:   onlineUsers,
		TotalUsers:    totalUsers,
		LastHeartbeat: time.Now(),
		CPUUsage:      cpuUsage,
		MemoryUsage:   memoryUsage,
	}
	
	presenceJSON, _ := json.Marshal(nodePresence)
	
	pipe := ps.redisClient.Pipeline()
	pipe.HSet(ctx, key, presenceJSON)
	pipe.Expire(ctx, key, expiryDuration)
	pipe.Exec(ctx)
}

func (ps *PresenceService) GetActiveNodes() ([]*NodePresence, error) {
	ctx := context.Background()
	nodeKeys, err := ps.redisClient.Keys(ctx, "presence:node:*").Result()
	if err != nil {
		return nil, err
	}
	
	if len(nodeKeys) == 0 {
		return []*NodePresence{}, nil
	}
	
	pipe := ps.redisClient.Pipeline()
	cmds := make([]*redis.StringStringMapCmd, len(nodeKeys))
	
	for i, key := range nodeKeys {
		cmds[i] = pipe.HGetAll(ctx, key)
	}
	
	_, err = pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}
	
	var nodes []*NodePresence
	for _, cmd := range cmds {
		result, err := cmd.Result()
		if err != nil {
			continue
		}
		
		for _, value := range result {
			var nodePresence NodePresence
			if err := json.Unmarshal([]byte(value), &nodePresence); err == nil {
				// Only include nodes with recent heartbeat (last 5 minutes)
				if time.Since(nodePresence.LastHeartbeat) < 5*time.Minute {
					nodes = append(nodes, &nodePresence)
				}
				break
			}
		}
	}
	
	return nodes, nil
}

func (ps *PresenceService) flushLoop() {
	defer ps.wg.Done()
	
	ps.flushTicker = time.NewTicker(flushInterval)
	defer ps.flushTicker.Stop()
	
	for {
		select {
		case <-ps.flushTicker.C:
			ps.flushBuffer()
		case <-ps.shutdown:
			ps.flushBuffer() // Final flush
			return
		}
	}
}

func (ps *PresenceService) flushBuffer() {
	ps.localBufferMu.Lock()
	if len(ps.localBuffer) == 0 {
		ps.localBufferMu.Unlock()
		return
	}
	
	// Copy buffer and clear
	batch := make(map[string]bool)
	for userID := range ps.localBuffer {
		batch[userID] = true
		delete(ps.localBuffer, userID)
	}
	ps.localBufferMu.Unlock()
	
	// Batch update Redis bitmap
	ctx := context.Background()
	pipe := ps.redisClient.Pipeline()
	
	for userID := range batch {
		userIDInt := ps.hashUserID(userID)
		pipe.SetBit(ctx, bitmapKey, userIDInt, 1)
	}
	
	pipe.Expire(ctx, bitmapKey, expiryDuration)
	
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("Failed to flush presence batch: %v", err)
		// Re-add failed items to buffer
		ps.localBufferMu.Lock()
		for userID := range batch {
			ps.localBuffer[userID] = true
		}
		ps.localBufferMu.Unlock()
	}
}

func (ps *PresenceService) hashUserID(userID string) int64 {
	// FNV-1a hash for better distribution
	hash := uint64(14695981039346656037)
	for _, c := range userID {
		hash ^= uint64(c)
		hash *= 1099511628211
	}
	return int64(hash % 100000000) // Mod by 100M for bitmap size
}

func (ps *PresenceService) Shutdown() {
	close(ps.shutdown)
	ps.wg.Wait()
}

// Cleanup expired presence entries
func (ps *PresenceService) CleanupExpiredPresence() error {
	ctx := context.Background()
	
	// This would typically be run as a background job
	// For now, we rely on Redis TTL to clean up expired keys
	
	// Clean up old node presence entries (nodes that haven't sent heartbeat in 1 hour)
	cutoff := time.Now().Add(-time.Hour)
	nodeKeys, err := ps.redisClient.Keys(ctx, "presence:node:*").Result()
	if err != nil {
		return err
	}
	
	for _, key := range nodeKeys {
		result, err := ps.redisClient.HGetAll(ctx, key).Result()
		if err != nil {
			continue
		}
		
		for _, value := range result {
			var nodePresence NodePresence
			if err := json.Unmarshal([]byte(value), &nodePresence); err == nil {
				if nodePresence.LastHeartbeat.Before(cutoff) {
					ps.redisClient.Del(ctx, key)
					break
				}
			}
		}
	}
	
	return nil
}
