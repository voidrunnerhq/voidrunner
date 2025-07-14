# VoidRunner Multi-Stage Dockerfile
# Supports development, production, and scheduler targets

# =============================================================================
# Base builder stage
# =============================================================================
FROM golang:1.24.5-alpine AS builder

# Install build dependencies
RUN apk --no-cache add ca-certificates git make

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build all binaries
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o bin/api cmd/api/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o bin/scheduler cmd/scheduler/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o bin/migrate cmd/migrate/main.go

# =============================================================================
# Development stage
# =============================================================================
FROM golang:1.24.5-alpine AS development

# Install development tools
RUN apk --no-cache add ca-certificates curl git make bash

# Install air for live reloading (optional)
RUN go install github.com/air-verse/air@latest

# Create non-root user
RUN addgroup -g 1001 -S voidrunner && \
    adduser -u 1001 -S voidrunner -G voidrunner

# Set working directory
WORKDIR /app

# Copy source code for development
COPY --chown=voidrunner:voidrunner . .

# Create logs and tmp directories, and set up Go module cache permissions
RUN mkdir -p logs tmp && \
    chown voidrunner:voidrunner logs tmp && \
    mkdir -p /go/pkg/mod/cache && \
    chown -R voidrunner:voidrunner /go/pkg/mod

# Switch to non-root user
USER voidrunner

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Development entry point (use Go directly for now)
CMD ["go", "run", "cmd/api/main.go"]

# =============================================================================
# Production base stage
# =============================================================================
FROM alpine:3.19 AS base

# Install runtime dependencies
RUN apk --no-cache add ca-certificates curl tzdata

# Create non-root user
RUN addgroup -g 1001 -S voidrunner && \
    adduser -u 1001 -S voidrunner -G voidrunner

# Set working directory
WORKDIR /app

# Create necessary directories
RUN mkdir -p logs && chown voidrunner:voidrunner logs

# Switch to non-root user
USER voidrunner

# =============================================================================
# Production API stage
# =============================================================================
FROM base AS production

# Copy API binary from builder
COPY --from=builder --chown=voidrunner:voidrunner /app/bin/api ./api

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run the API server
CMD ["./api"]

# =============================================================================
# Scheduler stage (for future distributed mode)
# =============================================================================
FROM base AS scheduler

# Copy scheduler binary from builder
COPY --from=builder --chown=voidrunner:voidrunner /app/bin/scheduler ./scheduler

# Health check for scheduler (no HTTP endpoint, check process)
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD pgrep scheduler || exit 1

# Run the scheduler service
CMD ["./scheduler"]

# =============================================================================
# Migration stage (for database migrations)
# =============================================================================
FROM base AS migration

# Copy migration binary from builder
COPY --from=builder --chown=voidrunner:voidrunner /app/bin/migrate ./migrate

# Copy migration files
COPY --chown=voidrunner:voidrunner migrations/ ./migrations/

# Default command (can be overridden)
CMD ["./migrate", "up"]

# =============================================================================
# All-in-one stage (includes all binaries for flexibility)
# =============================================================================
FROM base AS all

# Copy all binaries from builder
COPY --from=builder --chown=voidrunner:voidrunner /app/bin/ ./bin/

# Copy migration files
COPY --chown=voidrunner:voidrunner migrations/ ./migrations/

# Default to API server
CMD ["./bin/api"]