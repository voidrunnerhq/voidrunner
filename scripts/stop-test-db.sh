#!/bin/bash

# Stop Test Database Script
# This script stops the PostgreSQL test database Docker container

set -e

echo "Stopping VoidRunner test database..."

# Check if docker-compose is available
if ! command -v docker-compose &> /dev/null; then
    echo "Error: docker-compose is not installed"
    exit 1
fi

# Check if database is running
if ! docker-compose -f docker-compose.test.yml ps | grep -q "Up"; then
    echo "✅ Test database is not running"
    exit 0
fi

# Stop the test database
echo "Stopping PostgreSQL test database..."
docker-compose -f docker-compose.test.yml down

echo "✅ Test database stopped successfully!"