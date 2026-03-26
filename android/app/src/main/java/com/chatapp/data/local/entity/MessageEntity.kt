package com.chatapp.data.local.entity

import androidx.room.Entity
import androidx.room.PrimaryKey
import com.chatapp.domain.model.MessageStatus
import com.chatapp.domain.model.MessageType

@Entity(tableName = "messages")
data class MessageEntity(
    @PrimaryKey val id: String,
    val conversationId: String,
    val senderId: String,
    val content: String,
    val encryptedContent: ByteArray?,
    val timestamp: Long,
    val status: MessageStatus,
    val type: MessageType,
    val synced: Boolean = false
) {
    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (javaClass != other?.javaClass) return false
        other as MessageEntity
        return id == other.id
    }

    override fun hashCode(): Int {
        return id.hashCode()
    }
}
