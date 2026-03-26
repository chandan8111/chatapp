# ⚡ ChatApp Quick Reference Guide

This guide provides all essential commands, configurations, and access points for the enhanced ChatApp.

## 🚀 Quick Start Commands

### One-Command Demo
```bash
# Windows
demo.bat

# Mac/Linux
chmod +x demo.sh && ./demo.sh
```

### Manual Development Setup
```bash
# Start services
docker-compose -f docker-compose.dev.yml up -d

# Setup database
docker exec chatapp-scylla-dev cqlsh -e "CREATE KEYSPACE IF NOT EXISTS chatapp WITH REPLICATION = {'class': 'SimpleStrategy', 'replication_factor': 1};"

# Start backend
cd backend && go run examples/enhanced_gateway.go &

# Start frontend
cd frontend && npm start &
```

## 🌐 Access Points

| Service | URL | Username | Password |
|---------|-----|----------|----------|
| **Chat Application** | http://localhost:3000 | - | - |
| **Monitoring Dashboard** | http://localhost:3000/monitoring | - | - |
| **Backend Health** | http://localhost:8080/health | - | - |
| **Backend API** | http://localhost:8081/api/v1 | - | - |
| **Prometheus** | http://localhost:9090 | - | - |
| **Grafana** | http://localhost:3001 | admin | admin |

## 🔧 Essential Commands

### Docker Commands
```bash
# View running services
docker-compose ps

# View logs
docker-compose logs -f [service-name]

# Stop all services
docker-compose down

# Restart specific service
docker-compose restart [service-name]

# Execute command in container
docker exec chatapp-redis-dev redis-cli ping
docker exec chatapp-kafka-dev kafka-topics --list
docker exec chatapp-scylla-dev cqlsh
```

### Backend Commands
```bash
# Install dependencies
go mod tidy

# Run tests
go test ./...

# Build binary
go build -o chatapp examples/enhanced_gateway.go

# Run with custom config
REDIS_ADDR=localhost:6379 go run examples/enhanced_gateway.go

# View Go version
go version

# View modules
go mod list
```

### Frontend Commands
```bash
# Install dependencies
npm install

# Install missing package
npm install recharts

# Start development server
npm start

# Build for production
npm run build

# Run tests
npm test

# Run linting
npm run lint

# Fix linting
npm run lint:fix

# Type checking
npm run type-check

# Analyze bundle
npm run analyze
```

### Database Commands
```bash
# Redis
redis-cli -h localhost -p 6379 ping
redis-cli -h localhost -p 6379 keys "*"
redis-cli -h localhost -p 6379 flushall

# Kafka
kafka-topics.sh --list --bootstrap-server localhost:9092
kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic messages --from-beginning

# ScyllaDB
cqlsh localhost 9042
DESCRIBE KEYSPACES;
USE chatapp;
DESCRIBE TABLES;
SELECT * FROM users LIMIT 10;
```

## 📝 Configuration Files

### Backend Environment (.env)
```bash
REDIS_ADDR=localhost:6379
KAFKA_BROKERS=localhost:9092
SCYLLA_HOSTS=localhost:9042
SCYLLA_KEYSPACE=chatapp
PORT=8080
METRICS_PORT=9090
API_PORT=8081
LOG_LEVEL=info
LOG_FORMAT=json
LOG_FILE=/var/log/chatapp/gateway.log
SHUTDOWN_TIMEOUT=30s
```

### Frontend Environment (.env)
```bash
REACT_APP_SOCKET_URL=ws://localhost:8080
REACT_APP_API_URL=http://localhost:8081/api/v1
REACT_APP_VERSION=2.0.0
REACT_APP_ENV=development
```

### Docker Compose (Development)
```yaml
# docker-compose.dev.yml
version: '3.8'
services:
  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
    
  kafka:
    image: confluentinc/cp-kafka:7.3.0
    ports: ["9092:9092"]
    depends_on: [zookeeper]
    
  scylla:
    image: scylladb/scylla:5.1.0
    ports: ["9042:9042"]
    
  prometheus:
    image: prom/prometheus:v2.40.0
    ports: ["9090:9090"]
```

## 🔍 Health Checks

### Backend Health
```bash
curl http://localhost:8080/health
```

**Expected Response:**
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

### Frontend Health
```bash
curl http://localhost:3000
```

### Service Health
```bash
# Redis
docker exec chatapp-redis-dev redis-cli ping

# Kafka
docker exec chatapp-kafka-dev kafka-broker-api-versions --bootstrap-server localhost:9092

# ScyllaDB
docker exec chatapp-scylla-dev cqlsh -e "describe keyspaces"
```

## 📊 Monitoring Commands

### Prometheus Metrics
```bash
# View all metrics
curl http://localhost:9090/metrics

# Specific metrics
curl http://localhost:9090/metrics | grep chatapp_gateway

# Health endpoint
curl http://localhost:9090/-/healthy
```

### Key Metrics to Monitor
```
chatapp_gateway_connections_active
chatapp_gateway_connections_total
chatapp_gateway_messages_total
chatapp_gateway_messages_duration_seconds
chatapp_gateway_redis_operations_total
chatapp_gateway_kafka_messages_produced_total
chatapp_gateway_scylla_operations_total
```

## 🧪 Testing Commands

### Backend Tests
```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./pkg/resilience/...

# Run tests with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./...

# Run race condition tests
go test -race ./...
```

### Frontend Tests
```bash
# Run all tests
npm test

# Run tests in watch mode
npm test --watch

# Run tests with coverage
npm test --coverage

# Run specific test
npm test -- --testNamePattern="WebSocket"
```

### Load Testing
```bash
# Install wrk
brew install wrk  # Mac
sudo apt-get install wrk  # Ubuntu

# Load test WebSocket
wscat -c ws://localhost:8080/ws

# Load test API
wrk -t12 -c400 -d30s http://localhost:8081/api/v1/health
```

## 🔒 Security Commands

### Rate Limiting Test
```bash
# Test connection rate limiting
for i in {1..15}; do
  curl -H "X-Forwarded-For: 192.168.1.$i" http://localhost:8080/ws
done

# Test API rate limiting
for i in {1..1100}; do
  curl http://localhost:8081/api/v1/conversations
done
```

### Security Scan
```bash
# Install security scanner
npm install -g audit-ci

# Run security audit
npm audit

# Run security audit with fix
npm audit fix
```

## 🚨 Troubleshooting Commands

### Common Issues
```bash
# Port conflicts
netstat -tulpn | grep :8080
lsof -i :8080

# Docker issues
docker system prune
docker volume prune

# Node.js issues
rm -rf node_modules package-lock.json
npm install

# Go issues
go clean -modcache
go mod download

# Clear Docker logs
docker system prune -a
```

### Debug Commands
```bash
# Backend debug
go run examples/enhanced_gateway.go -debug

# Frontend debug
npm start --inspect

# Docker debug
docker-compose logs -f backend
docker exec -it chatapp-redis-dev sh
```

## 📈 Performance Tuning

### Backend Tuning
```bash
# Environment variables
export GOMAXPROCS=4
export GOGC=100
export GOMEMLIMIT=1Gi

# Run with profiling
go run -cpuprofile=cpu.prof -memprofile=mem.prof examples/enhanced_gateway.go

# View profiles
go tool pprof cpu.prof
go tool pprof mem.prof
```

### Frontend Tuning
```bash
# Build with optimization
npm run build

# Analyze bundle
npm run analyze

# Source maps
npm run build -- --source-map
```

## 🔄 Deployment Commands

### Docker Compose Deployment
```bash
# Deploy
docker-compose up -d

# Scale services
docker-compose up -d --scale backend=3

# Update services
docker-compose pull
docker-compose up -d

# Remove all
docker-compose down -v
```

### Kubernetes Deployment
```bash
# Apply all manifests
kubectl apply -f k8s/

# Check status
kubectl get pods -n chatapp
kubectl get services -n chatapp

# View logs
kubectl logs -f deployment/backend -n chatapp

# Scale deployment
kubectl scale deployment backend --replicas=5 -n chatapp

# Update deployment
kubectl set image deployment/backend backend=chatapp/backend:v2.0.0 -n chatapp
```

## 📱 Browser Console Commands

### WebSocket Testing
```javascript
// Test WebSocket connection
const ws = new WebSocket('ws://localhost:8080/ws');
ws.onopen = () => console.log('Connected');
ws.onmessage = (e) => console.log('Message:', e.data);

// Send test message
ws.send(JSON.stringify({
  type: 'send_message',
  conversation_id: 'test-conv',
  content: 'Hello World',
  message_type: 'text'
}));
```

### API Testing
```javascript
// Test API
fetch('http://localhost:8081/api/v1/health')
  .then(r => r.json())
  .then(console.log);

// Test with error
fetch('http://localhost:8081/api/v1/nonexistent')
  .catch(console.error);
```

### Performance Monitoring
```javascript
// Monitor performance
performance.mark('start');
// ... do something
performance.mark('end');
performance.measure('operation', 'start', 'end');
console.log(performance.getEntriesByName('operation'));
```

## 🛠️ Development Workflow

### Daily Development
```bash
# 1. Start services
docker-compose -f docker-compose.dev.yml up -d

# 2. Start backend (terminal 1)
cd backend && go run examples/enhanced_gateway.go

# 3. Start frontend (terminal 2)
cd frontend && npm start

# 4. Run tests (terminal 3)
cd backend && go test ./... && cd ../frontend && npm test
```

### Code Changes
```bash
# Backend changes
cd backend
go test ./...
go run examples/enhanced_gateway.go

# Frontend changes
cd frontend
npm run lint
npm test
npm start
```

### Before Commit
```bash
# Backend
cd backend
go test ./...
go vet ./...
go fmt ./...
go mod tidy

# Frontend
cd frontend
npm run lint
npm run type-check
npm test
npm run build
```

## 📞 Emergency Commands

### Reset Everything
```bash
# Stop all services
docker-compose down -v
pkill -f "go run"
pkill -f "npm start"

# Clean up
docker system prune -a
rm -rf node_modules package-lock.json
go clean -modcache

# Restart
docker-compose up -d
npm install
go mod download
```

### Database Reset
```bash
# Clear Redis
docker exec chatapp-redis-dev redis-cli flushall

# Clear ScyllaDB
docker exec chatapp-scylla-dev cqlsh -e "DROP KEYSPACE IF EXISTS chatapp;"

# Reinitialize
docker exec chatapp-scylla-dev cqlsh -e "CREATE KEYSPACE chatapp WITH REPLICATION = {'class': 'SimpleStrategy', 'replication_factor': 1};"
```

---

**💡 Pro Tip**: Bookmark this guide for quick reference during development and operations!
