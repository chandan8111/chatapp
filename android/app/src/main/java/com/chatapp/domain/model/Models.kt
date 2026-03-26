package com.chatapp.domain.model

enum class MessageStatus {
    SENT,
    DELIVERED,
    READ
}

enum class MessageType {
    TEXT,
    IMAGE,
    FILE,
    AUDIO,
    VIDEO
}

data class Message(
    val id: String,
    val conversationId: String,
    val senderId: String,
    val content: String,
    val timestamp: Long,
    val status: MessageStatus,
    val type: MessageType
)

data class Conversation(
    val id: String,
    val name: String,
    val avatarUrl: String?,
    val participants: List<String>,
    val lastMessage: Message?,
    val unreadCount: Int,
    val isGroup: Boolean,
    val createdAt: Long
)

data class User(
    val id: String,
    val username: String,
    val email: String,
    val avatarUrl: String?,
    val status: UserStatus,
    val lastSeen: Long?
)

enum class UserStatus {
    ONLINE,
    OFFLINE,
    AWAY
}
