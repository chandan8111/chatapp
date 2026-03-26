# Complete ChatApp Integration Guide

This guide shows how to integrate and run the complete enhanced ChatApp with both frontend and backend improvements.

## 🎯 System Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Frontend      │    │   Backend       │    │   External      │
│   (React)       │◄──►│   (Go)          │◄──►│   Services      │
│   Port: 3000    │    │   Port: 8080    │    │   Redis: 6379   │
│   Port: 8081    │    │   Port: 9090    │    │   Kafka: 9092   │
│                 │    │                 │    │   Scylla: 9042  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 🚀 Quick Start - Complete System

### Prerequisites

- Go 1.19+
- Node.js 16+
- Docker (for Redis, Kafka, ScyllaDB)
- Git

### Step 1: Clone and Setup Repository

```bash
git clone <repository-url>
cd chatapp
```

### Step 2: Start External Services

```bash
# Start Redis, Kafka, and ScyllaDB using Docker
docker-compose up -d

# Wait for services to be ready (30 seconds)
sleep 30
```

### Step 3: Start Enhanced Backend

```bash
cd backend

# Install Go dependencies
go mod tidy

# Run the enhanced backend
go run examples/enhanced_gateway.go

# Backend will start on:
# - WebSocket: ws://localhost:8080
# - API: http://localhost:8081
# - Metrics: http://localhost:9090
```

### Step 4: Start Enhanced Frontend

```bash
cd frontend

# Install dependencies
npm install
npm install recharts

# Start frontend (in new terminal)
npm start

# Frontend will be available at:
# - Chat App: http://localhost:3000
# - Monitoring: http://localhost:3000/monitoring
```

## 📊 Access Points

Once everything is running, you can access:

| Service | URL | Description |
|---------|-----|-------------|
| **Chat Application** | http://localhost:3000 | Main chat interface |
| **Monitoring Dashboard** | http://localhost:3000/monitoring | Real-time metrics |
| **Backend API** | http://localhost:8081/api/v1 | REST API endpoints |
| **Backend Health** | http://localhost:8080/health | Backend health check |
| **Metrics** | http://localhost:9090/metrics | Prometheus metrics |
| **Redis** | localhost:6379 | Message storage |
| **Kafka** | localhost:9092 | Message streaming |
| **ScyllaDB** | localhost:9042 | Persistent storage |

## 🔧 Configuration Files

### Backend Environment

Create `backend/.env`:
```bash
# Redis
REDIS_ADDR=localhost:6379

# Kafka
KAFKA_BROKERS=localhost:9092

# ScyllaDB
SCYLLA_HOSTS=localhost:9042
SCYLLA_KEYSPACE=chatapp

# Service Ports
PORT=8080
METRICS_PORT=9090
API_PORT=8081

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
LOG_FILE=/var/log/chatapp/gateway.log

# Resilience
SHUTDOWN_TIMEOUT=30s
```

### Frontend Environment

Create `frontend/.env`:
```bash
REACT_APP_SOCKET_URL=ws://localhost:8080
REACT_APP_API_URL=http://localhost:8081/api/v1
REACT_APP_VERSION=2.0.0
REACT_APP_ENV=development
```

## 🧪 Testing the Integration

### 1. Test Backend Health

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{
  "status": "healthy",
  "timestamp": 1640995200,
  "service": "chatapp-gateway",
  "version": "2.0.0",
  "redis": "healthy",
  "scylla": "healthy",
  "metrics": {
    "active_connections": 0,
    "total_connections": 0,
    "messages_total": 0
  }
}
```

### 2. Test WebSocket Connection

Open browser console and navigate to `http://localhost:3000`:

```javascript
// Should see WebSocket connection logs
// WebSocket connected successfully
// Connection metrics updated
```

### 3. Test API Resilience

```bash
# Test rate limiting
for i in {1..15}; do
  curl http://localhost:8081/api/v1/conversations
done
```

Expected: Rate limit after 10 requests

### 4. Test Circuit Breaker

```bash
# Stop Redis temporarily
docker-compose stop redis

# Make API calls
curl http://localhost:8081/api/v1/conversations

# Restart Redis
docker-compose start redis
```

Expected: Circuit breaker activation and recovery

## 📈 Monitoring Features

### Frontend Monitoring Dashboard

Access `http://localhost:3000/monitoring` to see:

- **System Health Score**: Overall system health (0-100%)
- **Performance Metrics**: Response times, success rates
- **Circuit Breaker Status**: Real-time API endpoint status
- **Connection Metrics**: WebSocket connection statistics
- **Historical Charts**: Performance trends over time

### Backend Metrics

Access `http://localhost:9090/metrics` for Prometheus metrics:

```
chatapp_gateway_connections_active 5
chatapp_gateway_messages_total 150
chatapp_gateway_redis_operations_total 200
chatapp_gateway_scylla_operations_total 100
```

### Health Checks

- **Backend**: `http://localhost:8080/health`
- **Frontend**: Connection status in UI
- **External Services**: Docker container health

## 🔄 Testing Resilience Features

### 1. WebSocket Resilience

```javascript
// In browser console
// Test connection loss and recovery
window.__CHAT_APP_WS_DISCONNECT__();
// Wait 5 seconds
window.__CHAT_APP_WS_RECONNECT__();
```

### 2. API Circuit Breaker

```bash
# Simulate service failure
docker-compose stop redis

# Make API calls - should fail fast after circuit breaker opens
curl http://localhost:8081/api/v1/conversations

# Restore service
docker-compose start redis

# Should recover automatically
curl http://localhost:8081/api/v1/conversations
```

### 3. Message Queuing

1. Disconnect from internet
2. Send messages in chat
3. Reconnect
4. Messages should be sent automatically

## 🚨 Troubleshooting

### Common Issues

1. **Port Conflicts**
   ```bash
   # Check what's using ports
   netstat -tulpn | grep :8080
   netstat -tulpn | grep :3000
   ```

2. **Docker Services Not Starting**
   ```bash
   # Check Docker status
   docker-compose ps
   docker-compose logs
   ```

3. **WebSocket Connection Fails**
   - Check backend is running on port 8080
   - Verify firewall settings
   - Check browser console for errors

4. **API Requests Fail**
   - Verify API URL in frontend .env
   - Check CORS settings
   - Monitor circuit breaker status

### Reset Everything

```bash
# Stop all services
docker-compose down
pkill -f "go run"
pkill -f "npm start"

# Clean restart
docker-compose up -d
cd backend && go run examples/enhanced_gateway.go &
cd frontend && npm start &
```

## 📊 Performance Testing

### Load Testing Script

```bash
#!/bin/bash
# load-test.sh

echo "Starting load test..."

# Test WebSocket connections
for i in {1..100}; do
  curl -s http://localhost:8080/health > /dev/null &
done

# Test API endpoints
for i in {1..1000}; do
  curl -s http://localhost:8081/api/v1/health > /dev/null &
done

echo "Load test complete"
```

### Monitoring During Load

Watch the monitoring dashboard during load testing to observe:
- Response time increases
- Circuit breaker activations
- Connection pool usage
- Memory and CPU usage

## 🔒 Security Testing

### Test Rate Limiting

```bash
# Test connection rate limiting
for i in {1..20}; do
  curl -H "X-Forwarded-For: 192.168.1.$i" \
       http://localhost:8080/ws
done

# Test API rate limiting
for i in {1..1100}; do
  curl http://localhost:8081/api/v1/conversations
done
```

### Test Input Validation

```bash
# Test malicious input
curl -X POST http://localhost:8081/api/v1/conversations \
  -H "Content-Type: application/json" \
  -d '{"name": "<script>alert(\"xss\")</script>"}'
```

## 📈 Production Deployment

### Docker Compose Production

```yaml
# docker-compose.prod.yml
version: '3.8'
services:
  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
    restart: always
    
  kafka:
    image: confluentinc/cp-kafka:latest
    ports: ["9092:9092"]
    environment:
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092
    depends_on: [zookeeper]
    
  scylla:
    image: scylladb/scylla:latest
    ports: ["9042:9042"]
    restart: always
    
  backend:
    build: ./backend
    ports: ["8080:8080", "8081:8081", "9090:9090"]
    environment:
      - REDIS_ADDR=redis
      - KAFKA_BROKERS=kafka
      - SCYLLA_HOSTS=scylla
    depends_on: [redis, kafka, scylla]
    restart: always
    
  frontend:
    build: ./frontend
    ports: ["3000:3000"]
    environment:
      - REACT_APP_SOCKET_URL=ws://localhost:8080
      - REACT_APP_API_URL=http://localhost:8081/api/v1
    depends_on: [backend]
    restart: always
```

### Production Deployment

```bash
# Deploy production stack
docker-compose -f docker-compose.prod.yml up -d

# Scale services
docker-compose -f docker-compose.prod.yml up -d --scale backend=3
```

## 🎯 Success Metrics

Your enhanced ChatApp is successfully integrated when:

- ✅ **Backend starts** without errors on all ports
- ✅ **Frontend loads** at http://localhost:3000
- ✅ **WebSocket connects** automatically
- ✅ **Monitoring dashboard** shows real-time metrics
- ✅ **Messages send** and receive correctly
- ✅ **Circuit breakers** activate on failures
- ✅ **Rate limiting** prevents abuse
- ✅ **Health checks** pass for all services
- ✅ **Resilience features** work on disconnection

## 📞 Support

For issues:

1. **Check logs**: Backend logs and browser console
2. **Verify connectivity**: All services running on correct ports
3. **Monitor metrics**: Dashboard shows system health
4. **Test resilience**: Disconnect/reconnect scenarios work

---

**Enjoy your production-ready, resilient ChatApp!** 🎉
