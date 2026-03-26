// @ts-nocheck
import { useEffect, useRef, useCallback, useState } from 'react';
import { useDispatch, useSelector } from 'react-redux';
import { RootState } from '../store';
import {
  addMessage,
  updateMessageStatus,
  addTypingUser,
  removeTypingUser,
  updateConversation,
  setConnectionStatus,
} from '../store/slices/chatSlice';
import { updateUserStatus } from '../store/slices/authSlice';

// Enhanced WebSocket configuration with retry logic and monitoring
const WEBSOCKET_CONFIG = {
  url: process.env.REACT_APP_SOCKET_URL || 'ws://localhost:8080',
  maxReconnectAttempts: 10,
  reconnectDelay: 1000,
  reconnectBackoffMultiplier: 1.5,
  maxReconnectDelay: 30000,
  heartbeatInterval: 30000,
  connectionTimeout: 10000,
  messageQueueSize: 100,
};

// Connection status types
export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'error' | 'reconnecting';

// Message queue for offline/reconnecting state
interface QueuedMessage {
  id: string;
  type: string;
  data: any;
  timestamp: number;
  retryCount: number;
}

// Performance monitoring
interface PerformanceMetrics {
  connectionTime: number;
  messagesSent: number;
  messagesReceived: number;
  reconnections: number;
  errors: number;
  lastPing: number;
  averageLatency: number;
}

export const useEnhancedWebSocket = () => {
  const socketRef = useRef<WebSocket | null>(null);
  const dispatch = useDispatch();
  const { user, token } = useSelector((state: RootState) => state.auth);
  const typingTimeoutRef = useRef<Record<string, NodeJS.Timeout>>({});
  
  // Enhanced state management
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('disconnected');
  const [messageQueue, setMessageQueue] = useState<QueuedMessage[]>([]);
  const [metrics, setMetrics] = useState<PerformanceMetrics>({
    connectionTime: 0,
    messagesSent: 0,
    messagesReceived: 0,
    reconnections: 0,
    errors: 0,
    lastPing: 0,
    averageLatency: 0,
  });
  const [error, setError] = useState<string | null>(null);
  
  // Reconnection management
  const reconnectAttempts = useRef(0);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const heartbeatIntervalRef = useRef<NodeJS.Timeout | null>(null);
  const latencyMeasurements = useRef<number[]>([]);
  const connectionStartTime = useRef<number>(0);

  // Generate unique message ID
  const generateMessageId = () => `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;

  // Calculate connection URL with parameters
  const getConnectionUrl = () => {
    if (!user?.id) return null;
    
    const params = new URLSearchParams({
      user_id: user.id,
      device_id: `web-${navigator.userAgent.substring(0, 50)}`,
      node_id: 'web-client',
    });
    
    return `${WEBSOCKET_CONFIG.url}/ws?${params.toString()}`;
  };

  // Measure latency
  const measureLatency = useCallback(() => {
    if (!socketRef.current?.connected) return;
    
    const startTime = Date.now();
    const messageId = generateMessageId();
    
    const timeout = setTimeout(() => {
      setError('Latency measurement timeout');
      metrics.errors++;
    }, 5000);
    
    // Send ping message
    socketRef.current.send(JSON.stringify({
      type: 'ping',
      message_id: messageId,
      timestamp: startTime,
    }));
    
    // Listen for pong response
    const handlePong = (event: MessageEvent) => {
      try {
        const data = JSON.parse(event.data);
        if (data.type === 'pong' && data.message_id === messageId) {
          clearTimeout(timeout);
          const latency = Date.now() - startTime;
          
          latencyMeasurements.current.push(latency);
          if (latencyMeasurements.current.length > 10) {
            latencyMeasurements.current.shift();
          }
          
          const avgLatency = latencyMeasurements.current.reduce((a, b) => a + b, 0) / latencyMeasurements.current.length;
          
          setMetrics(prev => ({
            ...prev,
            lastPing: Date.now(),
            averageLatency: Math.round(avgLatency),
          }));
          
          socketRef.current?.removeEventListener('message', handlePong);
        }
      } catch (error) {
        console.error('Error handling pong:', error);
      }
    };
    
    socketRef.current.addEventListener('message', handlePong);
  }, []);

  // Start heartbeat
  const startHeartbeat = useCallback(() => {
    if (heartbeatIntervalRef.current) {
      clearInterval(heartbeatIntervalRef.current);
    }
    
    heartbeatIntervalRef.current = setInterval(() => {
      measureLatency();
    }, WEBSOCKET_CONFIG.heartbeatInterval);
  }, [measureLatency]);

  // Stop heartbeat
  const stopHeartbeat = useCallback(() => {
    if (heartbeatIntervalRef.current) {
      clearInterval(heartbeatIntervalRef.current);
      heartbeatIntervalRef.current = null;
    }
  }, []);

  // Process message queue
  const processMessageQueue = useCallback(() => {
    if (messageQueue.length === 0 || !socketRef.current?.connected) return;
    
    const queueCopy = [...messageQueue];
    const processedMessages: string[] = [];
    
    queueCopy.forEach((queuedMessage) => {
      try {
        socketRef.current!.send(JSON.stringify(queuedMessage.data));
        processedMessages.push(queuedMessage.id);
      } catch (error) {
        console.error('Failed to send queued message:', error);
        queuedMessage.retryCount++;
        
        if (queuedMessage.retryCount > 3) {
          processedMessages.push(queuedMessage.id);
          setError(`Failed to send message after 3 retries: ${queuedMessage.type}`);
        }
      }
    });
    
    setMessageQueue(prev => prev.filter(msg => !processedMessages.includes(msg.id)));
  }, [messageQueue]);

  // Queue message for sending
  const queueMessage = useCallback((type: string, data: any) => {
    const queuedMessage: QueuedMessage = {
      id: generateMessageId(),
      type,
      data,
      timestamp: Date.now(),
      retryCount: 0,
    };
    
    setMessageQueue(prev => {
      const newQueue = [...prev, queuedMessage];
      // Keep only the most recent messages
      if (newQueue.length > WEBSOCKET_CONFIG.messageQueueSize) {
        return newQueue.slice(-WEBSOCKET_CONFIG.messageQueueSize);
      }
      return newQueue;
    });
    
    // Try to send immediately if connected
    if (socketRef.current?.connected) {
      processMessageQueue();
    }
  }, [processMessageQueue]);

  // Handle WebSocket connection
  const connect = useCallback(() => {
    if (!token || !user?.id) {
      setError('Authentication required for WebSocket connection');
      return;
    }

    const url = getConnectionUrl();
    if (!url) {
      setError('Invalid connection URL');
      return;
    }

    setConnectionStatus('connecting');
    connectionStartTime.current = Date.now();
    setError(null);

    try {
      socketRef.current = new WebSocket(url);
      const socket = socketRef.current;

      // Connection timeout
      const connectionTimeout = setTimeout(() => {
        if (socket.readyState === WebSocket.CONNECTING) {
          socket.close();
          setError('Connection timeout');
          setConnectionStatus('error');
        }
      }, WEBSOCKET_CONFIG.connectionTimeout);

      socket.onopen = () => {
        clearTimeout(connectionTimeout);
        const connectionTime = Date.now() - connectionStartTime.current;
        
        setConnectionStatus('connected');
        setMetrics(prev => ({
          ...prev,
          connectionTime,
          reconnections: reconnectAttempts.current,
        }));
        
        dispatch(setConnectionStatus('connected'));
        dispatch(updateUserStatus('online'));
        
        // Reset reconnection attempts
        reconnectAttempts.current = 0;
        
        // Start heartbeat
        startHeartbeat();
        
        // Process message queue
        processMessageQueue();
        
        console.log('WebSocket connected successfully');
      };

      socket.onclose = (event) => {
        clearTimeout(connectionTimeout);
        stopHeartbeat();
        
        setConnectionStatus('disconnected');
        dispatch(setConnectionStatus('disconnected'));
        dispatch(updateUserStatus('offline'));
        
        console.log('WebSocket disconnected:', event.code, event.reason);
        
        // Attempt reconnection if not a normal closure
        if (event.code !== 1000 && reconnectAttempts.current < WEBSOCKET_CONFIG.maxReconnectAttempts) {
          attemptReconnection();
        }
      };

      socket.onerror = (event) => {
        clearTimeout(connectionTimeout);
        setError('WebSocket connection error');
        setMetrics(prev => ({ ...prev, errors: prev.errors + 1 }));
        console.error('WebSocket error:', event);
      };

      socket.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          handleMessage(data);
        } catch (error) {
          console.error('Error parsing WebSocket message:', error);
          setMetrics(prev => ({ ...prev, errors: prev.errors + 1 }));
        }
      };

    } catch (error) {
      setError(`Failed to create WebSocket connection: ${error}`);
      setConnectionStatus('error');
    }
  }, [token, user, dispatch, startHeartbeat, stopHeartbeat, processMessageQueue]);

  // Attempt reconnection with exponential backoff
  const attemptReconnection = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
    }

    reconnectAttempts.current++;
    setConnectionStatus('reconnecting');
    
    const delay = Math.min(
      WEBSOCKET_CONFIG.reconnectDelay * Math.pow(WEBSOCKET_CONFIG.reconnectBackoffMultiplier, reconnectAttempts.current - 1),
      WEBSOCKET_CONFIG.maxReconnectDelay
    );

    console.log(`Attempting reconnection ${reconnectAttempts.current}/${WEBSOCKET_CONFIG.maxReconnectAttempts} in ${delay}ms`);

    reconnectTimeoutRef.current = setTimeout(() => {
      connect();
    }, delay);
  }, [connect]);

  // Handle incoming messages
  const handleMessage = useCallback((data: any) => {
    setMetrics(prev => ({ ...prev, messagesReceived: prev.messagesReceived + 1 }));

    switch (data.type) {
      case 'message':
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
        break;

      case 'message_status':
        dispatch(updateMessageStatus({
          conversationId: data.conversation_id,
          messageId: data.message_id,
          status: data.status,
        }));
        break;

      case 'typing_start':
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
        break;

      case 'typing_stop':
        dispatch(removeTypingUser({
          conversationId: data.conversation_id,
          userId: data.user_id,
        }));
        break;

      case 'user_status':
        if (data.user_id === user?.id) {
          dispatch(updateUserStatus(data.status));
        }
        break;

      case 'conversation_updated':
        dispatch(updateConversation(data));
        break;

      case 'pong':
        // Handled in measureLatency
        break;

      case 'error':
        setError(data.message || 'WebSocket error received');
        setMetrics(prev => ({ ...prev, errors: prev.errors + 1 }));
        break;

      default:
        console.log('Unknown message type:', data.type);
    }
  }, [dispatch, user?.id]);

  // Send message with queuing and retry logic
  const sendMessage = useCallback((conversationId: string, content: string, type: string = 'text') => {
    const messageData = {
      type: 'send_message',
      conversation_id: conversationId,
      content,
      message_type: type,
      timestamp: Date.now(),
    };

    queueMessage('send_message', messageData);
    setMetrics(prev => ({ ...prev, messagesSent: prev.messagesSent + 1 }));
    
    return true;
  }, [queueMessage]);

  // Send typing indicator
  const sendTypingStart = useCallback((conversationId: string) => {
    queueMessage('typing_start', {
      type: 'typing_start',
      conversation_id: conversationId,
      timestamp: Date.now(),
    });
  }, [queueMessage]);

  const sendTypingStop = useCallback((conversationId: string) => {
    queueMessage('typing_stop', {
      type: 'typing_stop',
      conversation_id: conversationId,
      timestamp: Date.now(),
    });
  }, [queueMessage]);

  // Mark message as read
  const markAsRead = useCallback((conversationId: string, messageId: string) => {
    queueMessage('mark_read', {
      type: 'mark_read',
      conversation_id: conversationId,
      message_id: messageId,
      timestamp: Date.now(),
    });
  }, [queueMessage]);

  // Disconnect WebSocket
  const disconnect = useCallback(() => {
    // Clear timeouts
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    
    stopHeartbeat();

    // Clear typing timeouts
    Object.values(typingTimeoutRef.current).forEach(timeout => {
      clearTimeout(timeout);
    });
    typingTimeoutRef.current = {};

    // Close WebSocket
    if (socketRef.current) {
      socketRef.current.close(1000, 'Client disconnect');
      socketRef.current = null;
    }

    setConnectionStatus('disconnected');
    setMessageQueue([]);
    reconnectAttempts.current = 0;
  }, [stopHeartbeat]);

  // Auto-connect when token/user changes
  useEffect(() => {
    if (token && user?.id) {
      connect();
    } else {
      disconnect();
    }

    return () => {
      disconnect();
    };
  }, [token, user?.id, connect, disconnect]);

  // Cleanup on unmount
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
    
    // Enhanced state
    connectionStatus,
    metrics,
    error,
    messageQueue,
    
    // Computed properties
    isConnected: connectionStatus === 'connected',
    isReconnecting: connectionStatus === 'reconnecting',
    canSendMessage: connectionStatus === 'connected',
    
    // Actions
    clearError: () => setError(null),
    retryConnection: connect,
  };
};
