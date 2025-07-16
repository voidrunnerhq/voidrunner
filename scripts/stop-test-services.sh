#!/bin/bash

# Stop Test Services Script
# This script stops PostgreSQL and Redis test services using Docker Compose

set -e

echo "Stopping VoidRunner test services..."

# Check if docker-compose is available
if ! command -v docker-compose &> /dev/null; then
    echo "Error: docker-compose is not installed"
    exit 1
fi

# Check if Docker is running
if ! docker info &> /dev/null; then
    echo "Error: Docker is not running"
    exit 1
fi

# Check if services are running
if ! docker-compose -f docker-compose.test.yml ps | grep -q "Up"; then
    echo "✅ Test services are already stopped"
    exit 0
fi

# Stop the test services
echo "Stopping PostgreSQL and Redis test services..."
docker-compose -f docker-compose.test.yml down

echo "✅ Test services stopped successfully!"
echo "Run 'make services-start' to start the services again"