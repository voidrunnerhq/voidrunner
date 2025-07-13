#!/bin/bash

# start-dev.sh - Start VoidRunner API with embedded workers for development
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting VoidRunner development environment...${NC}"

# Change to project root directory
cd "$(dirname "$0")/.."

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}Error: Docker is not running. Please start Docker and try again.${NC}"
    exit 1
fi

# Check if Redis is running (try to connect)
if ! redis-cli ping > /dev/null 2>&1; then
    echo -e "${YELLOW}Redis is not running. Starting Redis...${NC}"
    if command -v redis-server > /dev/null; then
        redis-server --daemonize yes --port 6379
        sleep 2
        if ! redis-cli ping > /dev/null 2>&1; then
            echo -e "${RED}Failed to start Redis server${NC}"
            exit 1
        fi
    else
        echo -e "${RED}Redis is not installed. Please install Redis or use Docker to run it:${NC}"
        echo "  docker run -d --name redis -p 6379:6379 redis:7-alpine"
        exit 1
    fi
fi

# Check if PostgreSQL is running
echo -e "${YELLOW}Checking PostgreSQL connection...${NC}"
if ! ./scripts/start-test-db.sh > /dev/null 2>&1; then
    echo -e "${YELLOW}Starting test database...${NC}"
    ./scripts/start-test-db.sh
fi

# Set development environment variables
export SERVER_ENV=development
export EMBEDDED_WORKERS=true
export LOG_LEVEL=debug
export LOG_FORMAT=console

# Redis configuration
export REDIS_HOST=localhost
export REDIS_PORT=6379
export REDIS_PASSWORD=""
export REDIS_DATABASE=0

# Queue configuration  
export QUEUE_TASK_QUEUE_NAME="voidrunner:tasks:dev"
export QUEUE_RETRY_QUEUE_NAME="voidrunner:tasks:retry:dev"
export QUEUE_DEAD_LETTER_QUEUE_NAME="voidrunner:tasks:dead:dev"

# Worker configuration
export WORKER_POOL_SIZE=3
export WORKER_MAX_CONCURRENT_TASKS=10
export WORKER_MAX_USER_CONCURRENT_TASKS=3
export WORKER_TASK_TIMEOUT=5m
export WORKER_HEARTBEAT_INTERVAL=30s

# Executor configuration for development
export EXECUTOR_DEFAULT_MEMORY_LIMIT_MB=256
export EXECUTOR_DEFAULT_CPU_QUOTA=50000
export EXECUTOR_DEFAULT_TIMEOUT_SECONDS=300
export EXECUTOR_ENABLE_SECCOMP=false  # Disable for easier development

echo -e "${GREEN}Environment configured for development:${NC}"
echo "  - Embedded workers: ${EMBEDDED_WORKERS}"
echo "  - Worker pool size: ${WORKER_POOL_SIZE}"
echo "  - Redis: ${REDIS_HOST}:${REDIS_PORT}"
echo "  - Log level: ${LOG_LEVEL}"

# Build the API server
echo -e "${YELLOW}Building API server...${NC}"
if ! make build > /dev/null 2>&1; then
    echo -e "${RED}Failed to build API server${NC}"
    exit 1
fi

echo -e "${GREEN}Starting VoidRunner API server with embedded workers...${NC}"
echo -e "${YELLOW}Server will be available at http://localhost:8080${NC}"
echo -e "${YELLOW}Health endpoint: http://localhost:8080/health${NC}"
echo -e "${YELLOW}Worker status: http://localhost:8080/health/workers${NC}"
echo -e "${YELLOW}API docs: http://localhost:8080/docs${NC}"
echo ""
echo -e "${YELLOW}Press Ctrl+C to stop the server${NC}"

# Start the API server
./bin/api