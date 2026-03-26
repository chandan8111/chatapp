-- ScyllaDB/Cassandra Schema for Distributed Chat System
-- Optimized for 100M concurrent users with time-based partitioning and bucketing

-- Create keyspace with proper replication for multi-DC deployment
CREATE KEYSPACE IF NOT EXISTS chatapp 
WITH REPLICATION = {
    'class': 'NetworkTopologyStrategy',
    'dc1': 3,
    'dc2': 3,
    'dc3': 3
} 
AND DURABLE_WRITES = true;

-- Use the chatapp keyspace
USE chatapp;

-- Messages table with time-based bucketing for long-lived conversations
-- Each bucket represents a time period (e.g., 1 week) to prevent wide rows
CREATE TABLE IF NOT EXISTS messages (
    conversation_id uuid,
    bucket_id int,              -- Time-based bucket: (timestamp / bucket_duration)
    message_id timeuuid,        -- TimeUUID for natural ordering and uniqueness
    sender_id uuid,
    message_type int,           -- 0: text, 1: image, 2: file, 3: voice
    ciphertext blob,            -- Encrypted message content
    ephemeral_public_key blob,  -- E2EE ephemeral key
    metadata map<text, text>,   -- Additional message metadata
    previous_message_signature blob,
    created_at timestamp,
    updated_at timestamp,
    PRIMARY KEY ((conversation_id, bucket_id), message_id)
) WITH CLUSTERING ORDER BY (message_id DESC)
  AND compaction = {
    'class': 'TimeWindowCompactionStrategy',
    'compaction_window_unit': 'DAYS',
    'compaction_window_size': 7
  }
  AND compression = {
    'sstable_compression': 'LZ4Compressor',
    'chunk_length_kb': 64
  }
  AND gc_grace_seconds = 86400;  -- 24 hours

-- Materialized view for chronological order (ascending)
CREATE MATERIALIZED VIEW IF NOT EXISTS messages_by_time_asc AS
    SELECT * FROM messages
    WHERE conversation_id IS NOT NULL
      AND bucket_id IS NOT NULL
      AND message_id IS NOT NULL
    PRIMARY KEY ((conversation_id, bucket_id), message_id)
    WITH CLUSTERING ORDER BY (message_id ASC);

-- User conversation list for quick lookup
CREATE TABLE IF NOT EXISTS user_conversations (
    user_id uuid,
    conversation_id uuid,
    last_message_id timeuuid,
    last_message_at timestamp,
    unread_count counter,
    is_archived boolean,
    is_muted boolean,
    conversation_type int,     -- 0: direct, 1: group, 2: channel
    display_name text,
    avatar_url text,
    participant_count int,
    created_at timestamp,
    updated_at timestamp,
    PRIMARY KEY (user_id, conversation_id)
) WITH CLUSTERING ORDER BY (conversation_id DESC)
  AND compaction = {
    'class': 'LeveledCompactionStrategy',
    'sstable_size_in_mb': 160
  };

-- Conversation participants table for fan-out operations
CREATE TABLE IF NOT EXISTS conversation_participants (
    conversation_id uuid,
    user_id uuid,
    role int,                  -- 0: member, 1: admin, 2: owner
    joined_at timestamp,
    last_read_message_id timeuuid,
    is_active boolean,
    permissions set<text>,     -- Custom permissions
    PRIMARY KEY (conversation_id, user_id)
) WITH CLUSTERING ORDER BY (user_id ASC);

-- Index for finding conversations by user
CREATE TABLE IF NOT EXISTS user_conversations_by_type (
    user_id uuid,
    conversation_type int,
    conversation_id uuid,
    last_message_at timestamp,
    PRIMARY KEY ((user_id, conversation_type), conversation_id)
) WITH CLUSTERING ORDER BY (conversation_id DESC);

-- Message delivery tracking for read receipts
CREATE TABLE IF NOT EXISTS message_delivery (
    message_id timeuuid,
    user_id uuid,
    conversation_id uuid,
    delivery_status int,        -- 0: sent, 1: delivered, 2: read
    delivered_at timestamp,
    read_at timestamp,
    device_id uuid,
    PRIMARY KEY (message_id, user_id)
) WITH CLUSTERING ORDER BY (user_id ASC);

-- User presence and last seen tracking
CREATE TABLE IF NOT EXISTS user_presence (
    user_id uuid PRIMARY KEY,
    online boolean,
    last_seen timestamp,
    current_node_id text,
    device_count int,
    active_devices map<uuid, text>, -- device_id -> device_type
    status_text text,
    status_emoji text,
    updated_at timestamp
) WITH default_time_to_live = 2592000; -- 30 days TTL

-- Conversation metadata and settings
CREATE TABLE IF NOT EXISTS conversations (
    conversation_id uuid PRIMARY KEY,
    conversation_type int,
    created_by uuid,
    created_at timestamp,
    updated_at timestamp,
    display_name text,
    description text,
    avatar_url text,
    is_public boolean,
    max_participants int,
    message_retention_days int,
    is_encrypted boolean,
    settings map<text, text>,
    metadata map<text, text>
);

-- Message reactions and engagement
CREATE TABLE IF NOT EXISTS message_reactions (
    message_id timeuuid,
    reaction_type int,          -- 0: like, 1: love, 2: laugh, 3: angry, 4: sad, 5: wow
    user_id uuid,
    created_at timestamp,
    PRIMARY KEY ((message_id, reaction_type), user_id)
) WITH CLUSTERING ORDER BY (user_id ASC);

-- Message edits history
CREATE TABLE IF NOT EXISTS message_edits (
    message_id timeuuid,
    edit_version int,
    editor_id uuid,
    old_ciphertext blob,
    new_ciphertext blob,
    edit_reason text,
    edited_at timestamp,
    PRIMARY KEY (message_id, edit_version)
) WITH CLUSTERING ORDER BY (edit_version DESC);

-- File attachments metadata
CREATE TABLE IF NOT EXISTS file_attachments (
    file_id uuid PRIMARY KEY,
    message_id timeuuid,
    conversation_id uuid,
    uploader_id uuid,
    file_name text,
    file_size bigint,
    mime_type text,
    storage_path text,
    cdn_url text,
    thumbnail_url text,
    encryption_key blob,       -- File encryption key
    is_encrypted boolean,
    download_count counter,
    created_at timestamp,
    expires_at timestamp
) WITH default_time_to_live = 31536000; -- 1 year TTL

-- User sessions for multi-device support
CREATE TABLE IF NOT EXISTS user_sessions (
    user_id uuid,
    session_id uuid,
    device_id uuid,
    device_type text,          -- mobile, desktop, web
    device_name text,
    app_version text,
    ip_address inet,
    last_active timestamp,
    push_token text,
    is_active boolean,
    created_at timestamp,
    PRIMARY KEY (user_id, session_id)
) WITH CLUSTERING ORDER BY (session_id DESC)
  AND default_time_to_live = 2592000; -- 30 days TTL

-- Typing indicators (short TTL)
CREATE TABLE IF NOT EXISTS typing_indicators (
    conversation_id uuid,
    user_id uuid,
    is_typing boolean,
    last_typing timestamp,
    device_id uuid,
    PRIMARY KEY (conversation_id, user_id)
) WITH default_time_to_live = 10; -- 10 seconds TTL

-- Rate limiting for message sending
CREATE TABLE IF NOT EXISTS rate_limits (
    user_id uuid,
    window_start timestamp,     -- Time window start
    message_count counter,
    PRIMARY KEY ((user_id, window_start))
) WITH default_time_to_live = 3600; -- 1 hour TTL

-- Search index for messages (using SASI or custom indexing)
CREATE TABLE IF NOT EXISTS message_search (
    conversation_id uuid,
    search_term text,
    message_id timeuuid,
    sender_id uuid,
    message_type int,
    created_at timestamp,
    PRIMARY KEY ((conversation_id, search_term), message_id)
) WITH CLUSTERING ORDER BY (message_id DESC);

-- Analytics and metrics tables
CREATE TABLE IF NOT EXISTS message_metrics (
    bucket_date date,           -- Daily aggregation
    conversation_type int,
    message_type int,
    total_messages counter,
    total_users counter,
    total_size bigint,
    PRIMARY KEY (bucket_date, conversation_type, message_type)
);

CREATE TABLE IF NOT EXISTS user_metrics (
    user_id uuid,
    metric_date date,
    messages_sent counter,
    messages_received counter,
    active_time_seconds counter,
    data_sent_bytes counter,
    data_received_bytes counter,
    PRIMARY KEY (user_id, metric_date)
) WITH default_time_to_live = 7776000; -- 90 days TTL

-- Secondary indexes for common queries
CREATE CUSTOM INDEX IF NOT EXISTS messages_sender_idx ON messages (sender_id) 
USING 'org.apache.cassandra.index.sasi.SASIIndex' 
WITH OPTIONS = {
    'mode': 'SPARSE'
};

CREATE CUSTOM INDEX IF NOT EXISTS messages_type_idx ON messages (message_type) 
USING 'org.apache.cassandra.index.sasi.SASIIndex' 
WITH OPTIONS = {
    'mode': 'SPARSE'
};

CREATE CUSTOM INDEX IF NOT EXISTS conversations_type_idx ON conversations (conversation_type) 
USING 'org.apache.cassandra.index.sasi.SASIIndex' 
WITH OPTIONS = {
    'mode': 'SPARSE'
};

CREATE CUSTOM INDEX IF NOT EXISTS conversations_public_idx ON conversations (is_public) 
USING 'org.apache.cassandra.index.sasi.SASIIndex' 
WITH OPTIONS = {
    'mode': 'SPARSE'
};

-- User-defined types for structured data
CREATE TYPE IF NOT EXISTS address (
    street text,
    city text,
    state text,
    country text,
    postal_code text
);

CREATE TYPE IF NOT EXISTS geo_location (
    latitude double,
    longitude double,
    accuracy double
);

-- Stored procedures for common operations
CREATE OR REPLACE FUNCTION get_bucket_id(message_timestamp timestamp)
RETURNS NULL ON NULL INPUT
RETURNS int
LANGUAGE java AS
    'return (int)(message_timestamp.getTime() / (7 * 24 * 60 * 60 * 1000));'; -- Weekly buckets

-- Create triggers for automatic timestamp updates
-- Note: Triggers in Cassandra are limited, these are examples
CREATE TRIGGER IF NOT EXISTS messages_update_trigger
ON messages
USING 'org.apache.cassandra.triggers.TriggerExample';

-- Prepare statements for optimal performance
-- These would be prepared in the application layer

-- Insert message with automatic bucket calculation
-- PREPARE insert_message AS
-- INSERT INTO messages (
--     conversation_id, bucket_id, message_id, sender_id, message_type,
--     ciphertext, ephemeral_public_key, metadata, created_at, updated_at
-- ) VALUES (?, get_bucket_id(now()), ?, ?, ?, ?, ?, ?, now(), now());

-- Get recent messages for a conversation
-- PREPARE get_recent_messages AS
-- SELECT * FROM messages 
-- WHERE conversation_id = ? AND bucket_id = ? 
-- ORDER BY message_id DESC LIMIT ?;

-- Get unread count for user
-- PREPARE get_unread_count AS
-- SELECT SUM(unread_count) FROM user_conversations 
-- WHERE user_id = ?;

-- Update last read message
-- PREPARE update_last_read AS
-- UPDATE conversation_participants 
-- SET last_read_message_id = ? 
-- WHERE conversation_id = ? AND user_id = ?;

-- Mark messages as delivered/read
-- PREPARE mark_message_delivered AS
-- INSERT INTO message_delivery (
--     message_id, user_id, conversation_id, delivery_status, delivered_at
-- ) VALUES (?, ?, ?, 1, now());

-- PREPARE mark_message_read AS
-- UPDATE message_delivery 
-- SET delivery_status = 2, read_at = now() 
-- WHERE message_id = ? AND user_id = ?;
