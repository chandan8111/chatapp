package com.chatapp.data.remote.dto

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass
import java.util.Date

/**
 * Data Transfer Objects for API communication
 */

@JsonClass(generateAdapter = true)
data class ConversationDto(
    @Json(name = "id")
    val id: String,
    @Json(name = "name")
    val name: String,
    @Json(name = "avatar")
    val avatar: String? = null,
    @Json(name = "participants")
    val participants: List<String>,
    @Json(name = "last_message")
    val lastMessage: MessageDto? = null,
    @Json(name = "unread_count")
    val unreadCount: Int = 0,
    @Json(name = "is_group")
    val isGroup: Boolean = false,
    @Json(name = "created_at")
    val createdAt: Long = System.currentTimeMillis(),
    @Json(name = "updated_at")
    val updatedAt: Long = System.currentTimeMillis()
)

@JsonClass(generateAdapter = true)
data class MessageDto(
    @Json(name = "id")
    val id: String,
    @Json(name = "conversation_id")
    val conversationId: String,
    @Json(name = "sender_id")
    val senderId: String,
    @Json(name = "content")
    val content: String,
    @Json(name = "message_type")
    val type: Int = 0, // 0: text, 1: image, 2: file, 3: voice
    @Json(name = "timestamp")
    val timestamp: Long = System.currentTimeMillis(),
    @Json(name = "status")
    val status: String = "sent", // sent, delivered, read, failed
    @Json(name = "metadata")
    val metadata: Map<String, String> = emptyMap()
)

@JsonClass(generateAdapter = true)
data class UserDto(
    @Json(name = "id")
    val id: String,
    @Json(name = "username")
    val username: String,
    @Json(name = "email")
    val email: String,
    @Json(name = "avatar")
    val avatar: String? = null,
    @Json(name = "status")
    val status: String = "offline", // online, offline, away, busy
    @Json(name = "last_seen")
    val lastSeen: Long = 0,
    @Json(name = "created_at")
    val createdAt: Long = System.currentTimeMillis()
)

@JsonClass(generateAdapter = true)
data class PresenceDto(
    @Json(name = "user_id")
    val userId: String,
    @Json(name = "status")
    val status: String = "offline",
    @Json(name = "last_seen")
    val lastSeen: Long = 0,
    @Json(name = "device_info")
    val deviceInfo: Map<String, String> = emptyMap(),
    @Json(name = "location")
    val location: Map<String, String> = emptyMap()
)

@JsonClass(generateAdapter = true)
data class HealthStatusDto(
    @Json(name = "status")
    val status: String,
    @Json(name = "timestamp")
    val timestamp: Long = System.currentTimeMillis(),
    @Json(name = "services")
    val services: Map<String, String> = emptyMap(),
    @Json(name = "metrics")
    val metrics: Map<String, Any> = emptyMap()
)

@JsonClass(generateAdapter = true)
data class ApiErrorDto(
    @Json(name = "error")
    val error: String,
    @Json(name = "code")
    val code: String? = null,
    @Json(name = "details")
    val details: Map<String, Any>? = null,
    @Json(name = "timestamp")
    val timestamp: Long = System.currentTimeMillis()
)

@JsonClass(generateAdapter = true)
data class SearchUserDto(
    @Json(name = "id")
    val id: String,
    @Json(name = "username")
    val username: String,
    @Json(name = "email")
    val email: String,
    @Json(name = "avatar")
    val avatar: String? = null,
    @Json(name = "status")
    val status: String = "offline",
    @Json(name = "last_seen")
    val lastSeen: Long = 0
)

@JsonClass(generateAdapter = true)
data class LoginRequestDto(
    @Json(name = "email")
    val email: String,
    @Json(name = "password")
    val password: String,
    @Json(name = "device_info")
    val deviceInfo: Map<String, String> = emptyMap()
)

@JsonClass(generateAdapter = true)
data class LoginResponseDto(
    @Json(name = "token")
    val token: String,
    @Json(name = "refresh_token")
    val refreshToken: String,
    @Json(name = "user")
    val user: UserDto,
    @Json(name = "expires_in")
    val expiresIn: Long
)

@JsonClass(generateAdapter = true)
data class RegisterRequestDto(
    @Json(name = "username")
    val username: String,
    @Json(name = "email")
    val email: String,
    @Json(name = "password")
    val password: String,
    @Json(name = "device_info")
    val deviceInfo: Map<String, String> = emptyMap()
)

@JsonClass(generateAdapter = true)
data class RegisterResponseDto(
    @Json(name = "token")
    val token: String,
    @Json(name = "refresh_token")
    val refreshToken: String,
    @Json(name = "user")
    val user: UserDto,
    @Json(name = "expires_in")
    val expiresIn: Long
)

@JsonClass(generateAdapter = true)
data class CreateConversationRequestDto(
    @Json(name = "name")
    val name: String,
    @Json(name = "participants")
    val participants: List<String>,
    @Json(name = "is_group")
    val isGroup: Boolean = false,
    @Json(name = "avatar")
    val avatar: String? = null
)

@JsonClass(generateAdapter = true)
data class SendMessageRequestDto(
    @Json(name = "content")
    val content: String,
    @Json(name = "message_type")
    val messageType: Int = 0,
    @Json(name = "metadata")
    val metadata: Map<String, String> = emptyMap(),
    @Json(name = "reply_to")
    val replyTo: String? = null,
    @Json(name = "attachments")
    val attachments: List<String> = emptyList()
)

@JsonClass(generateAdapter = true)
data class UpdateProfileRequestDto(
    @Json(name = "username")
    val username: String? = null,
    @Json(name = "avatar")
    val avatar: String? = null,
    @Json(name = "status")
    val status: String? = null
)

@JsonClass(generateAdapter = true)
data class UpdatePresenceRequestDto(
    @Json(name = "status")
    val status: String,
    @Json(name = "device_info")
    val deviceInfo: Map<String, String> = emptyMap(),
    @Json(name = "location")
    val location: Map<String, String> = emptyMap()
)

@JsonClass(generateAdapter = true)
data class TypingNotificationDto(
    @Json(name = "type")
    val type: String, // typing_start, typing_stop
    @Json(name = "conversation_id")
    val conversationId: String,
    @Json(name = "user_id")
    val userId: String,
    @Json(name = "timestamp")
    val timestamp: Long = System.currentTimeMillis()
)

@JsonClass(generateAdapter = true)
data class MessageStatusNotificationDto(
    @Json(name = "type")
    val type: String, // message_status
    @Json(name = "conversation_id")
    val conversationId: String,
    @Json(name = "message_id")
    val messageId: String,
    @Json(name = "status")
    val status: String, // sent, delivered, read, failed
    @Json(name = "timestamp")
    val timestamp: Long = System.currentTimeMillis()
)

@JsonClass(generateAdapter = true)
data class PresenceNotificationDto(
    @Json(name = "type")
    val type: String, // presence_update
    @Json(name = "user_id")
    val userId: String,
    @Json(name = "status")
    val status: String,
    @Json(name = "last_seen")
    val lastSeen: Long = 0,
    @Json(name = "timestamp")
    val timestamp: Long = System.currentTimeMillis()
)

@JsonClass(generateAdapter = true)
data class UserStatusNotificationDto(
    @Json(name = "type")
    val type: String, // user_status
    @Json(name = "user_id")
    val userId: String,
    @Json(name = "status")
    val status: String,
    @Json(name = "timestamp")
    val timestamp: Long = System.currentTimeMillis()
)

@JsonClass(generateAdapter = true)
data class ConversationUpdateNotificationDto(
    @Json(name = "type")
    val type: String, // conversation_updated
    @Json(name = "conversation")
    val conversation: ConversationDto,
    @Json(name = "timestamp")
    val timestamp: Long = System.currentTimeMillis()
)

@JsonClass(generateAdapter = true)
data class ErrorNotificationDto(
    @Json(name = "type")
    val type: String, // error
    @Json(name = "message")
    val message: String,
    @Json(name = "code")
    val code: String? = null,
    @Json(name = "timestamp")
    val timestamp: Long = System.currentTimeMillis()
)

@JsonClass(generateAdapter = true)
data class FileUploadDto(
    @Json(name = "file_id")
    val fileId: String,
    @Json(name = "file_name")
    val fileName: String,
    @Json(name = "file_size")
    val fileSize: Long,
    @Json(name = "mime_type")
    val mimeType: String,
    @Json(name = "url")
    val url: String,
    @Json(name = "thumbnail_url")
    val thumbnailUrl: String? = null,
    @Json(name = "metadata")
    val metadata: Map<String, Any> = emptyMap()
)

@JsonClass(generateAdapter = true)
data class FileUploadRequestDto(
    @Json(name = "file_name")
    val fileName: String,
    @Json(name = "file_size")
    val fileSize: Long,
    @Json(name = "mime_type")
    val mimeType: String,
    @Json(name = "conversation_id")
    val conversationId: String,
    @Json(name = "metadata")
    val metadata: Map<String, Any> = emptyMap()
)

@JsonClass(generateAdapter = true)
data class SearchHistoryDto(
    @Json(name = "id")
    val id: String,
    @Json(name = "query")
    val query: String,
    @Json(name = "timestamp")
    val timestamp: Long = System.currentTimeMillis(),
    @Json(name = "results")
    val results: List<SearchUserDto> = emptyList()
)

@JsonClass(generateAdapter = true)
data class NotificationSettingsDto(
    @Json(name = "push_notifications")
    val pushNotifications: Boolean = true,
    @Json(name = "message_notifications")
    val messageNotifications: Boolean = true,
    @Json(name = "typing_notifications")
    val typingNotifications: Boolean = true,
    @Json(name = "online_notifications")
    val onlineNotifications: Boolean = true,
    @Json(name = "sound_enabled")
    val soundEnabled: Boolean = true,
    @Json(name = "vibration_enabled")
    val vibrationEnabled: Boolean = true,
    @Json(name = "do_not_disturb")
    val doNotDisturb: Boolean = false
)

@JsonClass(generateAdapter = true)
data class NotificationSettingsUpdateDto(
    @Json(name = "push_notifications")
    val pushNotifications: Boolean? = null,
    @Json(name = "message_notifications")
    val messageNotifications: Boolean? = null,
    @Json(name = "typing_notifications")
    val typingNotifications: Boolean? = null,
    @Json(name = "online_notifications")
    val onlineNotifications: Boolean? = null,
    @Json(name = "sound_enabled")
    val soundEnabled: Boolean? = null,
    @Json(name = "vibration_enabled")
    val vibrationEnabled: Boolean? = null,
    @Json(name = "do_not_disturb")
    val doNotDisturb: Boolean? = null
)
