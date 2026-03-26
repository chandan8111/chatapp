# ChatApp Development Guide

## Overview

This guide provides comprehensive instructions for developing, testing, and deploying the ChatApp distributed chat system.

## Prerequisites

### Required Tools
- **Go 1.21+**: Programming language
- **Docker**: Containerization
- **Kubernetes**: Container orchestration
- **Helm**: Kubernetes package manager
- **kubectl**: Kubernetes CLI
- **Redis**: Presence system
- **Apache Kafka**: Message broker
- **ScyllaDB**: Database

### Development Environment Setup

```bash
# Clone the repository
git clone https://github.com/chatapp/chatapp.git
cd chatapp

# Install Go dependencies
go mod download

# Build all services
./scripts/build.sh all

# Start local development stack
docker-compose up -d
```

## Architecture Overview

### Core Components

1. **WebSocket Gateway** (`gateway/`)
   - Handles 200K connections per node
   - Real-time message delivery
   - Connection management

2. **Message Processor** (`kafka/`)
   - Processes chat messages
   - Handles delivery receipts
   - Manages message persistence

3. **Presence Service** (`presence/`)
   - Tracks user online/offline status
   - Uses Redis Bitmaps for efficiency
   - Supports 100M+ users

4. **Fanout Service** (`cmd/fanout/`)
   - Message distribution
   - Hybrid push/pull model
   - Celebrity conversation optimization

5. **API Server** (`api/`)
   - REST API endpoints
   - Authentication and authorization
   - Rate limiting

6. **Storage Layer** (`storage/`)
   - ScyllaDB integration
   - Time-based partitioning
   - Materialized views

## Development Workflow

### 1. Local Development

```bash
# Start local infrastructure
docker-compose up -d redis kafka scylladb

# Run individual services
go run cmd/gateway/main.go
go run cmd/processor/main.go
go run cmd/presence/main.go
go run cmd/fanout/main.go
go run cmd/api/main.go
```

### 2. Testing

```bash
# Run unit tests
go test -v ./...

# Run integration tests
go test -v -tags=integration ./...

# Run benchmarks
go test -bench=. ./...

# Run specific test
go test -v ./gateway -run TestWebSocketGateway
```

### 3. Building

```bash
# Build all services
./scripts/build.sh all

# Build specific service
./scripts/build.sh gateway

# Build and push to registry
./scripts/build.sh gateway push
```

### 4. Deployment

```bash
# Deploy to development
./scripts/deploy.sh development all

# Deploy to staging
./scripts/deploy.sh staging all

# Deploy to production
./scripts/deploy.sh production all
```

## Configuration

### Environment Variables

```bash
# Database Configuration
SCYLLA_HOSTS=localhost:9042
SCYLLA_KEYSPACE=chatapp

# Redis Configuration
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=

# Kafka Configuration
KAFKA_BROKERS=localhost:9092
KAFKA_TOPIC_PREFIX=chatapp

# Service Configuration
ENVIRONMENT=development
LOG_LEVEL=debug
METRICS_ENABLED=true
```

### Configuration Files

- `config/config.yaml`: Main configuration
- `config/development.yaml`: Development overrides
- `config/production.yaml`: Production overrides

## API Documentation

### WebSocket API

#### Connection
```
ws://localhost:8080/ws?user_id={user_id}&device_id={device_id}&node_id={node_id}
```

#### Message Format
```json
{
  "type": "message",
  "content": "Hello World",
  "sender_id": "user-123",
  "conversation_id": "conv-456",
  "timestamp": 1640995200000
}
```

### REST API

#### Health Check
```
GET /health
```

#### Presence
```
GET /api/v1/presence/{user_id}
POST /api/v1/presence/batch
GET /api/v1/presence/online
```

#### Messages
```
POST /api/v1/messages
GET /api/v1/messages/{message_id}
GET /api/v1/conversations/{conversation_id}/messages
```

## Database Schema

### Messages Table
```sql
CREATE TABLE messages (
    message_id timeuuid PRIMARY KEY,
    conversation_id text,
    sender_id text,
    content text,
    message_type text,
    timestamp timestamp,
    status text,
    metadata map<text, text>
) WITH CLUSTERING ORDER BY (timestamp DESC);
```

### Presence Table
```sql
CREATE TABLE user_presence (
    user_id text PRIMARY KEY,
    online boolean,
    last_seen timestamp,
    node_id text,
    device_id text
);
```

## Monitoring and Observability

### Metrics
- **Connection Count**: Active WebSocket connections
- **Message Rate**: Messages per second
- **Latency**: P50, P95, P99 latencies
- **Error Rate**: Failed requests percentage

### Logging
- **Structured JSON**: All logs in JSON format
- **Correlation IDs**: Request tracing
- **Log Levels**: DEBUG, INFO, WARN, ERROR

### Alerting
- **High Error Rate**: >5% error rate
- **High Latency**: P99 >100ms
- **Connection Limits**: >85% capacity
- **Service Health**: Service unavailable

## Performance Optimization

### Memory Management
```go
// Set memory limits
runtime.SetMemoryProfileRate(1)
runtime.SetGCPercent(100)

// Use object pools
var messagePool = sync.Pool{
    New: func() interface{} {
        return &Message{}
    },
}
```

### Connection Optimization
```go
// Connection pooling
redisPool := &redis.Pool{
    MaxIdle:     100,
    MaxActive:   1000,
    IdleTimeout: 240 * time.Second,
}
```

### Batching
```go
// Batch Redis operations
batch := redis.NewPipeline()
batch.Set(key1, value1)
batch.Set(key2, value2)
_, err := batch.Exec()
```

## Security

### End-to-End Encryption
- **Double Ratchet Protocol**: Forward secrecy
- **X3DH Key Exchange**: Initial key agreement
- **Message Authentication**: HMAC-SHA256

### Authentication
- **JWT Tokens**: API authentication
- **WebSocket Auth**: Connection-level auth
- **Rate Limiting**: Per-user and per-IP limits

### Network Security
- **TLS 1.2+**: All connections encrypted
- **Network Policies**: Kubernetes network isolation
- **Input Validation**: Prevent injection attacks

## Troubleshooting

### Common Issues

#### Connection Failures
```bash
# Check WebSocket gateway logs
kubectl logs -l app.kubernetes.io/component=gateway

# Check connection limits
curl http://localhost:8080/metrics | grep websocket_connections
```

#### High Latency
```bash
# Check Kafka consumer lag
kubectl exec -it kafka-0 -- kafka-consumer-groups.sh --bootstrap-server localhost:9092 --describe --group message-processors

# Check database performance
kubectl exec -it scylla-0 -- nodetool cfstats
```

#### Memory Issues
```bash
# Check memory usage
kubectl top pods

# Check Go memory metrics
curl http://localhost:9090/metrics | grep go_mem
```

### Debug Mode
```bash
# Enable debug logging
export LOG_LEVEL=debug

# Run with race detector
go run -race cmd/gateway/main.go

# Enable profiling
curl http://localhost:8080/debug/pprof/profile > cpu.prof
```

## Contributing

### Code Style
- **Go fmt**: `go fmt ./...`
- **Go vet**: `go vet ./...`
- **Gosec**: `gosec ./...`
- **Golangci-lint**: `golangci-lint run`

### Pull Request Process
1. Create feature branch
2. Write tests
3. Update documentation
4. Submit pull request
5. Code review
6. Merge to main

### Testing Requirements
- **Unit Tests**: >80% coverage
- **Integration Tests**: All API endpoints
- **Load Tests**: Performance validation
- **Security Tests**: Vulnerability scanning

## Deployment

### Kubernetes Deployment
```bash
# Deploy using Helm
helm install chatapp ./k8s/helm --values ./k8s/helm/values.yaml

# Upgrade deployment
helm upgrade chatapp ./k8s/helm --values ./k8s/helm/values.yaml

# Rollback deployment
helm rollback chatapp
```

### CI/CD Pipeline
```bash
# Run full pipeline
./scripts/ci_cd_pipeline.sh production main v1.0.0

# Run specific stages
./scripts/ci_cd_pipeline.sh development main latest test-only
```

## Scaling

### Horizontal Scaling
- **WebSocket Gateway**: 200K connections per pod
- **Message Processor**: Based on Kafka lag
- **Presence Service**: Based on Redis memory
- **API Server**: Based on request rate

### Vertical Scaling
- **CPU**: 2-4 cores per pod
- **Memory**: 4-8GB per pod
- **Storage**: SSD with high IOPS

### Auto Scaling
```yaml
# HPA Configuration
metrics:
- type: Pods
  pods:
    metric:
      name: websocket_connections
    target:
      type: AverageValue
      averageValue: "150000"
```

## Best Practices

### Performance
- Use connection pooling
- Implement batching
- Optimize memory usage
- Monitor GC pauses

### Security
- Validate all inputs
- Use least privilege access
- Encrypt all data
- Regular security audits

### Reliability
- Implement circuit breakers
- Use retry logic
- Monitor health checks
- Plan for failures

### Observability
- Log everything
- Use structured logging
- Implement tracing
- Set up alerts

## Support

### Documentation
- **API Docs**: `/docs/api`
- **Architecture**: `/docs/architecture`
- **Deployment**: `/docs/deployment`

### Monitoring
- **Grafana**: Metrics dashboards
- **Prometheus**: Metrics collection
- **Alertmanager**: Alert management

### Contact
- **Issues**: GitHub Issues
- **Discussions**: GitHub Discussions
- **Email**: team@chatapp.com
