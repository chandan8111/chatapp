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

	"github.com/chatapp/api"
	"github.com/chatapp/config"
	"github.com/chatapp/e2ee"
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
		fmt.Printf("ChatApp API Server v1.0.0\n")
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

	logger.Info("Starting ChatApp API Server",
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

	// Setup API server
	apiServer := api.NewAPIServer(cfg.Server.Port, logger.WithService("api-server"))
	
	// Initialize handlers
	apiServer.PresenceHandler = api.NewPresenceHandler(components.Presence, logger.Logger)
	apiServer.MessageHandler = api.NewMessageHandler(components.MessageService, logger.Logger)
	apiServer.UserHandler = api.NewUserHandler(components.UserService, logger.Logger)
	apiServer.ConversationHandler = api.NewConversationHandler(components.ConversationService, logger.Logger)
	apiServer.AnalyticsHandler = api.NewAnalyticsHandler(components.AnalyticsService, logger.Logger)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := apiServer.Start(); err != nil {
			serverErr <- fmt.Errorf("API server failed: %w", err)
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		logger.Error("Server error", zap.Error(err))
		os.Exit(1)
	case sig := <-sigCh:
		logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.Info("Shutting down API server...")
	if err := apiServer.Stop(ctx); err != nil {
		logger.Error("Error during shutdown", zap.Error(err))
	}

	// Cleanup components
	cleanupComponents(components, logger)

	logger.Info("API server stopped")
}

// Components holds all initialized services
type Components struct {
	ScyllaClient       *storage.ScyllaClient
	KafkaProducer      *kafka.MessageProducer
	KafkaConsumer      *kafka.MessageConsumer
	Presence           *presence.Service
	E2EE               *e2ee.DoubleRatchetService
	MessageService     interface{}
	UserService        interface{}
	ConversationService interface{}
	AnalyticsService   interface{}
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

	// Initialize E2EE service
	logger.Info("Initializing E2EE service...")
	e2eeService, err := e2ee.NewDoubleRatchetService(cfg.E2EE, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize E2EE service: %w", err)
	}
	components.E2EE = e2eeService

	// Initialize other services (mock implementations for now)
	components.MessageService = &mockMessageService{scylla: scyllaClient, kafka: kafkaProducer}
	components.UserService = &mockUserService{scylla: scyllaClient}
	components.ConversationService = &mockConversationService{scylla: scyllaClient}
	components.AnalyticsService = &mockAnalyticsService{logger: logger}

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

// Mock service implementations (would be replaced with real implementations)

type mockMessageService struct {
	scylla *storage.ScyllaClient
	kafka  *kafka.MessageProducer
}

type mockUserService struct {
	scylla *storage.ScyllaClient
}

type mockConversationService struct {
	scylla *storage.ScyllaClient
}

type mockAnalyticsService struct {
	logger *logging.Logger
}
