# Build stage
FROM golang:1.24-alpine AS builder

# Install dependencies
RUN apk --no-cache add ca-certificates git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api cmd/api/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates and curl for health checks
RUN apk --no-cache add ca-certificates curl

# Create non-root user
RUN addgroup -g 1001 -S voidrunner && \
    adduser -u 1001 -S voidrunner -G voidrunner

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/api .

# Change ownership to non-root user
RUN chown -R voidrunner:voidrunner /app

# Switch to non-root user
USER voidrunner

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run the application
CMD ["./api"]