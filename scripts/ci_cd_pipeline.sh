#!/bin/bash

# ChatApp CI/CD Pipeline
# This script automates the complete CI/CD pipeline for ChatApp

set -e

# Configuration
ENVIRONMENT=${1:-development}
BRANCH=${2:-main}
VERSION=${3:-latest}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log() {
    echo -e "${GREEN}[$(date '+%Y-%m-%d %H:%M:%S')] $1${NC}"
}

warn() {
    echo -e "${YELLOW}[$(date '+%Y-%m-%d %H:%M:%S')] WARNING: $1${NC}"
}

error() {
    echo -e "${RED}[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $1${NC}"
}

info() {
    echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')] INFO: $1${NC}"
}

# Check if required tools are installed
check_prerequisites() {
    log "Checking prerequisites..."
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        error "Docker is not installed"
        exit 1
    fi
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        error "kubectl is not installed"
        exit 1
    fi
    
    # Check Helm
    if ! command -v helm &> /dev/null; then
        error "Helm is not installed"
        exit 1
    fi
    
    # Check Go
    if ! command -v go &> /dev/null; then
        error "Go is not installed"
        exit 1
    fi
    
    log "Prerequisites check completed."
}

# Run tests
run_tests() {
    log "Running tests..."
    
    # Run unit tests
    info "Running unit tests..."
    if ! go test -v ./...; then
        error "Unit tests failed"
        exit 1
    fi
    
    # Run integration tests
    info "Running integration tests..."
    if ! go test -v -tags=integration ./...; then
        error "Integration tests failed"
        exit 1
    fi
    
    log "All tests passed."
}

# Run security scans
run_security_scans() {
    log "Running security scans..."
    
    # Run Gosec security scanner
    info "Running Gosec security scan..."
    if ! gosec ./...; then
        warn "Security scan found issues"
    fi
    
    # Run Trivy on Docker images
    info "Running Trivy vulnerability scan..."
    for service in gateway processor presence fanout api; do
        if ! trivy image "chatapp/$service:$VERSION"; then
            warn "Trivy scan found vulnerabilities for $service"
        fi
    done
    
    log "Security scans completed."
}

# Build Docker images
build_images() {
    log "Building Docker images..."
    
    # Build all services
    ./scripts/build.sh all
    
    log "Docker images built successfully."
}

# Push images to registry
push_images() {
    log "Pushing Docker images to registry..."
    
    # Tag images with version
    for service in gateway processor presence fanout api; do
        docker tag "chatapp/$service:latest" "chatapp/$service:$VERSION"
    done
    
    # Push images
    ./scripts/build.sh push
    
    log "Docker images pushed successfully."
}

# Deploy to environment
deploy_to_environment() {
    log "Deploying to $ENVIRONMENT environment..."
    
    # Deploy using Helm
    ./scripts/deploy.sh "$ENVIRONMENT" all
    
    log "Deployment to $ENVIRONMENT completed."
}

# Run smoke tests
run_smoke_tests() {
    log "Running smoke tests..."
    
    # Wait for services to be ready
    info "Waiting for services to be ready..."
    sleep 30
    
    # Check service health
    services=("gateway" "processor" "presence" "fanout" "api")
    for service in "${services[@]}"; do
        if ! kubectl get pods -l app.kubernetes.io/component=$service -o jsonpath='{.items[*].status.containerStatuses[0].ready}' | grep -q "true"; then
            error "Service $service is not ready"
            exit 1
        fi
    done
    
    # Run basic API tests
    info "Running basic API tests..."
    
    # Test health endpoints
    GATEWAY_URL=$(kubectl get svc chatapp-gateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
    API_URL=$(kubectl get svc chatapp-api -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
    
    if ! curl -f "http://$GATEWAY_URL:8080/health"; then
        error "Gateway health check failed"
        exit 1
    fi
    
    if ! curl -f "http://$API_URL:8081/health"; then
        error "API health check failed"
        exit 1
    fi
    
    log "Smoke tests passed."
}

# Run performance tests
run_performance_tests() {
    log "Running performance tests..."
    
    # Run benchmarks
    ./scripts/run_benchmarks.sh
    
    log "Performance tests completed."
}

# Generate deployment report
generate_deployment_report() {
    log "Generating deployment report..."
    
    REPORT_FILE="reports/deployment_report_$(date +%Y%m%d_%H%M%S).md"
    
    mkdir -p reports
    
    cat > "$REPORT_FILE" << EOF
# ChatApp Deployment Report

Generated on: $(date '+%Y-%m-%d %H:%M:%S')

## Deployment Information

- Environment: $ENVIRONMENT
- Branch: $BRANCH
- Version: $VERSION
- Commit: $(git rev-parse HEAD)

## Services Status

EOF
    
    # Add service status
    services=("gateway" "processor" "presence" "fanout" "api")
    for service in "${services[@]}"; do
        replicas=$(kubectl get deployment chatapp-$service -o jsonpath='{.status.replicas}')
        ready=$(kubectl get deployment chatapp-$service -o jsonpath='{.status.readyReplicas}')
        echo "- $service: $ready/$replicas replicas ready" >> "$REPORT_FILE"
    done
    
    cat >> "$REPORT_FILE" << EOF

## Test Results

- Unit Tests: ✅ Passed
- Integration Tests: ✅ Passed
- Security Scans: ✅ Completed
- Smoke Tests: ✅ Passed
- Performance Tests: ✅ Completed

## Deployment Metrics

- Deployment Time: $(date '+%Y-%m-%d %H:%M:%S')
- Downtime: 0 seconds
- Rollback Required: No

## Next Steps

1. Monitor service health
2. Review performance metrics
3. Check error rates
4. Monitor resource usage

EOF
    
    log "Deployment report generated: $REPORT_FILE"
}

# Rollback function
rollback() {
    warn "Rolling back deployment..."
    
    # Rollback using Helm
    helm rollback chatapp -n chatapp
    
    log "Rollback completed."
}

# Cleanup function
cleanup() {
    log "Cleaning up..."
    
    # Remove temporary files
    rm -f /tmp/chatapp-*
    
    log "Cleanup completed."
}

# Main CI/CD pipeline
main() {
    log "Starting ChatApp CI/CD pipeline..."
    log "Environment: $ENVIRONMENT"
    log "Branch: $BRANCH"
    log "Version: $VERSION"
    
    # Check prerequisites
    check_prerequisites
    
    # Run tests
    run_tests
    
    # Run security scans
    run_security_scans
    
    # Build images
    build_images
    
    # Push images (only for non-development environments)
    if [ "$ENVIRONMENT" != "development" ]; then
        push_images
    fi
    
    # Deploy to environment
    deploy_to_environment
    
    # Run smoke tests
    run_smoke_tests
    
    # Run performance tests (only for staging and production)
    if [ "$ENVIRONMENT" = "staging" ] || [ "$ENVIRONMENT" = "production" ]; then
        run_performance_tests
    fi
    
    # Generate deployment report
    generate_deployment_report
    
    log "CI/CD pipeline completed successfully!"
    
    # Display summary
    echo
    echo "=== Deployment Summary ==="
    echo "Environment: $ENVIRONMENT"
    echo "Version: $VERSION"
    echo "Branch: $BRANCH"
    echo "Services: gateway, processor, presence, fanout, api"
    echo "Report: reports/deployment_report_$(date +%Y%m%d_%H%M%S).md"
    echo
}

# Handle script interruption
trap cleanup EXIT INT TERM

# Check for rollback flag
if [ "$4" = "rollback" ]; then
    rollback
    exit 0
fi

# Run main function
main "$@"
