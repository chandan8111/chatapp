package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

type ScyllaDBClient struct {
	session *gocql.Session
	config  *gocql.ClusterConfig
}

type MessageRepository struct {
	client *ScyllaDBClient
}

type ConversationRepository struct {
	client *ScyllaDBClient
}

type UserRepository struct {
	client *ScyllaDBClient
}

type PresenceRepository struct {
	client *ScyllaDBClient
}

const (
	bucketDuration = 7 * 24 * time.Hour // 1 week buckets
)

type Message struct {
	ConversationID        uuid.UUID
	BucketID              int
	MessageID             uuid.UUID
	SenderID              uuid.UUID
	MessageType           int
	Ciphertext            []byte
	EphemeralPublicKey    []byte
	Metadata              map[string]string
	PreviousMessageSig    []byte
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type Conversation struct {
	ConversationID    uuid.UUID
	ConversationType  int
	CreatedBy         uuid.UUID
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DisplayName       string
	Description       string
	AvatarURL         string
	IsPublic          bool
	MaxParticipants   int
	MessageRetentionDays int
	IsEncrypted       bool
	Settings          map[string]string
	Metadata          map[string]string
}

type UserConversation struct {
	UserID           uuid.UUID
	ConversationID   uuid.UUID
	LastMessageID    uuid.UUID
	LastMessageAt    time.Time
	UnreadCount      int64
	IsArchived       bool
	IsMuted          bool
	ConversationType int
	DisplayName      string
	AvatarURL        string
	ParticipantCount int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ConversationParticipant struct {
	ConversationID uuid.UUID
	UserID         uuid.UUID
	Role           int
	JoinedAt       time.Time
	LastReadMsgID  uuid.UUID
	IsActive       bool
	Permissions    []string
}

type UserPresence struct {
	UserID         uuid.UUID
	Online         bool
	LastSeen       time.Time
	CurrentNodeID  string
	DeviceCount    int
	ActiveDevices  map[uuid.UUID]string
	StatusText     string
	StatusEmoji    string
	UpdatedAt      time.Time
}

func NewScyllaDBClient(hosts []string, keyspace string) (*ScyllaDBClient, error) {
	config := gocql.NewCluster(hosts...)
	config.Keyspace = keyspace
	config.Consistency = gocql.Quorum
	config.NumConns = 4
	config.Timeout = 5 * time.Second
	config.ConnectTimeout = 10 * time.Second
	config.ReconnectInterval = 30 * time.Second
	config.PoolSize = 100
	
	// Enable compression
	config.Compressor = gocql.SnappyCompressor
	
	// Enable host filtering for multi-DC awareness
	config.HostFilter = gocql.DataCentreHostFilter("dc1")
	
	session, err := config.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create ScyllaDB session: %w", err)
	}
	
	return &ScyllaDBClient{
		session: session,
		config:  config,
	}, nil
}

func (c *ScyllaDBClient) Close() {
	c.session.Close()
}

func (c *ScyllaDBClient) GetSession() *gocql.Session {
	return c.session
}

func NewMessageRepository(client *ScyllaDBClient) *MessageRepository {
	return &MessageRepository{client: client}
}

func (r *MessageRepository) StoreMessage(ctx context.Context, chatMsg *ChatMessage) error {
	bucketID := r.calculateBucketID(time.Unix(0, chatMsg.Timestamp))
	
	message := &Message{
		ConversationID:     uuid.MustParse(chatMsg.ConversationId),
		BucketID:           bucketID,
		MessageID:          uuid.MustParse(chatMsg.MessageId),
		SenderID:           uuid.MustParse(chatMsg.SenderId),
		MessageType:        int(chatMsg.MessageType),
		Ciphertext:         chatMsg.Ciphertext,
		EphemeralPublicKey: chatMsg.EphemeralPublicKey,
		Metadata:           chatMsg.Metadata,
		CreatedAt:          time.Unix(0, chatMsg.Timestamp),
		UpdatedAt:          time.Now(),
	}
	
	query := r.client.session.Query(`
		INSERT INTO messages (
			conversation_id, bucket_id, message_id, sender_id, message_type,
			ciphertext, ephemeral_public_key, metadata, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`).BindStruct(message)
	
	if err := query.Exec(); err != nil {
		return fmt.Errorf("failed to store message: %w", err)
	}
	
	return nil
}

func (r *MessageRepository) GetMessages(ctx context.Context, conversationID string, limit int, beforeTime time.Time) ([]*Message, error) {
	convoUUID := uuid.MustParse(conversationID)
	bucketID := r.calculateBucketID(beforeTime)
	
	var messages []*Message
	query := r.client.session.Query(`
		SELECT conversation_id, bucket_id, message_id, sender_id, message_type,
		       ciphertext, ephemeral_public_key, metadata, created_at, updated_at
		FROM messages 
		WHERE conversation_id = ? AND bucket_id = ? 
		ORDER BY message_id DESC 
		LIMIT ?
	`).Bind(convoUUID, bucketID, limit)
	
	iter := query.Iter()
	for {
		var message Message
		if !iter.Scan(&message.ConversationID, &message.BucketID, &message.MessageID,
			&message.SenderID, &message.MessageType, &message.Ciphertext,
			&message.EphemeralPublicKey, &message.Metadata, &message.CreatedAt, &message.UpdatedAt) {
			break
		}
		// Allocate new variable to avoid pointer to loop variable issue
		msgCopy := message
		messages = append(messages, &msgCopy)
	}
	
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("failed to iterate messages: %w", err)
	}
	
	return messages, nil
}

func (r *MessageRepository) GetMessagesByTimeRange(ctx context.Context, conversationID string, startTime, endTime time.Time, limit int) ([]*Message, error) {
	convoUUID := uuid.MustParse(conversationID)
	
	// Calculate bucket range
	startBucket := r.calculateBucketID(startTime)
	endBucket := r.calculateBucketID(endTime)
	
	var allMessages []*Message
	
	// Query across multiple buckets if needed
	for bucketID := startBucket; bucketID <= endBucket; bucketID++ {
		var messages []*Message
		query := r.client.session.Query(`
			SELECT conversation_id, bucket_id, message_id, sender_id, message_type,
			       ciphertext, ephemeral_public_key, metadata, created_at, updated_at
			FROM messages 
			WHERE conversation_id = ? AND bucket_id = ? 
			AND created_at >= ? AND created_at <= ?
			ORDER BY message_id DESC 
			LIMIT ?
		`).Bind(convoUUID, bucketID, startTime, endTime, limit)
		
		iter := query.Iter()
		for {
			var message Message
			if !iter.Scan(&message.ConversationID, &message.BucketID, &message.MessageID,
				&message.SenderID, &message.MessageType, &message.Ciphertext,
				&message.EphemeralPublicKey, &message.Metadata, &message.CreatedAt, &message.UpdatedAt) {
				break
			}
			// Allocate new variable to avoid pointer to loop variable issue
			msgCopy := message
			messages = append(messages, &msgCopy)
		}
		
		if err := iter.Close(); err != nil {
			return nil, fmt.Errorf("failed to iterate messages in bucket %d: %w", bucketID, err)
		}
		
		allMessages = append(allMessages, messages...)
		
		// Stop if we've reached the limit
		if len(allMessages) >= limit {
			break
		}
	}
	
	// Trim to exact limit
	if len(allMessages) > limit {
		allMessages = allMessages[:limit]
	}
	
	return allMessages, nil
}

func (r *MessageRepository) calculateBucketID(timestamp time.Time) int {
	return int(timestamp.Unix() / int64(bucketDuration.Seconds()))
}

func NewConversationRepository(client *ScyllaDBClient) *ConversationRepository {
	return &ConversationRepository{client: client}
}

func (r *ConversationRepository) CreateConversation(ctx context.Context, conv *Conversation) error {
	query := r.client.session.Query(`
		INSERT INTO conversations (
			conversation_id, conversation_type, created_by, created_at, updated_at,
			display_name, description, avatar_url, is_public, max_participants,
			message_retention_days, is_encrypted, settings, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`).BindStruct(conv)
	
	if err := query.Exec(); err != nil {
		return fmt.Errorf("failed to create conversation: %w", err)
	}
	
	return nil
}

func (r *ConversationRepository) GetConversation(ctx context.Context, conversationID string) (*Conversation, error) {
	var conv Conversation
	query := r.client.session.Query(`
		SELECT conversation_id, conversation_type, created_by, created_at, updated_at,
		       display_name, description, avatar_url, is_public, max_participants,
		       message_retention_days, is_encrypted, settings, metadata
		FROM conversations WHERE conversation_id = ?
	`).Bind(uuid.MustParse(conversationID))
	
	if err := query.Scan(&conv.ConversationID, &conv.ConversationType, &conv.CreatedBy,
		&conv.CreatedAt, &conv.UpdatedAt, &conv.DisplayName, &conv.Description,
		&conv.AvatarURL, &conv.IsPublic, &conv.MaxParticipants,
		&conv.MessageRetentionDays, &conv.IsEncrypted, &conv.Settings, &conv.Metadata); err != nil {
		if err == gocql.ErrNotFound {
			return nil, fmt.Errorf("conversation not found: %s", conversationID)
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}
	
	return &conv, nil
}

func (r *ConversationRepository) AddParticipant(ctx context.Context, participant *ConversationParticipant) error {
	query := r.client.session.Query(`
		INSERT INTO conversation_participants (
			conversation_id, user_id, role, joined_at, last_read_message_id,
			is_active, permissions
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`).BindStruct(participant)
	
	if err := query.Exec(); err != nil {
		return fmt.Errorf("failed to add participant: %w", err)
	}
	
	return nil
}

func (r *ConversationRepository) GetParticipants(ctx context.Context, conversationID string) ([]*ConversationParticipant, error) {
	var participants []*ConversationParticipant
	query := r.client.session.Query(`
		SELECT conversation_id, user_id, role, joined_at, last_read_message_id,
		       is_active, permissions
		FROM conversation_participants WHERE conversation_id = ?
	`).Bind(uuid.MustParse(conversationID))
	
	iter := query.Iter()
	for {
		var participant ConversationParticipant
		if !iter.Scan(&participant.ConversationID, &participant.UserID, &participant.Role,
			&participant.JoinedAt, &participant.LastReadMsgID, &participant.IsActive,
			&participant.Permissions) {
			break
		}
		// Allocate new variable to avoid pointer to loop variable issue
		partCopy := participant
		participants = append(participants, &partCopy)
	}
	
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("failed to iterate participants: %w", err)
	}
	
	return participants, nil
}

func NewUserRepository(client *ScyllaDBClient) *UserRepository {
	return &UserRepository{client: client}
}

func (r *UserRepository) AddUserConversation(ctx context.Context, userConv *UserConversation) error {
	query := r.client.session.Query(`
		INSERT INTO user_conversations (
			user_id, conversation_id, last_message_id, last_message_at,
			unread_count, is_archived, is_muted, conversation_type,
			display_name, avatar_url, participant_count, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`).BindStruct(userConv)
	
	if err := query.Exec(); err != nil {
		return fmt.Errorf("failed to add user conversation: %w", err)
	}
	
	return nil
}

func (r *UserRepository) GetUserConversations(ctx context.Context, userID string, limit int) ([]*UserConversation, error) {
	var conversations []*UserConversation
	query := r.client.session.Query(`
		SELECT user_id, conversation_id, last_message_id, last_message_at,
		       unread_count, is_archived, is_muted, conversation_type,
		       display_name, avatar_url, participant_count, created_at, updated_at
		FROM user_conversations WHERE user_id = ? LIMIT ?
	`).Bind(uuid.MustParse(userID), limit)
	
	iter := query.Iter()
	for {
		var userConv UserConversation
		if !iter.Scan(&userConv.UserID, &userConv.ConversationID, &userConv.LastMessageID,
			&userConv.LastMessageAt, &userConv.UnreadCount, &userConv.IsArchived,
			&userConv.IsMuted, &userConv.ConversationType, &userConv.DisplayName,
			&userConv.AvatarURL, &userConv.ParticipantCount, &userConv.CreatedAt, &userConv.UpdatedAt) {
			break
		}
		// Allocate new variable to avoid pointer to loop variable issue
		ucCopy := userConv
		conversations = append(conversations, &ucCopy)
	}
	
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("failed to iterate user conversations: %w", err)
	}
	
	return conversations, nil
}

func (r *UserRepository) IncrementUnreadCount(ctx context.Context, userID, conversationID string) error {
	query := r.client.session.Query(`
		UPDATE user_conversations SET unread_count = unread_count + 1
		WHERE user_id = ? AND conversation_id = ?
	`).Bind(uuid.MustParse(userID), uuid.MustParse(conversationID))
	
	if err := query.Exec(); err != nil {
		return fmt.Errorf("failed to increment unread count: %w", err)
	}
	
	return nil
}

func (r *UserRepository) ResetUnreadCount(ctx context.Context, userID, conversationID string) error {
	query := r.client.session.Query(`
		UPDATE user_conversations SET unread_count = 0
		WHERE user_id = ? AND conversation_id = ?
	`).Bind(uuid.MustParse(userID), uuid.MustParse(conversationID))
	
	if err := query.Exec(); err != nil {
		return fmt.Errorf("failed to reset unread count: %w", err)
	}
	
	return nil
}

func NewPresenceRepository(client *ScyllaDBClient) *PresenceRepository {
	return &PresenceRepository{client: client}
}

func (r *PresenceRepository) UpdatePresence(ctx context.Context, presence *UserPresence) error {
	query := r.client.session.Query(`
		INSERT INTO user_presence (
			user_id, online, last_seen, current_node_id, device_count,
			active_devices, status_text, status_emoji, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`).BindStruct(presence)
	
	if err := query.Exec(); err != nil {
		return fmt.Errorf("failed to update presence: %w", err)
	}
	
	return nil
}

func (r *PresenceRepository) GetPresence(ctx context.Context, userID string) (*UserPresence, error) {
	var presence UserPresence
	query := r.client.session.Query(`
		SELECT user_id, online, last_seen, current_node_id, device_count,
		       active_devices, status_text, status_emoji, updated_at
		FROM user_presence WHERE user_id = ?
	`).Bind(uuid.MustParse(userID))
	
	if err := query.Scan(&presence.UserID, &presence.Online, &presence.LastSeen,
		&presence.CurrentNodeID, &presence.DeviceCount, &presence.ActiveDevices,
		&presence.StatusText, &presence.StatusEmoji, &presence.UpdatedAt); err != nil {
		if err == gocql.ErrNotFound {
			return nil, fmt.Errorf("presence not found for user: %s", userID)
		}
		return nil, fmt.Errorf("failed to get presence: %w", err)
	}
	
	return &presence, nil
}

func (r *PresenceRepository) GetOnlineUsers(ctx context.Context, userIDs []string) (map[string]*UserPresence, error) {
	if len(userIDs) == 0 {
		return make(map[string]*UserPresence), nil
	}
	
	// Batch query for online users
	uuids := make([]uuid.UUID, len(userIDs))
	for i, userID := range userIDs {
		uuids[i] = uuid.MustParse(userID)
	}
	
	query := r.client.session.Query(`
		SELECT user_id, online, last_seen, current_node_id, device_count,
		       active_devices, status_text, status_emoji, updated_at
		FROM user_presence WHERE user_id IN ?
	`).Bind(uuids)
	
	iter := query.Iter()
	result := make(map[string]*UserPresence)
	
	for {
		var presence UserPresence
		if !iter.Scan(&presence.UserID, &presence.Online, &presence.LastSeen,
			&presence.CurrentNodeID, &presence.DeviceCount, &presence.ActiveDevices,
			&presence.StatusText, &presence.StatusEmoji, &presence.UpdatedAt) {
			break
		}
		// Allocate new variable to avoid pointer to loop variable issue
		presCopy := presence
		result[presence.UserID.String()] = &presCopy
	}
	
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("failed to iterate presence: %w", err)
	}
	
	return result, nil
}

// Batch operations for high performance
func (r *MessageRepository) BatchStoreMessages(ctx context.Context, messages []*Message) error {
	if len(messages) == 0 {
		return nil
	}
	
	batch := r.client.session.NewBatch(gocql.UnloggedBatch)
	
	for _, message := range messages {
		batch.Query(`
			INSERT INTO messages (
				conversation_id, bucket_id, message_id, sender_id, message_type,
				ciphertext, ephemeral_public_key, metadata, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`).BindStruct(message)
	}
	
	if err := r.client.session.ExecuteBatch(batch); err != nil {
		return fmt.Errorf("failed to batch store messages: %w", err)
	}
	
	return nil
}

// Async query support for non-blocking operations
func (r *MessageRepository) GetMessagesAsync(ctx context.Context, conversationID string, limit int, beforeTime time.Time) <-chan *Message {
	ch := make(chan *Message, 100)
	
	go func() {
		defer close(ch)
		
		convoUUID := uuid.MustParse(conversationID)
		bucketID := r.calculateBucketID(beforeTime)
		
		query := r.client.session.Query(`
			SELECT conversation_id, bucket_id, message_id, sender_id, message_type,
			       ciphertext, ephemeral_public_key, metadata, created_at, updated_at
			FROM messages 
			WHERE conversation_id = ? AND bucket_id = ? 
			ORDER BY message_id DESC 
			LIMIT ?
		`).Bind(convoUUID, bucketID, limit)
		
		iter := query.Iter()
		for {
			var message Message
			if !iter.Scan(&message.ConversationID, &message.BucketID, &message.MessageID,
				&message.SenderID, &message.MessageType, &message.Ciphertext,
				&message.EphemeralPublicKey, &message.Metadata, &message.CreatedAt, &message.UpdatedAt) {
				break
			}
			// Allocate new variable to avoid pointer to loop variable issue
			msgCopy := message
			select {
			case ch <- &msgCopy:
			case <-ctx.Done():
				iter.Close()
				return
			}
		}
		
		iter.Close()
	}()
	
	return ch
}

// Connection pooling and load balancing
func (c *ScyllaDBClient) SetLoadBalancingPolicy(policy gocql.HostSelectionPolicy) {
	c.config.HostSelectionPolicy = policy
}

func (c *ScyllaDBClient) SetRetryPolicy(policy gocql.RetryPolicy) {
	c.config.RetryPolicy = policy
}

// Metrics and monitoring
func (c *ScyllaDBClient) GetMetrics() map[string]interface{} {
	stats := c.client.session.Stats()
	return map[string]interface{}{
		"connections":          stats.Open,
		"closed_connections":   stats.Closed,
		"queries":             stats.Queries,
		"errors":              stats.Errors,
		"timeouts":            stats.Timeouts,
		"retries":             stats.Retries,
		"schema_agreements":   stats.SchemaAgreements,
	}
}
