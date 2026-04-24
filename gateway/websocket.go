package gateway

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"

	"github.com/chatapp/proto"
)

const (
	maxConnectionsPerNode = 200000
	writeWait            = 10 * time.Second
	pongWait             = 60 * time.Second
	pingPeriod           = (pongWait * 9) / 10
	maxMessageSize       = 8192
	heartbeatBatchSize   = 1000
	heartbeatFlushInterval = 5 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Production: implement proper origin checking
	},
}

type Connection struct {
	ws       *websocket.Conn
	userID   string
	nodeID   string
	deviceID string
	send     chan []byte
	mu       sync.Mutex
}

type Hub struct {
	connections    map[string]*Connection // userID -> Connection
	register       chan *Connection
	unregister     chan *Connection
	broadcast      chan []byte
	mu             sync.RWMutex
	connectionCount int64
	redisClient    *redis.Client
	heartbeatBatch map[string]bool // userID batch for Redis
	heartbeatMu    sync.Mutex
}

type PresenceService struct {
	redisClient *redis.Client
	hub         *Hub
}

type WebSocketGateway struct {
	hub      *Hub
	presence *PresenceService
	server   *http.Server
}

func NewHub(redisClient *redis.Client) *Hub {
	return &Hub{
		connections:    make(map[string]*Connection),
		register:       make(chan *Connection),
		unregister:     make(chan *Connection),
		broadcast:      make(chan []byte),
		redisClient:    redisClient,
		heartbeatBatch: make(map[string]bool),
	}
}

func NewPresenceService(redisClient *redis.Client, hub *Hub) *PresenceService {
	return &PresenceService{
		redisClient: redisClient,
		hub:         hub,
	}
}

func (h *Hub) run() {
	ticker := time.NewTicker(heartbeatFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			if existingConn, exists := h.connections[conn.userID]; exists {
				existingConn.mu.Lock()
				existingConn.ws.Close()
				existingConn.mu.Unlock()
			}
			h.connections[conn.userID] = conn
			atomic.AddInt64(&h.connectionCount, 1)
			h.mu.Unlock()

			// Mark user online in Redis bitmap
			h.markUserOnline(conn.userID)
			log.Printf("User %s connected. Total connections: %d", conn.userID, atomic.LoadInt64(&h.connectionCount))

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.connections[conn.userID]; ok {
				delete(h.connections, conn.userID)
				atomic.AddInt64(&h.connectionCount, -1)
				conn.mu.Lock()
				conn.ws.Close()
				conn.mu.Unlock()
				close(conn.send)
			}
			h.mu.Unlock()

			// Mark user offline in Redis bitmap
			h.markUserOffline(conn.userID)
			log.Printf("User %s disconnected. Total connections: %d", conn.userID, atomic.LoadInt64(&h.connectionCount))

		case message := <-h.broadcast:
			h.mu.Lock()
			for _, conn := range h.connections {
				select {
				case conn.send <- message:
				default:
					close(conn.send)
					delete(h.connections, conn.userID)
				}
			}
			h.mu.Unlock()

		case <-ticker.C:
			h.flushHeartbeatBatch()
		}
	}
}

func (h *Hub) markUserOnline(userID string) {
	h.heartbeatMu.Lock()
	h.heartbeatBatch[userID] = true
	h.heartbeatMu.Unlock()
}

func (h *Hub) markUserOffline(userID string) {
	ctx := context.Background()
	userIDInt := hashUserID(userID)
	
	// Use Redis BITMAP to track online status (100M users = 12.5MB bitmap)
	bitmapKey := "presence:online"
	h.redisClient.SetBit(ctx, bitmapKey, userIDInt, 0)
}

func (h *Hub) flushHeartbeatBatch() {
	h.heartbeatMu.Lock()
	if len(h.heartbeatBatch) == 0 {
		h.heartbeatMu.Unlock()
		return
	}

	// Copy batch and clear
	batch := make(map[string]bool)
	for userID := range h.heartbeatBatch {
		batch[userID] = true
		delete(h.heartbeatBatch, userID)
	}
	h.heartbeatMu.Unlock()

	// Batch update Redis bitmap
	ctx := context.Background()
	pipe := h.redisClient.Pipeline()
	bitmapKey := "presence:online"
	
	for userID := range batch {
		userIDInt := hashUserID(userID)
		pipe.SetBit(ctx, bitmapKey, userIDInt, 1)
	}
	
	// Set expiration for bitmap (24 hours)
	pipe.Expire(ctx, bitmapKey, 24*time.Hour)
	
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("Failed to flush heartbeat batch: %v", err)
	}
}

func hashUserID(userID string) int64 {
	// Simple hash function - production should use better distribution
	hash := int64(0)
	for _, c := range userID {
		hash = hash*31 + int64(c)
	}
	return hash % 100000000 // Mod by 100M for bitmap size
}

func (c *Connection) readPump(hub *Hub) {
	defer func() {
		hub.unregister <- c
		c.ws.Close()
	}()

	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Process incoming message
		c.handleMessage(message, hub)
	}
}

func (c *Connection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.ws.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Connection) handleMessage(message []byte, hub *Hub) {
	// Parse protobuf message
	var chatMsg proto.ChatMessage
	if err := proto.Unmarshal(message, &chatMsg); err != nil {
		log.Printf("Failed to unmarshal message: %v", err)
		return
	}

	// Validate message
	if chatMsg.GetSenderId() != c.userID {
		log.Printf("Message sender mismatch: expected %s, got %s", c.userID, chatMsg.GetSenderId())
		return
	}

	// Forward to Kafka producer (to be implemented)
	// For now, broadcast to all connections in the conversation
	hub.broadcast <- message
}

func (ws *WebSocketGateway) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	deviceID := r.URL.Query().Get("device_id")
	nodeID := r.URL.Query().Get("node_id")

	if userID == "" || deviceID == "" || nodeID == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	// Check connection limit
	if atomic.LoadInt64(&ws.hub.connectionCount) >= maxConnectionsPerNode {
		http.Error(w, "Server at capacity", http.StatusServiceUnavailable)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	connection := &Connection{
		ws:       conn,
		userID:   userID,
		deviceID: deviceID,
		nodeID:   nodeID,
		send:     make(chan []byte, 256),
	}

	ws.hub.register <- connection

	go connection.writePump()
	go connection.readPump(ws.hub)
}

func NewWebSocketGateway(redisAddr string) *WebSocketGateway {
	// Configure Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:            redisAddr,
		MaxRetries:      3,
		PoolSize:        100,
		MinIdleConns:    10,
		MaxConnAge:      time.Hour,
		ReadTimeout:     100 * time.Millisecond,
		WriteTimeout:    100 * time.Millisecond,
		PoolTimeout:     30 * time.Second,
		IdleTimeout:     5 * time.Minute,
		IdleCheckFrequency: 1 * time.Minute,
	})

	hub := NewHub(redisClient)
	presence := NewPresenceService(redisClient, hub)

	// Start hub in background
	go hub.run()

	return &WebSocketGateway{
		hub:      hub,
		presence: presence,
	}
}

func (ws *WebSocketGateway) Start(port int) error {
	// Optimize Go runtime for high concurrency
	runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.GC()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws.handleWebSocket)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "healthy",
			"connections":     atomic.LoadInt64(&ws.hub.connectionCount),
			"max_connections": maxConnectionsPerNode,
			"goroutines":      runtime.NumGoroutine(),
		})
	})

	ws.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	log.Printf("WebSocket Gateway starting on port %d", port)
	return ws.server.ListenAndServe()
}

func main() {
	redisAddr := "localhost:6379" // Configure from environment in production
	gateway := NewWebSocketGateway(redisAddr)
	
	if err := gateway.Start(8080); err != nil {
		log.Fatalf("Failed to start WebSocket Gateway: %v", err)
	}
}
