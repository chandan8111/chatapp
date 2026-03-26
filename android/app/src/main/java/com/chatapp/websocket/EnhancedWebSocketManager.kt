package com.chatapp.websocket

import android.content.Context
import android.util.Log
import com.chatapp.BuildConfig
import com.chatapp.data.remote.dto.MessageDto
import com.chatapp.data.remote.dto.PresenceDto
import com.chatapp.domain.model.Message
import com.chatapp.domain.model.User
import com.chatapp.encryption.MessageEncryption
import com.chatapp.monitoring.PerformanceMonitor
import com.chatapp.resilience.CircuitBreaker
import com.chatapp.ratelimit.RateLimiter
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import okhttp3.*
import okio.ByteString
import org.json.JSONObject
import java.util.concurrent.TimeUnit
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Enhanced WebSocket manager with resilience, monitoring, and security
 * Integrates with the production-ready backend
 */
@Singleton
class EnhancedWebSocketManager @Inject constructor(
    @ApplicationContext private val context: Context,
    private val messageEncryption: MessageEncryption,
    private val performanceMonitor: PerformanceMonitor,
    private val circuitBreaker: CircuitBreaker,
    private val rateLimiter: RateLimiter
) {
    companion object {
        private const val TAG = "EnhancedWebSocketManager"
        private const val CONNECT_TIMEOUT = 10_000L
        private const val PING_INTERVAL = 30_000L
        private const val MAX_RECONNECT_ATTEMPTS = 10
        private const val RECONNECT_DELAY = 1000L
        private const val MESSAGE_QUEUE_SIZE = 1000
    }

    private val json = Json {
        ignoreUnknownKeys = true
        encodeDefaults = true
    }

    private val client = OkHttpClient.Builder()
        .connectTimeout(CONNECT_TIMEOUT, TimeUnit.MILLISECONDS)
        .readTimeout(0, TimeUnit.MILLISECONDS) // No timeout for read
        .writeTimeout(10_000, TimeUnit.MILLISECONDS)
        .pingInterval(PING_INTERVAL, TimeUnit.MILLISECONDS)
        .retryOnConnectionFailure(false) // We handle retries ourselves
        .addInterceptor(AuthInterceptor())
        .addInterceptor(MetricsInterceptor())
        .addInterceptor(RateLimitInterceptor())
        .build()

    private var webSocket: WebSocket? = null
    private val _connectionState = MutableStateFlow(ConnectionState.DISCONNECTED)
    val connectionState: StateFlow<ConnectionState> = _connectionState.asStateFlow()

    private val _messageQueue = MutableStateFlow<List<MessageDto>>(emptyList())
    val messageQueue: StateFlow<List<MessageDto>> = _messageQueue.asStateFlow()

    private val _presenceUpdates = MutableStateFlow<List<PresenceDto>>(emptyList())
    val presenceUpdates: StateFlow<List<PresenceDto>> = _presenceUpdates.asStateFlow()

    private val _connectionMetrics = MutableStateFlow(ConnectionMetrics())
    val connectionMetrics: StateFlow<ConnectionMetrics> = _connectionMetrics.asStateFlow()

    private val messageBuffer = mutableListOf<MessageDto>()
    private var reconnectAttempts = 0
    private var lastPingTime = 0L
    private var messagesSent = 0
    private var messagesReceived = 0
    private var connectionStartTime = 0L

    // WebSocket listener with enhanced error handling
    private val webSocketListener = object : WebSocketListener() {
        override fun onOpen(webSocket: WebSocket, response: Response) {
            Log.d(TAG, "WebSocket connected")
            connectionStartTime = System.currentTimeMillis()
            reconnectAttempts = 0
            _connectionState.value = ConnectionState.CONNECTED
            updateMetrics()
            
            // Send authentication message
            sendAuthMessage()
        }

        override fun onMessage(webSocket: WebSocket, text: String) {
            handleTextMessage(text)
        }

        override fun onMessage(webSocket: WebSocket, bytes: ByteString) {
            handleBinaryMessage(bytes)
        }

        override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
            Log.d(TAG, "WebSocket closing: $code - $reason")
            _connectionState.value = ConnectionState.DISCONNECTING
        }

        override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
            Log.d(TAG, "WebSocket closed: $code - $reason")
            _connectionState.value = ConnectionState.DISCONNECTED
            scheduleReconnect()
        }

        override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
            Log.e(TAG, "WebSocket failure", t)
            _connectionState.value = ConnectionState.ERROR
            handleConnectionFailure(t)
        }
    }

    /**
     * Connect to WebSocket server
     */
    fun connect(userId: String, token: String) {
        if (_connectionState.value == ConnectionState.CONNECTED) {
            Log.d(TAG, "Already connected")
            return
        }

        val request = Request.Builder()
            .url("${BuildConfig.WEBSOCKET_URL}/ws?user_id=$userId&device_id=android&node_id=mobile")
            .header("Authorization", "Bearer $token")
            .header("User-Agent", "ChatApp-Android/${BuildConfig.VERSION_NAME}")
            .build()

        circuitBreaker.execute("websocket_connect") {
            webSocket = client.newWebSocket(request, webSocketListener)
            _connectionState.value = ConnectionState.CONNECTING
        }
    }

    /**
     * Disconnect from WebSocket server
     */
    fun disconnect() {
        webSocket?.close(1000, "Client disconnect")
        webSocket = null
        _connectionState.value = ConnectionState.DISCONNECTED
        messageBuffer.clear()
    }

    /**
     * Send message with encryption and rate limiting
     */
    suspend fun sendMessage(message: Message): Result<Unit> {
        if (!isConnected()) {
            return Result.failure(Exception("Not connected"))
        }

        // Check rate limit
        if (!rateLimiter.canSendMessage(message.senderId)) {
            return Result.failure(Exception("Rate limit exceeded"))
        }

        return circuitBreaker.execute("send_message") {
            // Encrypt message content
            val encryptedContent = messageEncryption.encrypt(message.content)
            
            val messageDto = MessageDto(
                id = message.id,
                conversationId = message.conversationId,
                senderId = message.senderId,
                content = encryptedContent,
                messageType = message.type,
                timestamp = message.timestamp,
                metadata = message.metadata
            )

            val messageJson = json.encodeToString(messageDto)
            val success = webSocket?.send(messageJson) == true
            
            if (success) {
                messagesSent++
                updateMetrics()
                rateLimiter.recordMessageSent(message.senderId)
                performanceMonitor.recordMessageSent()
                Result.success(Unit)
            } else {
                Result.failure(Exception("Failed to send message"))
            }
        }
    }

    /**
     * Send typing indicator
     */
    suspend fun sendTypingStart(conversationId: String, userId: String) {
        if (!isConnected()) return

        val typingMessage = mapOf(
            "type" to "typing_start",
            "conversation_id" to conversationId,
            "user_id" to userId,
            "timestamp" to System.currentTimeMillis()
        )

        val messageJson = JSONObject(typingMessage).toString()
        webSocket?.send(messageJson)
    }

    /**
     * Stop typing indicator
     */
    suspend fun sendTypingStop(conversationId: String, userId: String) {
        if (!isConnected()) return

        val typingMessage = mapOf(
            "type" to "typing_stop",
            "conversation_id" to conversationId,
            "user_id" to userId,
            "timestamp" to System.currentTimeMillis()
        )

        val messageJson = JSONObject(typingMessage).toString()
        webSocket?.send(messageJson)
    }

    /**
     * Mark message as read
     */
    suspend fun markAsRead(conversationId: String, messageId: String) {
        if (!isConnected()) return

        val readMessage = mapOf(
            "type" to "mark_read",
            "conversation_id" to conversationId,
            "message_id" to messageId,
            "timestamp" to System.currentTimeMillis()
        )

        val messageJson = JSONObject(readMessage).toString()
        webSocket?.send(messageJson)
    }

    /**
     * Update presence
     */
    suspend fun updatePresence(userId: String, status: String) {
        if (!isConnected()) return

        val presenceMessage = mapOf(
            "type" to "presence_update",
            "user_id" to userId,
            "status" to status,
            "timestamp" to System.currentTimeMillis()
        )

        val messageJson = JSONObject(presenceMessage).toString()
        webSocket?.send(messageJson)
    }

    private fun sendAuthMessage() {
        val authMessage = mapOf(
            "type" to "auth",
            "token" to "android_token", // This should be the actual auth token
            "timestamp" to System.currentTimeMillis()
        )

        val messageJson = JSONObject(authMessage).toString()
        webSocket?.send(messageJson)
    }

    private fun handleTextMessage(text: String) {
        try {
            val jsonObject = JSONObject(text)
            val type = jsonObject.getString("type")

            when (type) {
                "message" -> handleMessage(jsonObject)
                "message_status" -> handleMessageStatus(jsonObject)
                "typing_start" -> handleTypingStart(jsonObject)
                "typing_stop" -> handleTypingStop(jsonObject)
                "presence_update" -> handlePresenceUpdate(jsonObject)
                "user_status" -> handleUserStatus(jsonObject)
                "pong" -> handlePong(jsonObject)
                "error" -> handleError(jsonObject)
                else -> Log.w(TAG, "Unknown message type: $type")
            }

            messagesReceived++
            updateMetrics()
            performanceMonitor.recordMessageReceived()

        } catch (e: Exception) {
            Log.e(TAG, "Error handling message", e)
        }
    }

    private fun handleBinaryMessage(bytes: ByteString) {
        // Handle binary messages (e.g., file transfers, images)
        Log.d(TAG, "Received binary message: ${bytes.size} bytes")
    }

    private fun handleMessage(jsonObject: JSONObject) {
        try {
            val messageDto = json.decodeFromString<MessageDto>(jsonObject.toString())
            
            // Decrypt message content
            val decryptedContent = messageEncryption.decrypt(messageDto.content)
            val decryptedMessage = messageDto.copy(content = decryptedContent)
            
            // Add to queue
            val currentQueue = _messageQueue.value.toMutableList()
            currentQueue.add(decryptedMessage)
            _messageQueue.value = currentQueue
        } catch (e: Exception) {
            Log.e(TAG, "Error handling message", e)
        }
    }

    private fun handleMessageStatus(jsonObject: JSONObject) {
        // Handle message status updates (sent, delivered, read, failed)
        Log.d(TAG, "Message status update: ${jsonObject.getString("status")}")
    }

    private fun handleTypingStart(jsonObject: JSONObject) {
        try {
            val presenceDto = json.decodeFromString<PresenceDto>(jsonObject.toString())
            val currentPresence = _presenceUpdates.value.toMutableList()
            currentPresence.add(presenceDto)
            _presenceUpdates.value = currentPresence
        } catch (e: Exception) {
            Log.e(TAG, "Error handling typing start", e)
        }
    }

    private fun handleTypingStop(jsonObject: JSONObject) {
        try {
            val presenceDto = json.decodeFromString<PresenceDto>(jsonObject.toString())
            val currentPresence = _presenceUpdates.value.toMutableList()
            currentPresence.removeAll { it.userId == presenceDto.userId }
            _presenceUpdates.value = currentPresence
        } catch (e: Exception) {
            Log.e(TAG, "Error handling typing stop", e)
        }
    }

    private fun handlePresenceUpdate(jsonObject: JSONObject) {
        try {
            val presenceDto = json.decodeFromString<PresenceDto>(jsonObject.toString())
            val currentPresence = _presenceUpdates.value.toMutableList()
            
            // Remove old presence for this user
            currentPresence.removeAll { it.userId == presenceDto.userId }
            currentPresence.add(presenceDto)
            _presenceUpdates.value = currentPresence
        } catch (e: Exception) {
            Log.e(TAG, "Error handling presence update", e)
        }
    }

    private fun handleUserStatus(jsonObject: JSONObject) {
        // Handle user status updates (online, offline, away)
        Log.d(TAG, "User status update: ${jsonObject.getString("status")}")
    }

    private fun handlePong(jsonObject: JSONObject) {
        lastPingTime = System.currentTimeMillis()
        updateMetrics()
    }

    private fun handleError(jsonObject: JSONObject) {
        Log.e(TAG, "Server error: ${jsonObject.getString("message")}")
        _connectionState.value = ConnectionState.ERROR
    }

    private fun handleConnectionFailure(t: Throwable) {
        performanceMonitor.recordConnectionError()
        circuitBreaker.recordFailure("websocket_connection")
        scheduleReconnect()
    }

    private fun scheduleReconnect() {
        if (reconnectAttempts >= MAX_RECONNECT_ATTEMPTS) {
            Log.w(TAG, "Max reconnect attempts reached")
            _connectionState.value = ConnectionState.ERROR
            return
        }

        val delay = RECONNECT_DELAY * (1 shl minOf(reconnectAttempts, 5))
        
        _connectionState.value = ConnectionState.RECONNECTING
        
        // Schedule reconnect with exponential backoff
        kotlinx.coroutines.GlobalScope.launch {
            kotlinx.coroutines.delay(delay)
            if (_connectionState.value != ConnectionState.CONNECTED) {
                reconnectAttempts++
                Log.d(TAG, "Attempting reconnect $reconnectAttempts/$MAX_RECONNECT_ATTEMPTS")
                // Reconnect logic would go here
            }
        }
    }

    private fun updateMetrics() {
        val uptime = if (connectionStartTime > 0) System.currentTimeMillis() - connectionStartTime else 0
        val latency = if (lastPingTime > 0) System.currentTimeMillis() - lastPingTime else 0

        _connectionMetrics.value = ConnectionMetrics(
            uptime = uptime,
            latency = latency,
            messagesSent = messagesSent,
            messagesReceived = messagesReceived,
            reconnectAttempts = reconnectAttempts,
            queueSize = messageBuffer.size
        )
    }

    private fun isConnected(): Boolean {
        return _connectionState.value == ConnectionState.CONNECTED && webSocket != null
    }

    /**
     * Interceptor for adding authentication headers
     */
    private inner class AuthInterceptor : Interceptor {
        override fun intercept(chain: Interceptor.Chain): Response {
            val request = chain.request().newBuilder()
                .addHeader("Authorization", "Bearer ${getAuthToken()}")
                .addHeader("X-Client-Version", BuildConfig.VERSION_NAME)
                .addHeader("X-Client-Platform", "android")
                .build()
            return chain.proceed(request)
        }

        private fun getAuthToken(): String {
            // Get auth token from secure storage
            return "Bearer token" // This should be retrieved from secure storage
        }
    }

    /**
     * Interceptor for collecting metrics
     */
    private inner class MetricsInterceptor : Interceptor {
        override fun intercept(chain: Interceptor.Chain): Response {
            val startTime = System.currentTimeMillis()
            val response = chain.proceed(chain.request())
            val duration = System.currentTimeMillis() - startTime
            
            performanceMonitor.recordApiCall(chain.request().url.toString(), duration, response.isSuccessful)
            return response
        }
    }

    /**
     * Interceptor for rate limiting
     */
    private inner class RateLimitInterceptor : Interceptor {
        override fun intercept(chain: Interceptor.Chain): Response {
            val url = chain.request().url.toString()
            
            if (!rateLimiter.canMakeRequest(url)) {
                throw Exception("Rate limit exceeded")
            }
            
            val response = chain.proceed(chain.request())
            rateLimiter.recordRequest(url)
            return response
        }
    }
}

/**
 * Connection state enum
 */
enum class ConnectionState {
    DISCONNECTED,
    CONNECTING,
    CONNECTED,
    DISCONNECTING,
    ERROR,
    RECONNECTING
}

/**
 * Connection metrics data class
 */
data class ConnectionMetrics(
    val uptime: Long = 0,
    val latency: Long = 0,
    val messagesSent: Int = 0,
    val messagesReceived: Int = 0,
    val reconnectAttempts: Int = 0,
    val queueSize: Int = 0
)
