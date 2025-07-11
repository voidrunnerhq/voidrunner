# VoidRunner

A distributed task execution platform that provides secure, scalable code execution in containerized environments with comprehensive task management capabilities.

## What is VoidRunner?

VoidRunner is designed to safely execute user-submitted code in isolated containers while providing a robust API for task management, real-time monitoring, and execution tracking. The platform prioritizes security, scalability, and developer experience.

## Current Features

- **REST API**: Comprehensive HTTP API with 16+ endpoints for complete task lifecycle management
- **JWT Authentication**: Secure user authentication with access and refresh tokens
- **Task Management**: Full CRUD operations for code tasks with metadata support
- **Execution Tracking**: Detailed execution history with performance metrics
- **Database Integration**: PostgreSQL with optimized schema and cursor pagination
- **Security**: Input validation, rate limiting, and secure request handling
- **Testing**: 80%+ code coverage with unit and integration tests
- **Documentation**: OpenAPI/Swagger specifications with comprehensive examples

## System Architecture

```
Current Implementation (✅ Complete)
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Web Clients   │    │   API Gateway    │    │   PostgreSQL    │
│  (Postman/curl) │◄──►│   (Gin Server)   │◄──►│    Database     │
│                 │    │                  │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │
                                │ JWT Auth & Task Management
                                ▼
                       ┌──────────────────┐
                       │  Internal APIs   │
                       │ - Auth Service   │
                       │ - Task Service   │
                       │ - User Service   │
                       └──────────────────┘

Planned Extensions (📋 Roadmap)
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  Svelte Web UI  │    │  Task Scheduler  │    │ Container Engine│
│   (Frontend)    │    │   (Microservice) │    │  (Docker/gVisor)│
│                 │    │                  │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## API Endpoints

### Authentication
- `POST /api/v1/auth/register` - Register new user
- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/refresh` - Refresh access token
- `POST /api/v1/auth/logout` - User logout
- `GET /api/v1/auth/me` - Get current user info

### Task Management
- `POST /api/v1/tasks` - Create new task
- `GET /api/v1/tasks` - List user's tasks (with pagination)
- `GET /api/v1/tasks/{id}` - Get task details
- `PUT /api/v1/tasks/{id}` - Update task
- `DELETE /api/v1/tasks/{id}` - Delete task

### Task Execution
- `POST /api/v1/tasks/{id}/executions` - Start task execution
- `GET /api/v1/tasks/{id}/executions` - List task executions
- `GET /api/v1/executions/{id}` - Get execution details
- `PUT /api/v1/executions/{id}` - Update execution status
- `DELETE /api/v1/executions/{id}` - Cancel execution

### System Health
- `GET /health` - Health check endpoint
- `GET /ready` - Readiness check endpoint

## Quick Start

### Prerequisites

- Go 1.24.4+ installed
- PostgreSQL 15+ (for database operations)
- Docker (for containerization and testing)

### Setup

1. **Clone the repository**
   ```bash
   git clone https://github.com/voidrunnerhq/voidrunner.git
   cd voidrunner
   ```

2. **Setup development environment**
   ```bash
   make setup
   ```

3. **Configure environment**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

4. **Start the database (for testing)**
   ```bash
   make db-start
   ```

5. **Run database migrations**
   ```bash
   make migrate-up
   ```

6. **Start the development server**
   ```bash
   make dev
   ```

The server will start on `http://localhost:8080` by default.

## Development

### Available Commands

```bash
# Development
make dev                # Start with auto-reload
make run               # Build and run
make build             # Build binary

# Testing
make test              # Unit tests
make test-integration  # Integration tests (requires database)
make test-all          # All tests
make coverage          # Generate coverage report

# Database
make db-start          # Start test database
make db-stop           # Stop test database
make migrate-up        # Apply migrations
make migrate-down      # Rollback migration

# Code Quality
make lint              # Run linter
make fmt               # Format code
make vet               # Run go vet
make security          # Security scan

# Documentation
make docs              # Generate API docs
make docs-serve        # Serve docs locally
```

### Testing

Run unit tests:
```bash
make test
```

Run integration tests (requires PostgreSQL):
```bash
make test-integration
```

For detailed testing instructions, database setup, troubleshooting, and performance testing, see [Testing Guide](docs/testing.md).

### API Documentation

Interactive API documentation is available via Swagger:
```bash
make docs-serve
# Visit http://localhost:8081
```

## Roadmap

### Phase 1: Core Infrastructure ✅ Complete
Task management API with authentication, database integration, and comprehensive testing.

### Phase 2: Container Execution Engine 🔄 In Development
Secure Docker-based code execution with resource limiting, real-time log streaming, and safety controls.

### Phase 3: Web Interface 📋 Planned
Modern Svelte-based frontend with real-time task monitoring, code editor, and user dashboard.

### Phase 4: Advanced Features 📋 Planned
Collaborative features, advanced search and filtering, system metrics dashboard, and notification system.

## Configuration

The application uses environment variables for configuration. Copy `.env.example` to `.env` and adjust values as needed:

- **Server**: HOST, PORT, ENV settings
- **Database**: Connection details for PostgreSQL
- **JWT**: Token configuration and secrets
- **CORS**: Frontend domain configuration
- **Logging**: Level and format settings

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Please ensure all tests pass and maintain code coverage above 80%.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.