#!/bin/bash

# dev-env.sh - Manage VoidRunner development environment
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Environment configuration
COMPOSE_FILE="$PROJECT_ROOT/docker-compose.yml"
COMPOSE_DEV_FILE="$PROJECT_ROOT/docker-compose.dev.yml"
ENV_FILE="$PROJECT_ROOT/.env.dev"

# Check if docker-compose is available
check_docker_compose() {
    if ! command -v docker-compose &> /dev/null; then
        echo -e "${RED}Error: docker-compose is not installed${NC}"
        echo "Please install Docker Compose: https://docs.docker.com/compose/install/"
        exit 1
    fi
}

# Check if Docker is running
check_docker() {
    if ! docker info &> /dev/null; then
        echo -e "${RED}Error: Docker is not running${NC}"
        echo "Please start Docker and try again"
        exit 1
    fi
}

# Create development environment file if it doesn't exist
create_dev_env() {
    if [ ! -f "$ENV_FILE" ]; then
        echo -e "${YELLOW}Creating development environment file...${NC}"
        cat > "$ENV_FILE" << 'EOF'
# VoidRunner Development Environment
# This file contains environment variables for development

# Server Configuration
SERVER_HOST=localhost
SERVER_PORT=8080
SERVER_ENV=development
API_PORT=8080
BUILD_TARGET=development

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=voidrunner
DB_PASSWORD=voidrunner_dev_password
DB_NAME=voidrunner_dev
DB_SSL_MODE=disable

# Docker Compose Database Configuration (for containers)
POSTGRES_DB=voidrunner_dev
POSTGRES_USER=voidrunner
POSTGRES_PASSWORD=voidrunner_dev_password

# Redis Configuration
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DATABASE=0
REDIS_MAX_MEMORY=256mb
REDIS_POOL_SIZE=10
REDIS_MIN_IDLE_CONNECTIONS=5
REDIS_MAX_RETRIES=3
REDIS_DIAL_TIMEOUT=5s
REDIS_READ_TIMEOUT=3s
REDIS_WRITE_TIMEOUT=3s
REDIS_IDLE_TIMEOUT=5m

# Queue Configuration (Development)
QUEUE_TASK_QUEUE_NAME=voidrunner:tasks:dev
QUEUE_DEAD_LETTER_QUEUE_NAME=voidrunner:tasks:dead:dev
QUEUE_RETRY_QUEUE_NAME=voidrunner:tasks:retry:dev
QUEUE_DEFAULT_PRIORITY=5
QUEUE_MAX_RETRIES=3
QUEUE_RETRY_DELAY=30s
QUEUE_RETRY_BACKOFF_FACTOR=2.0
QUEUE_MAX_RETRY_DELAY=15m
QUEUE_VISIBILITY_TIMEOUT=30m
QUEUE_MESSAGE_TTL=24h
QUEUE_BATCH_SIZE=10

# Worker Configuration
WORKER_POOL_SIZE=2
WORKER_MAX_CONCURRENT_TASKS=10
WORKER_MAX_USER_CONCURRENT_TASKS=3
WORKER_TASK_TIMEOUT=5m
WORKER_HEARTBEAT_INTERVAL=30s
WORKER_SHUTDOWN_TIMEOUT=30s
WORKER_CLEANUP_INTERVAL=5m
WORKER_STALE_TASK_THRESHOLD=2h
WORKER_ID_PREFIX=voidrunner-worker

# Executor Configuration (Development - Relaxed Security)
DOCKER_ENDPOINT=unix:///var/run/docker.sock
EXECUTOR_DEFAULT_MEMORY_LIMIT_MB=256
EXECUTOR_DEFAULT_CPU_QUOTA=50000
EXECUTOR_DEFAULT_PIDS_LIMIT=128
EXECUTOR_DEFAULT_TIMEOUT_SECONDS=300
EXECUTOR_PYTHON_IMAGE=python:3.11-alpine
EXECUTOR_BASH_IMAGE=alpine:latest
EXECUTOR_JAVASCRIPT_IMAGE=node:18-alpine
EXECUTOR_GO_IMAGE=golang:1.21-alpine
EXECUTOR_ENABLE_SECCOMP=false
EXECUTOR_SECCOMP_PROFILE_PATH=/opt/voidrunner/seccomp-profile.json
EXECUTOR_ENABLE_APPARMOR=false
EXECUTOR_APPARMOR_PROFILE=voidrunner-executor
EXECUTOR_EXECUTION_USER=1000:1000

# JWT Configuration (Development Only - Change in Production)
JWT_SECRET_KEY=development-secret-key-change-in-production
JWT_ACCESS_TOKEN_DURATION=15m
JWT_REFRESH_TOKEN_DURATION=168h
JWT_ISSUER=voidrunner
JWT_AUDIENCE=voidrunner-api

# CORS Configuration (Development - Permissive)
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173
CORS_ALLOWED_METHODS=GET,POST,PUT,DELETE,OPTIONS
CORS_ALLOWED_HEADERS=Content-Type,Authorization,X-Request-ID

# Logging
LOG_LEVEL=debug
LOG_FORMAT=console

# Embedded Workers (disabled for development debugging)
EMBEDDED_WORKERS=false
EOF
        echo -e "${GREEN}Created $ENV_FILE${NC}"
        echo -e "${YELLOW}Please review and customize the environment variables if needed${NC}"
    fi
}

show_usage() {
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Manage VoidRunner development environment"
    echo ""
    echo "COMMANDS:"
    echo "  up          Start development environment"
    echo "  down        Stop development environment"
    echo "  restart     Restart development environment"
    echo "  logs        Show logs (optional: service name)"
    echo "  status      Show environment status"
    echo "  build       Build development images"
    echo "  shell       Open shell in API container"
    echo "  clean       Clean development environment"
    echo ""
    echo "EXAMPLES:"
    echo "  $0 up                 # Start development environment"
    echo "  $0 logs               # Show all logs"
    echo "  $0 logs api           # Show API logs only"
    echo "  $0 shell              # Open shell in API container"
}

# Start development environment
start_environment() {
    echo -e "${BLUE}Starting VoidRunner development environment...${NC}"
    
    check_docker_compose
    check_docker
    create_dev_env
    
    # Use both base and dev compose files
    docker-compose -f "$COMPOSE_FILE" -f "$COMPOSE_DEV_FILE" --env-file "$ENV_FILE" up -d
    
    echo -e "${GREEN}Development environment started!${NC}"
    echo ""
    echo "Services:"
    echo "  - API Server: http://localhost:8080"
    echo "  - PostgreSQL: localhost:5432 (voidrunner_dev)"
    echo "  - Redis: localhost:6379"
    echo ""
    echo "Useful commands:"
    echo "  make dev-logs         # View logs"
    echo "  make dev-status       # Check status"
    echo "  make dev-down         # Stop environment"
}

# Stop development environment
stop_environment() {
    echo -e "${BLUE}Stopping VoidRunner development environment...${NC}"
    
    docker-compose -f "$COMPOSE_FILE" -f "$COMPOSE_DEV_FILE" --env-file "$ENV_FILE" down
    
    echo -e "${GREEN}Development environment stopped${NC}"
}

# Restart development environment
restart_environment() {
    echo -e "${BLUE}Restarting VoidRunner development environment...${NC}"
    
    docker-compose -f "$COMPOSE_FILE" -f "$COMPOSE_DEV_FILE" --env-file "$ENV_FILE" restart
    
    echo -e "${GREEN}Development environment restarted${NC}"
}

# Show logs
show_logs() {
    local service=${1:-""}
    
    if [ -n "$service" ]; then
        echo -e "${BLUE}Showing logs for $service...${NC}"
        docker-compose -f "$COMPOSE_FILE" -f "$COMPOSE_DEV_FILE" --env-file "$ENV_FILE" logs -f "$service"
    else
        echo -e "${BLUE}Showing all development logs...${NC}"
        docker-compose -f "$COMPOSE_FILE" -f "$COMPOSE_DEV_FILE" --env-file "$ENV_FILE" logs -f
    fi
}

# Show status
show_status() {
    echo -e "${BLUE}Development Environment Status:${NC}"
    
    if docker-compose -f "$COMPOSE_FILE" -f "$COMPOSE_DEV_FILE" --env-file "$ENV_FILE" ps | grep -q "Up"; then
        docker-compose -f "$COMPOSE_FILE" -f "$COMPOSE_DEV_FILE" --env-file "$ENV_FILE" ps
        echo ""
        echo -e "${GREEN}Services are running${NC}"
        
        # Check API health
        if curl -s http://localhost:8080/health &> /dev/null; then
            echo -e "${GREEN}✓ API health check passed${NC}"
        else
            echo -e "${YELLOW}⚠ API health check failed (may still be starting)${NC}"
        fi
    else
        echo -e "${YELLOW}Development environment is not running${NC}"
        echo "Use 'make dev-up' to start it"
    fi
}

# Build development images
build_images() {
    echo -e "${BLUE}Building development images...${NC}"
    
    docker-compose -f "$COMPOSE_FILE" -f "$COMPOSE_DEV_FILE" --env-file "$ENV_FILE" build
    
    echo -e "${GREEN}Development images built${NC}"
}

# Open shell in API container
open_shell() {
    echo -e "${BLUE}Opening shell in API container...${NC}"
    
    if docker-compose -f "$COMPOSE_FILE" -f "$COMPOSE_DEV_FILE" --env-file "$ENV_FILE" ps api | grep -q "Up"; then
        docker-compose -f "$COMPOSE_FILE" -f "$COMPOSE_DEV_FILE" --env-file "$ENV_FILE" exec api /bin/sh
    else
        echo -e "${RED}API container is not running${NC}"
        echo "Use 'make dev-up' to start the development environment"
        exit 1
    fi
}

# Clean development environment
clean_environment() {
    echo -e "${YELLOW}This will remove all containers, volumes, and images for the development environment${NC}"
    read -p "Are you sure? [y/N] " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}Cleaning development environment...${NC}"
        
        docker-compose -f "$COMPOSE_FILE" -f "$COMPOSE_DEV_FILE" --env-file "$ENV_FILE" down -v --remove-orphans
        docker-compose -f "$COMPOSE_FILE" -f "$COMPOSE_DEV_FILE" --env-file "$ENV_FILE" rm -f
        
        # Remove development images
        docker images | grep voidrunner | grep -E "(dev|development)" | awk '{print $3}' | xargs -r docker rmi -f
        
        echo -e "${GREEN}Development environment cleaned${NC}"
    else
        echo "Clean cancelled"
    fi
}

# Main command handling
case "${1:-}" in
    "up")
        start_environment
        ;;
    "down")
        stop_environment
        ;;
    "restart")
        restart_environment
        ;;
    "logs")
        show_logs "${2:-}"
        ;;
    "status")
        show_status
        ;;
    "build")
        build_images
        ;;
    "shell")
        open_shell
        ;;
    "clean")
        clean_environment
        ;;
    "")
        show_usage
        ;;
    *)
        echo -e "${RED}Unknown command: $1${NC}"
        show_usage
        exit 1
        ;;
esac