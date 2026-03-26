package kafka

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"google.golang.org/protobuf/proto"
)

const (
	chatMessagesTopic      = "chat-messages"
	deliveryReceiptsTopic  = "delivery-receipts"
	presenceUpdatesTopic   = "presence-updates"
	numPartitions         = 100
	replicationFactor      = 3
	producerFlushFrequency = 100 * time.Millisecond
	producerFlushMessages  = 100
	batchSize             = 50
	batchTimeout          = 10 * time.Millisecond
)

type MessageProducer struct {
	producer sarama.SyncProducer
	config   *sarama.Config
	mu       sync.RWMutex
}

type MessageConsumer struct {
	consumer sarama.ConsumerGroup
	config   *sarama.Config
	handlers map[string]MessageHandler
	mu       sync.RWMutex
}

type MessageHandler interface {
	Handle(ctx context.Context, message *sarama.ConsumerMessage) error
}

type ChatMessageHandler struct {
	fanoutService *FanoutService
}

type DeliveryReceiptHandler struct {
	notificationService *NotificationService
}

type PresenceHandler struct {
	presenceService *PresenceService
}

type FanoutService struct {
	redisClient    RedisClient
	kafkaProducer  *MessageProducer
	localCache     map[string][]string // conversation_id -> user_ids
	cacheMu        sync.RWMutex
	cacheExpiry    time.Time
}

type NotificationService struct {
	pushProvider   PushProvider
	fcmClient      FCMClient
	redisClient    RedisClient
}

type KafkaCluster struct {
	producer *MessageProducer
	consumer *MessageConsumer
	brokers  []string
}

type MessageEnvelope struct {
	MessageID   string                 `json:"message_id"`
	Topic       string                 `json:"topic"`
	Partition   int32                  `json:"partition"`
	Offset      int64                  `json:"offset"`
	Timestamp   time.Time              `json:"timestamp"`
	Headers     map[string]string      `json:"headers"`
	Payload     interface{}            `json:"payload"`
	RetryCount  int                    `json:"retry_count"`
	TraceID     string                 `json:"trace_id"`
}

func NewKafkaConfig() *sarama.Config {
	config := sarama.NewConfig()
	
	// Producer configuration
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.Flush.Frequency = producerFlushFrequency
	config.Producer.Flush.Messages = producerFlushMessages
	config.Producer.MaxMessageBytes = 10 * 1024 * 1024 // 10MB
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Retry.Backoff = 100 * time.Millisecond
	config.Producer.Compression = sarama.CompressionSnappy
	config.Producer.Idempotent = true
	config.Producer.Transaction.ID = "chat-producer"
	
	// Consumer configuration
	config.Consumer.Return.Errors = true
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Group.Session.Timeout = 30 * time.Second
	config.Consumer.Group.Heartbeat.Interval = 3 * time.Second
	config.Consumer.MaxProcessingTime = 10 * time.Second
	config.Consumer.Fetch.Min = 1
	config.Consumer.Fetch.Max = 1024 * 1024 // 1MB
	config.Consumer.MaxWaitTime = 500 * time.Millisecond
	config.Consumer.ReadBackoff.Min = 100 * time.Millisecond
	config.Consumer.ReadBackoff.Max = 1 * time.Second
	
	// Performance tuning
	config.Net.MaxOpenRequests = 5
	config.Net.DialTimeout = 30 * time.Second
	config.Net.ReadTimeout = 30 * time.Second
	config.Net.WriteTimeout = 30 * time.Second
	config.Net.KeepAlive = 30 * time.Second
	
	return config
}

func NewMessageProducer(brokers []string) (*MessageProducer, error) {
	config := NewKafkaConfig()
	
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}
	
	return &MessageProducer{
		producer: producer,
		config:   config,
	}, nil
}

func (mp *MessageProducer) ProduceChatMessage(ctx context.Context, chatMsg *ChatMessage) error {
	// Serialize message
	messageBytes, err := proto.Marshal(chatMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal chat message: %w", err)
	}
	
	// Partition by conversation_id for ordering
	partition, offset, err := mp.producer.SendMessage(&sarama.ProducerMessage{
		Topic: chatMessagesTopic,
		Key:   sarama.StringEncoder(chatMsg.ConversationId),
		Value: sarama.ByteEncoder(messageBytes),
		Headers: []sarama.RecordHeader{
			{Key: []byte("message_id"), Value: []byte(chatMsg.MessageId)},
			{Key: []byte("sender_id"), Value: []byte(chatMsg.SenderId)},
			{Key: []byte("timestamp"), Value: []byte(fmt.Sprintf("%d", chatMsg.Timestamp))},
			{Key: []byte("message_type"), Value: []byte(fmt.Sprintf("%d", chatMsg.MessageType))},
		},
		Timestamp: time.Unix(0, chatMsg.Timestamp),
	})
	
	if err != nil {
		return fmt.Errorf("failed to produce message: %w", err)
	}
	
	log.Printf("Message %s produced to topic %s, partition %d, offset %d", 
		chatMsg.MessageId, chatMessagesTopic, partition, offset)
	
	return nil
}

func (mp *MessageProducer) ProduceDeliveryReceipt(ctx context.Context, receipt *DeliveryReceipt) error {
	receiptBytes, err := proto.Marshal(receipt)
	if err != nil {
		return fmt.Errorf("failed to marshal delivery receipt: %w", err)
	}
	
	partition, offset, err := mp.producer.SendMessage(&sarama.ProducerMessage{
		Topic: deliveryReceiptsTopic,
		Key:   sarama.StringEncoder(receipt.MessageId),
		Value: sarama.ByteEncoder(receiptBytes),
		Headers: []sarama.RecordHeader{
			{Key: []byte("user_id"), Value: []byte(receipt.UserId)},
			{Key: []byte("status"), Value: []byte(fmt.Sprintf("%d", receipt.Status))},
		},
		Timestamp: time.Unix(0, receipt.Timestamp),
	})
	
	if err != nil {
		return fmt.Errorf("failed to produce delivery receipt: %w", err)
	}
	
	log.Printf("Delivery receipt for message %s produced to partition %d, offset %d", 
		receipt.MessageId, partition, offset)
	
	return nil
}

func (mp *MessageProducer) ProducePresenceUpdate(ctx context.Context, heartbeat *Heartbeat) error {
	heartbeatBytes, err := proto.Marshal(heartbeat)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat: %w", err)
	}
	
	// Partition by user_id for consistent processing
	partition, offset, err := mp.producer.SendMessage(&sarama.ProducerMessage{
		Topic: presenceUpdatesTopic,
		Key:   sarama.StringEncoder(heartbeat.UserId),
		Value: sarama.ByteEncoder(heartbeatBytes),
		Headers: []sarama.RecordHeader{
			{Key: []byte("node_id"), Value: []byte(heartbeat.NodeId)},
		},
		Timestamp: time.Unix(0, heartbeat.Timestamp),
	})
	
	if err != nil {
		return fmt.Errorf("failed to produce presence update: %w", err)
	}
	
	return nil
}

func NewMessageConsumer(brokers []string, groupID string) (*MessageConsumer, error) {
	config := NewKafkaConfig()
	
	consumer, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}
	
	return &MessageConsumer{
		consumer: consumer,
		config:   config,
		handlers: make(map[string]MessageHandler),
	}, nil
}

func (mc *MessageConsumer) RegisterHandler(topic string, handler MessageHandler) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.handlers[topic] = handler
}

func (mc *MessageConsumer) Start(ctx context.Context) error {
	topics := []string{chatMessagesTopic, deliveryReceiptsTopic, presenceUpdatesTopic}
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err := mc.consumer.Consume(ctx, topics, mc)
			if err != nil {
				log.Printf("Consumer error: %v", err)
				time.Sleep(time.Second)
				continue
			}
		}
	}
}

func (mc *MessageConsumer) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (mc *MessageConsumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (mc *MessageConsumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}
			
			mc.mu.RLock()
			handler, exists := mc.handlers[message.Topic]
			mc.mu.RUnlock()
			
			if exists {
				if err := handler.Handle(session.Context(), message); err != nil {
					log.Printf("Error handling message from topic %s: %v", message.Topic, err)
					// Don't commit message on error
					continue
				}
			}
			
			session.MarkMessage(message, "")
			
		case <-session.Context().Done():
			return nil
		}
	}
}

func (h *ChatMessageHandler) Handle(ctx context.Context, message *sarama.ConsumerMessage) error {
	var chatMsg ChatMessage
	if err := proto.Unmarshal(message.Value, &chatMsg); err != nil {
		return fmt.Errorf("failed to unmarshal chat message: %w", err)
	}
	
	// Fan out to participants
	return h.fanoutService.FanoutMessage(ctx, &chatMsg)
}

func (h *DeliveryReceiptHandler) Handle(ctx context.Context, message *sarama.ConsumerMessage) error {
	var receipt DeliveryReceipt
	if err := proto.Unmarshal(message.Value, &receipt); err != nil {
		return fmt.Errorf("failed to unmarshal delivery receipt: %w", err)
	}
	
	return h.notificationService.ProcessDeliveryReceipt(ctx, &receipt)
}

func (h *PresenceHandler) Handle(ctx context.Context, message *sarama.ConsumerMessage) error {
	var heartbeat Heartbeat
	if err := proto.Unmarshal(message.Value, &heartbeat); err != nil {
		return fmt.Errorf("failed to unmarshal heartbeat: %w", err)
	}
	
	// Update presence in Redis
	return h.presenceService.UpdatePresence(ctx, &heartbeat)
}

func NewFanoutService(redisClient RedisClient, kafkaProducer *MessageProducer) *FanoutService {
	return &FanoutService{
		redisClient:   redisClient,
		kafkaProducer: kafkaProducer,
		localCache:    make(map[string][]string),
		cacheExpiry:   time.Now(),
	}
}

func (fs *FanoutService) FanoutMessage(ctx context.Context, chatMsg *ChatMessage) error {
	// Get conversation participants
	participants, err := fs.getConversationParticipants(ctx, chatMsg.ConversationId)
	if err != nil {
		return fmt.Errorf("failed to get conversation participants: %w", err)
	}
	
	// Check if this is a celebrity message (high follower count)
	isCelebrity, err := fs.isCelebrityMessage(ctx, chatMsg.SenderId, participants)
	if err != nil {
		log.Printf("Error checking celebrity status: %v", err)
	}
	
	if isCelebrity {
		return fs.fanoutCelebrityMessage(ctx, chatMsg, participants)
	}
	
	return fs.fanoutRegularMessage(ctx, chatMsg, participants)
}

func (fs *FanoutService) fanoutRegularMessage(ctx context.Context, chatMsg *ChatMessage, participants []string) error {
	// For regular messages, push directly to online users
	for _, participant := range participants {
		if participant == chatMsg.SenderId {
			continue // Skip sender
		}
		
		// Check if user is online
		online, err := fs.redisClient.IsUserOnline(ctx, participant)
		if err != nil {
			log.Printf("Error checking online status for user %s: %v", participant, err)
			continue
		}
		
		if online {
			// Push to WebSocket gateway
			if err := fs.pushToWebSocketGateway(ctx, participant, chatMsg); err != nil {
				log.Printf("Error pushing message to user %s: %v", participant, err)
			}
		} else {
			// Store for later retrieval (pull model)
			if err := fs.storeOfflineMessage(ctx, participant, chatMsg); err != nil {
				log.Printf("Error storing offline message for user %s: %v", participant, err)
			}
		}
	}
	
	return nil
}

func (fs *FanoutService) fanoutCelebrityMessage(ctx context.Context, chatMsg *ChatMessage, participants []string) error {
	// For celebrity messages, use hybrid push/pull with distributed processing
	
	// 1. Store message in offline storage for all participants
	for _, participant := range participants {
		if participant == chatMsg.SenderId {
			continue
		}
		
		if err := fs.storeOfflineMessage(ctx, participant, chatMsg); err != nil {
			log.Printf("Error storing celebrity message for user %s: %v", participant, err)
		}
	}
	
	// 2. Push notification to online users only (don't overwhelm the system)
	onlineParticipants := fs.getOnlineParticipants(ctx, participants)
	batchSize := 100
	
	for i := 0; i < len(onlineParticipants); i += batchSize {
		end := i + batchSize
		if end > len(onlineParticipants) {
			end = len(onlineParticipants)
		}
		
		batch := onlineParticipants[i:end]
		if err := fs.pushBatchToWebSocketGateways(ctx, batch, chatMsg); err != nil {
			log.Printf("Error pushing batch to WebSocket gateways: %v", err)
		}
	}
	
	// 3. Send push notifications to mobile apps
	if err := fs.sendPushNotifications(ctx, participants, chatMsg); err != nil {
		log.Printf("Error sending push notifications: %v", err)
	}
	
	return nil
}

func (fs *FanoutService) getConversationParticipants(ctx context.Context, conversationID string) ([]string, error) {
	fs.cacheMu.RLock()
	if participants, exists := fs.localCache[conversationID]; exists && time.Now().Before(fs.cacheExpiry) {
		fs.cacheMu.RUnlock()
		return participants, nil
	}
	fs.cacheMu.RUnlock()
	
	// Fetch from Redis or database
	participants, err := fs.redisClient.GetConversationParticipants(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	
	// Cache result
	fs.cacheMu.Lock()
	fs.localCache[conversationID] = participants
	fs.cacheExpiry = time.Now().Add(5 * time.Minute)
	fs.cacheMu.Unlock()
	
	return participants, nil
}

func (fs *FanoutService) isCelebrityMessage(ctx context.Context, senderID string, participants []string) (bool, error) {
	// Consider celebrity if sender has >100k followers or conversation has >100k participants
	followerCount, err := fs.redisClient.GetFollowerCount(ctx, senderID)
	if err != nil {
		return false, err
	}
	
	return followerCount > 100000 || len(participants) > 100000, nil
}

func (fs *FanoutService) getOnlineParticipants(ctx context.Context, participants []string) []string {
	var online []string
	
	// Batch check online status
	batchSize := 1000
	for i := 0; i < len(participants); i += batchSize {
		end := i + batchSize
		if end > len(participants) {
			end = len(participants)
		}
		
		batch := participants[i:end]
		onlineMap, err := fs.redisClient.GetOnlineUsersInBatch(ctx, batch)
		if err != nil {
			log.Printf("Error checking online status for batch: %v", err)
			continue
		}
		
		for userID, isOnline := range onlineMap {
			if isOnline {
				online = append(online, userID)
			}
		}
	}
	
	return online
}

func (fs *FanoutService) pushToWebSocketGateway(ctx context.Context, userID string, chatMsg *ChatMessage) error {
	// Find which WebSocket gateway node the user is connected to
	nodeID, err := fs.redisClient.GetUserNodeID(ctx, userID)
	if err != nil {
		return err
	}
	
	// Send message to the specific node via Redis pub/sub or direct HTTP call
	messageBytes, err := proto.Marshal(chatMsg)
	if err != nil {
		return err
	}
	
	return fs.redisClient.PublishToNode(ctx, nodeID, messageBytes)
}

func (fs *FanoutService) pushBatchToWebSocketGateways(ctx context.Context, userIDs []string, chatMsg *ChatMessage) error {
	// Group users by node
	nodeUsers := make(map[string][]string)
	
	for _, userID := range userIDs {
		nodeID, err := fs.redisClient.GetUserNodeID(ctx, userID)
		if err != nil {
			log.Printf("Error getting node ID for user %s: %v", userID, err)
			continue
		}
		
		nodeUsers[nodeID] = append(nodeUsers[nodeID], userID)
	}
	
	// Send batch messages to each node
	for nodeID, users := range nodeUsers {
		if err := fs.sendBatchToNode(ctx, nodeID, users, chatMsg); err != nil {
			log.Printf("Error sending batch to node %s: %v", nodeID, err)
		}
	}
	
	return nil
}

func (fs *FanoutService) storeOfflineMessage(ctx context.Context, userID string, chatMsg *ChatMessage) error {
	return fs.redisClient.StoreOfflineMessage(ctx, userID, chatMsg)
}

func (fs *FanoutService) sendBatchToNode(ctx context.Context, nodeID string, userIDs []string, chatMsg *ChatMessage) error {
	// Implement batch sending to specific WebSocket gateway node
	return fs.redisClient.PublishBatchToNode(ctx, nodeID, userIDs, chatMsg)
}

func (fs *FanoutService) sendPushNotifications(ctx context.Context, participants []string, chatMsg *ChatMessage) error {
	// Implement push notification sending
	return nil
}

func (mp *MessageProducer) Close() error {
	return mp.producer.Close()
}

func (mc *MessageConsumer) Close() error {
	return mc.consumer.Close()
}

// RedisClient interface for dependency injection
type RedisClient interface {
	IsUserOnline(ctx context.Context, userID string) (bool, error)
	GetConversationParticipants(ctx context.Context, conversationID string) ([]string, error)
	GetFollowerCount(ctx context.Context, userID string) (int64, error)
	GetOnlineUsersInBatch(ctx context.Context, userIDs []string) (map[string]bool, error)
	GetUserNodeID(ctx context.Context, userID string) (string, error)
	PublishToNode(ctx context.Context, nodeID string, message []byte) error
	PublishBatchToNode(ctx context.Context, nodeID string, userIDs []string, chatMsg *ChatMessage) error
	StoreOfflineMessage(ctx context.Context, userID string, chatMsg *ChatMessage) error
}

// PushProvider interface for mobile push notifications
type PushProvider interface {
	SendPushNotification(ctx context.Context, userID string, message *ChatMessage) error
}

// FCMClient interface for Firebase Cloud Messaging
type FCMClient interface {
	SendMessage(ctx context.Context, token string, notification map[string]interface{}) error
}

// PresenceService interface for presence updates
type PresenceService interface {
	UpdatePresence(ctx context.Context, heartbeat *Heartbeat) error
}
