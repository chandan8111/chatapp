import React, { useState, useEffect, useRef } from 'react';
import { useSelector, useDispatch } from 'react-redux';
import {
  Box,
  Paper,
  TextField,
  IconButton,
  Typography,
  Avatar,
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
  useTheme,
  useMediaQuery,
} from '@mui/material';
import {
  Send as SendIcon,
  AttachFile as AttachFileIcon,
  EmojiEmotions as EmojiIcon,
  Add as AddIcon,
  Search as SearchIcon,
} from '@mui/icons-material';
import EmojiPicker from 'emoji-picker-react';
import { RootState } from '../../store';
import {
  setConversations,
  setCurrentConversation,
  setMessages,
  addMessage,
} from '../../store/slices/chatSlice';
import { useWebSocket } from '../../hooks/useWebSocket';
import { conversationsAPI, messagesAPI, usersAPI } from '../../services/api';

const Chat: React.FC = () => {
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down('md'));
  const dispatch = useDispatch();
  const { user } = useSelector((state: RootState) => state.auth);
  const { conversations, currentConversationId, messages, typingUsers } = useSelector(
    (state: RootState) => state.chat
  );
  const { sendMessage, sendTypingStart, sendTypingStop } = useWebSocket();

  const [messageInput, setMessageInput] = useState('');
  const [showEmojiPicker, setShowEmojiPicker] = useState(false);
  const [newConversationOpen, setNewConversationOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<any[]>([]);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Load conversations on mount
  useEffect(() => {
    const loadConversations = async () => {
      try {
        const response = await conversationsAPI.getAll();
        dispatch(setConversations(response.data));
      } catch (error) {
        console.error('Failed to load conversations:', error);
      }
    };
    loadConversations();
  }, [dispatch]);

  // Load messages when conversation changes
  useEffect(() => {
    if (currentConversationId) {
      const loadMessages = async () => {
        try {
          const response = await messagesAPI.getByConversation(currentConversationId);
          dispatch(setMessages({
            conversationId: currentConversationId,
            messages: response.data,
          }));
        } catch (error) {
          console.error('Failed to load messages:', error);
        }
      };
      loadMessages();
    }
  }, [currentConversationId, dispatch]);

  // Scroll to bottom when messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, currentConversationId]);

  // Handle sending message
  const handleSendMessage = async () => {
    if (!messageInput.trim() || !currentConversationId) return;

    const messageContent = messageInput.trim();
    setMessageInput('');

    // Send via WebSocket
    sendMessage(currentConversationId, messageContent);

    // Optimistically add message to UI
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

    sendTypingStop(currentConversationId);
  };

  // Handle emoji selection
  const handleEmojiClick = (emojiObject: any) => {
    setMessageInput((prev) => prev + emojiObject.emoji);
    setShowEmojiPicker(false);
  };

  // Handle typing
  const handleTyping = (e: React.ChangeEvent<HTMLInputElement>) => {
    setMessageInput(e.target.value);
    if (currentConversationId && e.target.value) {
      sendTypingStart(currentConversationId);
    }
  };

  // Search users
  const handleSearch = async () => {
    if (!searchQuery.trim()) return;
    try {
      const response = await usersAPI.search(searchQuery);
      setSearchResults(response.data);
    } catch (error) {
      console.error('Search failed:', error);
    }
  };

  // Start new conversation
  const startConversation = async (userId: string) => {
    try {
      const response = await conversationsAPI.create({
        name: '',
        participants: [userId],
        isGroup: false,
      });
      dispatch(setConversations([response.data, ...conversations]));
      setNewConversationOpen(false);
      setSearchQuery('');
      setSearchResults([]);
    } catch (error) {
      console.error('Failed to create conversation:', error);
    }
  };

  const currentMessages = currentConversationId ? messages[currentConversationId] || [] : [];
  const currentConversation = conversations.find(c => c.id === currentConversationId);
  const currentTypingUsers = currentConversationId ? typingUsers[currentConversationId] || [] : [];

  return (
    <Box sx={{ display: 'flex', height: '100%', gap: 2 }}>
      {/* Conversations List */}
      <Paper
        sx={{
          width: isMobile ? '100%' : 320,
          display: isMobile && currentConversationId ? 'none' : 'flex',
          flexDirection: 'column',
        }}
      >
        <Box sx={{ p: 2, borderBottom: 1, borderColor: 'divider' }}>
          <Typography variant="h6" fontWeight="bold">
            Chats
          </Typography>
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
            <Box>
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
          </Box>

          {/* Messages */}
          <Box sx={{ flexGrow: 1, overflow: 'auto', p: 2 }}>
            {currentMessages.map((message) => (
              <Box
                key={message.id}
                sx={{
                  display: 'flex',
                  justifyContent: message.senderId === user?.id ? 'flex-end' : 'flex-start',
                  mb: 1,
                }}
              >
                <Paper
                  sx={{
                    p: 1.5,
                    maxWidth: '70%',
                    backgroundColor: message.senderId === user?.id ? 'primary.main' : 'grey.100',
                    color: message.senderId === user?.id ? 'white' : 'text.primary',
                    borderRadius: 2,
                  }}
                >
                  <Typography variant="body1">{message.content}</Typography>
                  <Typography variant="caption" sx={{ opacity: 0.7 }}>
                    {new Date(message.timestamp).toLocaleTimeString()}
                    {message.senderId === user?.id && (
                      <span> • {message.status}</span>
                    )}
                  </Typography>
                </Paper>
              </Box>
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
                placeholder="Type a message..."
                value={messageInput}
                onChange={handleTyping}
                onKeyPress={(e) => e.key === 'Enter' && handleSendMessage()}
                size="small"
              />
              <IconButton
                color="primary"
                onClick={handleSendMessage}
                disabled={!messageInput.trim()}
              >
                <SendIcon />
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
            />
            <Button variant="contained" onClick={handleSearch} startIcon={<SearchIcon />}>
              Search
            </Button>
          </Box>
          <List>
            {searchResults.map((user) => (
              <ListItem key={user.id} disablePadding>
                <ListItemButton onClick={() => startConversation(user.id)}>
                  <ListItemAvatar>
                    <Avatar src={user.avatar}>{user.username.charAt(0).toUpperCase()}</Avatar>
                  </ListItemAvatar>
                  <ListItemText primary={user.username} secondary={user.email} />
                </ListItemButton>
              </ListItem>
            ))}
          </List>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setNewConversationOpen(false)}>Cancel</Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default Chat;
