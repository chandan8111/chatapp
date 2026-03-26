#!/bin/bash

# Build script for ChatApp services
# Usage: ./build.sh [service] [tag]

set -e

# Default values
SERVICE=${1:-"all"}
TAG=${2:-"latest"}
REGISTRY=${REGISTRY:-"chatapp"}
BUILD_ARGS=${BUILD_ARGS:-""}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
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

# Function to build a single service
build_service() {
    local service=$1
    local dockerfile="build/${service}/Dockerfile"
    
    print_status "Building ${service} service..."
    
    if [ ! -f "$dockerfile" ]; then
        print_error "Dockerfile not found: $dockerfile"
        return 1
    fi
    
    # Build arguments for different services
    case $service in
        "gateway")
            BUILD_ARGS="--build-arg GO_VERSION=1.21 --build-arg ALPINE_VERSION=latest"
            ;;
        "processor")
            BUILD_ARGS="--build-arg GO_VERSION=1.21 --build-arg ALPINE_VERSION=latest"
            ;;
        "presence")
            BUILD_ARGS="--build-arg GO_VERSION=1.21 --build-arg ALPINE_VERSION=latest"
            ;;
        "fanout")
            BUILD_ARGS="--build-arg GO_VERSION=1.21 --build-arg ALPINE_VERSION=latest"
            ;;
    esac
    
    docker build \
        --pull \
        --tag "${REGISTRY}/${service}:${TAG}" \
        --tag "${REGISTRY}/${service}:$(git rev-parse --short HEAD 2>/dev/null || echo 'dev')" \
        -f "$dockerfile" \
        ${BUILD_ARGS} \
        .
    
    if [ $? -eq 0 ]; then
        print_status "Successfully built ${service}:${TAG}"
    else
        print_error "Failed to build ${service}"
        return 1
    fi
}

# Function to build all services
build_all() {
    print_status "Building all services..."
    
    services=("gateway" "processor" "presence" "fanout")
    failed_services=()
    
    for service in "${services[@]}"; do
        if ! build_service "$service"; then
            failed_services+=("$service")
        fi
    done
    
    if [ ${#failed_services[@]} -eq 0 ]; then
        print_status "All services built successfully!"
    else
        print_error "Failed to build services: ${failed_services[*]}"
        exit 1
    fi
}

# Function to push images to registry
push_images() {
    local service=$1
    
    if [ "$service" = "all" ]; then
        services=("gateway" "processor" "presence" "fanout")
    else
        services=("$service")
    fi
    
    for svc in "${services[@]}"; do
        print_status "Pushing ${svc}:${TAG} to registry..."
        docker push "${REGISTRY}/${svc}:${TAG}"
        
        # Also push the git commit tag if it exists
        git_tag=$(git rev-parse --short HEAD 2>/dev/null || echo "")
        if [ -n "$git_tag" ]; then
            docker push "${REGISTRY}/${svc}:${git_tag}"
        fi
    done
}

# Function to clean up old images
cleanup() {
    print_status "Cleaning up old images..."
    
    # Remove dangling images
    docker image prune -f
    
    # Remove old versions (keep last 5)
    docker images "${REGISTRY}/*" --format "table {{.Repository}}:{{.Tag}}" | \
        grep -v "latest\|HEAD" | \
        tail -n +6 | \
        awk '{print $1}' | \
        xargs -r docker rmi -f 2>/dev/null || true
    
    print_status "Cleanup completed"
}

# Function to show build info
show_info() {
    print_status "Build Information:"
    echo "  Service: $SERVICE"
    echo "  Tag: $TAG"
    echo "  Registry: $REGISTRY"
    echo "  Git Commit: $(git rev-parse --short HEAD 2>/dev/null || echo 'N/A')"
    echo "  Build Date: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
}

# Function to run security scan
security_scan() {
    local service=$1
    
    if ! command -v trivy &> /dev/null; then
        print_warning "Trivy not found, skipping security scan"
        return 0
    fi
    
    print_status "Running security scan on ${service}:${TAG}..."
    
    trivy image --exit-code 0 --severity HIGH,CRITICAL "${REGISTRY}/${service}:${TAG}"
    
    if [ $? -eq 0 ]; then
        print_status "Security scan passed for ${service}"
    else
        print_warning "Security vulnerabilities found in ${service}"
    fi
}

# Pre-build checks
check_prerequisites() {
    print_status "Checking prerequisites..."
    
    # Check if Docker is available
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed or not in PATH"
        exit 1
    fi
    
    # Check if Docker daemon is running
    if ! docker info &> /dev/null; then
        print_error "Docker daemon is not running"
        exit 1
    fi
    
    # Check available disk space (require at least 5GB)
    available_space=$(df . | tail -1 | awk '{print $4}')
    if [ "$available_space" -lt 5242880 ]; then # 5GB in KB
        print_warning "Low disk space detected. Available: $((available_space / 1024 / 1024))GB"
    fi
    
    print_status "Prerequisites check passed"
}

# Main execution
main() {
    print_status "Starting ChatApp build process..."
    
    check_prerequisites
    show_info
    
    case $SERVICE in
        "gateway"|"processor"|"presence"|"fanout")
            build_service "$SERVICE"
            security_scan "$SERVICE"
            ;;
        "all")
            build_all
            for service in "gateway" "processor" "presence" "fanout"; do
                security_scan "$service"
            done
            ;;
        "push")
            push_images "all"
            ;;
        "cleanup")
            cleanup
            ;;
        "info")
            show_info
            ;;
        *)
            print_error "Unknown service: $SERVICE"
            echo "Usage: $0 [gateway|processor|presence|fanout|all|push|cleanup|info] [tag]"
            exit 1
            ;;
    esac
    
    print_status "Build process completed!"
}

# Handle script interruption
trap 'print_error "Build interrupted"; exit 1' INT TERM

# Run main function
main "$@"
