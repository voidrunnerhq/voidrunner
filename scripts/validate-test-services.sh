#!/bin/bash

# Validate Test Services Script
# This script validates that PostgreSQL and Redis test services are accessible
# with the correct configuration for integration tests

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values (same as testutil)
TEST_DB_HOST=${TEST_DB_HOST:-localhost}
TEST_DB_PORT=${TEST_DB_PORT:-5432}
TEST_DB_USER=${TEST_DB_USER:-testuser}
TEST_DB_PASSWORD=${TEST_DB_PASSWORD:-testpassword}
TEST_DB_NAME=${TEST_DB_NAME:-voidrunner_test}
TEST_DB_SSLMODE=${TEST_DB_SSLMODE:-disable}

REDIS_HOST=${REDIS_HOST:-localhost}
REDIS_PORT=${REDIS_PORT:-6379}
REDIS_PASSWORD=${REDIS_PASSWORD:-}
REDIS_DATABASE=${REDIS_DATABASE:-0}

echo "🔍 Validating test services configuration..."
echo ""
echo "PostgreSQL Configuration:"
echo "  Host: $TEST_DB_HOST"
echo "  Port: $TEST_DB_PORT"
echo "  Database: $TEST_DB_NAME"
echo "  User: $TEST_DB_USER"
echo "  SSL Mode: $TEST_DB_SSLMODE"
echo ""
echo "Redis Configuration:"
echo "  Host: $REDIS_HOST"
echo "  Port: $REDIS_PORT"
echo "  Database: $REDIS_DATABASE"
echo "  Password: ${REDIS_PASSWORD:-'(none)'}"
echo ""

# Test PostgreSQL connectivity
echo "🐘 Testing PostgreSQL connectivity..."
if command -v psql &> /dev/null; then
    if PGPASSWORD=$TEST_DB_PASSWORD psql -h $TEST_DB_HOST -p $TEST_DB_PORT -U $TEST_DB_USER -d $TEST_DB_NAME -c "SELECT 1;" &> /dev/null; then
        echo -e "${GREEN}✅ PostgreSQL: Connected successfully${NC}"
        
        # Test database content
        table_count=$(PGPASSWORD=$TEST_DB_PASSWORD psql -h $TEST_DB_HOST -p $TEST_DB_PORT -U $TEST_DB_USER -d $TEST_DB_NAME -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" 2>/dev/null | tr -d ' \n' || echo "0")
        if [ "$table_count" -gt "0" ]; then
            echo -e "${GREEN}✅ PostgreSQL: Found $table_count tables (migrations applied)${NC}"
        else
            echo -e "${YELLOW}⚠️  PostgreSQL: No tables found (run: make migrate-up)${NC}"
        fi
    else
        echo -e "${RED}❌ PostgreSQL: Connection failed${NC}"
        echo "   Try: make services-start"
        exit 1
    fi
else
    echo -e "${YELLOW}⚠️  PostgreSQL: psql not available, skipping detailed test${NC}"
    
    # Try basic TCP connection test
    if timeout 5 bash -c "</dev/tcp/$TEST_DB_HOST/$TEST_DB_PORT" &> /dev/null; then
        echo -e "${GREEN}✅ PostgreSQL: Port $TEST_DB_PORT is accessible${NC}"
    else
        echo -e "${RED}❌ PostgreSQL: Port $TEST_DB_PORT is not accessible${NC}"
        exit 1
    fi
fi

echo ""

# Test Redis connectivity
echo "📡 Testing Redis connectivity..."
if command -v redis-cli &> /dev/null; then
    redis_cmd="redis-cli -h $REDIS_HOST -p $REDIS_PORT"
    if [ -n "$REDIS_PASSWORD" ]; then
        redis_cmd="$redis_cmd -a $REDIS_PASSWORD"
    fi
    redis_cmd="$redis_cmd -n $REDIS_DATABASE"
    
    if $redis_cmd ping &> /dev/null; then
        echo -e "${GREEN}✅ Redis: Connected successfully${NC}"
        
        # Test Redis database selection
        if $redis_cmd info server | grep -q "redis_version" &> /dev/null; then
            redis_version=$($redis_cmd info server | grep "redis_version" | cut -d: -f2 | tr -d '\r')
            echo -e "${GREEN}✅ Redis: Version $redis_version, database $REDIS_DATABASE accessible${NC}"
        fi
        
        # Test basic Redis operations
        if $redis_cmd set "test_connection" "ok" &> /dev/null && $redis_cmd get "test_connection" | grep -q "ok" &> /dev/null; then
            echo -e "${GREEN}✅ Redis: Read/write operations working${NC}"
            $redis_cmd del "test_connection" &> /dev/null
        else
            echo -e "${YELLOW}⚠️  Redis: Basic operations failed${NC}"
        fi
    else
        echo -e "${RED}❌ Redis: Connection failed${NC}"
        echo "   Try: make services-start"
        exit 1
    fi
else
    echo -e "${YELLOW}⚠️  Redis: redis-cli not available, skipping detailed test${NC}"
    
    # Try basic TCP connection test
    if timeout 5 bash -c "</dev/tcp/$REDIS_HOST/$REDIS_PORT" &> /dev/null; then
        echo -e "${GREEN}✅ Redis: Port $REDIS_PORT is accessible${NC}"
    else
        echo -e "${RED}❌ Redis: Port $REDIS_PORT is not accessible${NC}"
        exit 1
    fi
fi

echo ""

# Test Docker connectivity (for executor tests)
echo "🐳 Testing Docker connectivity..."
if command -v docker &> /dev/null; then
    if docker info &> /dev/null; then
        echo -e "${GREEN}✅ Docker: Connected successfully${NC}"
        
        # Test basic Docker operations
        if docker run --rm hello-world &> /dev/null; then
            echo -e "${GREEN}✅ Docker: Container execution working${NC}"
        else
            echo -e "${YELLOW}⚠️  Docker: Container execution failed${NC}"
        fi
    else
        echo -e "${RED}❌ Docker: Connection failed${NC}"
        echo "   Make sure Docker is running"
        exit 1
    fi
else
    echo -e "${YELLOW}⚠️  Docker: docker command not available${NC}"
fi

echo ""
echo -e "${GREEN}🎉 All test services are ready for integration tests!${NC}"
echo ""
echo "You can now run:"
echo "  make test-integration    # Run integration tests"
echo "  make test-all           # Run all tests"