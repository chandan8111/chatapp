package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for demo
	},
}

type Hub struct {
	connections map[string]*websocket.Conn
	register    chan *websocket.Conn
	unregister  chan *websocket.Conn
	broadcast   chan []byte
	redisClient *redis.Client
}

func NewHub(redisAddr string) *Hub {
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	return &Hub{
		connections: make(map[string]*websocket.Conn),
		register:    make(chan *websocket.Conn),
		unregister:  make(chan *websocket.Conn),
		broadcast:   make(chan []byte),
		redisClient: redisClient,
	}
}

func (h *Hub) run() {
	for {
		select {
		case <-h.register:
			// Handle new connection
			log.Println("New connection registered")

		case <-h.unregister:
			// Handle disconnection
			log.Println("Connection unregistered")

		case message := <-h.broadcast:
			// Broadcast message to all connections
			for _, conn := range h.connections {
				err := conn.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					log.Printf("Error writing message: %v", err)
					conn.Close()
					delete(h.connections, "user")
				}
			}
		}
	}
}

func handleWebSocket(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Upgrade HTTP connection to WebSocket
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		// Register connection
		hub.register <- conn

		// Handle messages
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Read error: %v", err)
				break
			}

			if messageType == websocket.TextMessage {
				log.Printf("Received message: %s", string(message))

				// Echo message back for demo
				hub.broadcast <- message
			}
		}

		// Unregister connection
		hub.unregister <- conn
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "healthy",
		"connections": 0,
		"timestamp":   time.Now().Unix(),
	})
}

func main() {
	// Initialize hub
	hub := NewHub("localhost:6379")
	go hub.run()

	// Setup HTTP routes
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/ws", handleWebSocket(hub))

	// Start server
	port := ":8080"
	log.Printf("Starting WebSocket Gateway on %s", port)
	log.Printf("Health check: http://localhost:8080/health")
	log.Printf("WebSocket endpoint: ws://localhost:8080/ws")

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
