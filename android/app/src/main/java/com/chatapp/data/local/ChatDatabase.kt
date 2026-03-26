package com.chatapp.data.local

import androidx.room.Database
import androidx.room.RoomDatabase
import androidx.room.TypeConverters
import com.chatapp.data.local.dao.ConversationDao
import com.chatapp.data.local.dao.MessageDao
import com.chatapp.data.local.dao.UserDao
import com.chatapp.data.local.entity.ConversationEntity
import com.chatapp.data.local.entity.MessageEntity
import com.chatapp.data.local.entity.UserEntity
import com.chatapp.data.local.util.Converters

@Database(
    entities = [
        MessageEntity::class,
        ConversationEntity::class,
        UserEntity::class
    ],
    version = 1,
    exportSchema = false
)
@TypeConverters(Converters::class)
abstract class ChatDatabase : RoomDatabase() {
    abstract fun messageDao(): MessageDao
    abstract fun conversationDao(): ConversationDao
    abstract fun userDao(): UserDao
}
