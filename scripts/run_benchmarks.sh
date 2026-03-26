#!/bin/bash

# ChatApp Benchmark Runner
# This script runs comprehensive benchmarks for the ChatApp system

set -e

# Configuration
DEFAULT_TARGET_URL="ws://localhost:8080"
DEFAULT_CONCURRENT_USERS=1000
DEFAULT_DURATION=60s
DEFAULT_RAMP_UP_TIME=30s
DEFAULT_MESSAGE_INTERVAL=1s
DEFAULT_MESSAGE_SIZE=256

# Parse command line arguments
TARGET_URL=${1:-$DEFAULT_TARGET_URL}
CONCURRENT_USERS=${2:-$DEFAULT_CONCURRENT_USERS}
DURATION=${3:-$DEFAULT_DURATION}
RAMP_UP_TIME=${4:-$DEFAULT_RAMP_UP_TIME}
MESSAGE_INTERVAL=${5:-$DEFAULT_MESSAGE_INTERVAL}
MESSAGE_SIZE=${6:-$DEFAULT_MESSAGE_SIZE}

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
        error "Go is not installed. Please install Go to run benchmarks."
        exit 1
    fi
    
    # Check if target is reachable
    if ! curl -s "$TARGET_URL/health" > /dev/null 2>&1; then
        warn "Target URL $TARGET_URL may not be reachable. Continuing anyway..."
    fi
    
    log "Prerequisites check completed."
}

# Build benchmark tool
build_benchmark() {
    log "Building benchmark tool..."
    
    cd "$(dirname "$0")/.."
    
    if ! go build -o bin/benchmark ./benchmark/; then
        error "Failed to build benchmark tool"
        exit 1
    fi
    
    log "Benchmark tool built successfully."
}

# Run connection benchmark
run_connection_benchmark() {
    log "Running connection benchmark..."
    
    CONNECTIONS=${CONCURRENT_USERS}
    RAMP_UP=${RAMP_UP_TIME}
    
    info "Target: $TARGET_URL"
    info "Connections: $CONNECTIONS"
    info "Ramp-up time: $RAMP_UP"
    
    cd "$(dirname "$0")/.."
    ./bin/benchmark connections \
        --target="$TARGET_URL" \
        --connections="$CONNECTIONS" \
        --ramp-up="$RAMP_UP" \
        --output="results/connection_benchmark_$(date +%Y%m%d_%H%M%S).json"
    
    log "Connection benchmark completed."
}

# Run throughput benchmark
run_throughput_benchmark() {
    log "Running throughput benchmark..."
    
    CONNECTIONS=${CONCURRENT_USERS}
    DURATION=${DURATION}
    MESSAGES_PER_SECOND=100
    
    info "Target: $TARGET_URL"
    info "Connections: $CONNECTIONS"
    info "Duration: $DURATION"
    info "Messages per second: $MESSAGES_PER_SECOND"
    
    cd "$(dirname "$0")/.."
    ./bin/benchmark throughput \
        --target="$TARGET_URL" \
        --connections="$CONNECTIONS" \
        --duration="$DURATION" \
        --messages-per-second="$MESSAGES_PER_SECOND" \
        --output="results/throughput_benchmark_$(date +%Y%m%d_%H%M%S).json"
    
    log "Throughput benchmark completed."
}

# Run load test
run_load_test() {
    log "Running load test..."
    
    info "Target: $TARGET_URL"
    info "Concurrent users: $CONCURRENT_USERS"
    info "Duration: $DURATION"
    info "Ramp-up time: $RAMP_UP_TIME"
    info "Message interval: $MESSAGE_INTERVAL"
    info "Message size: $MESSAGE_SIZE"
    
    cd "$(dirname "$0")/.."
    ./bin/benchmark loadtest \
        --target="$TARGET_URL" \
        --concurrent-users="$CONCURRENT_USERS" \
        --duration="$DURATION" \
        --ramp-up="$RAMP_UP_TIME" \
        --message-interval="$MESSAGE_INTERVAL" \
        --message-size="$MESSAGE_SIZE" \
        --output="results/load_test_$(date +%Y%m%d_%H%M%S).json"
    
    log "Load test completed."
}

# Run stress test
run_stress_test() {
    log "Running stress test..."
    
    STRESS_USERS=$((CONCURRENT_USERS * 2))
    STRESS_DURATION=$(echo "$DURATION" | sed 's/s//') # Remove 's' suffix
    STRESS_DURATION="${STRESS_DURATION}s"
    
    info "Target: $TARGET_URL"
    info "Stress users: $STRESS_USERS"
    info "Duration: $STRESS_DURATION"
    
    cd "$(dirname "$0")/.."
    ./bin/benchmark loadtest \
        --target="$TARGET_URL" \
        --concurrent-users="$STRESS_USERS" \
        --duration="$STRESS_DURATION" \
        --ramp-up="$RAMP_UP_TIME" \
        --message-interval="500ms" \
        --message-size="512" \
        --output="results/stress_test_$(date +%Y%m%d_%H%M%S).json"
    
    log "Stress test completed."
}

# Generate report
generate_report() {
    log "Generating benchmark report..."
    
    REPORT_FILE="results/benchmark_report_$(date +%Y%m%d_%H%M%S).md"
    
    cat > "$REPORT_FILE" << EOF
# ChatApp Benchmark Report

Generated on: $(date '+%Y-%m-%d %H:%M:%S')

## Test Configuration

- Target URL: $TARGET_URL
- Concurrent Users: $CONCURRENT_USERS
- Duration: $DURATION
- Ramp-up Time: $RAMP_UP_TIME
- Message Interval: $MESSAGE_INTERVAL
- Message Size: $MESSAGE_SIZE bytes

## Test Results

### Connection Benchmark
Results saved in: results/connection_benchmark_*.json

### Throughput Benchmark
Results saved in: results/throughput_benchmark_*.json

### Load Test
Results saved in: results/load_test_*.json

### Stress Test
Results saved in: results/stress_test_*.json

## Summary

Please refer to the individual JSON files for detailed metrics.

## Recommendations

Based on the benchmark results, consider the following:

1. **Connection Handling**: Monitor connection establishment rates and failures
2. **Message Throughput**: Ensure message processing can handle peak loads
3. **Latency**: Monitor P95 and P99 latency metrics
4. **Resource Usage**: Monitor CPU, memory, and network utilization
5. **Error Rates**: Keep error rates below 1% for optimal performance

EOF
    
    log "Benchmark report generated: $REPORT_FILE"
}

# Cleanup function
cleanup() {
    log "Cleaning up..."
    
    # Stop any running processes
    pkill -f "benchmark" || true
    
    log "Cleanup completed."
}

# Main execution
main() {
    log "Starting ChatApp benchmark suite..."
    
    # Create results directory
    mkdir -p results
    
    # Check prerequisites
    check_prerequisites
    
    # Build benchmark tool
    build_benchmark
    
    # Run benchmarks
    run_connection_benchmark
    sleep 5
    
    run_throughput_benchmark
    sleep 5
    
    run_load_test
    sleep 5
    
    run_stress_test
    sleep 5
    
    # Generate report
    generate_report
    
    log "Benchmark suite completed successfully!"
    log "Results are available in the 'results' directory."
    
    # Display summary
    echo
    echo "=== Benchmark Summary ==="
    echo "Target URL: $TARGET_URL"
    echo "Concurrent Users: $CONCURRENT_USERS"
    echo "Test Duration: $DURATION"
    echo "Results Directory: results/"
    echo "Report: results/benchmark_report_$(date +%Y%m%d_%H%M%S).md"
    echo
}

# Handle script interruption
trap cleanup EXIT INT TERM

# Run main function
main "$@"
