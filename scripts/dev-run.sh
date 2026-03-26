#!/bin/bash

# ChatApp Development Runner
# This script starts all ChatApp services locally for development

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging function
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
    
    # Check Go
    if ! command -v go &> /dev/null; then
        error "Go is not installed. Please install Go to continue."
        exit 1
    fi
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        error "Docker is not installed. Please install Docker to continue."
        exit 1
    fi
    
    # Check Docker Compose
    if ! command -v docker-compose &> /dev/null; then
        error "Docker Compose is not installed. Please install Docker Compose to continue."
        exit 1
    fi
    
    log "Prerequisites check completed."
}

# Start infrastructure services
start_infrastructure() {
    log "Starting infrastructure services..."
    
    # Start Redis, Kafka, ScyllaDB
    if ! docker-compose up -d redis kafka scylladb; then
        error "Failed to start infrastructure services"
        exit 1
    fi
    
    # Wait for services to be ready
    info "Waiting for services to be ready..."
    sleep 10
    
    # Check if services are running
    if ! docker-compose ps | grep -q "Up"; then
        error "Some services are not running properly"
        docker-compose ps
        exit 1
    fi
    
    log "Infrastructure services started successfully."
}

# Build binaries
build_binaries() {
    log "Building ChatApp binaries..."
    
    # Build all services
    if ! make build; then
        error "Failed to build binaries"
        exit 1
    fi
    
    log "Binaries built successfully."
}

# Start ChatApp services
start_services() {
    log "Starting ChatApp services..."
    
    # Create logs directory
    mkdir -p logs
    
    # Start WebSocket Gateway
    info "Starting WebSocket Gateway..."
    nohup ./bin/gateway > logs/gateway.log 2>&1 &
    GATEWAY_PID=$!
    echo $GATEWAY_PID > pids/gateway.pid
    
    # Start Message Processor
    info "Starting Message Processor..."
    nohup ./bin/processor > logs/processor.log 2>&1 &
    PROCESSOR_PID=$!
    echo $PROCESSOR_PID > pids/processor.pid
    
    # Start Presence Service
    info "Starting Presence Service..."
    nohup ./bin/presence > logs/presence.log 2>&1 &
    PRESENCE_PID=$!
    echo $PRESENCE_PID > pids/presence.pid
    
    # Start Fanout Service
    info "Starting Fanout Service..."
    nohup ./bin/fanout > logs/fanout.log 2>&1 &
    FANOUT_PID=$!
    echo $FANOUT_PID > pids/fanout.pid
    
    # Start API Server
    info "Starting API Server..."
    nohup ./bin/api > logs/api.log 2>&1 &
    API_PID=$!
    echo $API_PID > pids/api.pid
    
    # Wait for services to start
    sleep 5
    
    log "All ChatApp services started successfully."
}

# Check service health
check_health() {
    log "Checking service health..."
    
    # Check WebSocket Gateway
    if curl -f http://localhost:8080/health > /dev/null 2>&1; then
        log "✅ WebSocket Gateway is healthy"
    else
        warn "⚠️  WebSocket Gateway is not responding"
    fi
    
    # Check API Server
    if curl -f http://localhost:8081/health > /dev/null 2>&1; then
        log "✅ API Server is healthy"
    else
        warn "⚠️  API Server is not responding"
    fi
    
    # Check if processes are running
    if kill -0 $GATEWAY_PID 2>/dev/null; then
        log "✅ Gateway process is running (PID: $GATEWAY_PID)"
    else
        warn "⚠️  Gateway process is not running"
    fi
    
    if kill -0 $PROCESSOR_PID 2>/dev/null; then
        log "✅ Processor process is running (PID: $PROCESSOR_PID)"
    else
        warn "⚠️  Processor process is not running"
    fi
    
    if kill -0 $PRESENCE_PID 2>/dev/null; then
        log "✅ Presence process is running (PID: $PRESENCE_PID)"
    else
        warn "⚠️  Presence process is not running"
    fi
    
    if kill -0 $FANOUT_PID 2>/dev/null; then
        log "✅ Fanout process is running (PID: $FANOUT_PID)"
    else
        warn "⚠️  Fanout process is not running"
    fi
    
    if kill -0 $API_PID 2>/dev/null; then
        log "✅ API process is running (PID: $API_PID)"
    else
        warn "⚠️  API process is not running"
    fi
}

# Show service URLs
show_urls() {
    log "Service URLs:"
    echo ""
    echo "🌐 WebSocket Gateway: ws://localhost:8080/ws"
    echo "🔗 API Server: http://localhost:8081"
    echo "📊 API Health: http://localhost:8081/health"
    echo "📈 API Metrics: http://localhost:8081/metrics"
    echo "📋 Gateway Health: http://localhost:8080/health"
    echo "📊 Gateway Metrics: http://localhost:8080/metrics"
    echo ""
    echo "📝 Logs directory: ./logs/"
    echo "🔧 PIDs directory: ./pids/"
    echo ""
    echo "🛑 To stop services: ./scripts/dev-stop.sh"
    echo "📋 To view logs: tail -f logs/{service}.log"
    echo ""
}

# Cleanup function
cleanup() {
    warn "Interrupt received. Cleaning up..."
    
    # Stop all services
    ./scripts/dev-stop.sh
    
    exit 1
}

# Main execution
main() {
    log "Starting ChatApp development environment..."
    
    # Create necessary directories
    mkdir -p logs pids bin
    
    # Set up signal handlers
    trap cleanup SIGINT SIGTERM
    
    # Check prerequisites
    check_prerequisites
    
    # Start infrastructure
    start_infrastructure
    
    # Build binaries
    build_binaries
    
    # Start services
    start_services
    
    # Check health
    check_health
    
    # Show URLs
    show_urls
    
    log "ChatApp development environment is ready! 🚀"
    
    # Keep script running
    info "Press Ctrl+C to stop all services..."
    while true; do
        sleep 10
        # Optional: periodic health check
        if ! kill -0 $GATEWAY_PID 2>/dev/null; then
            error "Gateway process died. Stopping all services..."
            ./scripts/dev-stop.sh
            exit 1
        fi
    done
}

# Run main function
main "$@"
