# VoidRunner Development Guidelines

This document defines code style guidelines, review criteria, project-specific rules, and preferred patterns for the VoidRunner distributed task execution platform.

## Project Overview

VoidRunner is a distributed task execution platform designed for secure, scalable code execution. The project follows an incremental development approach through well-defined Epic milestones.

### Current Implementation Status (Epic 1 ✅ Complete)
- **Backend**: Go + Gin framework + PostgreSQL (pgx driver)
- **API**: RESTful API with JWT authentication and comprehensive task management
- **Database**: PostgreSQL with optimized schema and cursor pagination
- **Testing**: 80%+ code coverage with unit and integration tests
- **Documentation**: OpenAPI/Swagger specs with comprehensive examples

### Planned Architecture (Epic 2-4 📋 Roadmap)
- **Container Execution**: Docker + gVisor security for safe code execution
- **Frontend**: Svelte + SvelteKit + TypeScript web interface
- **Infrastructure**: Kubernetes (GKE) deployment with microservices
- **Queue System**: Redis for task scheduling and real-time updates
- **Monitoring**: Real-time metrics, logging, and alerting systems

## Go Code Standards

### Project Structure

```
voidrunner/
├── cmd/                    # Application entrypoints
│   ├── api/               # ✅ API server main (implemented)
│   ├── migrate/           # ✅ Database migration tool (implemented)
│   ├── scheduler/         # 📋 Task scheduler main (planned - Epic 2)
│   └── executor/          # 📋 Task executor main (planned - Epic 2)
├── internal/              # Private application code
│   ├── api/              # ✅ API handlers and routes (implemented)
│   ├── auth/             # ✅ Authentication logic (implemented)
│   ├── config/           # ✅ Configuration management (implemented)
│   ├── database/         # ✅ Database layer (implemented)
│   ├── models/           # ✅ Data models (implemented)
│   ├── services/         # ✅ Business logic services (implemented)
│   ├── executor/         # 📋 Task execution engine (planned - Epic 2)
│   ├── queue/            # 📋 Message queue integration (planned - Epic 2)
│   └── security/         # 📋 Security utilities (planned - Epic 2)
├── pkg/                   # Public libraries
│   ├── logger/           # ✅ Structured logging (implemented)
│   ├── metrics/          # 📋 Prometheus metrics (planned - Epic 2)
│   └── utils/            # 📋 Shared utilities (planned)
├── api/                   # ✅ API specifications (OpenAPI) (implemented)
├── migrations/           # ✅ Database migrations (implemented)
├── tests/                # ✅ Integration tests (implemented)
├── scripts/              # ✅ Build and deployment scripts (implemented)
├── docs/                 # ✅ Documentation (implemented)
├── deployments/          # 📋 Kubernetes manifests (planned - Epic 3)
└── frontend/             # 📋 Svelte web interface (planned - Epic 3)
```

#### Epic Development Roadmap

**Epic 1: Core API Infrastructure** ✅ **Complete**
- JWT authentication system
- Task management CRUD operations
- PostgreSQL database with pgx
- Comprehensive testing suite
- OpenAPI documentation

**Epic 2: Container Execution Engine** 🔄 **In Development**
- Docker client integration with security
- Task execution workflow and state management
- Real-time log collection and streaming
- Error handling and cleanup mechanisms

**Epic 3: Frontend Interface** 📋 **Planned**
- Svelte project setup and architecture
- Authentication UI and user management
- Task creation and management interface
- Real-time task status updates

**Epic 4: Advanced Features** 📋 **Planned**
- Collaborative features and sharing
- Advanced search and filtering
- Real-time dashboard and system metrics
- Advanced notifications and alerting

### Coding Standards

#### 1. Naming Conventions

- **Packages**: lowercase, single words when possible (`auth`, `database`, `executor`)
- **Functions**: CamelCase for exported, camelCase for private
- **Constants**: ALL_CAPS for package-level constants
- **Interfaces**: Add "er" suffix (`TaskExecutor`, `LogStreamer`)

#### 2. Error Handling

```go
// PREFERRED: Structured error handling with context
func (s *TaskService) CreateTask(ctx context.Context, req CreateTaskRequest) (*Task, error) {
    if err := s.validateTaskRequest(req); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    task, err := s.repo.CreateTask(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("failed to create task: %w", err)
    }

    return task, nil
}

// AVOID: Generic error messages without context
func (s *TaskService) CreateTask(req CreateTaskRequest) (*Task, error) {
    task, err := s.repo.CreateTask(req)
    if err != nil {
        return nil, err // Too generic
    }
    return task, nil
}
```

#### 3. Database Interactions

```go
// PREFERRED: Use pgx with prepared statements and proper error handling
func (r *TaskRepository) GetTaskByID(ctx context.Context, taskID string) (*Task, error) {
    query := `
        SELECT id, name, description, status, created_at, updated_at
        FROM tasks
        WHERE id = $1 AND deleted_at IS NULL
    `

    var task Task
    err := r.pool.QueryRow(ctx, query, taskID).Scan(
        &task.ID, &task.Name, &task.Description,
        &task.Status, &task.CreatedAt, &task.UpdatedAt,
    )

    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, ErrTaskNotFound
        }
        return nil, fmt.Errorf("failed to get task %s: %w", taskID, err)
    }

    return &task, nil
}
```

#### 4. Dependency Injection

```go
// PREFERRED: Constructor pattern with interfaces
type TaskService struct {
    repo     TaskRepository
    executor TaskExecutor
    logger   *slog.Logger
    metrics  *prometheus.Registry
}

func NewTaskService(
    repo TaskRepository,
    executor TaskExecutor,
    logger *slog.Logger,
    metrics *prometheus.Registry,
) *TaskService {
    return &TaskService{
        repo:     repo,
        executor: executor,
        logger:   logger,
        metrics:  metrics,
    }
}
```

#### 5. Context Usage

```go
// PREFERRED: Always pass context as first parameter
func (s *TaskService) ExecuteTask(ctx context.Context, taskID string) error {
    // Check context cancellation
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    // Use context in downstream calls
    task, err := s.repo.GetTaskByID(ctx, taskID)
    if err != nil {
        return err
    }

    return s.executor.Execute(ctx, task)
}
```

### Security Guidelines

#### 1. Container Execution Security

```go
// REQUIRED: All container executions must use security constraints
func (e *DockerExecutor) Execute(ctx context.Context, task *Task) error {
    containerConfig := &container.Config{
        Image:      e.getExecutorImage(task.Language),
        User:       "1000:1000", // REQUIRED: Non-root execution
        WorkingDir: "/tmp/workspace",
        Env:        e.sanitizeEnvironment(task.Environment),
    }

    hostConfig := &container.HostConfig{
        Resources: container.Resources{
            Memory:    task.MemoryLimit,
            CPUQuota:  task.CPUQuota,
            PidsLimit: ptr(int64(128)), // REQUIRED: Limit processes
        },
        SecurityOpt: []string{
            "no-new-privileges",
            "seccomp=/opt/voidrunner/seccomp-profile.json",
        },
        NetworkMode:    "none", // REQUIRED: No network access
        ReadonlyRootfs: true,   // REQUIRED: Read-only filesystem
        AutoRemove:     true,   // REQUIRED: Automatic cleanup
    }

    return e.executeWithTimeout(ctx, containerConfig, hostConfig, task.Timeout)
}
```

#### 2. Input Validation

```go
// REQUIRED: Validate all user inputs
func validateTaskRequest(req CreateTaskRequest) error {
    if strings.TrimSpace(req.Name) == "" {
        return ErrTaskNameRequired
    }

    if len(req.Name) > 255 {
        return ErrTaskNameTooLong
    }

    if !isValidLanguage(req.Language) {
        return ErrUnsupportedLanguage
    }

    if len(req.Code) > MaxCodeSize {
        return ErrCodeTooLarge
    }

    // Sanitize code content
    if containsDangerousPatterns(req.Code) {
        return ErrDangerousCodePattern
    }

    return nil
}
```

### Logging Standards

#### 1. Structured Logging with slog

```go
// PREFERRED: Use structured logging with context
func (s *TaskService) CreateTask(ctx context.Context, req CreateTaskRequest) (*Task, error) {
    logger := s.logger.With(
        "operation", "create_task",
        "user_id", getUserID(ctx),
        "task_name", req.Name,
    )

    logger.Info("creating new task")

    task, err := s.repo.CreateTask(ctx, req)
    if err != nil {
        logger.Error("failed to create task", "error", err)
        return nil, err
    }

    logger.Info("task created successfully", "task_id", task.ID)
    return task, nil
}
```

#### 2. Log Levels

- **DEBUG**: Detailed flow information for troubleshooting
- **INFO**: General operational information
- **WARN**: Something unexpected happened but system continues
- **ERROR**: Error condition that needs attention

### Testing Standards

#### 1. Test File Organization

```go
// File: internal/api/task_handler_test.go
package api

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestTaskHandler_CreateTask(t *testing.T) {
    tests := []struct {
        name           string
        request        CreateTaskRequest
        mockSetup      func(*MockTaskService)
        expectedStatus int
        expectedError  string
    }{
        {
            name: "successful task creation",
            request: CreateTaskRequest{
                Name:        "test-task",
                Language:    "python",
                Code:        "print('hello')",
            },
            mockSetup: func(m *MockTaskService) {
                m.On("CreateTask", mock.Anything, mock.Anything).
                    Return(&Task{ID: "123"}, nil)
            },
            expectedStatus: 201,
        },
        // More test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

#### 2. Integration Tests

```go
// REQUIRED: Integration tests for critical paths
func TestTaskExecution_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Setup test database
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)

    // Setup test containers
    executor := setupTestExecutor(t)
    defer cleanupTestExecutor(t, executor)

    // Test execution flow
    service := NewTaskService(db, executor, logger)

    task, err := service.CreateTask(context.Background(), CreateTaskRequest{
        Name:     "integration-test",
        Language: "python",
        Code:     "print('integration test')",
    })
    require.NoError(t, err)

    err = service.ExecuteTask(context.Background(), task.ID)
    require.NoError(t, err)

    // Verify execution results
    result, err := service.GetTaskResult(context.Background(), task.ID)
    require.NoError(t, err)
    assert.Equal(t, "completed", result.Status)
}
```

## Kubernetes and Infrastructure Standards

### 1. Resource Specifications

```yaml
# REQUIRED: All deployments must specify resource limits
apiVersion: apps/v1
kind: Deployment
metadata:
  name: voidrunner-api
spec:
  template:
    spec:
      containers:
        - name: api
          image: voidrunner/api:latest
          resources:
            requests:
              memory: "256Mi"
              cpu: "100m"
            limits:
              memory: "1Gi"
              cpu: "500m"
          # REQUIRED: Security context
          securityContext:
            allowPrivilegeEscalation: false
            runAsNonRoot: true
            runAsUser: 1000
            readOnlyRootFilesystem: true
```

### 2. Health Checks

```yaml
# REQUIRED: All services must have health checks
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 2
```

## Code Review Criteria

### Mandatory Checks

- [ ] **Security**: No hardcoded secrets, proper input validation
- [ ] **Error Handling**: All errors properly wrapped with context
- [ ] **Testing**: Unit tests for new functionality, integration tests for critical paths
- [ ] **Performance**: Database queries optimized, no N+1 problems
- [ ] **Logging**: Structured logging with appropriate levels
- [ ] **Documentation**: Public functions and complex logic documented

### Performance Requirements

- API response times: < 200ms for 95% of requests
- Database queries: < 50ms median response time
- Container startup: < 5 seconds for cold starts
- Memory usage: < 1GB per API instance

### Security Requirements

- All user inputs validated and sanitized
- Container execution with security constraints
- Secrets managed through Kubernetes secrets
- No privilege escalation in containers
- Network policies enforced

## Git Workflow and Commit Standards

### Branch Naming

- `feature/issue-number-short-description`
- `bugfix/issue-number-short-description`
- `hotfix/issue-number-short-description`

### Commit Messages

```
type(scope): short description

Longer description if needed

Fixes #123
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`
Scopes: `api`, `frontend`, `executor`, `scheduler`, `k8s`, `security`

### Pull Request Requirements

- [ ] All CI checks passing
- [ ] Code coverage maintains > 80%
- [ ] Security scan passes
- [ ] Documentation updated
- [ ] Breaking changes documented

## Environment-Specific Configurations

### Development

```yaml
# config/development.yaml
database:
  host: localhost
  port: 5432
  ssl_mode: disable

executor:
  timeout: 30s
  memory_limit: 512Mi

logging:
  level: debug
  format: console
```

### Production

```yaml
# config/production.yaml
database:
  host: ${DB_HOST}
  port: 5432
  ssl_mode: require

executor:
  timeout: 3600s
  memory_limit: 1Gi

logging:
  level: info
  format: json
```

### Testing

Testing configuration is unified between CI and local environments for consistency:

```bash
# Integration test environment variables (used by both CI and local)
TEST_DB_HOST=localhost
TEST_DB_PORT=5432
TEST_DB_USER=testuser
TEST_DB_PASSWORD=testpassword
TEST_DB_NAME=voidrunner_test
TEST_DB_SSLMODE=disable
JWT_SECRET_KEY=test-secret-key-for-integration
```

**Key Principles:**
- **Unified Configuration**: Same database and JWT settings for CI and local testing
- **Environment Detection**: `CI=true` used only for output formats (SARIF, coverage)
- **Database Independence**: Tests automatically skip when database unavailable
- **Consistent Behavior**: Integration tests behave identically in both environments

## Common Patterns and Anti-Patterns

### ✅ Preferred Patterns

```go
// Repository pattern with interfaces
type TaskRepository interface {
    CreateTask(ctx context.Context, task *Task) error
    GetTask(ctx context.Context, id string) (*Task, error)
    UpdateTaskStatus(ctx context.Context, id string, status TaskStatus) error
}

// Service layer with dependency injection
type TaskService struct {
    repo TaskRepository
    exec TaskExecutor
}

// Proper context cancellation handling
func (s *Service) LongRunningOperation(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            // Continue processing
        }
    }
}
```

### ❌ Anti-Patterns to Avoid

```go
// DON'T: Global variables
var GlobalDB *sql.DB

// DON'T: Panic in library code
func ProcessTask(task *Task) {
    if task == nil {
        panic("task is nil") // Use error returns instead
    }
}

// DON'T: Ignoring errors
result, _ := dangerousOperation() // Always handle errors

// DON'T: Magic numbers
time.Sleep(300 * time.Second) // Use named constants
```

## Monitoring and Observability

### Metrics

```go
// REQUIRED: Add metrics for all critical operations
var (
    taskExecutionDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "voidrunner_task_execution_duration_seconds",
            Help: "Time spent executing tasks",
        },
        []string{"task_type", "status"},
    )
)

func (s *TaskService) ExecuteTask(ctx context.Context, task *Task) error {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        taskExecutionDuration.WithLabelValues(task.Language, task.Status).Observe(duration.Seconds())
    }()

    return s.executor.Execute(ctx, task)
}
```

### Tracing

```go
// REQUIRED: Add tracing for complex operations
func (s *TaskService) ExecuteTask(ctx context.Context, taskID string) error {
    ctx, span := tracer.Start(ctx, "TaskService.ExecuteTask")
    defer span.End()

    span.SetAttributes(attribute.String("task.id", taskID))

    // Implementation...
}
```

## CLI Commands and Scripts

### Development Commands

```bash
# Setup development environment
make setup

# Start development server with auto-reload
make dev

# Run tests
make test              # Unit tests (with coverage in CI)
make test-fast         # Fast unit tests (short mode)
make test-integration  # Integration tests
make test-all          # Both unit and integration tests

# Coverage analysis
make coverage          # Generate coverage report
make coverage-check    # Check coverage meets 80% threshold

# Code quality
make fmt               # Format code
make vet               # Run go vet
make lint              # Run linting (with format check in CI)
make security          # Security scan

# Build and run
make build             # Build API server
make run               # Run API server locally

# Documentation
make docs              # Generate API docs
make docs-serve        # Serve docs locally

# Development tools
make install-tools     # Install development tools
make clean             # Clean build artifacts

# Database management
make db-start          # Start test database (Docker)
make db-stop           # Stop test database
make db-reset          # Reset test database (clean slate)

# Database migrations
make migrate-up        # Run database migrations up
make migrate-down      # Run database migrations down (rollback one)
make migrate-reset     # Reset database (rollback all migrations)
make migration name=X  # Create new migration file

# Dependencies and setup
make deps              # Download and tidy dependencies
make deps-update       # Update dependencies
make setup             # Setup complete development environment
```

### Database Operations

```bash
# Database management (implemented)
make db-start          # Start test database container
make db-stop           # Stop test database container
make db-reset          # Reset test database to clean state

# Migration management (implemented)
make migrate-up        # Apply all pending migrations
make migrate-down      # Rollback last migration
make migrate-reset     # Rollback all migrations
make migration name=add_feature  # Create new migration files

# Legacy scripts (planned for Epic 2)
./scripts/backup-db.sh production    # Database backup utility
./scripts/restore-db.sh backup.sql   # Database restore utility
```

## Documentation Standards

### API Documentation

- OpenAPI specifications for all endpoints
- Include request/response examples
- Document error codes and meanings
- Rate limiting information

### Code Documentation

```go
// TaskExecutor handles the execution of user-submitted code in secure containers.
// It manages the complete lifecycle from container creation to cleanup.
//
// Example usage:
//   executor := NewDockerExecutor(client, logger)
//   result, err := executor.Execute(ctx, task)
//   if err != nil {
//       return fmt.Errorf("execution failed: %w", err)
//   }
type TaskExecutor interface {
    // Execute runs the given task in a secure container environment.
    // It returns the execution result or an error if execution fails.
    Execute(ctx context.Context, task *Task) (*ExecutionResult, error)
}
```

## Release and Deployment

### Version Tagging

- Use semantic versioning: `v1.2.3`
- Tag format: `git tag -a v1.2.3 -m "Release v1.2.3"`

### Deployment Checklist

- [ ] All tests passing
- [ ] Security scan completed
- [ ] Database migrations tested
- [ ] Rollback plan prepared
- [ ] Monitoring dashboards updated
- [ ] Documentation updated

---

**Document Version**: 1.1  
**Last Updated**: 2025-07-10  
**Next Review**: 2025-08-10

For questions about these guidelines, please reach out to the technical lead or create an issue in the repository.
