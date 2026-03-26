#!/bin/bash

# ChatApp Development Stopper
# This script stops all ChatApp services and infrastructure

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

# Stop ChatApp services
stop_services() {
    log "Stopping ChatApp services..."
    
    # Stop services using PID files
    if [ -d "pids" ]; then
        for pid_file in pids/*.pid; do
            if [ -f "$pid_file" ]; then
                service_name=$(basename "$pid_file" .pid)
                pid=$(cat "$pid_file")
                
                if kill -0 "$pid" 2>/dev/null; then
                    info "Stopping $service_name (PID: $pid)..."
                    kill "$pid"
                    
                    # Wait for graceful shutdown
                    sleep 2
                    
                    # Force kill if still running
                    if kill -0 "$pid" 2>/dev/null; then
                        warn "Force killing $service_name (PID: $pid)..."
                        kill -9 "$pid"
                    fi
                    
                    log "✅ $service_name stopped"
                else
                    warn "⚠️  $service_name process (PID: $pid) was not running"
                fi
                
                # Remove PID file
                rm -f "$pid_file"
            fi
        done
    else
        warn "⚠️  No PID directory found. Trying to stop by process name..."
        
        # Fallback: stop by process name
        pkill -f "bin/gateway" || true
        pkill -f "bin/processor" || true
        pkill -f "bin/presence" || true
        pkill -f "bin/fanout" || true
        pkill -f "bin/api" || true
    fi
    
    # Additional cleanup for any remaining processes
    pkill -f "chatapp" || true
    
    log "ChatApp services stopped."
}

# Stop infrastructure services
stop_infrastructure() {
    log "Stopping infrastructure services..."
    
    if [ -f "docker-compose.yml" ]; then
        if docker-compose ps | grep -q "Up"; then
            docker-compose down
            log "✅ Infrastructure services stopped"
        else
            info "No infrastructure services are running"
        fi
    else
        warn "⚠️  docker-compose.yml not found"
    fi
}

# Clean up temporary files
cleanup() {
    log "Cleaning up temporary files..."
    
    # Remove PID files
    if [ -d "pids" ]; then
        rm -f pids/*.pid
        rmdir pids 2>/dev/null || true
    fi
    
    # Remove log files (optional)
    if [ "$1" = "--clean-logs" ]; then
        if [ -d "logs" ]; then
            rm -f logs/*.log
            rmdir logs 2>/dev/null || true
            log "✅ Log files cleaned"
        fi
    fi
    
    # Remove binary files (optional)
    if [ "$1" = "--clean-bin" ]; then
        if [ -d "bin" ]; then
            rm -f bin/*
            rmdir bin 2>/dev/null || true
            log "✅ Binary files cleaned"
        fi
    fi
    
    log "Cleanup completed."
}

# Show status
show_status() {
    log "Checking service status..."
    
    # Check if any ChatApp processes are running
    if pgrep -f "chatapp" > /dev/null; then
        warn "⚠️  Some ChatApp processes are still running:"
        pgrep -f "chatapp" | while read pid; do
            ps -p "$pid" -o pid,cmd --no-headers
        done
    else
        log "✅ No ChatApp processes are running"
    fi
    
    # Check Docker containers
    if command -v docker-compose > /dev/null && [ -f "docker-compose.yml" ]; then
        if docker-compose ps | grep -q "Up"; then
            warn "⚠️  Some Docker containers are still running:"
            docker-compose ps
        else
            log "✅ No Docker containers are running"
        fi
    fi
}

# Main execution
main() {
    log "Stopping ChatApp development environment..."
    
    # Parse arguments
    CLEAN_LOGS=false
    CLEAN_BIN=false
    SHOW_STATUS=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --clean-logs)
                CLEAN_LOGS=true
                shift
                ;;
            --clean-bin)
                CLEAN_BIN=true
                shift
                ;;
            --status)
                SHOW_STATUS=true
                shift
                ;;
            --clean-all)
                CLEAN_LOGS=true
                CLEAN_BIN=true
                shift
                ;;
            -h|--help)
                echo "Usage: $0 [OPTIONS]"
                echo ""
                echo "Options:"
                echo "  --clean-logs  Clean log files"
                echo "  --clean-bin   Clean binary files"
                echo "  --clean-all   Clean logs and binaries"
                echo "  --status      Show service status"
                echo "  -h, --help    Show this help message"
                echo ""
                echo "Examples:"
                echo "  $0                    # Stop services only"
                echo "  $0 --clean-all        # Stop services and clean all files"
                echo "  $0 --status           # Show current status"
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                echo "Use -h or --help for usage information"
                exit 1
                ;;
        esac
    done
    
    # Stop services
    stop_services
    
    # Stop infrastructure
    stop_infrastructure
    
    # Cleanup if requested
    if [ "$CLEAN_LOGS" = true ] || [ "$CLEAN_BIN" = true ]; then
        cleanup_args=""
        [ "$CLEAN_LOGS" = true ] && cleanup_args="$cleanup_args --clean-logs"
        [ "$CLEAN_BIN" = true ] && cleanup_args="$cleanup_args --clean-bin"
        cleanup $cleanup_args
    fi
    
    # Show status if requested
    if [ "$SHOW_STATUS" = true ]; then
        show_status
    fi
    
    log "ChatApp development environment stopped successfully! 👋"
    
    # Show what was cleaned
    if [ "$CLEAN_LOGS" = true ] || [ "$CLEAN_BIN" = true ]; then
        echo ""
        info "Cleaned items:"
        [ "$CLEAN_LOGS" = true ] && echo "  - Log files"
        [ "$CLEAN_BIN" = true ] && echo "  - Binary files"
    fi
}

# Run main function with all arguments
main "$@"
