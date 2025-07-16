#!/bin/bash

# Start Test Services Script
# This script starts PostgreSQL and Redis test services using Docker Compose

set -e

echo "Starting VoidRunner test services..."

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

# Check if services are already running
if docker-compose -f docker-compose.test.yml ps | grep -q "Up"; then
    echo "✅ Test services are already running"
    echo "PostgreSQL connection details:"
    echo "  Host: localhost"
    echo "  Port: 5432"
    echo "  Database: voidrunner_test"
    echo "  User: testuser"
    echo "  Password: testpassword"
    echo ""
    echo "Redis connection details:"
    echo "  Host: localhost"
    echo "  Port: 6379"
    echo "  Password: (none)"
    echo "  Database: 0"
    exit 0
fi

# Start the test services
echo "Starting PostgreSQL and Redis test services..."
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

# Show service status
echo ""
echo "Service connection details:"
echo "PostgreSQL:"
echo "  Host: localhost"
echo "  Port: 5432"
echo "  Database: voidrunner_test"
echo "  User: testuser"
echo "  Password: testpassword"
echo ""
echo "Redis:"
echo "  Host: localhost"
echo "  Port: 6379"
echo "  Password: (none)"
echo "  Database: 0"

echo ""
echo "✅ Test services started successfully!"
echo "Run 'make test-integration' to run integration tests"
echo "Run 'make services-stop' to stop the services"