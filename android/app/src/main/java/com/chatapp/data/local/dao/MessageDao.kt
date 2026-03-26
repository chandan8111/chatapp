package com.chatapp.data.local.dao

import androidx.room.*
import com.chatapp.data.local.entity.MessageEntity
import com.chatapp.domain.model.MessageStatus
import kotlinx.coroutines.flow.Flow

@Dao
interface MessageDao {
    
    @Query("SELECT * FROM messages WHERE conversationId = :conversationId ORDER BY timestamp DESC")
    fun getMessagesByConversation(conversationId: String): Flow<List<MessageEntity>>
    
    @Query("SELECT * FROM messages WHERE id = :messageId LIMIT 1")
    suspend fun getMessageById(messageId: String): MessageEntity?
    
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertMessage(message: MessageEntity)
    
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertMessages(messages: List<MessageEntity>)
    
    @Update
    suspend fun updateMessage(message: MessageEntity)
    
    @Delete
    suspend fun deleteMessage(message: MessageEntity)
    
    @Query("DELETE FROM messages WHERE conversationId = :conversationId")
    suspend fun deleteMessagesByConversation(conversationId: String)
    
    @Query("UPDATE messages SET status = :status WHERE id = :messageId")
    suspend fun updateMessageStatus(messageId: String, status: MessageStatus)
    
    @Query("SELECT * FROM messages WHERE synced = 0 ORDER BY timestamp ASC")
    suspend fun getUnsyncedMessages(): List<MessageEntity>
    
    @Query("UPDATE messages SET synced = 1 WHERE id = :messageId")
    suspend fun markMessageAsSynced(messageId: String)
    
    @Query("SELECT COUNT(*) FROM messages WHERE conversationId = :conversationId AND status != :readStatus")
    suspend fun getUnreadCount(conversationId: String, readStatus: MessageStatus = MessageStatus.READ): Int
}
