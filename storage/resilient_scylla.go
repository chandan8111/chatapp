package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/chatapp/pkg/resilience"
	"github.com/gocql/gocql"
	"go.uber.org/zap"
)

// ResilientScyllaDBClient wraps ScyllaDB client with circuit breaker and retry logic
type ResilientScyllaDBClient struct {
	session        *gocql.Session
	circuitBreaker *resilience.CircuitBreaker
	bulkhead       *resilience.Bulkhead
	logger         *zap.Logger
}

// ResilientScyllaConfig holds configuration for resilient ScyllaDB client
type ResilientScyllaConfig struct {
	Hosts            []string
	Keyspace         string
	Username         string
	Password         string
	ConnectTimeout   time.Duration
	Timeout          time.Duration
	NumConns         int
	Consistency      gocql.Consistency
	CircuitBreaker   resilience.CircuitBreakerConfig
	Bulkhead         resilience.BulkheadConfig
	Logger           *zap.Logger
}

// NewResilientScyllaDBClient creates a new resilient ScyllaDB client
func NewResilientScyllaDBClient(config ResilientScyllaConfig) (*ResilientScyllaDBClient, error) {
	// Create cluster configuration
	clusterConfig := gocql.NewCluster(config.Hosts...)
	clusterConfig.Keyspace = config.Keyspace
	clusterConfig.Consistency = config.Consistency
	clusterConfig.ConnectTimeout = config.ConnectTimeout
	clusterConfig.Timeout = config.Timeout
	clusterConfig.NumConns = config.NumConns
	clusterConfig.PoolConfig.HostSelectionPolicy = gocql.DCAwareRoundRobinPolicy("")
	
	if config.Username != "" && config.Password != "" {
		clusterConfig.Authenticator = gocql.PasswordAuthenticator{
			Username: config.Username,
			Password: config.Password,
		}
	}

	// Create session
	session, err := clusterConfig.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create ScyllaDB session: %w", err)
	}

	// Create circuit breaker
	circuitBreaker := resilience.NewCircuitBreaker(config.CircuitBreaker)

	// Create bulkhead
	bulkhead := resilience.NewBulkhead(config.Bulkhead)

	resilientClient := &ResilientScyllaDBClient{
		session:        session,
		circuitBreaker: circuitBreaker,
		bulkhead:       bulkhead,
		logger:         config.Logger,
	}

	// Test connection
	if err := resilientClient.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to ScyllaDB: %w", err)
	}

	config.Logger.Info("Resilient ScyllaDB client initialized",
		zap.Strings("hosts", config.Hosts),
		zap.String("keyspace", config.Keyspace),
	)

	return resilientClient, nil
}

// Ping tests ScyllaDB connection
func (s *ResilientScyllaDBClient) Ping() error {
	ctx := context.Background()
	return s.circuitBreaker.Execute(ctx, func() error {
		return s.bulkhead.Execute(ctx, func() error {
			return resilience.Retry(ctx, resilience.RetryConfig{
				MaxRetries:    3,
				InitialDelay:  10 * time.Millisecond,
				MaxDelay:      100 * time.Millisecond,
				BackoffFactor: 2.0,
			}, func() error {
				return s.session.Query("SELECT now() FROM system.local").Exec()
			}, s.logger)
		})
	})
}

// Query executes a query and returns the result
func (s *ResilientScyllaDBClient) Query(stmt string, values ...interface{}) *gocql.Query {
	return s.session.Query(stmt, values...)
}

// QueryWithContext executes a query with context and circuit breaker protection
func (s *ResilientScyllaDBClient) QueryWithContext(ctx context.Context, stmt string, values ...interface{}) (*gocql.Query, error) {
	var query *gocql.Query
	err := s.circuitBreaker.Execute(ctx, func() error {
		return s.bulkhead.Execute(ctx, func() error {
			return resilience.Retry(ctx, resilience.RetryConfig{
				MaxRetries:    3,
				InitialDelay:  10 * time.Millisecond,
				MaxDelay:      100 * time.Millisecond,
				BackoffFactor: 2.0,
			}, func() error {
				query = s.session.Query(stmt, values...)
				return nil
			}, s.logger)
		})
	})
	
	if err != nil {
		s.logger.Error("Failed to create ScyllaDB query",
			zap.String("statement", stmt),
			zap.Error(err),
		)
		return nil, err
	}
	
	return query, nil
}

// Execute executes a query without returning results
func (s *ResilientScyllaDBClient) Execute(ctx context.Context, stmt string, values ...interface{}) error {
	return s.circuitBreaker.Execute(ctx, func() error {
		return s.bulkhead.Execute(ctx, func() error {
			return resilience.Retry(ctx, resilience.RetryConfig{
				MaxRetries:    3,
				InitialDelay:  10 * time.Millisecond,
				MaxDelay:      100 * time.Millisecond,
				BackoffFactor: 2.0,
			}, func() error {
				return s.session.Query(stmt, values...).Exec()
			}, s.logger)
		})
	})
}

// StoreMessage stores a message in ScyllaDB
func (s *ResilientScyllaDBClient) StoreMessage(ctx context.Context, message *Message) error {
	stmt := `
		INSERT INTO messages (
			conversation_id, bucket_id, message_id, sender_id, message_type,
			ciphertext, ephemeral_public_key, metadata, previous_message_sig,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	
	return s.Execute(ctx, stmt,
		message.ConversationID,
		message.BucketID,
		message.MessageID,
		message.SenderID,
		message.MessageType,
		message.Ciphertext,
		message.EphemeralPublicKey,
		message.Metadata,
		message.PreviousMessageSig,
		message.CreatedAt,
		message.UpdatedAt,
	)
}

// GetMessages retrieves messages from ScyllaDB
func (s *ResilientScyllaDBClient) GetMessages(ctx context.Context, conversationID string, bucketID int, limit int) ([]*Message, error) {
	stmt := `
		SELECT conversation_id, bucket_id, message_id, sender_id, message_type,
			   ciphertext, ephemeral_public_key, metadata, previous_message_sig,
			   created_at, updated_at
		FROM messages 
		WHERE conversation_id = ? AND bucket_id = ?
		ORDER BY created_at DESC
		LIMIT ?`
	
	query, err := s.QueryWithContext(ctx, stmt, conversationID, bucketID, limit)
	if err != nil {
		return nil, err
	}
	
	var messages []*Message
	iter := query.Iter()
	
	for {
		var msg Message
		if !iter.Scan(
			&msg.ConversationID,
			&msg.BucketID,
			&msg.MessageID,
			&msg.SenderID,
			&msg.MessageType,
			&msg.Ciphertext,
			&msg.EphemeralPublicKey,
			&msg.Metadata,
			&msg.PreviousMessageSig,
			&msg.CreatedAt,
			&msg.UpdatedAt,
		) {
			break
		}
		messages = append(messages, &msg)
	}
	
	if err := iter.Close(); err != nil {
		s.logger.Error("Failed to close ScyllaDB iterator",
			zap.String("conversation_id", conversationID),
			zap.Int("bucket_id", bucketID),
			zap.Error(err),
		)
	}
	
	return messages, nil
}

// UpdateConversation updates conversation metadata
func (s *ResilientScyllaDBClient) UpdateConversation(ctx context.Context, conversationID string, timestamp time.Time) error {
	stmt := `UPDATE conversations SET updated_at = ? WHERE conversation_id = ?`
	return s.Execute(ctx, stmt, timestamp, conversationID)
}

// StoreDeliveryReceipt stores a delivery receipt
func (s *ResilientScyllaDBClient) StoreDeliveryReceipt(ctx context.Context, receipt *DeliveryReceipt) error {
	stmt := `
		INSERT INTO delivery_receipts (message_id, user_id, timestamp, status)
		VALUES (?, ?, ?, ?)`
	
	return s.Execute(ctx, stmt,
		receipt.MessageID,
		receipt.UserID,
		receipt.Timestamp,
		receipt.Status,
	)
}

// GetDeliveryReceipts retrieves delivery receipts for a message
func (s *ResilientScyllaDBClient) GetDeliveryReceipts(ctx context.Context, messageID string) ([]*DeliveryReceipt, error) {
	stmt := `
		SELECT message_id, user_id, timestamp, status
		FROM delivery_receipts 
		WHERE message_id = ?`
	
	query, err := s.QueryWithContext(ctx, stmt, messageID)
	if err != nil {
		return nil, err
	}
	
	var receipts []*DeliveryReceipt
	iter := query.Iter()
	
	for {
		var receipt DeliveryReceipt
		if !iter.Scan(
			&receipt.MessageID,
			&receipt.UserID,
			&receipt.Timestamp,
			&receipt.Status,
		) {
			break
		}
		receipts = append(receipts, &receipt)
	}
	
	if err := iter.Close(); err != nil {
		s.logger.Error("Failed to close ScyllaDB iterator",
			zap.String("message_id", messageID),
			zap.Error(err),
		)
	}
	
	return receipts, nil
}

// UpdateMessageStatus updates message status
func (s *ResilientScyllaDBClient) UpdateMessageStatus(ctx context.Context, messageID string, status string) error {
	stmt := `UPDATE messages SET status = ? WHERE message_id = ?`
	return s.Execute(ctx, stmt, status, messageID)
}

// StorePresenceUpdate stores a presence update
func (s *ResilientScyllaDBClient) StorePresenceUpdate(ctx context.Context, update *PresenceUpdate) error {
	stmt := `
		INSERT INTO user_presence (user_id, online, last_seen, current_node_id, device_count)
		VALUES (?, ?, ?, ?, ?)`
	
	return s.Execute(ctx, stmt,
		update.UserID,
		update.Online,
		update.LastSeen,
		update.CurrentNodeID,
		update.DeviceCount,
	)
}

// Close closes the ScyllaDB session
func (s *ResilientScyllaDBClient) Close() {
	s.session.Close()
}

// GetMetrics returns Prometheus metrics for the resilient ScyllaDB client
func (s *ResilientScyllaDBClient) GetMetrics() []interface{} {
	metrics := []interface{}{}
	
	// Add circuit breaker metrics
	for _, metric := range s.circuitBreaker.GetMetrics() {
		metrics = append(metrics, metric)
	}
	
	// Add bulkhead metrics
	for _, metric := range s.bulkhead.GetMetrics() {
		metrics = append(metrics, metric)
	}
	
	return metrics
}

// DeliveryReceipt represents a delivery receipt
type DeliveryReceipt struct {
	MessageID string
	UserID    string
	Timestamp time.Time
	Status    string
}

// PresenceUpdate represents a presence update
type PresenceUpdate struct {
	UserID       string
	Online       bool
	LastSeen     time.Time
	CurrentNodeID string
	DeviceCount  int
}
