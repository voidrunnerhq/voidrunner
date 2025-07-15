# VoidRunner Test Structure

This directory contains integration tests and shared testing utilities for the VoidRunner project. The test structure is organized for clear separation between unit tests (located with source code) and integration tests (located here).

## Directory Structure

```
tests/
├── integration/           # Integration tests
│   ├── auth_test.go      # Authentication integration tests
│   ├── contract_test.go  # OpenAPI contract validation tests
│   ├── database_test.go  # Database integration tests
│   ├── demo_test.go      # Infrastructure demo/example tests
│   ├── e2e_test.go       # End-to-end workflow tests
│   ├── example_test.go   # Example usage patterns
│   └── main_test.go      # Main application tests
├── testutil/             # Shared testing utilities
│   ├── database.go       # Database testing helpers
│   ├── factories.go      # Test data factories
│   ├── fixtures.go       # Pre-defined test fixtures
│   ├── http.go           # HTTP testing utilities
│   ├── openapi_validator.go # OpenAPI validation utilities
│   └── suite.go          # Test suite utilities
└── README.md            # This documentation
```

## Test Types

### Unit Tests
- **Location**: Co-located with source code (e.g., `internal/`, `pkg/`, `cmd/`)
- **Purpose**: Test individual components in isolation
- **Speed**: Fast (< 100ms per test)
- **Dependencies**: No external dependencies (database, network, etc.)
- **Run with**: `make test` or `make test-fast`

### Integration Tests
- **Location**: `tests/integration/`
- **Purpose**: Test component interactions and external dependencies
- **Speed**: Slower (may take seconds per test)
- **Dependencies**: Real database, HTTP server, etc.
- **Run with**: `make test-integration`

## Using the Test Infrastructure

### Database Setup for Integration Tests

Integration tests require a running PostgreSQL database. You need to explicitly set up the database before running integration tests.

#### Quick Start (Recommended)
```bash
# Step 1: Start the test database
make db-start

# Step 2: Run integration tests
make test-integration

# Step 3: Stop the database when done (optional)
make db-stop
```

#### Manual Database Management
Additional database management commands:

```bash
# Database lifecycle commands
make db-start          # Start test database
make db-stop           # Stop test database  
make db-reset          # Reset database (clean slate)
make migrate-up        # Run database migrations
make migrate-down      # Roll back one migration

# Development workflow
make setup              # Complete development setup
```

### Running Tests

```bash
# Run only unit tests (fast)
make test

# Run only integration tests (database must be running)
make test-integration

# Run all tests (unit + integration, database must be running)
make test-all

# Run unit tests with coverage
make coverage

# Run tests in short mode (even faster)
make test-fast
```

**Explicit Database Workflow:**
- `make db-start`: Start test database
- `make test-integration`: Run integration tests (requires database)
- `make db-stop`: Stop test database (optional)
- **Clear separation**: Database management is separate from testing
- **User control**: You decide when to start/stop the database
- **Predictable**: No automatic database lifecycle management

### Environment Setup

Integration tests require environment variables:

```bash
# Database configuration
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5432
export TEST_DB_USER=testuser
export TEST_DB_PASSWORD=testpassword
export TEST_DB_NAME=voidrunner_test
export TEST_DB_SSLMODE=disable

# JWT configuration
export JWT_SECRET_KEY=test-secret-key-for-integration
```

### Writing Integration Tests

#### Using the Integration Test Suite

```go
//go:build integration

package integration_test

import (
    "testing"
    "github.com/stretchr/testify/suite"
    "github.com/voidrunnerhq/voidrunner/tests/testutil"
)

type MyIntegrationSuite struct {
    testutil.IntegrationTestSuite
}

func TestMyIntegration(t *testing.T) {
    suite.Run(t, new(MyIntegrationSuite))
}

func (s *MyIntegrationSuite) TestSomething() {
    // Test with HTTP helper
    resp := s.HTTP.GET(s.T(), "/api/v1/health").ExpectOK()
    
    // Test with database helper
    user := s.DB.CreateMinimalUser(s.T(), context.Background(), "test@example.com", "Test User")
    
    // Test with factories
    task := testutil.NewTaskFactory(user.ID).Build()
    err := s.DB.Repositories.Tasks.Create(context.Background(), task)
    s.Require().NoError(err)
}
```

#### Standalone Database Tests

```go
func TestDatabase(t *testing.T) {
    testutil.WithTestDatabase(t, func(db *testutil.DatabaseHelper) {
        // Your database test code here
        user := testutil.NewUserFactory().Build()
        err := db.Repositories.Users.Create(context.Background(), user)
        require.NoError(t, err)
    })
}
```

#### Using Test Fixtures

```go
func TestWithFixtures(t *testing.T) {
    fixtures := testutil.NewAllFixtures()
    
    testutil.WithSeededTestDatabase(t, fixtures, func(db *testutil.DatabaseHelper) {
        // Test with pre-seeded data
        user, err := db.Repositories.Users.GetByEmail(context.Background(), fixtures.Users.AdminUser.Email)
        require.NoError(t, err)
        assert.Equal(t, "Admin User", user.Name)
    })
}
```

## Test Data Management

### Factories

Factories provide flexible test data creation:

```go
// User factory
user := testutil.NewUserFactory().
    WithEmail("test@example.com").
    WithName("Test User").
    Admin().
    Build()

// Task factory
task := testutil.NewTaskFactory(user.ID).
    WithName("Test Task").
    WithPythonScript("print('hello')").
    HighPriority().
    Completed().
    Build()

// Execution factory
execution := testutil.NewExecutionFactory(task.ID).
    Successful().
    WithStdout("hello\n").
    WithExecutionTime(1000).
    Build()
```

### Fixtures

Fixtures provide pre-defined test data:

```go
fixtures := testutil.NewAllFixtures()

// Access pre-defined users
adminUser := fixtures.Users.AdminUser
regularUser := fixtures.Users.RegularUser

// Access pre-defined tasks
pendingTask := fixtures.Tasks.PendingTask
completedTask := fixtures.Tasks.CompletedTask

// Access pre-defined executions
successfulExecution := fixtures.Executions.SuccessfulExecution
```

## HTTP Testing

### Basic HTTP Tests

```go
// Simple GET request
resp := httpHelper.GET(t, "/api/v1/health").ExpectOK()

// POST request with body
body := map[string]string{"key": "value"}
resp := httpHelper.POST(t, "/api/v1/endpoint", body).ExpectCreated()

// Authenticated request
auth := httpHelper.LoginUser(t, "user@example.com", "password")
resp := httpHelper.AuthenticatedGET(t, "/api/v1/tasks", auth).ExpectOK()
```

### Response Validation

```go
resp := httpHelper.GET(t, "/api/v1/tasks")
    .ExpectOK()
    .ExpectJSON()
    .ExpectBodyContains("tasks")

// Unmarshal response
var tasks []models.Task
resp.UnmarshalResponse(&tasks)
```

## OpenAPI Contract Testing

```go
// Validate response against OpenAPI spec
validator := testutil.NewOpenAPIValidator()
validator.ValidateResponse(t, "GET", "/api/v1/tasks", response, responseBody)

// Fluent interface
testutil.NewHTTPResponseValidator(t, response)
    .ExpectStatus(200)
    .ExpectContentType("application/json")
    .ExpectValidJSON()
    .ExpectOpenAPICompliance("GET", "/api/v1/tasks")
```

## Database Testing

### Database Helpers

```go
// Create minimal test data
user := db.CreateMinimalUser(t, ctx, "test@example.com", "Test User")
task := db.CreateMinimalTask(t, ctx, user.ID, "Test Task")
execution := db.CreateMinimalExecution(t, ctx, task.ID)

// Clean database
db.CleanupDatabase(t)

// Seed with fixtures
db.SeedDatabase(t, fixtures)
```

### Test Database Management

The test infrastructure automatically:
- Skips tests when database is unavailable
- Manages database connections
- Runs migrations before tests
- Cleans up data between tests

## Best Practices

### 1. Test Organization

- Use descriptive test names
- Group related tests in suites
- Use subtests for variations
- Keep tests independent

### 2. Test Data

- Use factories for dynamic data
- Use fixtures for consistent scenarios
- Clean up data between tests
- Don't rely on external data

### 3. Error Handling

- Use `require` for critical assertions
- Use `assert` for non-critical checks
- Provide meaningful error messages
- Test error conditions

### 4. Performance

- Run unit tests frequently
- Run integration tests before commits
- Use short mode for quick feedback
- Profile slow tests

## Configuration

### Test Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TEST_DB_HOST` | `localhost` | Database host |
| `TEST_DB_PORT` | `5432` | Database port |
| `TEST_DB_USER` | `testuser` | Database user |
| `TEST_DB_PASSWORD` | `testpassword` | Database password |
| `TEST_DB_NAME` | `voidrunner_test` | Database name |
| `TEST_DB_SSLMODE` | `disable` | SSL mode |
| `JWT_SECRET_KEY` | `test-secret-key-for-integration` | JWT secret |

### CI/CD Configuration

The test infrastructure adapts to CI environments:

- **Coverage reporting**: Enabled when `CI=true`
- **Format checking**: Enabled when `CI=true`
- **Security scanning**: SARIF format when `CI=true`
- **Test skipping**: Integration tests skipped when database unavailable

## Troubleshooting

### Common Issues

1. **Database connection errors**
   - Start database: `make db-start`
   - Restart database: `make db-reset`
   - Verify Docker is running: `docker info`
   - Check database status: `make db-status` or `docker-compose -f docker-compose.test.yml ps`

2. **Test timeouts**
   - Increase timeout values
   - Verify network connectivity
   - Reset database if needed: `make db-reset`

3. **Import errors**
   - Ensure Go modules are up to date: `go mod tidy`
   - Check import paths
   - Verify build tags

4. **Database setup issues**
   - Clean and restart: `make db-reset`
   - Check Docker Compose: `make db-status` or `docker-compose -f docker-compose.test.yml ps`
   - Verify migrations: `make migrate-up`

5. **Integration test failures**
   - Ensure database is running: `make db-start`
   - Run migrations: `make migrate-up`
   - Check test database configuration in `.env`

### Debug Tips

```go
// Print response body for debugging
resp.PrintBody()

// Skip tests conditionally
if testing.Short() {
    t.Skip("Skipping in short mode")
}

// Use test helpers
t.Helper() // Mark function as test helper
```

## Migration from Build Tags

This structure replaces the previous build tag approach:

**Old approach**: Used `//go:build integration` tags with tests scattered throughout the codebase
**New approach**: Physical separation with dedicated directories

Benefits:
- Clear separation of concerns
- Easier to manage test dependencies
- Better IDE support
- Simplified CI/CD configuration
- Reduced build complexity

The build tag `//go:build integration` is still used in integration tests but now serves as an additional safety mechanism rather than the primary organization method.