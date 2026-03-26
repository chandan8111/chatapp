package com.chatapp.presentation.chat

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.chatapp.data.remote.websocket.WebSocketManager
import com.chatapp.domain.model.Conversation
import com.chatapp.domain.model.Message
import com.chatapp.domain.repository.ChatRepository
import com.chatapp.domain.usecase.SendMessageUseCase
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.*
import kotlinx.coroutines.launch
import javax.inject.Inject

data class ChatUiState(
    val conversation: Conversation? = null,
    val messages: List<Message> = emptyList(),
    val currentUserId: String = "",
    val isLoading: Boolean = false,
    val error: String? = null,
    val typingUsers: List<String> = emptyList()
)

@HiltViewModel
class ChatViewModel @Inject constructor(
    private val chatRepository: ChatRepository,
    private val sendMessageUseCase: SendMessageUseCase,
    private val webSocketManager: WebSocketManager
) : ViewModel() {

    private val _uiState = MutableStateFlow(ChatUiState())
    val uiState: StateFlow<ChatUiState> = _uiState.asStateFlow()

    private var conversationId: String? = null

    fun loadConversation(convId: String) {
        conversationId = convId
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true) }
            
            try {
                // Load conversation details
                val conversation = chatRepository.getConversation(convId)
                _uiState.update { it.copy(conversation = conversation) }
                
                // Load messages
                chatRepository.getMessages(convId)
                    .collect { messages ->
                        _uiState.update { 
                            it.copy(
                                messages = messages,
                                isLoading = false
                            ) 
                        }
                    }
            } catch (e: Exception) {
                _uiState.update { 
                    it.copy(
                        error = e.message,
                        isLoading = false
                    ) 
                }
            }
        }
    }

    fun sendMessage(content: String) {
        val convId = conversationId ?: return
        
        viewModelScope.launch {
            try {
                sendMessageUseCase(convId, content)
            } catch (e: Exception) {
                _uiState.update { it.copy(error = e.message) }
            }
        }
    }

    fun sendTypingIndicator(isTyping: Boolean) {
        val convId = conversationId ?: return
        
        viewModelScope.launch {
            if (isTyping) {
                webSocketManager.sendTypingStart(convId)
            } else {
                webSocketManager.sendTypingStop(convId)
            }
        }
    }

    fun markMessageAsRead(messageId: String) {
        val convId = conversationId ?: return
        
        viewModelScope.launch {
            try {
                chatRepository.markMessageAsRead(convId, messageId)
            } catch (e: Exception) {
                // Silently fail
            }
        }
    }

    fun clearError() {
        _uiState.update { it.copy(error = null) }
    }
}
