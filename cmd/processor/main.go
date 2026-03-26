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
	"github.com/chatapp/storage"
	"go.uber.org/zap"
)

func main() {
	var configFile = flag.String("config", "config/config.yaml", "Path to configuration file")
	var version = flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *version {
		fmt.Printf("ChatApp Message Processor v1.0.0\n")
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

	logger.Info("Starting ChatApp Message Processor",
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

	// Setup message processor
	processor := NewMessageProcessor(components, logger.WithComponent("message-processor"))

	// Start processor in goroutine
	processorErr := make(chan error, 1)
	go func() {
		if err := processor.Start(); err != nil {
			processorErr <- fmt.Errorf("message processor failed: %w", err)
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-processorErr:
		logger.Error("Processor error", zap.Error(err))
		os.Exit(1)
	case sig := <-sigCh:
		logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.Info("Shutting down message processor...")
	if err := processor.Stop(ctx); err != nil {
		logger.Error("Error during shutdown", zap.Error(err))
	}

	// Cleanup components
	cleanupComponents(components, logger)

	logger.Info("Message processor stopped")
}

// MessageProcessor handles message processing logic
type MessageProcessor struct {
	components *Components
	logger     *logging.Logger
	running    bool
	stopCh     chan struct{}
}

// NewMessageProcessor creates a new message processor
func NewMessageProcessor(components *Components, logger *logging.Logger) *MessageProcessor {
	return &MessageProcessor{
		components: components,
		logger:     logger,
		stopCh:     make(chan struct{}),
	}
}

// Start starts the message processor
func (mp *MessageProcessor) Start() error {
	mp.logger.Info("Starting message processor")
	mp.running = true

	// Register message handlers
	mp.components.KafkaConsumer.RegisterHandler("chat_messages", mp.handleChatMessage)
	mp.components.KafkaConsumer.RegisterHandler("delivery_receipts", mp.handleDeliveryReceipt)
	mp.components.KafkaConsumer.RegisterHandler("presence_updates", mp.handlePresenceUpdate)

	// Start consumer
	if err := mp.components.KafkaConsumer.Start(); err != nil {
		return fmt.Errorf("failed to start Kafka consumer: %w", err)
	}

	mp.logger.Info("Message processor started successfully")
	return nil
}

// Stop stops the message processor
func (mp *MessageProcessor) Stop(ctx context.Context) error {
	mp.logger.Info("Stopping message processor")
	mp.running = false
	close(mp.stopCh)

	// Stop consumer
	if err := mp.components.KafkaConsumer.Stop(); err != nil {
		return fmt.Errorf("failed to stop Kafka consumer: %w", err)
	}

	mp.logger.Info("Message processor stopped")
	return nil
}

// handleChatMessage processes chat messages
func (mp *MessageProcessor) handleChatMessage(ctx context.Context, message *kafka.Message) error {
	mp.logger.Debug("Processing chat message",
		zap.String("message_id", message.ID),
		zap.String("conversation_id", message.ConversationID),
	)

	// Store message in database
	if err := mp.components.ScyllaClient.StoreMessage(ctx, message); err != nil {
		mp.logger.Error("Failed to store message",
			zap.String("message_id", message.ID),
			zap.Error(err),
		)
		return err
	}

	// Update conversation metadata
	if err := mp.components.ScyllaClient.UpdateConversation(ctx, message.ConversationID, message.Timestamp); err != nil {
		mp.logger.Error("Failed to update conversation",
			zap.String("conversation_id", message.ConversationID),
			zap.Error(err),
		)
		// Don't return error here, message is already stored
	}

	// Send to fanout service
	if err := mp.sendToFanout(message); err != nil {
		mp.logger.Error("Failed to send to fanout",
			zap.String("message_id", message.ID),
			zap.Error(err),
		)
		// Don't return error here, message is already stored
	}

	mp.logger.Debug("Chat message processed successfully",
		zap.String("message_id", message.ID),
	)

	return nil
}

// handleDeliveryReceipt processes delivery receipts
func (mp *MessageProcessor) handleDeliveryReceipt(ctx context.Context, receipt *kafka.DeliveryReceipt) error {
	mp.logger.Debug("Processing delivery receipt",
		zap.String("message_id", receipt.MessageID),
		zap.String("user_id", receipt.UserID),
		zap.String("status", receipt.Status),
	)

	// Store receipt in database
	if err := mp.components.ScyllaClient.StoreDeliveryReceipt(ctx, receipt); err != nil {
		mp.logger.Error("Failed to store delivery receipt",
			zap.String("message_id", receipt.MessageID),
			zap.String("user_id", receipt.UserID),
			zap.Error(err),
		)
		return err
	}

	// Update message status if all receipts are received
	if err := mp.updateMessageStatus(ctx, receipt.MessageID); err != nil {
		mp.logger.Error("Failed to update message status",
			zap.String("message_id", receipt.MessageID),
			zap.Error(err),
		)
		// Don't return error here, receipt is already stored
	}

	return nil
}

// handlePresenceUpdate processes presence updates
func (mp *MessageProcessor) handlePresenceUpdate(ctx context.Context, update *kafka.PresenceUpdate) error {
	mp.logger.Debug("Processing presence update",
		zap.String("user_id", update.UserID),
		zap.String("status", update.Status),
	)

	// Store presence update in database
	if err := mp.components.ScyllaClient.StorePresenceUpdate(ctx, update); err != nil {
		mp.logger.Error("Failed to store presence update",
			zap.String("user_id", update.UserID),
			zap.Error(err),
		)
		return err
	}

	return nil
}

// sendToFanout sends message to fanout service
func (mp *MessageProcessor) sendToFanout(message *kafka.Message) error {
	// Create fanout message
	fanoutMessage := &kafka.FanoutMessage{
		MessageID:      message.ID,
		ConversationID: message.ConversationID,
		SenderID:       message.SenderID,
		Timestamp:      message.Timestamp,
		Content:        message.Content,
		MessageType:    message.MessageType,
	}

	// Send to fanout topic
	return mp.components.KafkaProducer.Produce("fanout_messages", fanoutMessage)
}

// updateMessageStatus updates message status based on receipts
func (mp *MessageProcessor) updateMessageStatus(ctx context.Context, messageID string) error {
	// Get all receipts for the message
	receipts, err := mp.components.ScyllaClient.GetDeliveryReceipts(ctx, messageID)
	if err != nil {
		return err
	}

	// Determine overall status
	status := "sent"
	allDelivered := true
	allRead := true

	for _, receipt := range receipts {
		if receipt.Status != "delivered" && receipt.Status != "read" {
			allDelivered = false
		}
		if receipt.Status != "read" {
			allRead = false
		}
	}

	if allRead {
		status = "read"
	} else if allDelivered {
		status = "delivered"
	}

	// Update message status
	return mp.components.ScyllaClient.UpdateMessageStatus(ctx, messageID, status)
}

// Components holds all initialized services
type Components struct {
	ScyllaClient  *storage.ScyllaClient
	KafkaProducer *kafka.MessageProducer
	KafkaConsumer *kafka.MessageConsumer
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
