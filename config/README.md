# VoidRunner Configuration

This directory contains configuration files for different environments.

## Configuration Files

### `development.yaml`
- **Purpose**: Local development with embedded workers
- **Embedded Workers**: ✅ Enabled
- **Security**: Relaxed for easier development
- **Logging**: Console format with debug level
- **Database**: Local PostgreSQL with SSL disabled
- **Usage**: `SERVER_ENV=development ./bin/api`

### `production.yaml`
- **Purpose**: Production deployment with embedded workers
- **Embedded Workers**: ✅ Enabled (current implementation)
- **Security**: Hardened with seccomp, SSL required
- **Logging**: JSON format with info level
- **Database**: Environment variable configuration
- **Usage**: `make prod-up` or set environment variables and deploy with Docker

### `test.yaml`
- **Purpose**: Testing environments and CI/CD
- **Embedded Workers**: ✅ Enabled for test simplicity
- **Security**: Minimal for faster tests
- **Logging**: Console format with debug level
- **Database**: Test database configuration
- **Usage**: Automated testing and CI workflows

## Environment Variable Override

All configuration values can be overridden using environment variables. The configuration system follows this precedence:

1. **Environment Variables** (highest priority)
2. **Configuration File** (based on `SERVER_ENV`)
3. **Default Values** (lowest priority)

## Key Configuration Differences

| Setting | Development | Production | Test |
|---------|-------------|------------|------|
| Embedded Workers | ✅ Yes | ✅ Yes | ✅ Yes |
| Log Format | Console | JSON | Console |
| Log Level | Debug | Info | Debug |
| Database SSL | Disabled | Required | Disabled |
| Seccomp | Disabled | Enabled | Disabled |
| Worker Pool Size | 3 | 5 | 1 |
| Task Timeout | 5m | 1h | 1m |

## Required Environment Variables

### Production Environment
```bash
# Database
export DB_HOST="your-db-host"
export DB_USER="your-db-user"
export DB_PASSWORD="your-db-password"
export DB_NAME="your-db-name"

# Security
export JWT_SECRET_KEY="your-secure-jwt-secret"

# Redis
export REDIS_HOST="your-redis-host"
export REDIS_PORT="6379"  # Optional (default: 6379)
export REDIS_PASSWORD="your-redis-password"  # Optional
export REDIS_DATABASE="0"  # Optional (default: 0)

# CORS
export CORS_ALLOWED_ORIGINS="https://yourdomain.com"
```

### Development Environment
```bash
# Minimal required for development
export SERVER_ENV="development"
export EMBEDDED_WORKERS="true"
```

### Test Environment
```bash
# Test database (optional, has defaults)
export TEST_DB_HOST="localhost"
export TEST_DB_USER="testuser"
export TEST_DB_PASSWORD="testpassword"
export TEST_DB_NAME="voidrunner_test"
export JWT_SECRET_KEY="test-secret-key-for-integration"
```

## Deployment Scenarios

### 1. Local Development (Embedded Workers)
```bash
# Using Make (recommended)
make dev-up

# Or use development config directly
SERVER_ENV=development ./bin/api
```

### 2. Production (Embedded Workers)
```bash
# Using Make (recommended)
make prod-up

# Or use production config directly
SERVER_ENV=production ./bin/api
```

### 3. Testing Environment
```bash
# Start test services and run integration tests
make services-start
make test-integration

# Or run all tests
make test-all
```

### 4. Manual Docker Commands
```bash
# Development environment
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up

# Production environment  
docker-compose up
```

> **Note**: The Make commands are the recommended approach as they handle environment files and dependency management automatically.

## Configuration Validation

The configuration system validates all settings on startup:

- **Required fields** are checked for presence
- **Format validation** for durations, ports, etc.
- **Range validation** for numeric values
- **Dependency validation** (e.g., Redis required when embedded workers enabled)

## Security Considerations

### Development
- JWT secret is hardcoded (acceptable for development)
- Seccomp is disabled for easier debugging
- Database SSL is disabled
- CORS allows localhost origins

### Production
- JWT secret must be provided via environment variable
- Seccomp is enabled for container security
- Database SSL is required
- CORS is restricted to specified origins
- All sensitive values come from environment variables

## Troubleshooting

### Common Issues

1. **"embedded workers require Redis host"**
   - Solution: Set `REDIS_HOST` environment variable or disable embedded workers

2. **"JWT secret key is required"**
   - Solution: Set `JWT_SECRET_KEY` environment variable

3. **"database connection failed"**
   - Solution: Check database configuration and ensure DB is running

4. **Workers not processing tasks**
   - Development: Check `EMBEDDED_WORKERS=true`
   - Production: Ensure scheduler service is running

### Debug Mode
Enable debug logging to troubleshoot configuration issues:
```bash
LOG_LEVEL=debug ./bin/api
```