package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/chatapp/config"
	"github.com/chatapp/kafka"
	"github.com/chatapp/logging"
	"github.com/chatapp/presence"
	"github.com/chatapp/storage"
	"go.uber.org/zap"
)

func main() {
	var configFile = flag.String("config", "config/config.yaml", "Path to configuration file")
	var version = flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *version {
		fmt.Printf("ChatApp Fanout Service v1.0.0\n")
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger, err := logging.NewLogger(cfg.Logging)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting ChatApp Fanout Service",
		zap.String("version", "1.0.0"),
		zap.String("environment", cfg.Environment),
	)

	// Apply performance settings
	applyPerformanceSettings(logger)

	// Initialize components
	components, err := initializeComponents(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialize components", zap.Error(err))
	}

	// Setup fanout service
	fanoutService := NewFanoutService(components, logger.WithComponent("fanout-service"))

	// Start service in goroutine
	serviceErr := make(chan error, 1)
	go func() {
		if err := fanoutService.Start(); err != nil {
			serviceErr <- fmt.Errorf("fanout service failed: %w", err)
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serviceErr:
		logger.Error("Service error", zap.Error(err))
		os.Exit(1)
	case sig := <-sigCh:
		logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.Info("Shutting down fanout service...")
	if err := fanoutService.Stop(ctx); err != nil {
		logger.Error("Error during shutdown", zap.Error(err))
	}

	// Cleanup components
	cleanupComponents(components, logger)

	logger.Info("Fanout service stopped")
}

// FanoutService handles message distribution to connected users
type FanoutService struct {
	components *Components
	logger     *logging.Logger
	running    bool
	stopCh     chan struct{}
}

// NewFanoutService creates a new fanout service
func NewFanoutService(components *Components, logger *logging.Logger) *FanoutService {
	return &FanoutService{
		components: components,
		logger:     logger,
		stopCh:     make(chan struct{}),
	}
}

// Start starts the fanout service
func (fs *FanoutService) Start() error {
	fs.logger.Info("Starting fanout service")
	fs.running = true

	// Register message handlers
	fs.components.KafkaConsumer.RegisterHandler("fanout_messages", fs.handleFanoutMessage)

	// Start consumer
	if err := fs.components.KafkaConsumer.Start(); err != nil {
		return fmt.Errorf("failed to start Kafka consumer: %w", err)
	}

	// Start cleanup goroutine
	go fs.cleanupRoutine()

	fs.logger.Info("Fanout service started successfully")
	return nil
}

// Stop stops the fanout service
func (fs *FanoutService) Stop(ctx context.Context) error {
	fs.logger.Info("Stopping fanout service")
	fs.running = false
	close(fs.stopCh)

	// Stop consumer
	if err := fs.components.KafkaConsumer.Stop(); err != nil {
		return fmt.Errorf("failed to stop Kafka consumer: %w", err)
	}

	fs.logger.Info("Fanout service stopped")
	return nil
}

// handleFanoutMessage processes fanout messages
func (fs *FanoutService) handleFanoutMessage(ctx context.Context, message *kafka.FanoutMessage) error {
	fs.logger.Debug("Processing fanout message",
		zap.String("message_id", message.MessageID),
		zap.String("conversation_id", message.ConversationID),
	)

	// Get conversation participants
	participants, err := fs.components.ScyllaClient.GetConversationParticipants(ctx, message.ConversationID)
	if err != nil {
		fs.logger.Error("Failed to get conversation participants",
			zap.String("conversation_id", message.ConversationID),
			zap.Error(err),
		)
		return err
	}

	// Check if this is a celebrity conversation
	isCelebrity := len(participants) > fs.components.Presence.GetCelebrityThreshold()

	if isCelebrity {
		// Use hybrid push/pull model for celebrity conversations
		return fs.handleCelebrityMessage(ctx, message, participants)
	} else {
		// Use direct push for regular conversations
		return fs.handleRegularMessage(ctx, message, participants)
	}
}

// handleRegularMessage handles messages for regular conversations (direct push)
func (fs *FanoutService) handleRegularMessage(ctx context.Context, message *kafka.FanoutMessage, participants []string) error {
	fs.logger.Debug("Handling regular message",
		zap.String("message_id", message.MessageID),
		zap.Int("participants", len(participants)),
	)

	// Get online users from presence service
	onlineUsers, err := fs.components.Presence.GetOnlineUsersInBatch(ctx, participants)
	if err != nil {
		fs.logger.Error("Failed to get online users",
			zap.String("message_id", message.MessageID),
			zap.Error(err),
		)
		return err
	}

	// Send to online users
	for _, participant := range participants {
		if online, exists := onlineUsers[participant]; exists && online {
			if err := fs.sendToUser(ctx, participant, message); err != nil {
				fs.logger.Error("Failed to send message to user",
					zap.String("message_id", message.MessageID),
					zap.String("user_id", participant),
					zap.Error(err),
				)
				// Continue with other users
			}
		}
	}

	return nil
}

// handleCelebrityMessage handles messages for celebrity conversations (hybrid push/pull)
func (fs *FanoutService) handleCelebrityMessage(ctx context.Context, message *kafka.FanoutMessage, participants []string) error {
	fs.logger.Debug("Handling celebrity message",
		zap.String("message_id", message.MessageID),
		zap.Int("participants", len(participants)),
	)

	// Store message in Redis for pull-based access
	if err := fs.storeMessageForPull(ctx, message); err != nil {
		fs.logger.Error("Failed to store message for pull",
			zap.String("message_id", message.MessageID),
			zap.Error(err),
		)
		return err
	}

	// Get online users from presence service
	onlineUsers, err := fs.components.Presence.GetOnlineUsersInBatch(ctx, participants)
	if err != nil {
		fs.logger.Error("Failed to get online users",
			zap.String("message_id", message.MessageID),
			zap.Error(err),
		)
		return err
	}

	// Send push notifications to a subset of online users (e.g., recent active users)
	pushUsers := fs.selectPushUsers(participants, onlineUsers, 1000) // Limit to 1000 users

	for _, participant := range pushUsers {
		if online, exists := onlineUsers[participant]; exists && online {
			if err := fs.sendToUser(ctx, participant, message); err != nil {
				fs.logger.Error("Failed to send push notification to user",
					zap.String("message_id", message.MessageID),
					zap.String("user_id", participant),
					zap.Error(err),
				)
				// Continue with other users
			}
		}
	}

	// Send notification to other users that new messages are available
	if err := fs.notifyNewMessages(ctx, message, participants); err != nil {
		fs.logger.Error("Failed to notify new messages",
			zap.String("message_id", message.MessageID),
			zap.Error(err),
		)
		// Don't return error here, message is already stored
	}

	return nil
}

// sendToUser sends a message to a specific user
func (fs *FanoutService) sendToUser(ctx context.Context, userID string, message *kafka.FanoutMessage) error {
	// Get user's active connections
	connections, err := fs.components.Presence.GetUserConnections(ctx, userID)
	if err != nil {
		return err
	}

	// Send to all active connections
	for _, connection := range connections {
		if err := fs.sendToConnection(connection, message); err != nil {
			fs.logger.Error("Failed to send to connection",
				zap.String("message_id", message.MessageID),
				zap.String("user_id", userID),
				zap.String("connection_id", connection),
				zap.Error(err),
			)
		}
	}

	return nil
}

// sendToConnection sends a message to a specific connection
func (fs *FanoutService) sendToConnection(connectionID string, message *kafka.FanoutMessage) error {
	// This would typically send the message through the WebSocket gateway
	// For now, we'll produce a message to a topic for the gateway to consume
	gatewayMessage := &kafka.GatewayMessage{
		ConnectionID: connectionID,
		MessageID:    message.MessageID,
		Content:      message.Content,
		MessageType:  message.MessageType,
		Timestamp:    message.Timestamp,
	}

	return fs.components.KafkaProducer.Produce("gateway_messages", gatewayMessage)
}

// storeMessageForPull stores a message for pull-based access
func (fs *FanoutService) storeMessageForPull(ctx context.Context, message *kafka.FanoutMessage) error {
	// Store in Redis with TTL
	key := fmt.Sprintf("conversation:%s:messages", message.ConversationID)
	
	// This would use the Redis client to store the message
	// For now, we'll just log it
	fs.logger.Debug("Storing message for pull",
		zap.String("key", key),
		zap.String("message_id", message.MessageID),
	)

	return nil
}

// selectPushUsers selects users to receive push notifications
func (fs *FanoutService) selectPushUsers(participants []string, onlineUsers map[string]bool, limit int) []string {
	var pushUsers []string
	count := 0

	// Select users based on activity level (recent messages, etc.)
	for _, participant := range participants {
		if count >= limit {
			break
		}

		if online, exists := onlineUsers[participant]; exists && online {
			pushUsers = append(pushUsers, participant)
			count++
		}
	}

	return pushUsers
}

// notifyNewMessages notifies users that new messages are available
func (fs *FanoutService) notifyNewMessages(ctx context.Context, message *kafka.FanoutMessage, participants []string) error {
	// Send notification to all participants
	notification := &kafka.NotificationMessage{
		Type:           "new_messages",
		ConversationID: message.ConversationID,
		MessageID:      message.MessageID,
		Timestamp:      message.Timestamp,
	}

	for _, participant := range participants {
		notification.UserID = participant
		if err := fs.components.KafkaProducer.Produce("notifications", notification); err != nil {
			fs.logger.Error("Failed to send notification",
				zap.String("message_id", message.MessageID),
				zap.String("user_id", participant),
				zap.Error(err),
			)
		}
	}

	return nil
}

// cleanupRoutine performs periodic cleanup
func (fs *FanoutService) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-fs.stopCh:
			return
		case <-ticker.C:
			fs.performCleanup()
		}
	}
}

// performCleanup performs cleanup operations
func (fs *FanoutService) performCleanup() {
	fs.logger.Info("Performing cleanup operations")

	// Clean up expired messages from Redis
	// This would be implemented based on the actual Redis client

	fs.logger.Info("Cleanup completed")
}

// Components holds all initialized services
type Components struct {
	ScyllaClient *storage.ScyllaClient
	KafkaProducer *kafka.MessageProducer
	KafkaConsumer *kafka.MessageConsumer
	Presence     *presence.Service
}

// initializeComponents initializes all required services
func initializeComponents(cfg *config.Config, logger *logging.Logger) (*Components, error) {
	components := &Components{}

	// Initialize ScyllaDB client
	logger.Info("Initializing ScyllaDB client...")
	scyllaClient, err := storage.NewScyllaClient(cfg.ScyllaDB, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ScyllaDB client: %w", err)
	}
	components.ScyllaClient = scyllaClient

	// Initialize Kafka producer
	logger.Info("Initializing Kafka producer...")
	kafkaProducer, err := kafka.NewMessageProducer(cfg.Kafka, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Kafka producer: %w", err)
	}
	components.KafkaProducer = kafkaProducer

	// Initialize Kafka consumer
	logger.Info("Initializing Kafka consumer...")
	kafkaConsumer, err := kafka.NewMessageConsumer(cfg.Kafka, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Kafka consumer: %w", err)
	}
	components.KafkaConsumer = kafkaConsumer

	// Initialize Presence service
	logger.Info("Initializing Presence service...")
	presenceService, err := presence.NewService(cfg.Redis, cfg.Presence, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Presence service: %w", err)
	}
	components.Presence = presenceService

	logger.Info("All components initialized successfully")
	return components, nil
}

// cleanupComponents performs cleanup of all components
func cleanupComponents(components *Components, logger *logging.Logger) {
	logger.Info("Cleaning up components...")

	if components.KafkaConsumer != nil {
		if err := components.KafkaConsumer.Close(); err != nil {
			logger.Error("Error closing Kafka consumer", zap.Error(err))
		}
	}

	if components.KafkaProducer != nil {
		if err := components.KafkaProducer.Close(); err != nil {
			logger.Error("Error closing Kafka producer", zap.Error(err))
		}
	}

	if components.ScyllaClient != nil {
		if err := components.ScyllaClient.Close(); err != nil {
			logger.Error("Error closing ScyllaDB client", zap.Error(err))
		}
	}

	if components.Presence != nil {
		components.Presence.Shutdown()
	}

	logger.Info("Components cleanup completed")
}

// applyPerformanceSettings applies Go runtime performance settings
func applyPerformanceSettings(logger *logging.Logger) {
	// Set GOMAXPROCS to number of available CPUs
	if gomaxprocs := runtime.GOMAXPROCS(0); gomaxprocs != runtime.NumCPU() {
		runtime.GOMAXPROCS(runtime.NumCPU())
		logger.Info("Updated GOMAXPROCS", zap.Int("value", runtime.NumCPU()))
	}

	// Set GC target percentage
	runtime.SetGCPercent(100)

	// Set memory limit if specified
	if memLimit := os.Getenv("GOMEMLIMIT"); memLimit != "" {
		runtime.SetMemoryProfileRate(1)
		logger.Info("Memory limit set", zap.String("limit", memLimit))
	}

	logger.Info("Performance settings applied",
		zap.Int("gomaxprocs", runtime.GOMAXPROCS(0)),
		zap.Int("num_cpu", runtime.NumCPU()),
		zap.Int("gc_percent", 100),
	)
}
