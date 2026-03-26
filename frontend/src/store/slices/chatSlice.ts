import { createSlice, PayloadAction } from '@reduxjs/toolkit';

interface Message {
  id: string;
  conversationId: string;
  senderId: string;
  content: string;
  timestamp: number;
  status: 'sent' | 'delivered' | 'read' | 'failed';
  type: 'text' | 'image' | 'file';
  metadata?: Record<string, any>;
}

interface Conversation {
  id: string;
  name: string;
  avatar?: string;
  participants: string[];
  lastMessage?: Message;
  unreadCount: number;
  isGroup: boolean;
  createdAt: number;
  updatedAt: number;
}

interface ChatState {
  conversations: Conversation[];
  currentConversationId: string | null;
  messages: Record<string, Message[]>;
  typingUsers: Record<string, string[]>;
  connectionStatus: 'connecting' | 'connected' | 'disconnected' | 'error' | 'reconnecting';
  isLoading: boolean;
  error: string | null;
}

const initialState: ChatState = {
  conversations: [],
  currentConversationId: null,
  messages: {},
  typingUsers: {},
  connectionStatus: 'disconnected',
  isLoading: false,
  error: null,
};

const chatSlice = createSlice({
  name: 'chat',
  initialState,
  reducers: {
    setConversations: (state, action: PayloadAction<Conversation[]>) => {
      state.conversations = action.payload;
    },
    addConversation: (state, action: PayloadAction<Conversation>) => {
      state.conversations.unshift(action.payload);
    },
    updateConversation: (state, action: PayloadAction<Conversation>) => {
      const index = state.conversations.findIndex(c => c.id === action.payload.id);
      if (index !== -1) {
        state.conversations[index] = action.payload;
      }
    },
    setCurrentConversation: (state, action: PayloadAction<string | null>) => {
      state.currentConversationId = action.payload;
      // Mark conversation as read
      if (action.payload) {
        const conversation = state.conversations.find(c => c.id === action.payload);
        if (conversation) {
          conversation.unreadCount = 0;
        }
      }
    },
    setMessages: (state, action: PayloadAction<{ conversationId: string; messages: Message[] }>) => {
      state.messages[action.payload.conversationId] = action.payload.messages;
    },
    addMessage: (state, action: PayloadAction<{ conversationId: string; message: Message }>) => {
      const { conversationId, message } = action.payload;
      if (!state.messages[conversationId]) {
        state.messages[conversationId] = [];
      }
      state.messages[conversationId].push(message);
      
      // Update last message in conversation
      const conversation = state.conversations.find(c => c.id === conversationId);
      if (conversation) {
        conversation.lastMessage = message;
        conversation.updatedAt = message.timestamp;
        
        // Increment unread count if not current conversation
        if (state.currentConversationId !== conversationId) {
          conversation.unreadCount += 1;
        }
      }
    },
    updateMessageStatus: (state, action: PayloadAction<{ conversationId: string; messageId: string; status: Message['status'] }>) => {
      const { conversationId, messageId, status } = action.payload;
      const messages = state.messages[conversationId];
      if (messages) {
        const message = messages.find(m => m.id === messageId);
        if (message) {
          message.status = status;
        }
      }
    },
    setTypingUsers: (state, action: PayloadAction<{ conversationId: string; users: string[] }>) => {
      state.typingUsers[action.payload.conversationId] = action.payload.users;
    },
    addTypingUser: (state, action: PayloadAction<{ conversationId: string; userId: string }>) => {
      const { conversationId, userId } = action.payload;
      if (!state.typingUsers[conversationId]) {
        state.typingUsers[conversationId] = [];
      }
      if (!state.typingUsers[conversationId].includes(userId)) {
        state.typingUsers[conversationId].push(userId);
      }
    },
    removeTypingUser: (state, action: PayloadAction<{ conversationId: string; userId: string }>) => {
      const { conversationId, userId } = action.payload;
      if (state.typingUsers[conversationId]) {
        state.typingUsers[conversationId] = state.typingUsers[conversationId].filter(
          id => id !== userId
        );
      }
    },
    setLoading: (state, action: PayloadAction<boolean>) => {
      state.isLoading = action.payload;
    },
    setError: (state, action: PayloadAction<string | null>) => {
      state.error = action.payload;
    },
    clearError: (state) => {
      state.error = null;
    },
    setConnectionStatus: (state, action: PayloadAction<'connecting' | 'connected' | 'disconnected' | 'error' | 'reconnecting'>) => {
      state.connectionStatus = action.payload;
    },
  },
});

export const {
  setConversations,
  addConversation,
  updateConversation,
  setCurrentConversation,
  setMessages,
  addMessage,
  updateMessageStatus,
  setTypingUsers,
  addTypingUser,
  removeTypingUser,
  setLoading,
  setError,
  clearError,
  setConnectionStatus,
} = chatSlice.actions;

export default chatSlice.reducer;
