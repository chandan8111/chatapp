package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/chatapp/config"
	"github.com/chatapp/e2ee"
	"github.com/chatapp/gateway"
	"github.com/chatapp/kafka"
	"github.com/chatapp/presence"
	"github.com/chatapp/storage"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	configPath = flag.String("config", "", "Path to configuration file")
	version    = flag.Bool("version", false, "Show version information")
	help       = flag.Bool("help", false, "Show help information")
)

const (
	AppName    = "ChatApp Gateway"
	AppVersion = "1.0.0"
	GitCommit  = "unknown"
	BuildTime  = "unknown"
)

type Application struct {
	config      *config.Config
	logger      *zap.Logger
	gateway     *gateway.WebSocketGateway
	presence    *presence.PresenceService
	kafkaProducer *kafka.MessageProducer
	kafkaConsumer *kafka.MessageConsumer
	scyllaClient *storage.ScyllaDBClient
	e2eeService *e2ee.DoubleRatchet
	ctx         context.Context
	cancel      context.CancelFunc
}

func main() {
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	if *version {
		showVersion()
		return
	}

	app, err := NewApplication(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize application: %v\n", err)
		os.Exit(1)
	}

	if err := app.Run(); err != nil {
		app.logger.Error("Application failed", zap.Error(err))
		os.Exit(1)
	}
}

func NewApplication(configPath string) (*Application, error) {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize logger
	logger, err := initLogger(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	app := &Application{
		config: cfg,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	// Apply performance settings
	if err := app.applyPerformanceSettings(); err != nil {
		return nil, fmt.Errorf("failed to apply performance settings: %w", err)
	}

	// Initialize components
	if err := app.initializeComponents(); err != nil {
		return nil, fmt.Errorf("failed to initialize components: %w", err)
	}

	logger.Info("Application initialized successfully",
		zap.String("version", AppVersion),
		zap.String("commit", GitCommit),
		zap.String("build_time", BuildTime),
		zap.String("node_id", cfg.GetNodeID()),
	)

	return app, nil
}

func (app *Application) applyPerformanceSettings() error {
	cfg := app.config.Performance

	// Set GOMAXPROCS
	if cfg.GOMAXPROCS > 0 {
		runtime.GOMAXPROCS(cfg.GOMAXPROCS)
		app.logger.Info("Set GOMAXPROCS", zap.Int("value", cfg.GOMAXPROCS))
	}

	// Set GOGC
	if cfg.GOGC != "" {
		debug.SetGCPercent(parseGOGC(cfg.GOGC))
		app.logger.Info("Set GOGC", zap.String("value", cfg.GOGC))
	}

	// Set GOMEMLIMIT
	if cfg.GOMEMLIMIT != "" {
		debug.SetMemoryLimit(parseMemoryLimit(cfg.GOMEMLIMIT))
		app.logger.Info("Set GOMEMLIMIT", zap.String("value", cfg.GOMEMLIMIT))
	}

	// Enable profiling if requested
	if cfg.ProfileEnabled {
		go app.startProfiling(cfg.ProfilePort)
	}

	return nil
}

func (app *Application) initializeComponents() error {
	cfg := app.config

	// Initialize ScyllaDB client
	scyllaClient, err := storage.NewScyllaDBClient(cfg.ScyllaDB.Hosts, cfg.ScyllaDB.Keyspace)
	if err != nil {
		return fmt.Errorf("failed to initialize ScyllaDB client: %w", err)
	}
	app.scyllaClient = scyllaClient

	// Initialize Redis client (this will be done inside gateway and presence)
	// For now, we'll pass the configuration

	// Initialize Kafka producer
	kafkaProducer, err := kafka.NewMessageProducer(cfg.Kafka.Brokers)
	if err != nil {
		return fmt.Errorf("failed to initialize Kafka producer: %w", err)
	}
	app.kafkaProducer = kafkaProducer

	// Initialize Kafka consumer
	kafkaConsumer, err := kafka.NewMessageConsumer(cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroup)
	if err != nil {
		return fmt.Errorf("failed to initialize Kafka consumer: %w", err)
	}
	app.kafkaConsumer = kafkaConsumer

	// Initialize E2EE service
	identityKeyPair, err := e2ee.GenerateIdentityKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate identity key pair: %w", err)
	}
	app.e2eeService = e2ee.NewDoubleRatchet(identityKeyPair)

	// Initialize presence service
	presenceService := presence.NewPresenceService(cfg.Redis.Addr, cfg.GetNodeID())
	app.presence = presenceService

	// Initialize WebSocket gateway
	app.gateway = gateway.NewWebSocketGateway(cfg.Redis.Addr)

	// Register Kafka message handlers
	app.registerKafkaHandlers()

	return nil
}

func (app *Application) registerKafkaHandlers() {
	// Register chat message handler
	chatHandler := &kafka.ChatMessageHandler{
		FanoutService: app.gateway.GetFanoutService(),
	}
	app.kafkaConsumer.RegisterHandler(app.config.Kafka.Topics.ChatMessages, chatHandler)

	// Register delivery receipt handler
	receiptHandler := &kafka.DeliveryReceiptHandler{
		NotificationService: app.gateway.GetNotificationService(),
	}
	app.kafkaConsumer.RegisterHandler(app.config.Kafka.Topics.DeliveryReceipts, receiptHandler)

	// Register presence handler
	presenceHandler := &kafka.PresenceHandler{
		PresenceService: app.presence,
	}
	app.kafkaConsumer.RegisterHandler(app.config.Kafka.Topics.PresenceUpdates, presenceHandler)
}

func (app *Application) Run() error {
	// Start Kafka consumer
	go func() {
		app.logger.Info("Starting Kafka consumer")
		if err := app.kafkaConsumer.Start(app.ctx); err != nil {
			app.logger.Error("Kafka consumer error", zap.Error(err))
			app.cancel()
		}
	}()

	// Start presence service
	go func() {
		app.logger.Info("Starting presence service")
		if err := app.presence.Start(app.ctx); err != nil {
			app.logger.Error("Presence service error", zap.Error(err))
			app.cancel()
		}
	}()

	// Start WebSocket gateway
	app.logger.Info("Starting WebSocket gateway",
		zap.Int("port", app.config.Server.Port),
		zap.Int("max_connections", app.config.WebSocket.MaxConnections),
	)

	serverErr := make(chan error, 1)
	go func() {
		if err := app.gateway.Start(app.config.Server.Port); err != nil {
			serverErr <- fmt.Errorf("WebSocket gateway failed: %w", err)
		}
	}()

	// Wait for shutdown signal or error
	app.waitForShutdown(serverErr)

	return nil
}

func (app *Application) waitForShutdown(serverErr chan error) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		app.logger.Error("Server error", zap.Error(err))
	case sig := <-sigChan:
		app.logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
	case <-app.ctx.Done():
		app.logger.Info("Context cancelled")
	}

	app.shutdown()
}

func (app *Application) shutdown() {
	app.logger.Info("Starting graceful shutdown")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), app.config.Server.GracefulShutdownTimeout)
	defer shutdownCancel()

	// Cancel main context
	app.cancel()

	// Shutdown WebSocket gateway
	if app.gateway != nil {
		app.logger.Info("Shutting down WebSocket gateway")
		if err := app.gateway.Shutdown(shutdownCtx); err != nil {
			app.logger.Error("Error shutting down gateway", zap.Error(err))
		}
	}

	// Shutdown Kafka consumer
	if app.kafkaConsumer != nil {
		app.logger.Info("Shutting down Kafka consumer")
		if err := app.kafkaConsumer.Close(); err != nil {
			app.logger.Error("Error closing Kafka consumer", zap.Error(err))
		}
	}

	// Shutdown Kafka producer
	if app.kafkaProducer != nil {
		app.logger.Info("Shutting down Kafka producer")
		if err := app.kafkaProducer.Close(); err != nil {
			app.logger.Error("Error closing Kafka producer", zap.Error(err))
		}
	}

	// Shutdown presence service
	if app.presence != nil {
		app.logger.Info("Shutting down presence service")
		app.presence.Shutdown()
	}

	// Close ScyllaDB client
	if app.scyllaClient != nil {
		app.logger.Info("Closing ScyllaDB client")
		app.scyllaClient.Close()
	}

	// Sync logger
	app.logger.Info("Graceful shutdown completed")
	app.logger.Sync()
}

func (app *Application) startProfiling(port int) {
	app.logger.Info("Starting profiling server", zap.Int("port", port))
	// Implementation would start pprof HTTP server
	// For brevity, this is a placeholder
}

func initLogger(cfg *config.Config) (*zap.Logger, error) {
	level, err := cfg.GetLogLevel()
	if err != nil {
		return nil, err
	}

	var zapConfig zap.Config
	if cfg.Logging.Format == "json" {
		zapConfig = zap.NewProductionConfig()
	} else {
		zapConfig = zap.NewDevelopmentConfig()
	}

	zapConfig.Level = level
	zapConfig.OutputPaths = []string{cfg.Logging.Output}
	if cfg.Logging.Filename != "" {
		zapConfig.OutputPaths = append(zapConfig.OutputPaths, cfg.Logging.Filename)
	}

	logger, err := zapConfig.Build()
	if err != nil {
		return nil, err
	}

	return logger, nil
}

func parseGOGC(value string) int {
	if value == "off" {
		return -1
	}
	var gc int
	_, err := fmt.Sscanf(value, "%d", &gc)
	if err != nil {
		return 100 // default
	}
	return gc
}

func parseMemoryLimit(value string) int64 {
	// Parse memory limit string (e.g., "1GiB", "512MB")
	// This is a simplified implementation
	var limit int64
	var unit string
	_, err := fmt.Sscanf(value, "%d%s", &limit, &unit)
	if err != nil {
		return 0 // no limit
	}

	switch unit {
	case "KB", "K":
		limit *= 1024
	case "MB", "M":
		limit *= 1024 * 1024
	case "GB", "G":
		limit *= 1024 * 1024 * 1024
	case "TB", "T":
		limit *= 1024 * 1024 * 1024 * 1024
	}

	return limit
}

func showHelp() {
	fmt.Printf(`%s - Distributed Chat System Gateway

Usage:
  %s [options]

Options:
  -config string     Path to configuration file (default: config.yaml)
  -version           Show version information
  -help              Show this help message

Environment Variables:
  NODE_ID            Unique identifier for this node
  POD_NAME           Kubernetes pod name
  POD_IP             Kubernetes pod IP
  NAMESPACE          Kubernetes namespace
  CLUSTER_NAME       Cluster name
  REGION             Geographic region
  ENV                Environment (development, staging, production)

Examples:
  %s -config config.yaml
  %s -config /etc/chatapp/config.yaml

For more information, see the documentation.
`, AppName, os.Args[0], os.Args[0], os.Args[0])
}

func showVersion() {
	fmt.Printf(`%s
Version: %s
Git Commit: %s
Build Time: %s
Go Version: %s
OS/Arch: %s/%s
`, AppName, AppVersion, GitCommit, BuildTime, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
