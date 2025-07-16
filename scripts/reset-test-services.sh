#!/bin/bash

# Reset Test Services Script
# This script resets PostgreSQL and Redis test services (stops, removes volumes, and restarts)

set -e

echo "Resetting VoidRunner test services..."

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

# Stop and remove containers with volumes
echo "Stopping test services and removing volumes..."
docker-compose -f docker-compose.test.yml down -v

# Remove any dangling volumes
echo "Cleaning up dangling volumes..."
docker volume prune -f

# Start the services again
echo "Starting fresh test services..."
docker-compose -f docker-compose.test.yml up -d

# Wait for PostgreSQL to be ready
echo "Waiting for PostgreSQL to be ready..."
timeout=60
elapsed=0
while [ $elapsed -lt $timeout ]; do
    if docker-compose -f docker-compose.test.yml exec -T postgres-test pg_isready -U testuser -d voidrunner_test &> /dev/null; then
        echo "✅ PostgreSQL is ready!"
        break
    fi
    sleep 2
    elapsed=$((elapsed + 2))
done

if [ $elapsed -ge $timeout ]; then
    echo "❌ Timeout waiting for PostgreSQL to be ready"
    exit 1
fi

# Wait for Redis to be ready
echo "Waiting for Redis to be ready..."
timeout=30
elapsed=0
while [ $elapsed -lt $timeout ]; do
    if docker-compose -f docker-compose.test.yml exec -T redis-test redis-cli ping &> /dev/null; then
        echo "✅ Redis is ready!"
        break
    fi
    sleep 2
    elapsed=$((elapsed + 2))
done

if [ $elapsed -ge $timeout ]; then
    echo "❌ Timeout waiting for Redis to be ready"
    exit 1
fi

echo ""
echo "✅ Test services reset successfully!"
echo "Fresh PostgreSQL and Redis instances are now running"
echo "Run 'make migrate-up' to set up the database schema"