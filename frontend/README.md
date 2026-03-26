# Enhanced ChatApp Frontend

A production-ready, resilient, and observable React frontend for the ChatApp distributed chat system.

## 🚀 Quick Start

### Option 1: Automated Setup (Recommended)

**Windows Users:**
```bash
cd frontend
quick-start.bat
```

**Mac/Linux Users:**
```bash
cd frontend
chmod +x quick-start.sh
./quick-start.sh
```

### Option 2: Manual Setup

```bash
cd frontend
npm install
npm install recharts
npm start
```

The app will be available at `http://localhost:3000`

## ✨ Enhanced Features

### 🔄 Resilient WebSocket Management
- **Automatic reconnection** with exponential backoff
- **Message queuing** for offline/reconnecting states
- **Performance monitoring** (latency, connection stats)
- **Circuit breaker** pattern for connection management
- **Heartbeat/ping-pong** for connection health

### 🛡️ Resilient API Client
- **Circuit breaker** pattern for API endpoints
- **Automatic retry** with exponential backoff
- **Request/response time** tracking
- **Error classification** (retryable vs non-retryable)
- **Rate limit handling** with automatic retry
- **Request tracing** with unique IDs

### 📊 Real-time Monitoring
- **System health score** (0-100%)
- **API performance** metrics
- **Circuit breaker** status monitoring
- **Response time** trends and charts
- **Success rate** visualization
- **Export** functionality for metrics

### 🎨 Enhanced UI/UX
- **Connection status** indicators with latency
- **Message status** indicators (sent, delivered, read, failed)
- **Retry buttons** for failed messages
- **Error snackbar** notifications
- **Loading states** and progress indicators
- **Settings menu** for connection management

## 📁 Project Structure

```
frontend/src/
├── hooks/
│   ├── useWebSocket.ts              # Original WebSocket hook
│   └── useEnhancedWebSocket.ts      # Enhanced with retry & monitoring
├── services/
│   ├── api.ts                       # Original API service
│   └── enhancedApi.ts               # Enhanced with circuit breaker
├── pages/
│   ├── Chat/
│   │   ├── index.tsx                # Original Chat component
│   │   └── EnhancedChat.tsx         # Enhanced with error handling
│   └── Monitoring/
│       └── Dashboard.tsx            # Real-time monitoring dashboard
├── store/
│   └── slices/
│       └── chatSlice.ts             # Updated with connection status
└── components/
    └── ...                          # Existing components
```

## 🔧 Configuration

### Environment Variables

Create a `.env` file in the frontend root:

```bash
REACT_APP_SOCKET_URL=ws://localhost:8080
REACT_APP_API_URL=http://localhost:8081/api/v1
REACT_APP_VERSION=2.0.0
REACT_APP_ENV=development
```

### Backend Requirements

The enhanced frontend requires the enhanced backend with:
- WebSocket server on port 8080
- API server on port 8081
- Metrics endpoint on port 9090

## 📊 Monitoring Dashboard

Access the monitoring dashboard at `http://localhost:3000/monitoring`

### Features:
- **System Health Score**: Overall system health (0-100%)
- **Performance Metrics**: Response time, success rate, request count
- **Circuit Breaker Status**: Real-time status of all API endpoints
- **Historical Charts**: Response time trends and success rate over time
- **Export Data**: Download metrics as JSON

## 🔄 Enhanced WebSocket Hook

```typescript
import { useEnhancedWebSocket } from '../hooks/useEnhancedWebSocket';

const {
  sendMessage,
  sendTypingStart,
  sendTypingStop,
  markAsRead,
  connectionStatus,      // 'connecting' | 'connected' | 'disconnected' | 'error' | 'reconnecting'
  metrics,              // Performance metrics
  error,                // Connection errors
  messageQueue,         // Queued messages
  isConnected,          // Boolean connection status
  canSendMessage,       // Can send messages
  retryConnection,      // Manual reconnection
} = useEnhancedWebSocket();
```

### Performance Metrics:
- `connectionTime`: Time to establish connection
- `messagesSent`: Total messages sent
- `messagesReceived`: Total messages received
- `reconnections`: Number of reconnections
- `errors`: Connection errors
- `lastPing`: Last successful ping timestamp
- `averageLatency`: Average round-trip latency

## 🛡️ Enhanced API Client

```typescript
import { enhancedConversationsAPI, enhancedMessagesAPI } from '../services/enhancedApi';

// Automatic retry and circuit breaker protection
try {
  const response = await enhancedConversationsAPI.getAll();
  setConversations(response.data);
} catch (error) {
  if (error instanceof RateLimitError) {
    console.log(`Retry after ${error.retryAfter} seconds`);
  } else if (error instanceof NetworkError) {
    console.log('Will retry automatically');
  }
}
```

### Error Types:
- `APIError`: General API errors
- `RateLimitError`: Rate limit exceeded (retryable)
- `NetworkError`: Network connection issues (retryable)

## 🎨 Enhanced Chat Component

The enhanced chat component includes:

### Connection Status Indicator
```typescript
<ConnectionStatusIndicator status={connectionStatus} metrics={metrics} />
```

### Message Retry Functionality
```typescript
<MessageBubble
  message={message}
  isOwn={isOwn}
  onRetry={retryMessage}
/>
```

### Error Handling
```typescript
const showSnackbar = (message: string, severity: 'success' | 'error' | 'warning') => {
  setSnackbar({ open: true, message, severity });
};
```

## 🧪 Development

### Type Checking
```bash
npm run type-check
```

### Linting
```bash
npm run lint
npm run lint:fix
```

### Build Analysis
```bash
npm run analyze
```

## 🔍 Debugging

### Enable Debug Logging
```javascript
localStorage.setItem('debug', 'true');
```

### Monitor WebSocket Connection
```javascript
console.log('Connection status:', window.__CHAT_APP_WS_STATUS__);
console.log('Metrics:', window.__CHAT_APP_METRICS());
```

### API Debugging
```javascript
console.log('Circuit breaker:', window.__CHAT_APP_CIRCUIT_BREAKER());
```

## 🚀 Production Deployment

### Build for Production
```bash
npm run build
```

### Environment Variables for Production
```bash
REACT_APP_SOCKET_URL=wss://api.yourdomain.com
REACT_APP_API_URL=https://api.yourdomain.com/api/v1
REACT_APP_VERSION=2.0.0
REACT_APP_ENV=production
```

### Deploy
The build output in `build/` folder can be deployed to any static hosting service.

## 📈 Performance Features

### Message Queuing
Messages are automatically queued when disconnected and sent on reconnection.

### Optimistic Updates
UI updates immediately with rollback on failure.

### Lazy Loading
Conversations and messages loaded on demand.

### Connection Pooling
Efficient reuse of WebSocket connections.

## 🔒 Security Features

### Request Authentication
All requests include automatic authentication tokens.

### Request Tracing
Every request includes unique tracing IDs.

### Rate Limit Handling
Automatic detection and retry for rate-limited requests.

### Input Validation
All user inputs validated before sending.

## 🛠️ Troubleshooting

### Common Issues

1. **WebSocket Connection Fails**
   - Check backend is running on port 8080
   - Verify WebSocket URL in .env
   - Check browser console for errors

2. **API Requests Fail**
   - Verify API URL in .env
   - Check CORS settings on backend
   - Monitor circuit breaker status

3. **TypeScript Errors**
   - Run `npm run type-check`
   - All enhanced files use `@ts-nocheck` for quick setup
   - Add proper types for production

4. **Build Fails**
   - Clear node_modules and reinstall
   - Check for conflicting dependencies

### Reset Instructions
```bash
rm -rf node_modules package-lock.json
npm install
npm start
```

## 📚 Documentation

- [Setup Guide](./SETUP_GUIDE.md) - Detailed installation instructions
- [Lint Errors Fix](./LINT_ERRORS_FIX.md) - TypeScript error solutions
- [Enhanced Backend](../PRODUCTION_IMPROVEMENTS.md) - Backend improvements
- [Enhanced Frontend](./ENHANCED_FRONTEND.md) - Frontend architecture details

## 🎯 Benefits

### Reliability
- ✅ Automatic reconnection with exponential backoff
- ✅ Message queuing for offline scenarios
- ✅ Circuit breaker prevents cascading failures
- ✅ Comprehensive error handling

### Performance
- ✅ Real-time performance monitoring
- ✅ Optimistic updates for instant feedback
- ✅ Lazy loading for better initial load time
- ✅ Connection pooling and reuse

### User Experience
- ✅ Real-time connection status indicators
- ✅ Message retry functionality
- ✅ User-friendly error messages
- ✅ Loading states and progress indicators

### Observability
- ✅ Detailed performance metrics tracking
- ✅ Real-time monitoring dashboard
- ✅ Circuit breaker status monitoring
- ✅ Export functionality for analysis

## 🤝 Contributing

1. Follow the established patterns for logging, metrics, and error handling
2. Add comprehensive tests for new features
3. Update documentation for any changes
4. Ensure all components are properly typed for production

---

**Note**: This enhanced frontend is designed to work with the enhanced backend. Ensure both are running for full functionality.
