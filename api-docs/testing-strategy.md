# VoidRunner Testing Strategy and Architecture

This document outlines the comprehensive testing strategy implemented in VoidRunner, explaining when and how to use different types of tests.

## Overview

VoidRunner follows a **proper testing pyramid** with clear separation between unit tests, integration tests, and end-to-end tests. Each type serves a specific purpose and should NOT require database connectivity for unit tests.

## Testing Pyramid

```
        E2E Tests (Few)
      Integration Tests (Some)  
    Unit Tests (Many - Fast & Isolated)
```

### 1. Unit Tests (No Database Required)

**Purpose**: Test individual functions, methods, and business logic in complete isolation.

**Characteristics**:
- ✅ **Fast execution** (< 100ms per test)
- ✅ **No external dependencies** (database, network, filesystem)
- ✅ **Use mocks** for all dependencies
- ✅ **Test business logic** and validation rules
- ✅ **100% deterministic** results

**Examples in VoidRunner**:

```go
// ✅ GOOD: Model validation unit test
func TestValidateEmail(t *testing.T) {
    tests := []struct {
        email   string
        wantErr bool
    }{
        {"test@example.com", false},
        {"invalid-email", true},
    }
    
    for _, tt := range tests {
        err := models.ValidateEmail(tt.email)
        if tt.wantErr {
            assert.Error(t, err)
        } else {
            assert.NoError(t, err)
        }
    }
}

// ✅ GOOD: Service unit test with repository mocks
func TestAuthService_Register(t *testing.T) {
    mockRepo := &MockUserRepository{}
    service := NewAuthService(mockRepo, jwtSvc, logger, cfg)
    
    mockRepo.On("GetByEmail", ctx, "test@example.com").Return(nil, database.ErrUserNotFound)
    mockRepo.On("Create", ctx, mock.AnythingOfType("*models.User")).Return(nil)
    
    response, err := service.Register(ctx, registerReq)
    assert.NoError(t, err)
    assert.NotNil(t, response)
    
    mockRepo.AssertExpectations(t)
}

// ✅ GOOD: Repository unit test with database mocks
func TestTaskRepository_Create(t *testing.T) {
    mockQuerier := new(MockQuerier)
    repo := &taskRepository{
        querier:       mockQuerier,
        cursorEncoder: NewCursorEncoder(),
    }
    
    row := &MockRow{}
    mockQuerier.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
    
    err := repo.Create(context.Background(), task)
    assert.NoError(t, err)
    
    mockQuerier.AssertExpectations(t)
}
```

**When to Write Unit Tests**:
- Model validation functions
- Service layer business logic
- Repository layer with database mocks
- Utility functions
- Middleware logic
- Error handling scenarios

### 2. Integration Tests (Database Required)

**Purpose**: Test component interactions with real external systems (database, cache, etc.).

**Characteristics**:
- ✅ **Use build tags** (`//go:build integration`)
- ✅ **Real database** connections
- ✅ **Test data setup/cleanup**
- ✅ **Environment variables** for configuration
- ✅ **Slower execution** (acceptable for CI)

**Examples in VoidRunner**:

```go
//go:build integration

func TestTaskRepository_Integration(t *testing.T) {
    _, repos := setupTestDatabase(t)
    ctx := context.Background()
    
    // Test real database operations
    user := &models.User{
        Email:        "integration.test@example.com",
        PasswordHash: "hashed_password_123",
    }
    
    err := repos.Users.Create(ctx, user)
    require.NoError(t, err)
    
    // Test retrieval
    retrievedUser, err := repos.Users.GetByID(ctx, user.ID)
    require.NoError(t, err)
    assert.Equal(t, user.Email, retrievedUser.Email)
    
    // Cleanup
    defer repos.Users.Delete(ctx, user.ID)
}

func TestMain(m *testing.M) {
    // Skip integration tests if not enabled
    if os.Getenv("INTEGRATION_TESTS") != "true" {
        os.Exit(0)
    }
    
    code := m.Run()
    os.Exit(code)
}
```

**Running Integration Tests**:
```bash
# Run integration tests
INTEGRATION_TESTS=true go test -tags=integration ./internal/database

# Run only unit tests (default)
go test ./internal/database
```

### 3. End-to-End Tests (Full System)

**Purpose**: Test complete user workflows through the API with all systems running.

**Characteristics**:
- ✅ **Full application stack**
- ✅ **Real HTTP requests**
- ✅ **Database, Redis, external services**
- ✅ **Test complete user journeys**

## Testing Guidelines by Layer

### Models Layer
```go
// ✅ Unit tests for validation functions
func TestValidatePassword(t *testing.T) { ... }
func TestValidateEmail(t *testing.T) { ... }
func TestUser_ToResponse(t *testing.T) { ... }
```

### Repository Layer
```go
// ✅ Unit tests with database mocks
func TestTaskRepository_Create(t *testing.T) {
    mockQuerier := new(MockQuerier)
    // Test business logic without database
}

// ✅ Integration tests with real database  
//go:build integration
func TestTaskRepository_Integration(t *testing.T) {
    db := setupTestDatabase(t)
    // Test actual database operations
}
```

### Service Layer
```go
// ✅ Unit tests with repository mocks
func TestAuthService_Register(t *testing.T) {
    mockRepo := &MockUserRepository{}
    // Test business logic without dependencies
}
```

### Handler Layer
```go
// ✅ Unit tests with service mocks
func TestAuthHandler_Register(t *testing.T) {
    mockService := &MockAuthService{}
    // Test HTTP handling without business logic
}
```

### Middleware Layer
```go
// ✅ Unit tests with Gin test context
func TestAuthMiddleware_RequireAuth(t *testing.T) {
    // Test middleware logic in isolation
}
```

## Mock Implementation Patterns

### Repository Mocks
```go
type MockUserRepository struct {
    mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *models.User) error {
    args := m.Called(ctx, user)
    return args.Error(0)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
    args := m.Called(ctx, email)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*models.User), args.Error(1)
}
```

### Database Interface Mocks
```go
type MockQuerier struct {
    mock.Mock
}

func (m *MockQuerier) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
    arguments := m.Called(ctx, sql, args)
    return arguments.Get(0).(pgx.Row)
}

type MockRow struct {
    mock.Mock
    err  error
    data []interface{}
}

func (m *MockRow) Scan(dest ...interface{}) error {
    if m.err != nil {
        return m.err
    }
    // Copy mock data to destination pointers
    for i := range dest {
        if i < len(m.data) {
            // Type-safe assignment logic
        }
    }
    return nil
}
```

## Test Organization

### File Structure
```
internal/
├── models/
│   ├── user.go
│   ├── user_test.go          # ✅ Unit tests only
│   ├── task.go
│   └── task_test.go          # ✅ Unit tests only
├── database/
│   ├── user_repository.go
│   ├── user_repository_test.go        # ✅ Unit tests with mocks
│   ├── integration_test.go            # ✅ Integration tests only
│   └── task_repository_test.go        # ✅ Unit tests with mocks
├── auth/
│   ├── service.go
│   └── service_test.go       # ✅ Unit tests with mocks
└── api/
    ├── handlers/
    │   └── auth_test.go      # ✅ Unit tests with mocks
    └── middleware/
        └── auth_test.go      # ✅ Unit tests
```

### Test Naming Conventions
```go
// ✅ Good test names
func TestUserService_CreateUser_Success(t *testing.T) { ... }
func TestUserService_CreateUser_EmailAlreadyExists(t *testing.T) { ... }
func TestUserRepository_Create_DatabaseError(t *testing.T) { ... }

// ❌ Bad test names  
func TestCreateUser(t *testing.T) { ... }
func TestError(t *testing.T) { ... }
```

## Running Tests

### Unit Tests (Default)
```bash
# Run all unit tests (fast)
go test ./...

# Run specific package unit tests
go test ./internal/auth -v

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Integration Tests
```bash
# Run integration tests
INTEGRATION_TESTS=true go test -tags=integration ./internal/database

# Run integration tests with verbose output
INTEGRATION_TESTS=true go test -tags=integration -v ./internal/database
```

### All Tests
```bash
# Run both unit and integration tests
make test-all

# Or manually:
go test ./...
INTEGRATION_TESTS=true go test -tags=integration ./internal/database
```

## Test Data Management

### Unit Tests
```go
// ✅ Use test data builders/factories
func newTestUser() *models.User {
    return &models.User{
        BaseModel: models.BaseModel{ID: uuid.New()},
        Email:     "test@example.com",
        Name:      "Test User",
    }
}

// ✅ Use table-driven tests
func TestValidateEmail(t *testing.T) {
    tests := []struct {
        name    string
        email   string
        wantErr bool
    }{
        {"valid email", "test@example.com", false},
        {"invalid email", "invalid", true},
    }
    // ...
}
```

### Integration Tests
```go
func setupTestDatabase(t *testing.T) (*Connection, *Repositories) {
    t.Helper()
    
    cfg := &config.DatabaseConfig{
        Host:     getEnvOrDefault("TEST_DB_HOST", "localhost"),
        Database: getEnvOrDefault("TEST_DB_NAME", "voidrunner_test"),
        // ...
    }
    
    conn, err := NewConnection(cfg, nil)
    require.NoError(t, err)
    
    // Run migrations
    err = MigrateUp(migrateConfig)
    require.NoError(t, err)
    
    t.Cleanup(func() {
        cleanupTestData(t, repos)
        conn.Close()
    })
    
    return conn, NewRepositories(conn)
}
```

## Performance Testing

### Benchmarks
```go
func BenchmarkUserService_CreateUser(b *testing.B) {
    service := setupBenchmarkService(b)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        user := newTestUser()
        service.CreateUser(context.Background(), user)
    }
}

// Run benchmarks
go test -bench=. ./internal/auth
```

## Continuous Integration

### GitHub Actions Configuration
```yaml
# .github/workflows/test.yml
- name: Run Unit Tests
  run: go test -race -cover ./...

- name: Run Integration Tests  
  env:
    INTEGRATION_TESTS: true
    TEST_DB_HOST: localhost
    TEST_DB_NAME: voidrunner_test
  run: go test -tags=integration ./internal/database
```

## Best Practices Summary

### ✅ DO
- Write unit tests for all business logic **without** database dependencies
- Use mocks to isolate units under test
- Write integration tests for database interactions
- Use build tags to separate integration tests
- Follow the testing pyramid (many unit, some integration, few e2e)
- Use table-driven tests for multiple scenarios
- Create test data builders/factories
- Test error conditions and edge cases
- Use meaningful test names that describe the scenario

### ❌ DON'T
- Write unit tests that require database connections
- Skip mocking dependencies in unit tests
- Mix unit and integration test concerns
- Write integration tests for business logic validation
- Use real external services in unit tests
- Ignore test coverage for critical business logic
- Write tests that depend on external state
- Use production data in tests

## Measuring Success

### Coverage Targets
- **Unit Test Coverage**: > 80% for business logic
- **Integration Test Coverage**: Cover all repository methods
- **Critical Path Coverage**: 100% for authentication, authorization, task execution

### Performance Targets
- **Unit Tests**: < 100ms average execution time
- **Integration Tests**: < 5s average execution time
- **Test Suite**: Complete in < 2 minutes in CI

## Testing Tools

### Dependencies
```go
// Testing framework
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
"github.com/stretchr/testify/mock"

// HTTP testing
"net/http/httptest"

// Gin testing
"github.com/gin-gonic/gin"
```

### Makefile Targets
```makefile
# Run unit tests
.PHONY: test
test:
	go test -race -cover ./...

# Run integration tests
.PHONY: test-integration
test-integration:
	INTEGRATION_TESTS=true go test -tags=integration ./internal/database

# Run all tests
.PHONY: test-all
test-all: test test-integration

# Generate coverage report
.PHONY: coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
```

---

This testing strategy ensures VoidRunner has reliable, fast, and maintainable tests that provide confidence in code changes while following industry best practices for the testing pyramid.