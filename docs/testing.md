# VoidRunner Testing Guide

This guide covers running tests for the VoidRunner distributed task execution platform, including unit tests, integration tests, and troubleshooting common issues.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Running Unit Tests](#running-unit-tests)
- [Running Integration Tests](#running-integration-tests)
- [Database Setup for Testing](#database-setup-for-testing)
- [Environment Variables](#environment-variables)
- [Troubleshooting](#troubleshooting)
- [Performance Testing](#performance-testing)
- [Test Organization](#test-organization)
- [Contributing Test Guidelines](#contributing-test-guidelines)

## Prerequisites

Before running tests, ensure you have the following installed:

- **Go 1.24.4** or later
- **PostgreSQL 15** or later (for integration tests)
- **Docker** (optional, for containerized testing)
- **Git** (for cloning and version control)

### Installing Dependencies

```bash
# Install Go dependencies
go mod download

# Install development tools and setup environment
make setup
```

## Running Unit Tests

Unit tests are fast, isolated tests that don't require external dependencies like databases.

### Basic Unit Tests

```bash
# Run all unit tests
make test

# Or use go test directly
go test -v ./...

# Run tests with race detection
go test -v -race ./...

# Run tests in short mode (skips slow tests)
go test -short -v ./...
```

### Unit Tests with Coverage

```bash
# Generate coverage report
make coverage

# Run coverage with detailed output
make coverage

# Check coverage meets threshold (80%)
make coverage-check
```

### Running Specific Package Tests

```bash
# Test specific package
go test -v ./internal/auth

# Test specific function
go test -v ./internal/auth -run TestJWTService_GenerateToken

# Run tests matching pattern
go test -v ./... -run "Test.*Auth.*"
```

## Running Integration Tests

Integration tests require a PostgreSQL database and test the full application stack.

### Quick Integration Test Setup

```bash
# Start PostgreSQL with Docker (recommended)
docker run --name voidrunner-test-db \
  -e POSTGRES_PASSWORD=testpassword \
  -e POSTGRES_USER=testuser \
  -e POSTGRES_DB=voidrunner_test \
  -p 5432:5432 \
  -d postgres:15

# Run integration tests
make test-integration
```

### Manual Database Setup

If you prefer to use a local PostgreSQL installation:

1. **Create test database:**
   ```sql
   CREATE DATABASE voidrunner_test;
   CREATE USER testuser WITH PASSWORD 'testpassword';
   GRANT ALL PRIVILEGES ON DATABASE voidrunner_test TO testuser;
   ```

2. **Set environment variables:**
   ```bash
   export TEST_DB_HOST=localhost
   export TEST_DB_PORT=5432
   export TEST_DB_USER=testuser
   export TEST_DB_PASSWORD=testpassword
   export TEST_DB_NAME=voidrunner_test
   export TEST_DB_SSLMODE=disable
   ```

3. **Run integration tests:**
   ```bash
   go test -v -race -tags=integration ./...
   ```

### Running All Tests

```bash
# Run both unit and integration tests
make test-all

# With coverage (both unit and integration)
make coverage
```

## Database Setup for Testing

### Environment Variables

The test suite uses these environment variables for database configuration:

| Variable | Default | Description |
|----------|---------|-------------|
| `TEST_DB_HOST` | `localhost` | Database host |
| `TEST_DB_PORT` | `5432` | Database port |
| `TEST_DB_USER` | `testuser` | Database username |
| `TEST_DB_PASSWORD` | `testpassword` | Database password |
| `TEST_DB_NAME` | `voidrunner_test` | Database name |
| `TEST_DB_SSLMODE` | `disable` | SSL mode |
| `JWT_SECRET_KEY` | (test key) | JWT secret for auth tests |

### Database Lifecycle

The test suite automatically:

1. **Creates database connections** using the provided configuration
2. **Runs migrations** to set up the schema
3. **Seeds test data** using fixtures
4. **Cleans up** after each test to ensure isolation

### Test Database Helpers

The `testutil` package provides helpers for database testing:

```go
// Create database helper
db := testutil.NewDatabaseHelper(t)
defer db.Close()

// Use with clean database
db.WithCleanDatabase(t, func() {
    // Your test code here
})

// Use with seeded data
fixtures := testutil.NewAllFixtures()
db.WithSeededDatabase(t, fixtures, func() {
    // Your test code here
})
```

## Environment Variables

### Test Configuration

Create a `.env.test` file for local testing:

```bash
# Database Configuration
TEST_DB_HOST=localhost
TEST_DB_PORT=5432
TEST_DB_USER=testuser
TEST_DB_PASSWORD=testpassword
TEST_DB_NAME=voidrunner_test
TEST_DB_SSLMODE=disable

# JWT Configuration
JWT_SECRET_KEY=test-secret-key-for-local-testing

# Test Behavior
TEST_TIMEOUT=30s
LOG_LEVEL=error
```

### Loading Environment Variables

```bash
# Load environment variables for testing
source .env.test

# Or use with specific test commands
env $(cat .env.test | xargs) go test -v ./internal/auth
```

## Troubleshooting

### Common Issues

#### 1. Database Connection Failed

**Error:** `failed to connect to test database`

**Solutions:**
- Check PostgreSQL is running: `pg_isready -h localhost -p 5432`
- Verify credentials are correct
- Ensure database exists: `psql -h localhost -U testuser -d voidrunner_test -c "SELECT 1;"`
- Check firewall settings

```bash
# Debug database connection
psql "host=localhost port=5432 user=testuser password=testpassword dbname=voidrunner_test sslmode=disable" -c "SELECT version();"
```

#### 2. Migration Errors

**Error:** `failed to run migrations`

**Solutions:**
- Check migration files exist: `ls migrations/`
- Verify database user has sufficient privileges
- Reset database and try again:
  ```bash
  dropdb voidrunner_test
  createdb voidrunner_test
  ```

#### 3. Tests Timing Out

**Error:** `context deadline exceeded`

**Solutions:**
- Increase test timeout: `go test -timeout 5m ./...`
- Check database performance
- Run tests with verbose output to identify slow tests: `go test -v ./...`

#### 4. Port Already in Use

**Error:** `bind: address already in use`

**Solutions:**
- Kill existing processes: `lsof -ti:5432 | xargs kill -9`
- Use different port: `export TEST_DB_PORT=5433`
- Stop existing PostgreSQL: `sudo systemctl stop postgresql`

#### 5. Permission Denied Errors

**Error:** `permission denied for database`

**Solutions:**
- Grant privileges to test user:
  ```sql
  GRANT ALL PRIVILEGES ON DATABASE voidrunner_test TO testuser;
  GRANT ALL ON SCHEMA public TO testuser;
  ```

### Test Database Reset

If tests are failing due to database state issues:

```bash
# Reset test database
dropdb voidrunner_test
createdb voidrunner_test
go test -v ./internal/database -run TestMigration
```

### Debugging Test Failures

```bash
# Run tests with detailed output
go test -v -race ./... 2>&1 | tee test-output.log

# Run specific failing test with debug logs
LOG_LEVEL=debug go test -v ./internal/auth -run TestSpecificFailingTest

# Check for race conditions
go test -race ./...

# Profile test performance
go test -cpuprofile=cpu.prof -memprofile=mem.prof ./...
```

## Performance Testing

### Load Testing

The test suite includes performance tests for load testing API endpoints:

```bash
# Run performance tests (requires integration test setup)
go test -v -tags=integration ./internal/testutil -run Performance

# Run with custom parameters
go test -v -tags=integration ./internal/testutil -run Performance -args -users=100 -duration=30s
```

### Benchmark Tests

```bash
# Run benchmark tests
make bench

# Run specific benchmarks
go test -bench=BenchmarkAuth ./internal/auth

# Profile benchmarks
go test -bench=. -cpuprofile=cpu.prof ./internal/auth
```

## Test Organization

### Directory Structure

```
internal/
├── auth/
│   ├── service.go
│   └── service_test.go          # Unit tests for auth service
├── database/
│   ├── user_repository.go
│   ├── user_repository_test.go  # Unit tests for repository
│   └── integration_test.go      # Integration tests
└── testutil/
    ├── database.go              # Database test helpers
    ├── fixtures.go              # Test data fixtures
    ├── factories.go             # Test data factories
    └── *_integration_test.go    # Integration test suites
```

### Test Categories

- **Unit Tests:** Fast, isolated tests (default)
- **Integration Tests:** Require database (tagged with `integration`)
- **Performance Tests:** Load and benchmark tests
- **Contract Tests:** API contract validation

### Test Naming Conventions

```go
// Unit test example
func TestUserService_CreateUser(t *testing.T) { }

// Integration test example
func TestUserRepository_CreateUser_Integration(t *testing.T) { }

// Performance test example
func TestAuth_ConcurrentLogin_Performance(t *testing.T) { }

// Benchmark example
func BenchmarkJWTGeneration(b *testing.B) { }
```

## Contributing Test Guidelines

### Writing Good Tests

1. **Follow the AAA pattern** (Arrange, Act, Assert)
2. **Use descriptive test names** that explain the scenario
3. **Test one thing at a time**
4. **Use table-driven tests** for multiple scenarios
5. **Clean up resources** in test teardown

### Example Test Structure

```go
func TestUserService_CreateUser(t *testing.T) {
    tests := []struct {
        name       string
        request    CreateUserRequest
        setupMock  func(*MockUserRepository)
        wantErr    bool
        wantUser   *User
    }{
        {
            name: "successful user creation",
            request: CreateUserRequest{
                Email: "user@example.com",
                Name:  "Test User",
            },
            setupMock: func(m *MockUserRepository) {
                m.On("Create", mock.Anything, mock.Anything).Return(nil)
            },
            wantErr: false,
        },
        // More test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Arrange
            mockRepo := new(MockUserRepository)
            tt.setupMock(mockRepo)
            service := NewUserService(mockRepo)

            // Act
            user, err := service.CreateUser(context.Background(), tt.request)

            // Assert
            if tt.wantErr {
                assert.Error(t, err)
                assert.Nil(t, user)
            } else {
                assert.NoError(t, err)
                assert.NotNil(t, user)
            }
            mockRepo.AssertExpectations(t)
        })
    }
}
```

### Test Data Management

- Use **factories** for creating test objects
- Use **fixtures** for consistent test data
- **Randomize data** where appropriate to avoid test dependencies
- **Clean up** test data after each test

### Mock Usage

- Mock external dependencies (databases, APIs, etc.)
- Use **testify/mock** for Go mocking
- **Verify mock expectations** in tests
- Keep mocks **simple and focused**

## IDE Integration

### VS Code

Create `.vscode/settings.json`:

```json
{
    "go.testFlags": ["-v", "-race"],
    "go.testTimeout": "30s",
    "go.testEnvVars": {
        "TEST_DB_HOST": "localhost",
        "TEST_DB_USER": "testuser",
        "TEST_DB_PASSWORD": "testpassword",
        "TEST_DB_NAME": "voidrunner_test"
    }
}
```

### GoLand

1. Go to **Run/Debug Configurations**
2. Create new **Go Test** configuration
3. Set **Environment variables:**
   - `TEST_DB_HOST=localhost`
   - `TEST_DB_USER=testuser` 
   - `TEST_DB_PASSWORD=testpassword`
   - `TEST_DB_NAME=voidrunner_test`

## Continuous Integration

The CI pipeline runs tests automatically:

- **Unit tests** run on every PR and push
- **Integration tests** run after unit tests pass
- **Coverage reports** are generated and uploaded
- **Performance tests** run on main branch commits

See `.github/workflows/ci.yml` for the complete CI configuration.

## Resources

- [Go Testing Package Documentation](https://pkg.go.dev/testing)
- [Testify Framework](https://github.com/stretchr/testify)
- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [VoidRunner API Documentation](./swagger.yaml)

---

For questions or issues with testing, please create an issue in the repository or contact the development team.