# Multi-stage build for Message Processor
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o processor ./cmd/processor

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S chatapp && \
    adduser -u 1001 -S chatapp -G chatapp

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/processor .

# Copy configuration files
COPY --from=builder /app/config ./config

# Create necessary directories
RUN mkdir -p /tmp /app/logs && \
    chown -R chatapp:chatapp /app

# Switch to non-root user
USER chatapp

# Expose metrics port
EXPOSE 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9090/health || exit 1

# Set environment variables
ENV GOMAXPROCS=4
ENV GOGC=100

# Run the application
CMD ["./processor", "-config", "config/config.yaml"]
