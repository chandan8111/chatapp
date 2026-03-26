#!/bin/bash

# Deploy script for ChatApp to Kubernetes
# Usage: ./deploy.sh [environment] [service]

set -e

# Default values
ENVIRONMENT=${1:-"development"}
SERVICE=${2:-"all"}
NAMESPACE=${NAMESPACE:-"chatapp"}
CHART_PATH=${CHART_PATH:-"./k8s/helm/chatapp"}
VALUES_FILE=${VALUES_FILE:-"./k8s/helm/values-${ENVIRONMENT}.yaml"}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${BLUE}================================${NC}"
    echo -e "${BLUE} $1 ${NC}"
    echo -e "${BLUE}================================${NC}"
}

# Function to check prerequisites
check_prerequisites() {
    print_status "Checking prerequisites..."
    
    # Check if kubectl is available
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl is not installed or not in PATH"
        exit 1
    fi
    
    # Check if helm is available
    if ! command -v helm &> /dev/null; then
        print_error "helm is not installed or not in PATH"
        exit 1
    fi
    
    # Check cluster connectivity
    if ! kubectl cluster-info &> /dev/null; then
        print_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    # Check if namespace exists, create if not
    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        print_status "Creating namespace: $NAMESPACE"
        kubectl create namespace "$NAMESPACE"
    fi
    
    print_status "Prerequisites check passed"
}

# Function to validate environment
validate_environment() {
    print_status "Validating environment: $ENVIRONMENT"
    
    case $ENVIRONMENT in
        "development"|"staging"|"production")
            ;;
        *)
            print_error "Invalid environment: $ENVIRONMENT"
            echo "Valid environments: development, staging, production"
            exit 1
            ;;
    esac
    
    # Check if values file exists
    if [ ! -f "$VALUES_FILE" ]; then
        print_error "Values file not found: $VALUES_FILE"
        exit 1
    fi
    
    print_status "Environment validation passed"
}

# Function to build and push images
build_and_push() {
    print_header "Building and Pushing Images"
    
    # Get git commit hash for tag
    GIT_TAG=$(git rev-parse --short HEAD 2>/dev/null || echo "dev")
    IMAGE_TAG="${ENVIRONMENT}-${GIT_TAG}"
    
    print_status "Building images with tag: $IMAGE_TAG"
    
    # Build all services
    ./scripts/build.sh all "$IMAGE_TAG"
    
    # Push images
    print_status "Pushing images to registry..."
    ./scripts/build.sh push "$IMAGE_TAG"
    
    export IMAGE_TAG
}

# Function to deploy using Helm
deploy_helm() {
    print_header "Deploying with Helm"
    
    # Add/update Helm dependencies
    if [ -f "$CHART_PATH/requirements.yaml" ]; then
        print_status "Updating Helm dependencies..."
        helm dependency update "$CHART_PATH"
    fi
    
    # Deploy or upgrade the release
    RELEASE_NAME="chatapp-${ENVIRONMENT}"
    
    if helm list -n "$NAMESPACE" | grep -q "$RELEASE_NAME"; then
        print_status "Upgrading existing release: $RELEASE_NAME"
        helm upgrade "$RELEASE_NAME" "$CHART_PATH" \
            --namespace "$NAMESPACE" \
            --values "$VALUES_FILE" \
            --set image.tag="$IMAGE_TAG" \
            --wait \
            --timeout 10m
    else
        print_status "Installing new release: $RELEASE_NAME"
        helm install "$RELEASE_NAME" "$CHART_PATH" \
            --namespace "$NAMESPACE" \
            --values "$VALUES_FILE" \
            --set image.tag="$IMAGE_TAG" \
            --wait \
            --timeout 10m
    fi
}

# Function to deploy specific service
deploy_service() {
    local service=$1
    print_header "Deploying Service: $service"
    
    # Deploy specific deployment using kubectl
    case $service in
        "gateway")
            kubectl apply -f k8s/deployments.yaml -n "$NAMESPACE" \
                -l app=websocket-gateway
            kubectl apply -f k8s/hpa-config.yaml -n "$NAMESPACE" \
                -l app=websocket-gateway
            ;;
        "processor")
            kubectl apply -f k8s/deployments.yaml -n "$NAMESPACE" \
                -l app=message-processor
            kubectl apply -f k8s/hpa-config.yaml -n "$NAMESPACE" \
                -l app=message-processor
            ;;
        "presence")
            kubectl apply -f k8s/deployments.yaml -n "$NAMESPACE" \
                -l app=presence-service
            kubectl apply -f k8s/hpa-config.yaml -n "$NAMESPACE" \
                -l app=presence-service
            ;;
        "fanout")
            kubectl apply -f k8s/deployments.yaml -n "$NAMESPACE" \
                -l app=fanout-service
            kubectl apply -f k8s/hpa-config.yaml -n "$NAMESPACE" \
                -l app=fanout-service
            ;;
        *)
            print_error "Unknown service: $service"
            echo "Valid services: gateway, processor, presence, fanout"
            exit 1
            ;;
    esac
}

# Function to run health checks
health_check() {
    print_header "Running Health Checks"
    
    # Wait for pods to be ready
    print_status "Waiting for pods to be ready..."
    kubectl wait --for=condition=ready pod \
        -l app.kubernetes.io/name=chatapp \
        -n "$NAMESPACE" \
        --timeout=300s
    
    # Check pod status
    print_status "Checking pod status..."
    kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=chatapp
    
    # Check services
    print_status "Checking services..."
    kubectl get services -n "$NAMESPACE" -l app.kubernetes.io/name=chatapp
    
    # Check HPA status
    print_status "Checking HPA status..."
    kubectl get hpa -n "$NAMESPACE" -l app.kubernetes.io/name=chatapp
}

# Function to run smoke tests
smoke_tests() {
    print_header "Running Smoke Tests"
    
    # Get WebSocket gateway URL
    GATEWAY_URL=$(kubectl get service websocket-gateway -n "$NAMESPACE" -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
    if [ -z "$GATEWAY_URL" ]; then
        GATEWAY_URL="localhost"
        PORT=$(kubectl get service websocket-gateway -n "$NAMESPACE" -o jsonpath='{.spec.ports[0].nodePort}')
        GATEWAY_URL="${GATEWAY_URL}:${PORT}"
    else
        GATEWAY_URL="${GATEWAY_URL}:8080"
    fi
    
    print_status "Testing WebSocket gateway at: $GATEWAY_URL"
    
    # Test health endpoint
    if curl -f "http://${GATEWAY_URL}/health" &> /dev/null; then
        print_status "✓ Health endpoint is responding"
    else
        print_error "✗ Health endpoint is not responding"
        return 1
    fi
    
    # Test metrics endpoint
    if curl -f "http://${GATEWAY_URL}:9090/metrics" &> /dev/null; then
        print_status "✓ Metrics endpoint is responding"
    else
        print_warning "⚠ Metrics endpoint is not responding"
    fi
    
    print_status "Smoke tests completed"
}

# Function to rollback deployment
rollback() {
    print_header "Rolling Back Deployment"
    
    RELEASE_NAME="chatapp-${ENVIRONMENT}"
    
    print_status "Rolling back Helm release: $RELEASE_NAME"
    helm rollback "$RELEASE_NAME" -n "$NAMESPACE" --wait --timeout 5m
    
    health_check
}

# Function to scale deployment
scale() {
    local service=$1
    local replicas=$2
    
    print_header "Scaling $service to $replicas replicas"
    
    kubectl scale deployment "$service" \
        --replicas="$replicas" \
        -n "$NAMESPACE"
    
    kubectl rollout status deployment "$service" -n "$NAMESPACE" --timeout=300s
}

# Function to show deployment status
show_status() {
    print_header "Deployment Status"
    
    echo "Namespace: $NAMESPACE"
    echo "Environment: $ENVIRONMENT"
    echo ""
    
    print_status "Pods:"
    kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=chatapp -o wide
    
    echo ""
    print_status "Services:"
    kubectl get services -n "$NAMESPACE" -l app.kubernetes.io/name=chatapp
    
    echo ""
    print_status "HPA Status:"
    kubectl get hpa -n "$NAMESPACE" -l app.kubernetes.io/name=chatapp
    
    echo ""
    print_status "Recent Events:"
    kubectl get events -n "$NAMESPACE" --sort-by='.lastTimestamp' | tail -10
}

# Function to cleanup resources
cleanup() {
    print_header "Cleaning Up Resources"
    
    read -p "Are you sure you want to delete all ChatApp resources from $NAMESPACE? (y/N): " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        print_status "Deleting Helm release..."
        RELEASE_NAME="chatapp-${ENVIRONMENT}"
        helm uninstall "$RELEASE_NAME" -n "$NAMESPACE" || true
        
        print_status "Deleting remaining resources..."
        kubectl delete all -l app.kubernetes.io/name=chatapp -n "$NAMESPACE" || true
        
        print_status "Cleanup completed"
    else
        print_status "Cleanup cancelled"
    fi
}

# Main execution
main() {
    print_header "ChatApp Kubernetes Deployment"
    
    check_prerequisites
    validate_environment
    
    case $SERVICE in
        "all")
            build_and_push
            deploy_helm
            health_check
            smoke_tests
            ;;
        "gateway"|"processor"|"presence"|"fanout")
            build_and_push
            deploy_service "$SERVICE"
            health_check
            ;;
        "status")
            show_status
            ;;
        "health")
            health_check
            ;;
        "test")
            smoke_tests
            ;;
        "rollback")
            rollback
            ;;
        "scale")
            if [ -z "$3" ]; then
                print_error "Please specify number of replicas"
                echo "Usage: $0 $ENVIRONMENT scale <service> <replicas>"
                exit 1
            fi
            scale "$3" "$4"
            ;;
        "cleanup")
            cleanup
            ;;
        *)
            print_error "Unknown service: $SERVICE"
            echo "Usage: $0 [environment] [all|gateway|processor|presence|fanout|status|health|test|rollback|scale|cleanup]"
            exit 1
            ;;
    esac
    
    if [ "$SERVICE" != "status" ] && [ "$SERVICE" != "health" ] && [ "$SERVICE" != "test" ] && [ "$SERVICE" != "cleanup" ]; then
        print_status "Deployment completed successfully!"
        show_status
    fi
}

# Handle script interruption
trap 'print_error "Deployment interrupted"; exit 1' INT TERM

# Run main function
main "$@"
