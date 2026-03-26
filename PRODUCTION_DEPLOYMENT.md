# ChatApp Production Deployment Guide

## Overview

This guide provides step-by-step instructions for deploying the ChatApp distributed chat system to production environments.

## Prerequisites

### Infrastructure Requirements

### Minimum Production Setup
- **Kubernetes Cluster**: 5+ nodes, 16+ cores, 64+ GB RAM each
- **Load Balancer**: External LB with SSL termination
- **Storage**: 1TB+ SSD storage
- **Network**: 10Gbps+ network bandwidth

### Recommended Production Setup
- **Kubernetes Cluster**: 10+ nodes, 32+ cores, 128+ GB RAM each
- **Load Balancer**: Multiple LBs for high availability
- **Storage**: 5TB+ NVMe storage
- **Network**: 25Gbps+ network bandwidth
- **CDN**: For static assets and API caching
- **DDoS Protection**: Cloudflare or similar

### Software Requirements
- **Kubernetes**: 1.25+
- **Helm**: 3.10+
- **Docker**: 20.10+
- **kubectl**: 1.25+
- **Monitoring Stack**: Prometheus + Grafana

## Architecture Overview

### Production Architecture
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   CDN/Edge      │    │   Load Balancer │    │   Ingress       │
│   (Cloudflare)  │───▶│   (NLB/ALB)     │───▶│   (NGINX)       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                        │
                       ┌─────────────────────────────────┼─────────────────────────────────┐
                       │                                 │                                 │
            ┌─────────────────┐              ┌─────────────────┐              ┌─────────────────┐
            │   WebSocket     │              │   API Gateway   │              │   Monitoring    │
            │   Gateways      │              │   (REST API)    │              │   Stack         │
            │   (50 pods)     │              │   (20 pods)     │              │   (Prometheus)  │
            └─────────────────┘              └─────────────────┘              └─────────────────┘
                       │                                 │                                 │
            ┌─────────────────┐              ┌─────────────────┐              ┌─────────────────┐
            │   Message       │              │   Presence      │              │   Fanout        │
            │   Processors    │              │   Service       │              │   Service       │
            │   (100 pods)    │              │   (20 pods)     │              │   (50 pods)     │
            └─────────────────┘              └─────────────────┘              └─────────────────┘
                       │                                 │                                 │
            ┌─────────────────┐              ┌─────────────────┐              ┌─────────────────┐
            │   Kafka         │              │   Redis         │              │   ScyllaDB      │
            │   Cluster       │              │   Cluster       │              │   Cluster       │
            │   (12 brokers)  │              │   (30 nodes)    │              │   (30 nodes)    │
            └─────────────────┘              └─────────────────┘              └─────────────────┘
```

## Deployment Steps

### 1. Environment Preparation

#### 1.1 Create Kubernetes Namespace
```bash
kubectl create namespace chatapp
kubectl label namespace chatapp name=chatapp
```

#### 1.2 Create Service Accounts
```bash
kubectl apply -f k8s/rbac.yaml
```

#### 1.3 Create Secrets
```bash
# Create TLS secrets
kubectl create secret tls chatapp-tls \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key \
  -n chatapp

# Create API secrets
kubectl create secret generic chatapp-secrets \
  --from-literal=jwt-secret=your-jwt-secret \
  --from-literal=redis-password=your-redis-password \
  -n chatapp
```

#### 1.4 Deploy Infrastructure
```bash
# Deploy Redis Cluster
helm install redis bitnami/redis-cluster \
  --values k8s/redis-values.yaml \
  --namespace chatapp

# Deploy Kafka Cluster
helm install kafka bitnami/kafka \
  --values k8s/kafka-values.yaml \
  --namespace chatapp

# Deploy ScyllaDB Cluster
helm install scylladb scylladb/scylladb \
  --values k8s/scylladb-values.yaml \
  --namespace chatapp
```

### 2. Application Deployment

#### 2.1 Deploy ChatApp Services
```bash
# Deploy using Helm
helm install chatapp ./k8s/helm \
  --values k8s/helm/values.yaml \
  --values k8s/helm/production-values.yaml \
  --namespace chatapp
```

#### 2.2 Configure Ingress
```bash
# Deploy Ingress configuration
kubectl apply -f k8s/ingress.yaml -n chatapp
```

#### 2.3 Deploy Monitoring
```bash
# Deploy Prometheus
helm install prometheus prometheus-community/kube-prometheus-stack \
  --values k8s/monitoring-values.yaml \
  --namespace monitoring

# Import Grafana dashboards
kubectl apply -f monitoring/grafana-config.yaml -n monitoring
```

### 3. Configuration

#### 3.1 Production Values
```yaml
# k8s/helm/production-values.yaml
global:
  environment: production
  imageRegistry: "your-registry.com"
  imageTag: "v1.0.0"

websocketGateway:
  replicas: 50
  resources:
    limits:
      cpu: 4000m
      memory: 8Gi
    requests:
      cpu: 2000m
      memory: 4Gi
  hpa:
    enabled: true
    minReplicas: 20
    maxReplicas: 100

messageProcessor:
  replicas: 100
  resources:
    limits:
      cpu: 2000m
      memory: 4Gi
    requests:
      cpu: 1000m
      memory: 2Gi

presenceService:
  replicas: 20
  resources:
    limits:
      cpu: 1000m
      memory: 2Gi
    requests:
      cpu: 500m
      memory: 1Gi

fanoutService:
  replicas: 50
  resources:
    limits:
      cpu: 2000m
      memory: 4Gi
    requests:
      cpu: 1000m
      memory: 2Gi

apiServer:
  replicas: 20
  resources:
    limits:
      cpu: 2000m
      memory: 4Gi
    requests:
      cpu: 1000m
      memory: 2Gi
```

#### 3.2 Environment Variables
```bash
# Production environment variables
export ENVIRONMENT=production
export LOG_LEVEL=info
export METRICS_ENABLED=true
export TRACING_ENABLED=true

# Database configuration
export SCYLLA_HOSTS=scylla-0.scylladb:9042,scylla-1.scylladb:9042,scylla-2.scylladb:9042
export REDIS_ADDR=redis-cluster:6379
export KAFKA_BROKERS=kafka-0.kafka:9092,kafka-1.kafka:9092,kafka-2.kafka:9092

# Security configuration
export JWT_SECRET=your-production-jwt-secret
export TLS_CERT_PATH=/etc/tls/tls.crt
export TLS_KEY_PATH=/etc/tls/tls.key
```

### 4. Monitoring and Alerting

#### 4.1 Prometheus Configuration
```yaml
# monitoring/prometheus.yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "alert_rules.yml"

alerting:
  alertmanagers:
    - static_configs:
        - targets: ['alertmanager:9093']

scrape_configs:
  - job_name: 'chatapp'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - chatapp
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true
```

#### 4.2 Alert Rules
```yaml
# monitoring/alert_rules.yml
groups:
  - name: chatapp_alerts
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"

      - alert: HighLatency
        expr: histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m])) > 0.1
        for: 3m
        labels:
          severity: warning
        annotations:
          summary: "High latency detected"
```

### 5. Security Configuration

#### 5.1 Network Policies
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
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: TCP
      port: 53
    - protocol: UDP
      port: 53
```

#### 5.2 Pod Security Policies
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

### 6. Backup and Recovery

#### 6.1 Database Backup
```bash
# ScyllaDB backup script
#!/bin/bash
BACKUP_DIR="/backup/scylla/$(date +%Y%m%d)"
mkdir -p $BACKUP_DIR

# Create snapshot
kubectl exec -it scylla-0 -- nodetool snapshot

# Copy snapshot files
kubectl cp scylla-0:/var/lib/scylla/data/keyspace/table/snapshots $BACKUP_DIR

# Upload to S3
aws s3 sync $BACKUP_DIR s3://chatapp-backups/scylla/$(date +%Y%m%d)/
```

#### 6.2 Configuration Backup
```bash
# Backup Kubernetes configurations
kubectl get all -n chatapp -o yaml > backup/k8s-config-$(date +%Y%m%d).yaml

# Backup Helm values
helm get values chatapp -n chatapp > backup/helm-values-$(date +%Y%m%d).yaml
```

### 7. Performance Tuning

#### 7.1 Kubernetes Node Tuning
```bash
# Set kernel parameters
echo 'net.core.somaxconn = 65535' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_max_syn_backlog = 65535' >> /etc/sysctl.conf
echo 'fs.file-max = 2097152' >> /etc/sysctl.conf
sysctl -p

# Set limits for containers
echo 'root soft nofile 1048576' >> /etc/security/limits.conf
echo 'root hard nofile 1048576' >> /etc/security/limits.conf
```

#### 7.2 Application Tuning
```yaml
# Go runtime settings
env:
- name: GOMAXPROCS
  value: "8"
- name: GOGC
  value: "100"
- name: GOMEMLIMIT
  value: "6GiB"
```

### 8. Scaling Strategies

#### 8.1 Horizontal Pod Autoscaling
```yaml
# HPA configuration
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: chatapp-gateway-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: chatapp-gateway
  minReplicas: 20
  maxReplicas: 100
  metrics:
  - type: Pods
    pods:
      metric:
        name: websocket_connections
      target:
        type: AverageValue
        averageValue: "150000"
```

#### 8.2 Cluster Autoscaling
```bash
# Enable cluster autoscaler
kubectl apply -f https://raw.githubusercontent.com/kubernetes/autoscaler/master/cluster-autoscaler/cloudprovider/aws/examples/cluster-autoscaler-autodiscover.yaml
```

### 9. Disaster Recovery

#### 9.1 Multi-Region Deployment
```yaml
# Multi-region values
global:
  regions:
    - us-east-1
    - us-west-2
    - eu-west-1

replication:
  enabled: true
  factor: 3
  regions: 3
```

#### 9.2 Failover Procedures
```bash
# Failover script
#!/bin/bash
# Switch to backup region
kubectl config use-context backup-region

# Update DNS to point to backup region
aws route53 change-resource-record-sets \
  --hosted-zone-id ZONE_ID \
  --change-batch file://failover.json

# Verify failover
curl https://api.chatapp.com/health
```

### 10. Maintenance

#### 10.1 Rolling Updates
```bash
# Update application
helm upgrade chatapp ./k8s/helm \
  --values k8s/helm/production-values.yaml \
  --values k8s/helm/v1.1.0-values.yaml \
  --namespace chatapp

# Monitor rollout
kubectl rollout status deployment/chatapp-gateway -n chatapp
```

#### 10.2 Maintenance Windows
```bash
# Schedule maintenance
kubectl annotate deployment chatapp-gateway \
  maintenance="true" \
  maintenance-window="2024-01-15T02:00:00Z" \
  -n chatapp
```

## Monitoring and Observability

### Key Metrics
- **Connection Count**: Active WebSocket connections
- **Message Rate**: Messages per second
- **Latency**: P50, P95, P99 latencies
- **Error Rate**: Failed requests percentage
- **Resource Usage**: CPU, memory, network, storage

### Dashboards
- **System Overview**: Overall system health
- **Service Metrics**: Individual service performance
- **Infrastructure**: Database and message broker metrics
- **Business Metrics**: User activity and engagement

### Alerting Channels
- **Email**: Critical alerts
- **Slack**: Warning and info alerts
- **PagerDuty**: Emergency alerts
- **SMS**: Critical infrastructure alerts

## Security Best Practices

### Network Security
- **TLS 1.2+**: All connections encrypted
- **Network Policies**: Kubernetes network isolation
- **Firewall Rules**: Restrict access to services
- **DDoS Protection**: Cloud-based protection

### Application Security
- **Input Validation**: Prevent injection attacks
- **Rate Limiting**: Prevent abuse
- **Authentication**: JWT-based auth
- **Authorization**: Role-based access control

### Data Security
- **Encryption at Rest**: Database encryption
- **Encryption in Transit**: TLS for all traffic
- **Key Management**: Secure key rotation
- **Access Controls**: Least privilege access

## Troubleshooting

### Common Issues
1. **High Memory Usage**: Check for memory leaks
2. **Connection Drops**: Verify load balancer health
3. **Slow Queries**: Optimize database queries
4. **High CPU Usage**: Profile application performance

### Debug Commands
```bash
# Check pod logs
kubectl logs -f deployment/chatapp-gateway -n chatapp

# Check resource usage
kubectl top pods -n chatapp

# Check events
kubectl get events -n chatapp --sort-by=.metadata.creationTimestamp

# Debug networking
kubectl exec -it chatapp-gateway-xxx -- netstat -an
```

## Support and Documentation

### Documentation
- **API Documentation**: `/docs/api`
- **Architecture Guide**: `/docs/architecture`
- **Troubleshooting Guide**: `/docs/troubleshooting`

### Support Channels
- **24/7 Support**: Production support team
- **Incident Response**: Emergency contact procedures
- **Knowledge Base**: Internal documentation
- **Training**: Team training materials

---

**Note**: This guide assumes familiarity with Kubernetes, Docker, and cloud infrastructure. Always test deployments in staging environments before production deployment.
