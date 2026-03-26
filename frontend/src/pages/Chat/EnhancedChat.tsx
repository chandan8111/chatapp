// @ts-nocheck
import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useSelector, useDispatch } from 'react-redux';
import {
  Box,
  Paper,
  TextField,
  IconButton,
  Typography,
  Avatar,
  Divider,
  List,
  ListItem,
  ListItemButton,
  ListItemAvatar,
  ListItemText,
  Badge,
  Fab,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  Chip,
  useTheme,
  useMediaQuery,
  Alert,
  Snackbar,
  LinearProgress,
  Tooltip,
  Menu,
  MenuItem,
  CircularProgress,
} from '@mui/material';
import {
  Send as SendIcon,
  AttachFile as AttachFileIcon,
  EmojiEmotions as EmojiIcon,
  Add as AddIcon,
  Search as SearchIcon,
  MoreVert as MoreVertIcon,
  Refresh as RefreshIcon,
  Settings as SettingsIcon,
  Error as ErrorIcon,
  CheckCircle as CheckCircleIcon,
  Schedule as ScheduleIcon,
} from '@mui/icons-material';
import EmojiPicker from 'emoji-picker-react';
import { RootState } from '../../store';
import {
  setConversations,
  setCurrentConversation,
  setMessages,
  addMessage,
} from '../../store/slices/chatSlice';
import { useEnhancedWebSocket } from '../../hooks/useEnhancedWebSocket';
import { enhancedConversationsAPI, enhancedMessagesAPI, enhancedUsersAPI, APIError } from '../../services/enhancedApi';

// Performance monitoring
interface PerformanceMetrics {
  messageCount: number;
  averageResponseTime: number;
  errorCount: number;
  lastActivity: number;
}

// Connection status indicator
const ConnectionStatusIndicator: React.FC<{ status: string; metrics: any }> = ({ status, metrics }) => {
  const theme = useTheme();
  
  const getStatusColor = () => {
    switch (status) {
      case 'connected': return theme.palette.success.main;
      case 'connecting': return theme.palette.warning.main;
      case 'reconnecting': return theme.palette.warning.main;
      case 'error': return theme.palette.error.main;
      default: return theme.palette.grey[500];
    }
  };

  const getStatusText = () => {
    switch (status) {
      case 'connected': return 'Connected';
      case 'connecting': return 'Connecting...';
      case 'reconnecting': return 'Reconnecting...';
      case 'error': return 'Connection Error';
      default: return 'Offline';
    }
  };

  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
      <Box
        sx={{
          width: 8,
          height: 8,
          borderRadius: '50%',
          backgroundColor: getStatusColor(),
        }}
      />
      <Typography variant="caption" color="text.secondary">
        {getStatusText()}
      </Typography>
      {status === 'connected' && (
        <Tooltip title={`Latency: ${metrics.averageLatency}ms`}>
          <Typography variant="caption" color="text.secondary">
            {metrics.averageLatency}ms
          </Typography>
        </Tooltip>
      )}
    </Box>
  );
};

// Message component with status indicators
const MessageBubble: React.FC<{
  message: any;
  isOwn: boolean;
  onRetry?: (messageId: string) => void;
}> = ({ message, isOwn, onRetry }) => {
  const theme = useTheme();
  
  const getStatusIcon = () => {
    switch (message.status) {
      case 'sent':
        return <CheckCircleIcon sx={{ fontSize: 12, opacity: 0.7 }} />;
      case 'delivered':
        return <CheckCircleIcon sx={{ fontSize: 12, opacity: 0.9 }} />;
      case 'read':
        return <CheckCircleIcon sx={{ fontSize: 12, color: 'primary.main' }} />;
      case 'failed':
        return <ErrorIcon sx={{ fontSize: 12, color: 'error.main' }} />;
      default:
        return <ScheduleIcon sx={{ fontSize: 12, opacity: 0.5 }} />;
    }
  };

  return (
    <Box
      sx={{
        display: 'flex',
        justifyContent: isOwn ? 'flex-end' : 'flex-start',
        mb: 1,
        maxWidth: '70%',
      }}
    >
      <Paper
        sx={{
          p: 1.5,
          backgroundColor: isOwn ? theme.palette.primary.main : theme.palette.grey[100],
          color: isOwn ? 'white' : theme.palette.text.primary,
          borderRadius: 2,
          position: 'relative',
        }}
      >
        <Typography variant="body1">{message.content}</Typography>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mt: 0.5 }}>
          <Typography variant="caption" sx={{ opacity: 0.7, fontSize: '0.75rem' }}>
            {new Date(message.timestamp).toLocaleTimeString()}
          </Typography>
          {isOwn && (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
              {getStatusIcon()}
              {message.status === 'failed' && onRetry && (
                <Tooltip title="Retry sending">
                  <IconButton
                    size="small"
                    onClick={() => onRetry(message.id)}
                    sx={{ p: 0.5, ml: 0.5 }}
                  >
                    <RefreshIcon sx={{ fontSize: 12 }} />
                  </IconButton>
                </Tooltip>
              )}
            </Box>
          )}
        </Box>
      </Paper>
    </Box>
  );
};

const EnhancedChat: React.FC = () => {
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down('md'));
  const dispatch = useDispatch();
  const { user } = useSelector((state: RootState) => state.auth);
  const { conversations, currentConversationId, messages, typingUsers } = useSelector(
    (state: RootState) => state.chat
  );

  // Enhanced WebSocket hook
  const {
    sendMessage,
    sendTypingStart,
    sendTypingStop,
    markAsRead,
    connectionStatus,
    metrics,
    error: wsError,
    messageQueue,
    isConnected,
    isReconnecting,
    clearError,
    retryConnection,
  } = useEnhancedWebSocket();

  // Component state
  const [messageInput, setMessageInput] = useState('');
  const [showEmojiPicker, setShowEmojiPicker] = useState(false);
  const [newConversationOpen, setNewConversationOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [snackbar, setSnackbar] = useState<{ open: boolean; message: string; severity: 'success' | 'error' | 'warning' }>({
    open: false,
    message: '',
    severity: 'success',
  });
  const [performanceMetrics, setPerformanceMetrics] = useState<PerformanceMetrics>({
    messageCount: 0,
    averageResponseTime: 0,
    errorCount: 0,
    lastActivity: Date.now(),
  });
  const [settingsMenuAnchor, setSettingsMenuAnchor] = useState<null | HTMLElement>(null);

  // Refs
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const typingTimeoutRef = useRef<NodeJS.Timeout>();

  // Show snackbar
  const showSnackbar = useCallback((message: string, severity: 'success' | 'error' | 'warning' = 'success') => {
    setSnackbar({ open: true, message, severity });
  }, []);

  // Handle API errors
  const handleAPIError = useCallback((error: any, operation: string) => {
    console.error(`${operation} failed:`, error);
    
    if (error instanceof APIError) {
      if (error.retryable) {
        showSnackbar(`${operation} failed. Retrying...`, 'warning');
      } else {
        showSnackbar(error.message, 'error');
      }
    } else {
      showSnackbar(`${operation} failed. Please try again.`, 'error');
    }
    
    setPerformanceMetrics(prev => ({
      ...prev,
      errorCount: prev.errorCount + 1,
    }));
  }, [showSnackbar]);

  // Load conversations with error handling
  const loadConversations = useCallback(async () => {
    setLoading(true);
    try {
      const response = await enhancedConversationsAPI.getAll();
      dispatch(setConversations(response.data));
      showSnackbar('Conversations loaded successfully', 'success');
    } catch (error) {
      handleAPIError(error, 'Loading conversations');
    } finally {
      setLoading(false);
    }
  }, [dispatch, handleAPIError, showSnackbar]);

  // Load messages with error handling
  const loadMessages = useCallback(async (conversationId: string) => {
    setLoading(true);
    try {
      const startTime = Date.now();
      const response = await enhancedMessagesAPI.getByConversation(conversationId);
      const responseTime = Date.now() - startTime;
      
      dispatch(setMessages({
        conversationId,
        messages: response.data,
      }));
      
      setPerformanceMetrics(prev => ({
        ...prev,
        averageResponseTime: Math.round((prev.averageResponseTime + responseTime) / 2),
        lastActivity: Date.now(),
      }));
    } catch (error) {
      handleAPIError(error, 'Loading messages');
    } finally {
      setLoading(false);
    }
  }, [dispatch, handleAPIError]);

  // Search users with error handling
  const handleSearch = useCallback(async () => {
    if (!searchQuery.trim()) return;
    
    setLoading(true);
    try {
      const response = await enhancedUsersAPI.search(searchQuery);
      setSearchResults(response.data);
    } catch (error) {
      handleAPIError(error, 'User search');
    } finally {
      setLoading(false);
    }
  }, [searchQuery, handleAPIError]);

  // Start new conversation with error handling
  const startConversation = useCallback(async (userId: string) => {
    try {
      const response = await enhancedConversationsAPI.create({
        name: '',
        participants: [userId],
        isGroup: false,
      });
      
      dispatch(setConversations([response.data, ...conversations]));
      setNewConversationOpen(false);
      setSearchQuery('');
      setSearchResults([]);
      showSnackbar('Conversation created successfully', 'success');
    } catch (error) {
      handleAPIError(error, 'Creating conversation');
    }
  }, [dispatch, conversations, handleAPIError, showSnackbar]);

  // Handle sending message with enhanced error handling
  const handleSendMessage = useCallback(async () => {
    if (!messageInput.trim() || !currentConversationId) return;

    const messageContent = messageInput.trim();
    setMessageInput('');

    // Create optimistic message
    const optimisticMessage = {
      id: `temp-${Date.now()}`,
      conversationId: currentConversationId,
      senderId: user?.id || '',
      content: messageContent,
      timestamp: Date.now(),
      status: 'sent' as const,
      type: 'text' as const,
    };

    dispatch(addMessage({
      conversationId: currentConversationId,
      message: optimisticMessage,
    }));

    // Send via WebSocket
    const success = sendMessage(currentConversationId, messageContent);
    
    if (success) {
      sendTypingStop(currentConversationId);
      setPerformanceMetrics(prev => ({
        ...prev,
        messageCount: prev.messageCount + 1,
        lastActivity: Date.now(),
      }));
    } else {
      // Update message status to failed
      optimisticMessage.status = 'failed';
      dispatch(addMessage({
        conversationId: currentConversationId,
        message: optimisticMessage,
      }));
      showSnackbar('Failed to send message. Please try again.', 'error');
    }
  }, [messageInput, currentConversationId, user?.id, dispatch, sendMessage, sendTypingStop, showSnackbar]);

  // Retry failed message
  const retryMessage = useCallback((messageId: string) => {
    // Find the message and retry sending
    const currentMessages = currentConversationId ? messages[currentConversationId] || [] : [];
    const failedMessage = currentMessages.find(m => m.id === messageId);
    
    if (failedMessage) {
      sendMessage(failedMessage.conversationId, failedMessage.content);
      showSnackbar('Retrying message...', 'warning');
    }
  }, [currentConversationId, messages, sendMessage, showSnackbar]);

  // Handle typing with debouncing
  const handleTyping = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    setMessageInput(e.target.value);
    
    if (currentConversationId && e.target.value) {
      sendTypingStart(currentConversationId);
      
      // Clear existing timeout
      if (typingTimeoutRef.current) {
        clearTimeout(typingTimeoutRef.current);
      }
      
      // Set new timeout to stop typing indicator
      typingTimeoutRef.current = setTimeout(() => {
        sendTypingStop(currentConversationId);
      }, 1000);
    }
  }, [currentConversationId, sendTypingStart, sendTypingStop]);

  // Handle emoji selection
  const handleEmojiClick = useCallback((emojiObject: any) => {
    setMessageInput(prev => prev + emojiObject.emoji);
    setShowEmojiPicker(false);
  }, []);

  // Refresh conversations
  const refreshConversations = useCallback(() => {
    loadConversations();
    if (currentConversationId) {
      loadMessages(currentConversationId);
    }
  }, [loadConversations, loadMessages, currentConversationId]);

  // Effects
  useEffect(() => {
    loadConversations();
  }, [loadConversations]);

  useEffect(() => {
    if (currentConversationId) {
      loadMessages(currentConversationId);
    }
  }, [currentConversationId, loadMessages]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, currentConversationId]);

  // Show WebSocket errors
  useEffect(() => {
    if (wsError) {
      showSnackbar(wsError, 'error');
    }
  }, [wsError, showSnackbar]);

  // Cleanup
  useEffect(() => {
    return () => {
      if (typingTimeoutRef.current) {
        clearTimeout(typingTimeoutRef.current);
      }
    };
  }, []);

  const currentMessages = currentConversationId ? messages[currentConversationId] || [] : [];
  const currentConversation = conversations.find(c => c.id === currentConversationId);
  const currentTypingUsers = currentConversationId ? typingUsers[currentConversationId] || [] : [];

  return (
    <Box sx={{ display: 'flex', height: '100%', gap: 2 }}>
      {/* Loading overlay */}
      {loading && (
        <LinearProgress sx={{ position: 'absolute', top: 0, left: 0, right: 0, zIndex: 1000 }} />
      )}

      {/* Conversations List */}
      <Paper
        sx={{
          width: isMobile ? '100%' : 320,
          display: isMobile && currentConversationId ? 'none' : 'flex',
          flexDirection: 'column',
        }}
      >
        <Box sx={{ p: 2, borderBottom: 1, borderColor: 'divider' }}>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Typography variant="h6" fontWeight="bold">
              Chats
            </Typography>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <ConnectionStatusIndicator status={connectionStatus} metrics={metrics} />
              <IconButton
                size="small"
                onClick={(e) => setSettingsMenuAnchor(e.currentTarget)}
              >
                <MoreVertIcon />
              </IconButton>
            </Box>
          </Box>
        </Box>

        <List sx={{ flexGrow: 1, overflow: 'auto' }}>
          {conversations.map((conversation) => (
            <ListItem key={conversation.id} disablePadding>
              <ListItemButton
                selected={currentConversationId === conversation.id}
                onClick={() => dispatch(setCurrentConversation(conversation.id))}
              >
                <ListItemAvatar>
                  <Badge
                    color="error"
                    badgeContent={conversation.unreadCount}
                    invisible={conversation.unreadCount === 0}
                  >
                    <Avatar src={conversation.avatar}>
                      {conversation.name?.charAt(0).toUpperCase()}
                    </Avatar>
                  </Badge>
                </ListItemAvatar>
                <ListItemText
                  primary={conversation.name}
                  secondary={
                    conversation.lastMessage
                      ? `${conversation.lastMessage.content.substring(0, 30)}...`
                      : 'No messages'
                  }
                  primaryTypographyProps={{ fontWeight: conversation.unreadCount > 0 ? 'bold' : 'normal' }}
                />
              </ListItemButton>
            </ListItem>
          ))}
        </List>

        <Fab
          color="primary"
          sx={{ position: 'absolute', bottom: 24, left: 240 }}
          onClick={() => setNewConversationOpen(true)}
          disabled={!isConnected}
        >
          <AddIcon />
        </Fab>
      </Paper>

      {/* Chat Area */}
      {currentConversationId ? (
        <Paper sx={{ flexGrow: 1, display: 'flex', flexDirection: 'column' }}>
          {/* Chat Header */}
          <Box sx={{ p: 2, borderBottom: 1, borderColor: 'divider', display: 'flex', alignItems: 'center', gap: 2 }}>
            <Avatar src={currentConversation?.avatar}>
              {currentConversation?.name?.charAt(0).toUpperCase()}
            </Avatar>
            <Box sx={{ flexGrow: 1 }}>
              <Typography variant="subtitle1" fontWeight="bold">
                {currentConversation?.name}
              </Typography>
              {currentTypingUsers.length > 0 && (
                <Typography variant="body2" color="text.secondary">
                  {currentTypingUsers.length === 1
                    ? 'typing...'
                    : `${currentTypingUsers.length} people typing...`}
                </Typography>
              )}
            </Box>
            <ConnectionStatusIndicator status={connectionStatus} metrics={metrics} />
          </Box>

          {/* Messages */}
          <Box sx={{ flexGrow: 1, overflow: 'auto', p: 2 }}>
            {currentMessages.map((message) => (
              <MessageBubble
                key={message.id}
                message={message}
                isOwn={message.senderId === user?.id}
                onRetry={retryMessage}
              />
            ))}
            <div ref={messagesEndRef} />
          </Box>

          {/* Message Input */}
          <Box sx={{ p: 2, borderTop: 1, borderColor: 'divider' }}>
            {showEmojiPicker && (
              <Box sx={{ position: 'absolute', bottom: 80, right: 16, zIndex: 1000 }}>
                <EmojiPicker onEmojiClick={handleEmojiClick} />
              </Box>
            )}
            <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
              <IconButton onClick={() => setShowEmojiPicker(!showEmojiPicker)}>
                <EmojiIcon />
              </IconButton>
              <IconButton>
                <AttachFileIcon />
              </IconButton>
              <TextField
                fullWidth
                variant="outlined"
                placeholder={isConnected ? "Type a message..." : "Reconnecting..."}
                value={messageInput}
                onChange={handleTyping}
                onKeyPress={(e) => e.key === 'Enter' && !e.shiftKey && handleSendMessage()}
                size="small"
                disabled={!isConnected}
              />
              <IconButton
                color="primary"
                onClick={handleSendMessage}
                disabled={!messageInput.trim() || !isConnected}
              >
                {isConnected ? <SendIcon /> : <CircularProgress size={20} />}
              </IconButton>
            </Box>
          </Box>
        </Paper>
      ) : (
        <Paper
          sx={{
            flexGrow: 1,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Typography variant="h6" color="text.secondary">
            Select a conversation to start chatting
          </Typography>
        </Paper>
      )}

      {/* New Conversation Dialog */}
      <Dialog open={newConversationOpen} onClose={() => setNewConversationOpen(false)} fullWidth>
        <DialogTitle>Start New Conversation</DialogTitle>
        <DialogContent>
          <Box sx={{ display: 'flex', gap: 1, mb: 2 }}>
            <TextField
              fullWidth
              placeholder="Search users..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              onKeyPress={(e) => e.key === 'Enter' && handleSearch()}
              disabled={loading}
            />
            <Button variant="contained" onClick={handleSearch} startIcon={<SearchIcon />} disabled={loading}>
              {loading ? <CircularProgress size={20} /> : <SearchIcon />}
            </Button>
          </Box>
          <List>
            {searchResults.map((searchUser) => (
              <ListItem key={searchUser.id} disablePadding>
                <ListItemButton onClick={() => startConversation(searchUser.id)}>
                  <ListItemAvatar>
                    <Avatar src={searchUser.avatar}>{searchUser.username.charAt(0).toUpperCase()}</Avatar>
                  </ListItemAvatar>
                  <ListItemText primary={searchUser.username} secondary={searchUser.email} />
                </ListItemButton>
              </ListItem>
            ))}
          </List>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setNewConversationOpen(false)}>Cancel</Button>
        </DialogActions>
      </Dialog>

      {/* Settings Menu */}
      <Menu
        anchorEl={settingsMenuAnchor}
        open={Boolean(settingsMenuAnchor)}
        onClose={() => setSettingsMenuAnchor(null)}
      >
        <MenuItem onClick={refreshConversations}>
          <RefreshIcon sx={{ mr: 1 }} />
          Refresh
        </MenuItem>
        <MenuItem onClick={() => retryConnection()}>
          <SettingsIcon sx={{ mr: 1 }} />
          Reconnect
        </MenuItem>
      </Menu>

      {/* Error Snackbar */}
      <Snackbar
        open={snackbar.open}
        autoHideDuration={6000}
        onClose={() => setSnackbar(prev => ({ ...prev, open: false }))}
      >
        <Alert
          onClose={() => setSnackbar(prev => ({ ...prev, open: false }))}
          severity={snackbar.severity}
          sx={{ width: '100%' }}
        >
          {snackbar.message}
        </Alert>
      </Snackbar>
    </Box>
  );
};

export default EnhancedChat;
