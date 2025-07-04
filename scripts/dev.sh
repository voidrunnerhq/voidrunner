#!/bin/bash

# Development script for VoidRunner

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if .env file exists
check_env() {
    if [ ! -f ".env" ]; then
        print_warning ".env file not found. Creating from .env.example..."
        if [ -f ".env.example" ]; then
            cp .env.example .env
            print_status "Created .env file from .env.example"
            print_warning "Please edit .env file with your configuration"
        else
            print_error ".env.example file not found"
            exit 1
        fi
    fi
}

# Function to install dependencies
install_deps() {
    print_status "Installing Go dependencies..."
    go mod download
    go mod tidy
}

# Function to run tests
run_tests() {
    print_status "Running tests..."
    go test ./... -v -cover
}

# Function to build the application
build_app() {
    print_status "Building application..."
    mkdir -p bin
    go build -o bin/api cmd/api/main.go
    print_status "Built application: bin/api"
}

# Function to run the development server
run_server() {
    print_status "Starting development server..."
    go run cmd/api/main.go
}

# Function to clean build artifacts
clean() {
    print_status "Cleaning build artifacts..."
    rm -rf bin/
    go clean
}

# Function to run linter
lint() {
    print_status "Running linter..."
    if command -v golangci-lint &> /dev/null; then
        golangci-lint run
    else
        print_warning "golangci-lint not found. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
        go fmt ./...
        go vet ./...
    fi
}

# Function to format code
format() {
    print_status "Formatting code..."
    go fmt ./...
}

# Function to show help
show_help() {
    echo "VoidRunner Development Script"
    echo
    echo "Usage: $0 [command]"
    echo
    echo "Commands:"
    echo "  setup     - Install dependencies and setup environment"
    echo "  test      - Run tests"
    echo "  build     - Build the application"
    echo "  run       - Run the development server"
    echo "  clean     - Clean build artifacts"
    echo "  lint      - Run linter"
    echo "  format    - Format code"
    echo "  help      - Show this help message"
    echo
}

# Main script logic
case "$1" in
    setup)
        check_env
        install_deps
        print_status "Setup completed"
        ;;
    test)
        run_tests
        ;;
    build)
        build_app
        ;;
    run)
        check_env
        run_server
        ;;
    clean)
        clean
        ;;
    lint)
        lint
        ;;
    format)
        format
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        if [ -z "$1" ]; then
            show_help
        else
            print_error "Unknown command: $1"
            show_help
            exit 1
        fi
        ;;
esac