package com.chatapp.data.remote.api

import com.chatapp.data.remote.dto.*
import retrofit2.Response
import retrofit2.http.*

interface ChatApi {
    
    // Auth endpoints
    @POST("auth/login")
    suspend fun login(@Body request: LoginRequest): Response<AuthResponse>
    
    @POST("auth/register")
    suspend fun register(@Body request: RegisterRequest): Response<AuthResponse>
    
    @POST("auth/logout")
    suspend fun logout(): Response<Unit>
    
    @GET("auth/me")
    suspend fun getCurrentUser(): Response<UserDto>
    
    // Conversation endpoints
    @GET("conversations")
    suspend fun getConversations(): Response<List<ConversationDto>>
    
    @GET("conversations/{id}")
    suspend fun getConversation(@Path("id") id: String): Response<ConversationDto>
    
    @POST("conversations")
    suspend fun createConversation(@Body request: CreateConversationRequest): Response<ConversationDto>
    
    @PUT("conversations/{id}")
    suspend fun updateConversation(
        @Path("id") id: String,
        @Body request: UpdateConversationRequest
    ): Response<ConversationDto>
    
    @DELETE("conversations/{id}")
    suspend fun deleteConversation(@Path("id") id: String): Response<Unit>
    
    // Message endpoints
    @GET("conversations/{conversationId}/messages")
    suspend fun getMessages(
        @Path("conversationId") conversationId: String,
        @Query("limit") limit: Int = 50,
        @Query("offset") offset: Int = 0
    ): Response<List<MessageDto>>
    
    @POST("conversations/{conversationId}/messages")
    suspend fun sendMessage(
        @Path("conversationId") conversationId: String,
        @Body request: SendMessageRequest
    ): Response<MessageDto>
    
    @PUT("messages/{id}/status")
    suspend fun updateMessageStatus(
        @Path("id") id: String,
        @Body request: UpdateStatusRequest
    ): Response<Unit>
    
    // User endpoints
    @GET("users")
    suspend fun getUsers(): Response<List<UserDto>>
    
    @GET("users/{id}")
    suspend fun getUser(@Path("id") id: String): Response<UserDto>
    
    @GET("users/search")
    suspend fun searchUsers(@Query("q") query: String): Response<List<UserDto>>
    
    // Presence endpoints
    @GET("presence/{userId}")
    suspend fun getUserPresence(@Path("userId") userId: String): Response<PresenceDto>
    
    @POST("presence/batch")
    suspend fun getBatchPresence(@Body request: BatchPresenceRequest): Response<Map<String, PresenceDto>>
    
    @PUT("presence/status")
    suspend fun updatePresence(@Body request: UpdatePresenceRequest): Response<Unit>
}
