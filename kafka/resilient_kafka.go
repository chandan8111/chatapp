package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/chatapp/pkg/resilience"
	"go.uber.org/zap"
)

// ResilientKafkaProducer wraps Kafka producer with circuit breaker and retry logic
type ResilientKafkaProducer struct {
	producer       sarama.SyncProducer
	circuitBreaker *resilience.CircuitBreaker
	bulkhead       *resilience.Bulkhead
	logger         *zap.Logger
}

// ResilientKafkaConsumer wraps Kafka consumer with circuit breaker and retry logic
type ResilientKafkaConsumer struct {
	consumer       sarama.ConsumerGroup
	circuitBreaker *resilience.CircuitBreaker
	bulkhead       *resilience.Bulkhead
	logger         *zap.Logger
}

// ResilientKafkaConfig holds configuration for resilient Kafka client
type ResilientKafkaConfig struct {
	Brokers          []string
	ConsumerGroup    string
	MaxRetries       int
	RetryDelay       time.Duration
	CircuitBreaker   resilience.CircuitBreakerConfig
	Bulkhead         resilience.BulkheadConfig
	Logger           *zap.Logger
}

// NewResilientKafkaProducer creates a new resilient Kafka producer
func NewResilientKafkaProducer(config ResilientKafkaConfig) (*ResilientKafkaProducer, error) {
	// Create Kafka config
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Producer.Return.Successes = true
	kafkaConfig.Producer.Return.Errors = true
	kafkaConfig.Producer.Flush.Frequency = 100 * time.Millisecond
	kafkaConfig.Producer.Flush.Messages = 100
	kafkaConfig.Producer.MaxMessageBytes = 10 * 1024 * 1024 // 10MB
	kafkaConfig.Producer.RequiredAcks = sarama.WaitForAll
	kafkaConfig.Producer.Retry.Max = config.MaxRetries
	kafkaConfig.Producer.Retry.Backoff = config.RetryDelay

	// Create producer
	producer, err := sarama.NewSyncProducer(config.Brokers, kafkaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	// Create circuit breaker
	circuitBreaker := resilience.NewCircuitBreaker(config.CircuitBreaker)

	// Create bulkhead
	bulkhead := resilience.NewBulkhead(config.Bulkhead)

	resilientProducer := &ResilientKafkaProducer{
		producer:       producer,
		circuitBreaker: circuitBreaker,
		bulkhead:       bulkhead,
		logger:         config.Logger,
	}

	config.Logger.Info("Resilient Kafka producer initialized",
		zap.Strings("brokers", config.Brokers),
	)

	return resilientProducer, nil
}

// NewResilientKafkaConsumer creates a new resilient Kafka consumer
func NewResilientKafkaConsumer(config ResilientKafkaConfig) (*ResilientKafkaConsumer, error) {
	// Create Kafka config
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Consumer.Return.Errors = true
	kafkaConfig.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	kafkaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	kafkaConfig.Consumer.Group.Session.Timeout = 10 * time.Second
	kafkaConfig.Consumer.Group.Heartbeat.Interval = 3 * time.Second

	// Create consumer
	consumer, err := sarama.NewConsumerGroup(config.Brokers, config.ConsumerGroup, kafkaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka consumer: %w", err)
	}

	// Create circuit breaker
	circuitBreaker := resilience.NewCircuitBreaker(config.CircuitBreaker)

	// Create bulkhead
	bulkhead := resilience.NewBulkhead(config.Bulkhead)

	resilientConsumer := &ResilientKafkaConsumer{
		consumer:       consumer,
		circuitBreaker: circuitBreaker,
		bulkhead:       bulkhead,
		logger:         config.Logger,
	}

	config.Logger.Info("Resilient Kafka consumer initialized",
		zap.Strings("brokers", config.Brokers),
		zap.String("consumer_group", config.ConsumerGroup),
	)

	return resilientConsumer, nil
}

// SendMessage sends a message to Kafka topic
func (p *ResilientKafkaProducer) SendMessage(ctx context.Context, topic string, key []byte, value []byte) (int32, int64, error) {
	var partition int32
	var offset int64
	
	err := p.circuitBreaker.Execute(ctx, func() error {
		return p.bulkhead.Execute(ctx, func() error {
			return resilience.Retry(ctx, resilience.RetryConfig{
				MaxRetries:    3,
				InitialDelay:  10 * time.Millisecond,
				MaxDelay:      100 * time.Millisecond,
				BackoffFactor: 2.0,
			}, func() error {
				msg := &sarama.ProducerMessage{
					Topic: topic,
					Key:   sarama.ByteEncoder(key),
					Value: sarama.ByteEncoder(value),
				}
				
				part, off, err := p.producer.SendMessage(msg)
				if err != nil {
					return err
				}
				
				partition = part
				offset = off
				return nil
			}, p.logger)
		})
	})
	
	if err != nil {
		p.logger.Error("Failed to send Kafka message",
			zap.String("topic", topic),
			zap.Error(err),
		)
	}
	
	return partition, offset, err
}

// SendMessages sends multiple messages to Kafka topics
func (p *ResilientKafkaProducer) SendMessages(ctx context.Context, messages []*sarama.ProducerMessage) error {
	err := p.circuitBreaker.Execute(ctx, func() error {
		return p.bulkhead.Execute(ctx, func() error {
			return resilience.Retry(ctx, resilience.RetryConfig{
				MaxRetries:    3,
				InitialDelay:  10 * time.Millisecond,
				MaxDelay:      100 * time.Millisecond,
				BackoffFactor: 2.0,
			}, func() error {
				return p.producer.SendMessages(messages)
			}, p.logger)
		})
	})
	
	if err != nil {
		p.logger.Error("Failed to send Kafka messages",
			zap.Int("message_count", len(messages)),
			zap.Error(err),
		)
	}
	
	return err
}

// Consume starts consuming messages from Kafka topics
func (c *ResilientKafkaConsumer) Consume(ctx context.Context, topics []string, handler sarama.ConsumerGroupHandler) error {
	return c.circuitBreaker.Execute(ctx, func() error {
		return c.bulkhead.Execute(ctx, func() error {
			return resilience.Retry(ctx, resilience.RetryConfig{
				MaxRetries:    3,
				InitialDelay:  10 * time.Millisecond,
				MaxDelay:      100 * time.Millisecond,
				BackoffFactor: 2.0,
			}, func() error {
				return c.consumer.Consume(ctx, topics, handler)
			}, c.logger)
		})
	})
}

// Close closes the Kafka producer
func (p *ResilientKafkaProducer) Close() error {
	return p.producer.Close()
}

// Close closes the Kafka consumer
func (c *ResilientKafkaConsumer) Close() error {
	return c.consumer.Close()
}

// GetMetrics returns Prometheus metrics for the resilient Kafka client
func (p *ResilientKafkaProducer) GetMetrics() []interface{} {
	metrics := []interface{}{}
	
	// Add circuit breaker metrics
	for _, metric := range p.circuitBreaker.GetMetrics() {
		metrics = append(metrics, metric)
	}
	
	// Add bulkhead metrics
	for _, metric := range p.bulkhead.GetMetrics() {
		metrics = append(metrics, metric)
	}
	
	return metrics
}

// GetMetrics returns Prometheus metrics for the resilient Kafka client
func (c *ResilientKafkaConsumer) GetMetrics() []interface{} {
	metrics := []interface{}{}
	
	// Add circuit breaker metrics
	for _, metric := range c.circuitBreaker.GetMetrics() {
		metrics = append(metrics, metric)
	}
	
	// Add bulkhead metrics
	for _, metric := range c.bulkhead.GetMetrics() {
		metrics = append(metrics, metric)
	}
	
	return metrics
}

// ResilientConsumerGroupHandler wraps consumer group handler with error handling
type ResilientConsumerGroupHandler struct {
	handler sarama.ConsumerGroupHandler
	logger  *zap.Logger
}

// NewResilientConsumerGroupHandler creates a new resilient consumer group handler
func NewResilientConsumerGroupHandler(handler sarama.ConsumerGroupHandler, logger *zap.Logger) *ResilientConsumerGroupHandler {
	return &ResilientConsumerGroupHandler{
		handler: handler,
		logger:  logger,
	}
}

// Setup implements ConsumerGroupHandler
func (h *ResilientConsumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	return h.handler.Setup(nil)
}

// Cleanup implements ConsumerGroupHandler
func (h *ResilientConsumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return h.handler.Cleanup(nil)
}

// ConsumeClaim implements ConsumerGroupHandler
func (h *ResilientConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("Panic in consumer claim handler",
				zap.Any("panic", r),
			)
		}
	}()
	
	return h.handler.ConsumeClaim(nil, claim)
}
