#!/bin/bash

# Start Test Database Script
# This script starts a PostgreSQL test database using Docker Compose

set -e

echo "Starting VoidRunner test database..."

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

# Check if database is already running
if docker-compose -f docker-compose.test.yml ps | grep -q "Up"; then
    echo "✅ Test database is already running"
    echo "Database connection details:"
    echo "  Host: localhost"
    echo "  Port: 5432"
    echo "  Database: voidrunner_test"
    echo "  User: testuser"
    echo "  Password: testpassword"
    exit 0
fi

# Start the test database
echo "Starting PostgreSQL test database..."
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

# Show database status
echo "Database connection details:"
echo "  Host: localhost"
echo "  Port: 5432"
echo "  Database: voidrunner_test"
echo "  User: testuser"
echo "  Password: testpassword"

echo "✅ Test database started successfully!"
echo "Run 'make test-integration' to run integration tests"
echo "Run './scripts/stop-test-db.sh' to stop the database"