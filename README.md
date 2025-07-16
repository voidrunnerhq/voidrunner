# VoidRunner

A distributed task execution platform that provides secure, scalable code execution in containerized environments with comprehensive task management capabilities.

## What is VoidRunner?

VoidRunner is designed to safely execute user-submitted code in isolated containers while providing a robust API for task management, real-time monitoring, and execution tracking. The platform prioritizes security, scalability, and developer experience.

## Features

**Authentication & Security**
- JWT-based user authentication with access and refresh tokens
- Input validation, rate limiting, and secure request handling
- Docker container isolation with seccomp profiles and resource limits

**Task Management**  
- Full CRUD operations for code tasks with metadata support
- Asynchronous task execution with real-time status tracking
- Task prioritization and retry logic with dead letter handling

**Execution Engine**
- Secure Docker-based code execution in isolated containers
- Embedded worker pool with concurrency controls and health monitoring
- Redis-based task queuing with automatic cleanup

**Monitoring & API**
- RESTful API with 16+ endpoints for complete task lifecycle management
- Real-time health checks and worker status monitoring
- PostgreSQL integration with optimized schema and cursor pagination

## System Architecture

### Current Implementation (Embedded Workers)
Single-process architecture with embedded worker pool for development and production

```
┌─────────────────┐    ┌─────────────────────────────────┐    ┌─────────────────┐
│   Web Clients   │    │        API Server               │    │     Database    │
│  (Postman/curl) │◄──►│  ┌─────────────────────────┐    │◄──►│  (PostgreSQL)   │
│                 │    │  │   HTTP API              │    │    │                 │
└─────────────────┘    │  │   - Authentication      │    │    └─────────────────┘
                       │  │   - Task Management     │    │
                       │  └─────────────────────────┘    │    ┌─────────────────┐
                       │  ┌─────────────────────────┐    │◄──►│  Message Queues │
                       │  │   Embedded Workers      │    │    │     (Redis)     │
                       │  │   - Task Processing     │    │    │                 │
                       │  │   - Docker Execution    │    │    └─────────────────┘
                       │  │   - Health Monitoring   │    │
                       │  │   - Concurrency Control │    │    ┌───────────────────┐
                       │  └─────────────────────────┘    │◄──►│ Container Runtime │
                       └─────────────────────────────────┘    │     (Docker)      │
                                                              │                   │
                                                              └───────────────────┘
```


## Prerequisites

- **Docker & Docker Compose** - For containerized execution and development environment
- **Go 1.24.4+** - For local development and building from source
- **Make** - For standardized build and development commands
- **PostgreSQL 15+** - For database operations (auto-configured in development)
- **Redis 7+** - For task queuing (auto-configured in development)

## Getting Started

### Quick Start with Docker Compose

The fastest way to get started with VoidRunner:

```bash
# 1. Clone and setup
git clone https://github.com/voidrunnerhq/voidrunner.git
cd voidrunner
make setup

# 2. Start development environment (includes DB, Redis, API with embedded workers)
make dev-up

# 3. Verify it's running
make dev-status
```

This starts the complete environment on http://localhost:8080

### Verification

Once running, verify your setup:

**Health Check Endpoints:**
- API Health: http://localhost:8080/health
- Worker Status: http://localhost:8080/health/workers  
- API Documentation: http://localhost:8080/docs

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
- `GET /health` - API health check endpoint
- `GET /health/workers` - Embedded worker status and metrics
- `GET /ready` - Readiness check endpoint


## Development

### Manual Setup from Source

To build from source or customize the setup:

```bash
# 1. Clone the repository
git clone https://github.com/voidrunnerhq/voidrunner.git
cd voidrunner

# 2. Setup development environment
make setup

# 3. Configure environment
cp .env.example .env
# Edit .env with your configuration

# 4. Start dependencies and run migrations
make services-start
make migrate-up

# 5. Start the server
make dev
```

### Testing

```bash
make test              # Unit tests
make test-integration  # Integration tests (requires PostgreSQL)
```

For detailed testing instructions, see [Testing Guide](docs/testing.md).

### API Documentation

Interactive API documentation:
```bash
make docs-serve        # Serve docs locally at http://localhost:8081
```


## Configuration

The application uses environment variables for configuration. Copy `.env.example` to `.env` and adjust values as needed:

- **Server**: HOST, PORT, ENV settings
- **Database**: Connection details for PostgreSQL
- **Redis**: Queue system connection details (required for task processing)
- **JWT**: Token configuration and secrets
- **CORS**: Frontend domain configuration
- **Logging**: Level and format settings

**Note**: Redis configuration is required for task queuing and execution. The `.env.example` file includes complete database, Redis, and JWT settings. For manual setup, use `make services-start` to start both PostgreSQL and Redis test services.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Please ensure all tests pass and maintain code coverage above 80%.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
