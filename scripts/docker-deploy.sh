#!/bin/bash

# docker-deploy.sh - Deploy VoidRunner using Docker Compose
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

show_usage() {
    echo "Usage: $0 [OPTION] [COMMAND]"
    echo ""
    echo "Deploy VoidRunner using Docker Compose with different configurations"
    echo ""
    echo "OPTIONS:"
    echo "  -m, --mode MODE     Deployment mode (default|dev|prod)"
    echo "  -h, --help          Show this help message"
    echo ""
    echo "COMMANDS:"
    echo "  up                  Start services"
    echo "  down                Stop services"
    echo "  restart             Restart services"
    echo "  logs                Show logs"
    echo "  status              Show service status"
    echo "  build               Build images"
    echo "  clean               Clean up everything"
    echo ""
    echo "MODES:"
    echo "  default             Simple setup with embedded workers (docker-compose.yml)"
    echo "  dev                 Development setup with embedded workers (docker-compose.dev.yml)"
    echo "  prod                Production setup with separate services (docker-compose.prod.yml)"
    echo ""
    echo "EXAMPLES:"
    echo "  $0 up                    # Start with default configuration"
    echo "  $0 -m dev up            # Start development environment"
    echo "  $0 -m prod up           # Start production environment"
    echo "  $0 -m dev logs api      # Show API logs in dev mode"
}

# Default values
MODE="default"
COMMAND=""
SERVICE=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -m|--mode)
            MODE="$2"
            shift 2
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        up|down|restart|logs|status|build|clean)
            COMMAND="$1"
            shift
            # Remaining args are for the service name or docker-compose
            SERVICE="$*"
            break
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            show_usage
            exit 1
            ;;
    esac
done

if [[ -z "$COMMAND" ]]; then
    echo -e "${RED}Error: Command is required${NC}"
    show_usage
    exit 1
fi

# Determine compose file
case $MODE in
    default)
        COMPOSE_FILE="docker-compose.yml"
        ;;
    dev)
        COMPOSE_FILE="docker-compose.dev.yml"
        ;;
    prod)
        COMPOSE_FILE="docker-compose.prod.yml"
        ;;
    *)
        echo -e "${RED}Error: Invalid mode '$MODE'. Use 'default', 'dev', or 'prod'${NC}"
        exit 1
        ;;
esac

COMPOSE_PATH="$PROJECT_ROOT/$COMPOSE_FILE"

if [[ ! -f "$COMPOSE_PATH" ]]; then
    echo -e "${RED}Error: Compose file not found: $COMPOSE_PATH${NC}"
    exit 1
fi

echo -e "${BLUE}Using configuration: ${COMPOSE_FILE} (mode: ${MODE})${NC}"

cd "$PROJECT_ROOT"

# Execute commands
case $COMMAND in
    up)
        echo -e "${GREEN}Starting VoidRunner services...${NC}"
        if [[ "$MODE" == "prod" ]]; then
            echo -e "${YELLOW}Production mode: Make sure to set required environment variables${NC}"
            echo -e "${YELLOW}Required: POSTGRES_PASSWORD, JWT_SECRET_KEY${NC}"
        fi
        docker-compose -f "$COMPOSE_FILE" up -d $SERVICE
        echo -e "${GREEN}Services started successfully!${NC}"
        
        if [[ -z "$SERVICE" ]]; then
            echo ""
            echo -e "${BLUE}Service Status:${NC}"
            docker-compose -f "$COMPOSE_FILE" ps
            echo ""
            echo -e "${BLUE}Available Endpoints:${NC}"
            echo "  API:           http://localhost:8080"
            echo "  Health Check:  http://localhost:8080/health"
            if [[ "$MODE" != "prod" ]]; then
                echo "  Worker Status: http://localhost:8080/health/workers"
            fi
            echo "  API Docs:      http://localhost:8080/docs"
        fi
        ;;
    down)
        echo -e "${YELLOW}Stopping VoidRunner services...${NC}"
        docker-compose -f "$COMPOSE_FILE" down $SERVICE
        echo -e "${GREEN}Services stopped successfully!${NC}"
        ;;
    restart)
        echo -e "${YELLOW}Restarting VoidRunner services...${NC}"
        docker-compose -f "$COMPOSE_FILE" restart $SERVICE
        echo -e "${GREEN}Services restarted successfully!${NC}"
        ;;
    logs)
        echo -e "${BLUE}Showing logs for VoidRunner services...${NC}"
        docker-compose -f "$COMPOSE_FILE" logs -f $SERVICE
        ;;
    status)
        echo -e "${BLUE}VoidRunner Service Status:${NC}"
        docker-compose -f "$COMPOSE_FILE" ps
        echo ""
        echo -e "${BLUE}Health Checks:${NC}"
        # Try to check health endpoints
        if curl -s http://localhost:8080/health > /dev/null 2>&1; then
            echo -e "${GREEN}✓ API is healthy${NC}"
        else
            echo -e "${RED}✗ API is not responding${NC}"
        fi
        ;;
    build)
        echo -e "${YELLOW}Building VoidRunner images...${NC}"
        docker-compose -f "$COMPOSE_FILE" build $SERVICE
        echo -e "${GREEN}Images built successfully!${NC}"
        ;;
    clean)
        echo -e "${YELLOW}Cleaning up VoidRunner deployment...${NC}"
        docker-compose -f "$COMPOSE_FILE" down -v --remove-orphans
        echo -e "${YELLOW}Removing unused images...${NC}"
        docker image prune -f
        echo -e "${GREEN}Cleanup completed!${NC}"
        ;;
    *)
        echo -e "${RED}Unknown command: $COMMAND${NC}"
        show_usage
        exit 1
        ;;
esac