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
		fmt.Printf("ChatApp Presence Service v1.0.0\n")
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

	logger.Info("Starting ChatApp Presence Service",
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

	// Setup presence service
	presenceService := components.Presence

	// Start service in goroutine
	serviceErr := make(chan error, 1)
	go func() {
		if err := presenceService.Start(); err != nil {
			serviceErr <- fmt.Errorf("presence service failed: %w", err)
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

	logger.Info("Shutting down presence service...")
	if err := presenceService.Stop(ctx); err != nil {
		logger.Error("Error during shutdown", zap.Error(err))
	}

	// Cleanup components
	cleanupComponents(components, logger)

	logger.Info("Presence service stopped")
}

// Components holds all initialized services
type Components struct {
	ScyllaClient *storage.ScyllaClient
	KafkaProducer *kafka.MessageProducer
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

	// Initialize Presence service
	logger.Info("Initializing Presence service...")
	presenceService, err := presence.NewService(cfg.Redis, cfg.Presence, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Presence service: %w", err)
	}
	
	// Set dependencies
	presenceService.SetScyllaClient(scyllaClient)
	presenceService.SetKafkaProducer(kafkaProducer)
	
	components.Presence = presenceService

	logger.Info("All components initialized successfully")
	return components, nil
}

// cleanupComponents performs cleanup of all components
func cleanupComponents(components *Components, logger *logging.Logger) {
	logger.Info("Cleaning up components...")

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
