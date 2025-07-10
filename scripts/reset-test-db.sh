#!/bin/bash

# Reset Test Database Script
# This script resets the PostgreSQL test database by stopping, removing volumes, and starting fresh

set -e

echo "Resetting VoidRunner test database..."

# Check if docker-compose is available
if ! command -v docker-compose &> /dev/null; then
    echo "Error: docker-compose is not installed"
    exit 1
fi

# Stop and remove containers and volumes
echo "Stopping and removing test database containers and volumes..."
docker-compose -f docker-compose.test.yml down -v

# Remove any dangling volumes
echo "Cleaning up any dangling volumes..."
docker volume prune -f

# Start fresh
echo "Starting fresh test database..."
docker-compose -f docker-compose.test.yml up -d

# Wait for database to be ready
echo "Waiting for database to be ready..."
timeout=60
elapsed=0
while [ $elapsed -lt $timeout ]; do
    if docker-compose -f docker-compose.test.yml exec -T postgres-test pg_isready -U testuser -d voidrunner_test &> /dev/null; then
        echo "✅ Test database is ready!"
        break
    fi
    sleep 2
    elapsed=$((elapsed + 2))
done

if [ $elapsed -ge $timeout ]; then
    echo "❌ Timeout waiting for database to be ready"
    exit 1
fi

echo "✅ Test database reset successfully!"
echo "Run 'make test-integration' to run integration tests"