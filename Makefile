# Makefile for Borderless Coding Server

.PHONY: help build run test clean deps dev docker-build docker-run

# Default target
help:
	@echo "Available commands:"
	@echo "  build       - Build the application"
	@echo "  run         - Run the application"
	@echo "  test        - Run tests"
	@echo "  clean       - Clean build artifacts"
	@echo "  deps        - Download dependencies"
	@echo "  dev         - Run in development mode"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run  - Run Docker container"

# Build the application
build:
	@echo "Building application..."
	go build -o bin/server cmd/api/main.go

# Run the application
run: build
	@echo "Running application..."
	./bin/server

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod tidy
	go mod download

# Run in development mode
dev:
	@echo "Running in development mode..."
	go run cmd/api/main.go

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t borderless-coding-server .

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run -p 8080:8080 --env-file .env borderless-coding-server

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run with hot reload (requires air)
dev-air:
	@echo "Running with hot reload..."
	air

# Lint code
lint:
	@echo "Running linter..."
	golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	go vet ./...
