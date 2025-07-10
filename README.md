# VoidRunner

The LLM-powered distributed task execution platform built with Go and Kubernetes.

## Overview

VoidRunner is a Kubernetes-based distributed task execution platform that provides secure, scalable code execution in containerized environments. The platform is designed with security-first principles and follows microservices architecture.

## Features

- **Secure Execution**: Container-based task execution with gVisor security
- **RESTful API**: Clean HTTP API with structured logging and monitoring
- **Kubernetes Native**: Designed for cloud-native deployments
- **Authentication**: JWT-based authentication system
- **Monitoring**: Built-in health checks and observability

## Quick Start

### Prerequisites

- Go 1.23.4+ installed
- PostgreSQL 15+ (for database operations)
- Docker (for containerization)

### Setup

1. **Clone the repository**
   ```bash
   git clone https://github.com/voidrunnerhq/voidrunner.git
   cd voidrunner
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Configure environment**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

4. **Run the development server**
   ```bash
   go run cmd/api/main.go
   ```

The server will start on `http://localhost:8080` by default.

### API Endpoints

- `GET /health` - Health check endpoint
- `GET /ready` - Readiness check endpoint
- `GET /api/v1/ping` - Simple ping endpoint

### Testing

Run unit tests:
```bash
make test
```

Run integration tests (requires PostgreSQL):
```bash
make test-integration
```

Run all tests with coverage:
```bash
make coverage
```

For detailed testing instructions, database setup, troubleshooting, and performance testing, see [Testing Guide](docs/testing.md).

### Build

Build the application:
```bash
go build -o bin/api cmd/api/main.go
```

Run the binary:
```bash
./bin/api
```

## Architecture

VoidRunner follows the standard Go project layout:

```
voidrunner/
├── cmd/                    # Application entrypoints
│   └── api/               # API server main
├── internal/              # Private application code
│   ├── api/              # API handlers and routes
│   │   ├── handlers/     # HTTP handlers
│   │   ├── middleware/   # HTTP middleware
│   │   └── routes/       # Route definitions
│   ├── config/           # Configuration management
│   ├── database/         # Database layer
│   └── models/           # Data models
├── pkg/                   # Public libraries
│   ├── logger/           # Structured logging
│   ├── metrics/          # Prometheus metrics
│   └── utils/            # Shared utilities
├── migrations/           # Database migrations
├── scripts/              # Build and deployment scripts
└── docs/                 # Documentation
```

## Configuration

The application uses environment variables for configuration. See `.env.example` for available options:

- `SERVER_HOST`: Server bind address (default: localhost)
- `SERVER_PORT`: Server port (default: 8080)
- `SERVER_ENV`: Environment (development/production)
- `LOG_LEVEL`: Logging level (debug/info/warn/error)
- `LOG_FORMAT`: Log format (json/text)
- `CORS_ALLOWED_ORIGINS`: Comma-separated list of allowed origins

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
