package com.chatapp.data.remote.api

import android.content.Context
import android.util.Log
import com.chatapp.BuildConfig
import com.chatapp.data.remote.dto.*
import com.chatapp.domain.model.*
import com.chatapp.monitoring.PerformanceMonitor
import com.chatapp.resilience.CircuitBreaker
import com.chatapp.ratelimit.RateLimiter
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow
import okhttp3.*
import okhttp3.logging.HttpLoggingInterceptor
import retrofit2.HttpException
import retrofit2.Retrofit
import retrofit2.converter.gson.GsonConverterFactory
import retrofit2.http.*
import java.io.IOException
import java.util.concurrent.TimeUnit
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Enhanced API client with circuit breaker, rate limiting, and monitoring
 * Integrates with the production-ready backend
 */
@Singleton
class EnhancedChatApi @Inject constructor(
    @ApplicationContext private val context: Context,
    private val performanceMonitor: PerformanceMonitor,
    private val circuitBreaker: CircuitBreaker,
    private val rateLimiter: RateLimiter
) {
    companion object {
        private const val TAG = "EnhancedChatApi"
        private const val CONNECT_TIMEOUT = 10_000L
        private const val READ_TIMEOUT = 30_000L
        private const val WRITE_TIMEOUT = 30_000L
    }

    private val retrofit: Retrofit
    private val api: ChatApiService

    init {
        val okHttpClient = OkHttpClient.Builder()
            .connectTimeout(CONNECT_TIMEOUT, TimeUnit.MILLISECONDS)
            .readTimeout(READ_TIMEOUT, TimeUnit.MILLISECONDS)
            .writeTimeout(WRITE_TIMEOUT, TimeUnit.MILLISECONDS)
            .addInterceptor(AuthInterceptor())
            .addInterceptor(MetricsInterceptor())
            .addInterceptor(RateLimitInterceptor())
            .addInterceptor(RetryInterceptor())
            .addInterceptor(
                HttpLoggingInterceptor().apply {
                    level = if (BuildConfig.DEBUG) {
                        HttpLoggingInterceptor.Level.BODY
                    } else {
                        HttpLoggingInterceptor.Level.NONE
                    }
                }
            )
            .build()

        retrofit = Retrofit.Builder()
            .baseUrl(BuildConfig.API_BASE_URL)
            .client(okHttpClient)
            .addConverterFactory(GsonConverterFactory.create())
            .build()

        api = retrofit.create(ChatApiService::class.java)
    }

    /**
     * Get all conversations with resilience
     */
    suspend fun getConversations(): Result<List<Conversation>> {
        return circuitBreaker.execute("get_conversations") {
            rateLimiter.checkRateLimit("conversations")
            
            val startTime = System.currentTimeMillis()
            try {
                val response = api.getConversations()
                val duration = System.currentTimeMillis() - startTime
                
                performanceMonitor.recordApiCall("conversations", duration, true)
                Result.success(response.map { it.toDomainModel() })
            } catch (e: Exception) {
                val duration = System.currentTimeMillis() - startTime
                performanceMonitor.recordApiCall("conversations", duration, false)
                handleApiError(e)
            }
        }
    }

    /**
     * Get conversation by ID
     */
    suspend fun getConversation(id: String): Result<Conversation> {
        return circuitBreaker.execute("get_conversation") {
            rateLimiter.checkRateLimit("conversation")
            
            try {
                val response = api.getConversation(id)
                Result.success(response.toDomainModel())
            } catch (e: Exception) {
                handleApiError(e)
            }
        }
    }

    /**
     * Create new conversation
     */
    suspend fun createConversation(conversation: Conversation): Result<Conversation> {
        return circuitBreaker.execute("create_conversation") {
            rateLimiter.checkRateLimit("create_conversation")
            
            try {
                val response = api.createConversation(conversation.toDto())
                Result.success(response.toDomainModel())
            } catch (e: Exception) {
                handleApiError(e)
            }
        }
    }

    /**
     * Get messages for a conversation
     */
    suspend fun getMessages(conversationId: String, limit: Int = 50, offset: String? = null): Result<List<Message>> {
        return circuitBreaker.execute("get_messages") {
            rateLimiter.checkRateLimit("messages")
            
            try {
                val response = api.getMessages(conversationId, limit, offset)
                Result.success(response.map { it.toDomainModel() })
            } catch (e: Exception) {
                handleApiError(e)
            }
        }
    }

    /**
     * Send message
     */
    suspend fun sendMessage(message: Message): Result<Message> {
        return circuitBreaker.execute("send_message") {
            rateLimiter.checkRateLimit("send_message")
            
            try {
                val response = api.sendMessage(message.toDto())
                Result.success(response.toDomainModel())
            } catch (e: Exception) {
                handleApiError(e)
            }
        }
    }

    /**
     * Mark message as read
     */
    suspend fun markAsRead(conversationId: String, messageId: String): Result<Unit> {
        return circuitBreaker.execute("mark_as_read") {
            try {
                api.markAsRead(conversationId, messageId)
                Result.success(Unit)
            } catch (e: Exception) {
                handleApiError(e)
            }
        }
    }

    /**
     * Get user profile
     */
    suspend fun getUserProfile(userId: String): Result<User> {
        return circuitBreaker.execute("get_user_profile") {
            rateLimiter.checkRateLimit("user_profile")
            
            try {
                val response = api.getUserProfile(userId)
                Result.success(response.toDomainModel())
            } catch (e: Exception) {
                handleApiError(e)
            }
        }
    }

    /**
     * Update user profile
     */
    suspend fun updateUserProfile(user: User): Result<User> {
        return circuitBreaker.execute("update_user_profile") {
            rateLimiter.checkRateLimit("update_user_profile")
            
            try {
                val response = api.updateUserProfile(user.id, user.toDto())
                Result.success(response.toDomainModel())
            } catch (e: Exception) {
                handleApiError(e)
            }
        }
    }

    /**
     * Search users
     */
    suspend fun searchUsers(query: String, limit: Int = 20): Result<List<User>> {
        return circuitBreaker.execute("search_users") {
            rateLimiter.checkRateLimit("search_users")
            
            try {
                val response = api.searchUsers(query, limit)
                Result.success(response.map { it.toDomainModel() })
            } catch (e: Exception) {
                handleApiError(e)
            }
        }
    }

    /**
     * Get presence information
     */
    suspend fun getPresence(userIds: List<String>): Result<List<Presence>> {
        return circuitBreaker.execute("get_presence") {
            try {
                val response = api.getPresence(userIds)
                Result.success(response.map { it.toDomainModel() })
            } catch (e: Exception) {
                handleApiError(e)
            }
        }
    }

    /**
     * Update presence
     */
    suspend fun updatePresence(presence: Presence): Result<Unit> {
        return circuitBreaker.execute("update_presence") {
            try {
                api.updatePresence(presence.toDto())
                Result.success(Unit)
            } catch (e: Exception) {
                handleApiError(e)
            }
        }
    }

    /**
     * Health check
     */
    suspend fun healthCheck(): Result<HealthStatus> {
        return try {
            val response = api.healthCheck()
            Result.success(response.toDomainModel())
        } catch (e: Exception) {
            handleApiError(e)
        }
    }

    private fun handleApiError(exception: Exception): Result<Nothing> {
        return when (exception) {
            is HttpException -> {
                val statusCode = exception.code()
                val errorMessage = when (statusCode) {
                    401 -> "Authentication failed"
                    403 -> "Access forbidden"
                    404 -> "Resource not found"
                    409 -> "Conflict occurred"
                    422 -> "Invalid input"
                    429 -> "Rate limit exceeded"
                    in 500..599 -> "Server error"
                    else -> "HTTP error: $statusCode"
                }
                Result.failure(ApiException(errorMessage, statusCode, exception))
            }
            is IOException -> {
                Result.failure(NetworkException("Network error", exception))
            }
            else -> {
                Result.failure(exception)
            }
        }
    }

    /**
     * Auth interceptor for adding authentication headers
     */
    private inner class AuthInterceptor : Interceptor {
        override fun intercept(chain: Interceptor.Chain): Response {
            val request = chain.request().newBuilder()
                .addHeader("Authorization", "Bearer ${getAuthToken()}")
                .addHeader("X-Client-Version", BuildConfig.VERSION_NAME)
                .addHeader("X-Client-Platform", "android")
                .addHeader("Accept", "application/json")
                .addHeader("Content-Type", "application/json")
                .build()
            return chain.proceed(request)
        }

        private fun getAuthToken(): String {
            // Get auth token from secure storage
            return "Bearer token" // This should be retrieved from secure storage
        }
    }

    /**
     * Metrics interceptor for performance monitoring
     */
    private inner class MetricsInterceptor : Interceptor {
        override fun intercept(chain: Interceptor.Chain): Response {
            val startTime = System.currentTimeMillis()
            val url = chain.request().url.toString()
            
            val response = chain.proceed(chain.request())
            val duration = System.currentTimeMillis() - startTime
            
            performanceMonitor.recordApiCall(url, duration, response.isSuccessful)
            return response
        }
    }

    /**
     * Rate limiting interceptor
     */
    private inner class RateLimitInterceptor : Interceptor {
        override fun intercept(chain: Interceptor.Chain): Response {
            val url = chain.request().url.toString()
            
            if (!rateLimiter.canMakeRequest(url)) {
                throw RateLimitException("Rate limit exceeded for $url")
            }
            
            val response = chain.proceed(chain.request())
            rateLimiter.recordRequest(url)
            return response
        }
    }

    /**
     * Retry interceptor for automatic retries
     */
    private inner class RetryInterceptor : Interceptor {
        override fun intercept(chain: Interceptor.Chain): Response {
            val request = chain.request()
            
            // Don't retry for non-idempotent requests
            if (request.method != "GET" && request.method != "HEAD") {
                return chain.proceed(request)
            }
            
            var lastException: Exception? = null
            var response: Response? = null
            
            repeat(3) { attempt ->
                try {
                    response = chain.proceed(request)
                    if (response.isSuccessful) {
                        return response
                    }
                } catch (e: Exception) {
                    lastException = e
                    if (attempt < 2) {
                        Thread.sleep(1000L * (attempt + 1)) // Exponential backoff
                    }
                }
            }
            
            return response ?: throw lastException!!
        }
    }
}

/**
 * Retrofit API service interface
 */
interface ChatApiService {
    
    @GET("conversations")
    suspend fun getConversations(): List<ConversationDto>
    
    @GET("conversations/{id}")
    suspend fun getConversation(@Path("id") id: String): ConversationDto
    
    @POST("conversations")
    suspend fun createConversation(@Body conversation: ConversationDto): ConversationDto
    
    @GET("conversations/{id}/messages")
    suspend fun getMessages(
        @Path("id") conversationId: String,
        @Query("limit") limit: Int = 50,
        @Query("offset") offset: String? = null
    ): List<MessageDto>
    
    @POST("conversations/{id}/messages")
    suspend fun sendMessage(@Path("id") conversationId: String, @Body message: MessageDto): MessageDto
    
    @POST("conversations/{id}/messages/{messageId}/read")
    suspend fun markAsRead(@Path("id") conversationId: String, @Path("messageId") messageId: String)
    
    @GET("users/{id}")
    suspend fun getUserProfile(@Path("id") userId: String): UserDto
    
    @PUT("users/{id}")
    suspend fun updateUserProfile(@Path("id") userId: String, @Body user: UserDto): UserDto
    
    @GET("users/search")
    suspend fun searchUsers(@Query("q") query: String, @Query("limit") limit: Int = 20): List<UserDto>
    
    @POST("presence")
    suspend fun getPresence(@Body userIds: List<String>): List<PresenceDto>
    
    @PUT("presence")
    suspend fun updatePresence(@Body presence: PresenceDto)
    
    @GET("health")
    suspend fun healthCheck(): HealthStatusDto
}

/**
 * Custom exception classes
 */
class ApiException(message: String, val statusCode: Int, cause: Throwable? = null) : Exception(message, cause)
class NetworkException(message: String, cause: Throwable? = null) : Exception(message, cause)
class RateLimitException(message: String) : Exception(message)

/**
 * Extension functions for DTO to Domain Model conversion
 */
fun ConversationDto.toDomainModel(): Conversation {
    return Conversation(
        id = id,
        name = name,
        avatar = avatar,
        participants = participants,
        lastMessage = lastMessage?.toDomainModel(),
        unreadCount = unreadCount,
        isGroup = isGroup,
        createdAt = createdAt,
        updatedAt = updatedAt
    )
}

fun MessageDto.toDomainModel(): Message {
    return Message(
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

fun UserDto.toDomainModel(): User {
    return User(
        id = id,
        username = username,
        email = email,
        avatar = avatar,
        status = status,
        lastSeen = lastSeen,
        createdAt = createdAt
    )
}

fun PresenceDto.toDomainModel(): Presence {
    return Presence(
        userId = userId,
        status = status,
        lastSeen = lastSeen,
        deviceInfo = deviceInfo,
        location = location
    )
}

fun HealthStatusDto.toDomainModel(): HealthStatus {
    return HealthStatus(
        status = status,
        timestamp = timestamp,
        services = services,
        metrics = metrics
    )
}
