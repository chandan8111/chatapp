# Enhanced Frontend Setup Guide

This guide will help you set up the enhanced ChatApp frontend with all the production-ready improvements.

## 🚀 Quick Start

### Prerequisites

- Node.js 16+ 
- npm or yarn
- Existing ChatApp backend running (or use the enhanced backend)

### 1. Install Dependencies

```bash
cd frontend
npm install
```

**Note**: The `package.json` already includes all required dependencies:
- React 18.2.0
- Material-UI 5.13.6
- Redux Toolkit 1.9.5
- Socket.io Client 4.7.1
- Axios 1.4.0
- Recharts 2.5.0 (for monitoring dashboard)
- TypeScript 5.1.3

### 2. Environment Configuration

Create a `.env` file in the frontend root:

```bash
# .env
REACT_APP_SOCKET_URL=ws://localhost:8080
REACT_APP_API_URL=http://localhost:8081/api/v1
REACT_APP_VERSION=2.0.0
REACT_APP_ENV=development
```

### 3. Start Development Server

```bash
npm start
```

The app will be available at `http://localhost:3000`

## 🔧 Lint Error Fixes

The enhanced frontend files may show lint errors initially. Here's how to fix them:

### 1. Install Missing Dependencies

```bash
npm install recharts
```

### 2. Type Definitions

The project already includes `@types/node` in devDependencies, but ensure it's installed:

```bash
npm install --save-dev @types/node
```

### 3. Fix TypeScript Errors

Most TypeScript errors will be resolved by:

1. **Running type check**:
   ```bash
   npm run type-check
   ```

2. **Fixing lint issues**:
   ```bash
   npm run lint:fix
   ```

### 4. Common Issues and Solutions

#### Issue: "Cannot find module 'react'"
```bash
npm install react react-dom @types/react @types/react-dom
```

#### Issue: "Cannot find module 'react-redux'"
```bash
npm install react-redux @reduxjs/toolkit
```

#### Issue: "Cannot find module '@mui/material'"
```bash
npm install @mui/material @mui/icons-material @emotion/react @emotion/styled
```

#### Issue: "Cannot find module 'axios'"
```bash
npm install axios
```

#### Issue: "Cannot find module 'socket.io-client'"
```bash
npm install socket.io-client
```

#### Issue: "Cannot find module 'emoji-picker-react'"
```bash
npm install emoji-picker-react
```

#### Issue: "Cannot find module 'recharts'"
```bash
npm install recharts
```

#### Issue: "Cannot find name 'process'"
```bash
npm install --save-dev @types/node
```

## 📁 File Structure

After setup, your enhanced frontend structure should look like:

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

## 🧪 Testing the Enhanced Features

### 1. Test Enhanced WebSocket

Open the browser console and watch for connection logs:

```javascript
// Should see:
// WebSocket connected successfully
// Connection metrics updated
// Latency measurements
```

### 2. Test API Resilience

Temporarily stop the backend server and try to send a message:

```javascript
// Should see:
// Circuit breaker activation
// Retry attempts
// Queued messages
```

### 3. Test Monitoring Dashboard

Navigate to `/monitoring` to see:

- System health score
- Performance metrics
- Circuit breaker status
- Response time charts

## 🔍 Debugging Tips

### 1. Enable Debug Logging

```javascript
// In browser console
localStorage.setItem('debug', 'true');
```

### 2. Monitor WebSocket Connection

```javascript
// Check connection status
console.log('Connection status:', window.__CHAT_APP_WS_STATUS__);

// View metrics
console.log('Metrics:', window.__CHAT_APP_METRICS());
```

### 3. API Debugging

```javascript
// View circuit breaker status
console.log('Circuit breaker:', window.__CHAT_APP_CIRCUIT_BREAKER());
```

## 🚀 Production Deployment

### 1. Build for Production

```bash
npm run build
```

### 2. Environment Variables for Production

```bash
# .env.production
REACT_APP_SOCKET_URL=wss://api.yourdomain.com
REACT_APP_API_URL=https://api.yourdomain.com/api/v1
REACT_APP_VERSION=2.0.0
REACT_APP_ENV=production
```

### 3. Deploy

The build output in `build/` folder can be deployed to any static hosting service.

## 📊 Performance Optimization

### 1. Bundle Analysis

```bash
npm run analyze
```

### 2. Lazy Loading

The enhanced components already implement lazy loading. To verify:

```javascript
// Check network tab for lazy-loaded chunks
// Should see separate chunks for monitoring dashboard
```

### 3. Service Worker

For production, consider adding a service worker for offline support:

```bash
npm install workbox-webpack-plugin
```

## 🔒 Security Considerations

### 1. Environment Variables

Never expose sensitive data in frontend environment variables.

### 2. API Security

The enhanced API client includes:
- Request tracing
- Rate limit handling
- Error sanitization

### 3. WebSocket Security

- Uses WSS in production
- Validates all incoming messages
- Implements connection limits

## 🛠️ Troubleshooting

### Common Issues

1. **WebSocket Connection Fails**
   - Check backend is running on correct port
   - Verify WebSocket URL in .env
   - Check browser console for errors

2. **API Requests Fail**
   - Verify API URL in .env
   - Check CORS settings on backend
   - Monitor circuit breaker status

3. **TypeScript Errors**
   - Run `npm run type-check`
   - Ensure all dependencies installed
   - Check tsconfig.json settings

4. **Build Fails**
   - Clear node_modules and reinstall
   - Check for conflicting dependencies
   - Verify TypeScript version

### Reset Instructions

If you encounter persistent issues:

```bash
# Clean slate
rm -rf node_modules package-lock.json
npm install
npm start
```

## 📈 Monitoring Setup

### 1. Backend Integration

Ensure the enhanced backend is running with:
- Metrics endpoint on port 9090
- WebSocket endpoint on port 8080
- API endpoints on port 8081

### 2. Frontend Monitoring

The monitoring dashboard will automatically:
- Collect performance metrics
- Track connection status
- Display system health
- Show API response times

### 3. Custom Metrics

Add custom metrics by extending the hooks:

```typescript
// In useEnhancedWebSocket.ts
const customMetric = {
  userActions: 0,
  featureUsage: {},
  // Add your custom metrics
};
```

## 🎯 Next Steps

1. **Run the application** and verify all features work
2. **Test error scenarios** (network issues, server downtime)
3. **Monitor performance** using the dashboard
4. **Customize styling** to match your brand
5. **Add custom features** using the enhanced hooks

## 📞 Support

If you encounter issues:

1. Check browser console for errors
2. Verify backend services are running
3. Review this troubleshooting guide
4. Check the enhanced backend documentation

---

**Note**: The enhanced frontend is designed to work with the enhanced backend. Ensure both are running for full functionality.
