package com.chatapp.data.remote.dto

import com.chatapp.domain.model.*

// Extension functions for Domain Model to DTO conversion

fun Conversation.toDto(): ConversationDto {
    return ConversationDto(
        id = id,
        name = name,
        avatar = avatar,
        participants = participants,
        lastMessage = lastMessage?.toDto(),
        unreadCount = unreadCount,
        isGroup = isGroup,
        createdAt = createdAt,
        updatedAt = updatedAt
    )
}

fun Message.toDto(): MessageDto {
    return MessageDto(
        id = id,
        conversationId = conversationId,
        senderId = senderId,
        content = content,
        type = type,
        timestamp = timestamp,
        status = status,
        metadata = metadata
    )
}

fun User.toDto(): UserDto {
    return UserDto(
        id = id,
        username = username,
        email = email,
        avatar = avatar,
        status = status,
        lastSeen = lastSeen,
        createdAt = createdAt
    )
}

fun Presence.toDto(): PresenceDto {
    return PresenceDto(
        userId = userId,
        status = status,
        lastSeen = lastSeen,
        deviceInfo = deviceInfo,
        location = location
    )
}

fun HealthStatus.toDto(): HealthStatusDto {
    return HealthStatusDto(
        status = status,
        timestamp = timestamp,
        services = services,
        metrics = metrics
    )
}
