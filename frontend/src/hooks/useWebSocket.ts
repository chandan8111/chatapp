import { useEffect, useRef, useCallback } from 'react';
import { useDispatch, useSelector } from 'react-redux';
import { io, Socket } from 'socket.io-client';
import { RootState } from '../store';
import {
  addMessage,
  updateMessageStatus,
  addTypingUser,
  removeTypingUser,
  updateConversation,
} from '../store/slices/chatSlice';
import { updateUserStatus } from '../store/slices/authSlice';

const SOCKET_URL = process.env.REACT_APP_SOCKET_URL || 'ws://localhost:8080';

export const useWebSocket = () => {
  const socketRef = useRef<Socket | null>(null);
  const dispatch = useDispatch();
  const { user, token } = useSelector((state: RootState) => state.auth);
  const typingTimeoutRef = useRef<Record<string, NodeJS.Timeout>>({});

  const connect = useCallback(() => {
    if (!token || !user) return;

    socketRef.current = io(SOCKET_URL, {
      auth: { token },
      transports: ['websocket'],
      reconnection: true,
      reconnectionAttempts: 5,
      reconnectionDelay: 1000,
    });

    const socket = socketRef.current;

    // Connection events
    socket.on('connect', () => {
      console.log('WebSocket connected');
      dispatch(updateUserStatus('online'));
    });

    socket.on('disconnect', (reason) => {
      console.log('WebSocket disconnected:', reason);
      dispatch(updateUserStatus('offline'));
    });

    socket.on('connect_error', (error) => {
      console.error('WebSocket connection error:', error);
    });

    // Message events
    socket.on('message', (data) => {
      dispatch(addMessage({
        conversationId: data.conversation_id,
        message: {
          id: data.message_id,
          conversationId: data.conversation_id,
          senderId: data.sender_id,
          content: data.content,
          timestamp: data.timestamp,
          status: 'delivered',
          type: data.message_type || 'text',
          metadata: data.metadata,
        },
      }));
    });

    socket.on('message_status', (data) => {
      dispatch(updateMessageStatus({
        conversationId: data.conversation_id,
        messageId: data.message_id,
        status: data.status,
      }));
    });

    // Typing events
    socket.on('typing_start', (data) => {
      dispatch(addTypingUser({
        conversationId: data.conversation_id,
        userId: data.user_id,
      }));

      // Remove typing indicator after timeout
      const timeoutKey = `${data.conversation_id}_${data.user_id}`;
      if (typingTimeoutRef.current[timeoutKey]) {
        clearTimeout(typingTimeoutRef.current[timeoutKey]);
      }

      typingTimeoutRef.current[timeoutKey] = setTimeout(() => {
        dispatch(removeTypingUser({
          conversationId: data.conversation_id,
          userId: data.user_id,
        }));
        delete typingTimeoutRef.current[timeoutKey];
      }, 3000);
    });

    socket.on('typing_stop', (data) => {
      dispatch(removeTypingUser({
        conversationId: data.conversation_id,
        userId: data.user_id,
      }));
    });

    // Presence events
    socket.on('user_status', (data) => {
      if (data.user_id === user.id) {
        dispatch(updateUserStatus(data.status));
      }
    });

    // Conversation events
    socket.on('conversation_updated', (data) => {
      dispatch(updateConversation(data));
    });

    return () => {
      socket.disconnect();
    };
  }, [token, user, dispatch]);

  const disconnect = useCallback(() => {
    if (socketRef.current) {
      socketRef.current.disconnect();
      socketRef.current = null;
    }

    // Clear all typing timeouts
    Object.values(typingTimeoutRef.current).forEach(timeout => {
      clearTimeout(timeout);
    });
    typingTimeoutRef.current = {};
  }, []);

  const sendMessage = useCallback((conversationId: string, content: string, type: string = 'text') => {
    if (!socketRef.current?.connected) {
      console.error('WebSocket not connected');
      return false;
    }

    socketRef.current.emit('send_message', {
      conversation_id: conversationId,
      content,
      message_type: type,
      timestamp: Date.now(),
    });

    return true;
  }, []);

  const sendTypingStart = useCallback((conversationId: string) => {
    if (!socketRef.current?.connected) return;

    socketRef.current.emit('typing_start', {
      conversation_id: conversationId,
      timestamp: Date.now(),
    });
  }, []);

  const sendTypingStop = useCallback((conversationId: string) => {
    if (!socketRef.current?.connected) return;

    socketRef.current.emit('typing_stop', {
      conversation_id: conversationId,
      timestamp: Date.now(),
    });
  }, []);

  const markAsRead = useCallback((conversationId: string, messageId: string) => {
    if (!socketRef.current?.connected) return;

    socketRef.current.emit('mark_read', {
      conversation_id: conversationId,
      message_id: messageId,
    });
  }, []);

  useEffect(() => {
    return () => {
      disconnect();
    };
  }, [disconnect]);

  return {
    connect,
    disconnect,
    sendMessage,
    sendTypingStart,
    sendTypingStop,
    markAsRead,
    isConnected: socketRef.current?.connected || false,
  };
};
