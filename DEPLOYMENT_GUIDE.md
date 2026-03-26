# 🚀 ChatApp Deployment Guide

This guide covers deployment options for the enhanced ChatApp from development to production environments.

## 📋 Deployment Environments

### 1. Development Environment
**Purpose**: Local development and testing
**Infrastructure**: Docker Compose on single machine
**Services**: Single instances of all services

### 2. Staging Environment  
**Purpose**: Pre-production testing
**Infrastructure**: Docker Swarm or small Kubernetes cluster
**Services**: Multiple replicas, external dependencies

### 3. Production Environment
**Purpose**: Live user traffic
**Infrastructure**: Kubernetes cluster with auto-scaling
**Services**: High availability, load balancing, monitoring

## 🛠️ Deployment Options

### Option 1: Docker Compose (Development/Small Production)

#### Prerequisites
- Docker Engine 20.10+
- Docker Compose 2.0+
- 4GB RAM minimum
- 2 CPU cores minimum

#### Quick Deploy
```bash
# Clone repository
git clone <repository-url>
cd chatapp

# Start all services
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f
```

#### Configuration
```yaml
# docker-compose.yml (simplified)
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
    
  backend:
    build: ./backend
    ports: ["8080:8080", "8081:8081", "9090:9090"]
    depends_on: [redis, kafka, scylla]
    
  frontend:
    build: ./frontend
    ports: ["3000:3000"]
    depends_on: [backend]
```

### Option 2: Docker Swarm (Medium Production)

#### Prerequisites
- Docker Swarm mode initialized
- Multiple nodes (manager + workers)
- Shared storage for persistent data
- Load balancer for external access

#### Initialize Swarm
```bash
# Initialize swarm on manager node
docker swarm init --advertise-addr <MANAGER_IP>

# Join worker nodes
docker swarm join --token <TOKEN> <MANAGER_IP>:2377

# Verify cluster
docker node ls
```

#### Deploy Stack
```bash
# Create stack configuration
cat > docker-stack.yml << EOF
version: '3.8'
services:
  redis:
    image: redis:7-alpine
    deploy:
      replicas: 3
      update_config:
        parallelism: 1
        delay: 10s
      restart_policy:
        condition: on-failure
        
  kafka:
    image: confluentinc/cp-kafka:7.3.0
    deploy:
      replicas: 3
      placement:
        max_replicas_per_node: 1
        
  backend:
    image: chatapp/backend:latest
    deploy:
      replicas: 3
      update_config:
        parallelism: 1
        delay: 10s
      restart_policy:
        condition: on-failure
        
  frontend:
    image: chatapp/frontend:latest
    deploy:
      replicas: 2
      update_config:
        parallelism: 1
        delay: 10s
EOF

# Deploy stack
docker stack deploy -c docker-stack.yml chatapp

# Check services
docker stack services chatapp
```

### Option 3: Kubernetes (Large Production)

#### Prerequisites
- Kubernetes cluster 1.20+
- kubectl configured
- Helm 3.0+ (optional)
- Ingress controller
- Persistent storage provisioner

#### Namespace Configuration
```yaml
# k8s/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: chatapp
  labels:
    name: chatapp
```

#### ConfigMaps
```yaml
# k8s/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: chatapp-config
  namespace: chatapp
data:
  REDIS_ADDR: "redis-service:6379"
  KAFKA_BROKERS: "kafka-service:9092"
  SCYLLA_HOSTS: "scylla-service:9042"
  LOG_LEVEL: "info"
  LOG_FORMAT: "json"
```

#### Secrets
```yaml
# k8s/secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: chatapp-secrets
  namespace: chatapp
type: Opaque
data:
  # Base64 encoded values
  JWT_SECRET: <base64-encoded-secret>
  DB_PASSWORD: <base64-encoded-password>
```

#### Redis Deployment
```yaml
# k8s/redis.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: redis
  namespace: chatapp
spec:
  serviceName: redis-service
  replicas: 3
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        ports:
        - containerPort: 6379
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "500m"
        volumeMounts:
        - name: redis-data
          mountPath: /data
  volumeClaimTemplates:
  - metadata:
      name: redis-data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
---
apiVersion: v1
kind: Service
metadata:
  name: redis-service
  namespace: chatapp
spec:
  selector:
    app: redis
  ports:
  - port: 6379
    targetPort: 6379
  clusterIP: None
```

#### Backend Deployment
```yaml
# k8s/backend.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: backend
  namespace: chatapp
spec:
  replicas: 3
  selector:
    matchLabels:
      app: backend
  template:
    metadata:
      labels:
        app: backend
    spec:
      containers:
      - name: backend
        image: chatapp/backend:latest
        ports:
        - containerPort: 8080
        - containerPort: 8081
        - containerPort: 9090
        env:
        - name: REDIS_ADDR
          valueFrom:
            configMapKeyRef:
              name: chatapp-config
              key: REDIS_ADDR
        resources:
          requests:
            memory: "1Gi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: backend-service
  namespace: chatapp
spec:
  selector:
    app: backend
  ports:
  - name: websocket
    port: 8080
    targetPort: 8080
  - name: api
    port: 8081
    targetPort: 8081
  - name: metrics
    port: 9090
    targetPort: 9090
  type: ClusterIP
```

#### Frontend Deployment
```yaml
# k8s/frontend.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
  namespace: chatapp
spec:
  replicas: 2
  selector:
    matchLabels:
      app: frontend
  template:
    metadata:
      labels:
        app: frontend
    spec:
      containers:
      - name: frontend
        image: chatapp/frontend:latest
        ports:
        - containerPort: 3000
        env:
        - name: REACT_APP_SOCKET_URL
          value: "ws://backend-service:8080"
        - name: REACT_APP_API_URL
          value: "http://backend-service:8081/api/v1"
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
---
apiVersion: v1
kind: Service
metadata:
  name: frontend-service
  namespace: chatapp
spec:
  selector:
    app: frontend
  ports:
  - port: 3000
    targetPort: 3000
  type: ClusterIP
```

#### Ingress Configuration
```yaml
# k8s/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: chatapp-ingress
  namespace: chatapp
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  tls:
  - hosts:
    - chatapp.yourdomain.com
    - api.chatapp.yourdomain.com
    secretName: chatapp-tls
  rules:
  - host: chatapp.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: frontend-service
            port:
              number: 3000
  - host: api.chatapp.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: backend-service
            port:
              number: 8081
```

#### Horizontal Pod Autoscaler
```yaml
# k8s/hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: backend-hpa
  namespace: chatapp
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: backend
  minReplicas: 3
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 10
        periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60
```

## 🔧 Build & Push Images

### Backend Image
```bash
# Build backend image
cd backend
docker build -t chatapp/backend:latest .

# Tag for registry
docker tag chatapp/backend:latest your-registry/chatapp/backend:v2.0.0

# Push to registry
docker push your-registry/chatapp/backend:v2.0.0
```

### Frontend Image
```bash
# Build frontend image
cd frontend
docker build -t chatapp/frontend:latest .

# Tag for registry
docker tag chatapp/frontend:latest your-registry/chatapp/frontend:v2.0.0

# Push to registry
docker push your-registry/chatapp/frontend:v2.0.0
```

### Multi-Stage Dockerfiles
```dockerfile
# backend/Dockerfile
FROM golang:1.19-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main examples/enhanced_gateway.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8080 8081 9090
CMD ["./main"]
```

```dockerfile
# frontend/Dockerfile
FROM node:18-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/build /usr/share/nginx/html
COPY nginx.conf /etc/nginx/nginx.conf
EXPOSE 3000
CMD ["nginx", "-g", "daemon off;"]
```

## 📊 Monitoring & Logging

### Prometheus Configuration
```yaml
# monitoring/prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "alert_rules.yml"

scrape_configs:
  - job_name: 'kubernetes-pods'
    kubernetes_sd_configs:
    - role: pod
    relabel_configs:
    - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
      action: keep
      regex: true
    - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
      action: replace
      target_label: __metrics_path__
      regex: (.+)
```

### Grafana Dashboard
```json
{
  "dashboard": {
    "title": "ChatApp Monitoring",
    "panels": [
      {
        "title": "Active Connections",
        "type": "stat",
        "targets": [
          {
            "expr": "sum(chatapp_gateway_connections_active)"
          }
        ]
      },
      {
        "title": "Message Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(chatapp_gateway_messages_total[5m])"
          }
        ]
      }
    ]
  }
}
```

### ELK Stack for Logging
```yaml
# logging/elasticsearch.yml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: elasticsearch
spec:
  serviceName: elasticsearch
  replicas: 3
  selector:
    matchLabels:
      app: elasticsearch
  template:
    metadata:
      labels:
        app: elasticsearch
    spec:
      containers:
      - name: elasticsearch
        image: docker.elastic.co/elasticsearch/elasticsearch:8.5.0
        env:
        - name: discovery.type
          value: single-node
        - name: ES_JAVA_OPTS
          value: "-Xms512m -Xmx512m"
        ports:
        - containerPort: 9200
        volumeMounts:
        - name: elasticsearch-data
          mountPath: /usr/share/elasticsearch/data
  volumeClaimTemplates:
  - metadata:
      name: elasticsearch-data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 100Gi
```

## 🔒 Security Configuration

### Network Policies
```yaml
# k8s/network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: chatapp-network-policy
  namespace: chatapp
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
    - protocol: TCP
      port: 8081
    - protocol: TCP
      port: 3000
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: redis
    ports:
    - protocol: TCP
      port: 6379
  - to:
    - podSelector:
        matchLabels:
          app: kafka
    ports:
    - protocol: TCP
      port: 9092
```

### Pod Security Policies
```yaml
# k8s/pod-security-policy.yaml
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: chatapp-psp
spec:
  privileged: false
  allowPrivilegeEscalation: false
  requiredDropCapabilities:
    - ALL
  volumes:
    - 'configMap'
    - 'emptyDir'
    - 'projected'
    - 'secret'
    - 'downwardAPI'
    - 'persistentVolumeClaim'
  runAsUser:
    rule: 'MustRunAsNonRoot'
  seLinux:
    rule: 'RunAsAny'
  fsGroup:
    rule: 'RunAsAny'
```

## 🚀 Deployment Scripts

### Automated Deployment Script
```bash
#!/bin/bash
# deploy.sh

set -e

ENVIRONMENT=${1:-development}
VERSION=${2:-latest}

echo "🚀 Deploying ChatApp to $ENVIRONMENT environment..."

case $ENVIRONMENT in
  "development")
    echo "Starting development deployment..."
    docker-compose -f docker-compose.dev.yml up -d
    ;;
  "staging")
    echo "Starting staging deployment..."
    docker stack deploy -c docker-stack.yml chatapp
    ;;
  "production")
    echo "Starting production deployment..."
    kubectl apply -f k8s/namespace.yaml
    kubectl apply -f k8s/configmap.yaml
    kubectl apply -f k8s/secrets.yaml
    kubectl apply -f k8s/redis.yaml
    kubectl apply -f k8s/kafka.yaml
    kubectl apply -f k8s/scylla.yaml
    kubectl apply -f k8s/backend.yaml
    kubectl apply -f k8s/frontend.yaml
    kubectl apply -f k8s/ingress.yaml
    kubectl apply -f k8s/hpa.yaml
    ;;
  *)
    echo "Unknown environment: $ENVIRONMENT"
    echo "Usage: $0 [development|staging|production] [version]"
    exit 1
    ;;
esac

echo "✅ Deployment completed!"
echo "📊 Check status with:"
echo "   Development: docker-compose ps"
echo "   Staging: docker stack services chatapp"
echo "   Production: kubectl get pods -n chatapp"
```

### Health Check Script
```bash
#!/bin/bash
# health-check.sh

echo "🔍 Checking ChatApp health..."

# Check backend health
echo "Checking backend..."
curl -f http://localhost:8080/health || echo "❌ Backend unhealthy"

# Check frontend
echo "Checking frontend..."
curl -f http://localhost:3000 || echo "❌ Frontend unhealthy"

# Check external services
echo "Checking Redis..."
docker exec chatapp-redis redis-cli ping || echo "❌ Redis unhealthy"

echo "Checking Kafka..."
docker exec chatapp-kafka kafka-broker-api-versions --bootstrap-server localhost:9092 || echo "❌ Kafka unhealthy"

echo "✅ Health check completed!"
```

## 📈 Performance Tuning

### Backend Tuning
```go
// Environment variables for performance
REDIS_POOL_SIZE=100
KAFKA_PRODUCER_BATCH_SIZE=1000
SCYLLA_CONNECTIONS_PER_HOST=10
HTTP_READ_TIMEOUT=30s
HTTP_WRITE_TIMEOUT=30s
```

### Frontend Tuning
```javascript
// Performance optimizations
const config = {
  websocket: {
    reconnectInterval: 1000,
    maxReconnectAttempts: 10,
    messageQueueSize: 1000,
  },
  api: {
    timeout: 10000,
    retryAttempts: 3,
    retryDelay: 1000,
  },
};
```

### Database Tuning
```yaml
# ScyllaDB configuration
scylla:
  image: scylladb/scylla:5.1.0
  command:
    - --seeds=scylla
    - --broadcast-address=scylla
    - --listen-address=scylla
    - --rpc-address=scylla
    - --smp 2
    - --memory 4G
    - --overprovisioned 1
```

## 🔄 CI/CD Pipeline

### GitHub Actions
```yaml
# .github/workflows/deploy.yml
name: Deploy ChatApp

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - name: Setup Go
      uses: actions/setup-go@v3
      with:
        go-version: 1.19
    - name: Run tests
      run: go test ./...
    - name: Setup Node.js
      uses: actions/setup-node@v3
      with:
        node-version: 18
    - name: Run frontend tests
      run: cd frontend && npm test

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - name: Build backend image
      run: |
        docker build -t chatapp/backend:${{ github.sha }} ./backend
        docker tag chatapp/backend:${{ github.sha }} your-registry/chatapp/backend:latest
    - name: Build frontend image
      run: |
        docker build -t chatapp/frontend:${{ github.sha }} ./frontend
        docker tag chatapp/frontend:${{ github.sha }} your-registry/chatapp/frontend:latest
    - name: Push images
      run: |
        docker push your-registry/chatapp/backend:latest
        docker push your-registry/chatapp/frontend:latest

  deploy:
    needs: build
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
    - name: Deploy to production
      run: |
        kubectl set image deployment/backend backend=your-registry/chatapp/backend:${{ github.sha }} -n chatapp
        kubectl set image deployment/frontend frontend=your-registry/chatapp/frontend:${{ github.sha }} -n chatapp
```

## 🎯 Deployment Checklist

### Pre-Deployment
- [ ] All tests passing
- [ ] Security scan completed
- [ ] Performance benchmarks met
- [ ] Documentation updated
- [ ] Backup strategy in place
- [ ] Monitoring configured
- [ ] Alert rules set up
- [ ] Rollback plan prepared

### Post-Deployment
- [ ] Health checks passing
- [ ] Monitoring metrics normal
- [ ] No error spikes
- [ ] User functionality verified
- [ ] Performance within SLA
- [ ] Security scan passed
- [ ] Documentation updated

### Production Readiness
- [ ] Load testing completed
- [ ] Disaster recovery tested
- [ ] Security audit passed
- [ ] Compliance verified
- [ ] Team training completed
- [ ] Support procedures documented
- [ ] Incident response plan ready

---

This deployment guide provides comprehensive options for deploying the enhanced ChatApp from development to production environments. Choose the deployment method that best fits your infrastructure and scaling requirements.
