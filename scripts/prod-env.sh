#!/bin/bash

# prod-env.sh - Manage VoidRunner production environment
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
ENV_FILE="$PROJECT_ROOT/.env.prod"

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

# Create production environment file if it doesn't exist
create_prod_env() {
    if [ ! -f "$ENV_FILE" ]; then
        echo -e "${YELLOW}Creating production environment file...${NC}"
        cat > "$ENV_FILE" << 'EOF'
# VoidRunner Production Environment
# This file contains environment variables for production

# Server Configuration
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
SERVER_ENV=production
API_PORT=8080
BUILD_TARGET=production

# Database Configuration (REQUIRED)
DB_HOST=postgres
DB_PORT=5432
DB_USER=voidrunner
DB_PASSWORD=CHANGE_ME_IN_PRODUCTION
DB_NAME=voidrunner
DB_SSL_MODE=require

# Docker Compose Database Configuration
POSTGRES_DB=voidrunner
POSTGRES_USER=voidrunner
POSTGRES_PASSWORD=CHANGE_ME_IN_PRODUCTION

# Redis Configuration
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DATABASE=0
REDIS_MAX_MEMORY=512mb
REDIS_POOL_SIZE=10
REDIS_MIN_IDLE_CONNECTIONS=5
REDIS_MAX_RETRIES=3
REDIS_DIAL_TIMEOUT=5s
REDIS_READ_TIMEOUT=3s
REDIS_WRITE_TIMEOUT=3s
REDIS_IDLE_TIMEOUT=5m

# Queue Configuration
QUEUE_TASK_QUEUE_NAME=voidrunner:tasks
QUEUE_DEAD_LETTER_QUEUE_NAME=voidrunner:tasks:dead
QUEUE_RETRY_QUEUE_NAME=voidrunner:tasks:retry
QUEUE_DEFAULT_PRIORITY=5
QUEUE_MAX_RETRIES=3
QUEUE_RETRY_DELAY=30s
QUEUE_RETRY_BACKOFF_FACTOR=2.0
QUEUE_MAX_RETRY_DELAY=15m
QUEUE_VISIBILITY_TIMEOUT=30m
QUEUE_MESSAGE_TTL=24h
QUEUE_BATCH_SIZE=10

# Worker Configuration (Production Settings)
WORKER_POOL_SIZE=5
WORKER_MAX_CONCURRENT_TASKS=50
WORKER_MAX_USER_CONCURRENT_TASKS=5
WORKER_TASK_TIMEOUT=1h
WORKER_HEARTBEAT_INTERVAL=30s
WORKER_SHUTDOWN_TIMEOUT=30s
WORKER_CLEANUP_INTERVAL=5m
WORKER_STALE_TASK_THRESHOLD=2h
WORKER_ID_PREFIX=voidrunner-worker

# Embedded Workers
EMBEDDED_WORKERS=true

# Executor Configuration (Production - Enhanced Security)
DOCKER_ENDPOINT=unix:///var/run/docker.sock
EXECUTOR_DEFAULT_MEMORY_LIMIT_MB=512
EXECUTOR_DEFAULT_CPU_QUOTA=100000
EXECUTOR_DEFAULT_PIDS_LIMIT=128
EXECUTOR_DEFAULT_TIMEOUT_SECONDS=3600
EXECUTOR_PYTHON_IMAGE=python:3.11-alpine
EXECUTOR_BASH_IMAGE=alpine:latest
EXECUTOR_JAVASCRIPT_IMAGE=node:18-alpine
EXECUTOR_GO_IMAGE=golang:1.21-alpine
EXECUTOR_ENABLE_SECCOMP=true
EXECUTOR_SECCOMP_PROFILE_PATH=/opt/voidrunner/seccomp-profile.json
EXECUTOR_ENABLE_APPARMOR=false
EXECUTOR_APPARMOR_PROFILE=voidrunner-executor
EXECUTOR_EXECUTION_USER=1000:1000

# JWT Configuration (REQUIRED - CHANGE IN PRODUCTION)
JWT_SECRET_KEY=CHANGE_ME_IN_PRODUCTION
JWT_ACCESS_TOKEN_DURATION=15m
JWT_REFRESH_TOKEN_DURATION=168h
JWT_ISSUER=voidrunner
JWT_AUDIENCE=voidrunner-api

# CORS Configuration
CORS_ALLOWED_ORIGINS=https://your-domain.com
CORS_ALLOWED_METHODS=GET,POST,PUT,DELETE,OPTIONS
CORS_ALLOWED_HEADERS=Content-Type,Authorization,X-Request-ID

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
EOF
        echo -e "${GREEN}Created $ENV_FILE${NC}"
        echo -e "${RED}IMPORTANT: Please update the production environment variables:${NC}"
        echo -e "${YELLOW}  - POSTGRES_PASSWORD${NC}"
        echo -e "${YELLOW}  - JWT_SECRET_KEY${NC}"
        echo -e "${YELLOW}  - CORS_ALLOWED_ORIGINS${NC}"
        echo ""
        echo "Edit $ENV_FILE before starting production environment"
        exit 1
    fi
}

# Validate production environment
validate_prod_env() {
    local errors=0
    
    # Check for default/unsafe values
    if grep -q "CHANGE_ME_IN_PRODUCTION" "$ENV_FILE"; then
        echo -e "${RED}Error: Please update POSTGRES_PASSWORD and JWT_SECRET_KEY in $ENV_FILE${NC}"
        errors=$((errors + 1))
    fi
    
    # Check required variables
    local required_vars=("POSTGRES_PASSWORD" "JWT_SECRET_KEY" "CORS_ALLOWED_ORIGINS")
    for var in "${required_vars[@]}"; do
        if ! grep -q "^${var}=" "$ENV_FILE" || grep -q "^${var}=$" "$ENV_FILE"; then
            echo -e "${RED}Error: $var is not set in $ENV_FILE${NC}"
            errors=$((errors + 1))
        fi
    done
    
    if [ $errors -gt 0 ]; then
        echo -e "${RED}Production environment validation failed${NC}"
        echo "Please fix the issues above before starting production environment"
        exit 1
    fi
    
    echo -e "${GREEN}Production environment validation passed${NC}"
}

show_usage() {
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Manage VoidRunner production environment"
    echo ""
    echo "COMMANDS:"
    echo "  up          Start production environment"
    echo "  down        Stop production environment"
    echo "  restart     Restart production environment"
    echo "  logs        Show logs (optional: service name)"
    echo "  status      Show environment status"
    echo "  build       Build production images"
    echo "  backup      Backup production database"
    echo "  health      Check system health"
    echo "  deploy      Deploy with zero-downtime (restart services)"
    echo ""
    echo "EXAMPLES:"
    echo "  $0 up                 # Start production environment"
    echo "  $0 logs               # Show all logs"
    echo "  $0 logs api           # Show API logs only"
    echo "  $0 backup             # Backup database"
    echo "  $0 deploy             # Zero-downtime deployment"
}

# Start production environment
start_environment() {
    echo -e "${BLUE}Starting VoidRunner production environment...${NC}"
    
    check_docker_compose
    check_docker
    create_prod_env
    validate_prod_env
    
    # Start services with production settings
    docker-compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d
    
    echo -e "${GREEN}Production environment started!${NC}"
    echo ""
    echo "Services:"
    echo "  - API Server: http://localhost:$(grep API_PORT "$ENV_FILE" | cut -d= -f2)"
    echo "  - PostgreSQL: localhost:$(grep DB_PORT "$ENV_FILE" | cut -d= -f2)"
    echo "  - Redis: localhost:$(grep REDIS_PORT "$ENV_FILE" | cut -d= -f2)"
    echo ""
    echo "Useful commands:"
    echo "  make prod-logs        # View logs"
    echo "  make prod-status      # Check status"
    echo "  make prod-down        # Stop environment"
}

# Stop production environment
stop_environment() {
    echo -e "${BLUE}Stopping VoidRunner production environment...${NC}"
    
    docker-compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" down
    
    echo -e "${GREEN}Production environment stopped${NC}"
}

# Restart production environment
restart_environment() {
    echo -e "${BLUE}Restarting VoidRunner production environment...${NC}"
    
    docker-compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" restart
    
    echo -e "${GREEN}Production environment restarted${NC}"
}

# Show logs
show_logs() {
    local service=${1:-""}
    
    if [ -n "$service" ]; then
        echo -e "${BLUE}Showing logs for $service...${NC}"
        docker-compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" logs -f "$service"
    else
        echo -e "${BLUE}Showing all production logs...${NC}"
        docker-compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" logs -f
    fi
}

# Show status
show_status() {
    echo -e "${BLUE}Production Environment Status:${NC}"
    
    if docker-compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" ps | grep -q "Up"; then
        docker-compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" ps
        echo ""
        echo -e "${GREEN}Services are running${NC}"
        
        # Check API health
        local api_port=$(grep API_PORT "$ENV_FILE" | cut -d= -f2)
        if curl -s "http://localhost:${api_port}/health" &> /dev/null; then
            echo -e "${GREEN}✓ API health check passed${NC}"
        else
            echo -e "${YELLOW}⚠ API health check failed${NC}"
        fi
    else
        echo -e "${YELLOW}Production environment is not running${NC}"
        echo "Use 'make prod-up' to start it"
    fi
}

# Build production images
build_images() {
    echo -e "${BLUE}Building production images...${NC}"
    
    docker-compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" build
    
    echo -e "${GREEN}Production images built${NC}"
}

# Backup production database
backup_database() {
    echo -e "${BLUE}Backing up production database...${NC}"
    
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local backup_file="backup_${timestamp}.sql"
    local db_user=$(grep POSTGRES_USER "$ENV_FILE" | cut -d= -f2)
    local db_name=$(grep POSTGRES_DB "$ENV_FILE" | cut -d= -f2)
    
    docker-compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" exec -T postgres pg_dump -U "$db_user" "$db_name" > "$backup_file"
    
    echo -e "${GREEN}Database backup created: $backup_file${NC}"
}

# Check system health
check_health() {
    echo -e "${BLUE}Checking system health...${NC}"
    
    local api_port=$(grep API_PORT "$ENV_FILE" | cut -d= -f2)
    local health_status=0
    
    # Check API health
    if curl -s "http://localhost:${api_port}/health" > /dev/null; then
        echo -e "${GREEN}✓ API Server: Healthy${NC}"
    else
        echo -e "${RED}✗ API Server: Unhealthy${NC}"
        health_status=1
    fi
    
    # Check database
    if docker-compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" exec -T postgres pg_isready > /dev/null; then
        echo -e "${GREEN}✓ PostgreSQL: Healthy${NC}"
    else
        echo -e "${RED}✗ PostgreSQL: Unhealthy${NC}"
        health_status=1
    fi
    
    # Check Redis
    if docker-compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" exec -T redis redis-cli ping > /dev/null; then
        echo -e "${GREEN}✓ Redis: Healthy${NC}"
    else
        echo -e "${RED}✗ Redis: Unhealthy${NC}"
        health_status=1
    fi
    
    if [ $health_status -eq 0 ]; then
        echo -e "${GREEN}All systems healthy${NC}"
    else
        echo -e "${RED}Some systems are unhealthy${NC}"
        exit 1
    fi
}

# Zero-downtime deployment
deploy() {
    echo -e "${BLUE}Performing zero-downtime deployment...${NC}"
    
    validate_prod_env
    
    # Build new images
    echo "Building updated images..."
    build_images
    
    # Rolling restart
    echo "Performing rolling restart..."
    docker-compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d --force-recreate api
    
    # Wait for health check
    echo "Waiting for health check..."
    sleep 10
    
    if check_health &> /dev/null; then
        echo -e "${GREEN}Deployment successful!${NC}"
    else
        echo -e "${RED}Deployment health check failed${NC}"
        exit 1
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
    "backup")
        backup_database
        ;;
    "health")
        check_health
        ;;
    "deploy")
        deploy
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