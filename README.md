# Borderless Coding Server

A modern Go server application built with Gin, PostgreSQL, GORM, MinIO, and Redis.

## Features

- **Web Framework**: Gin HTTP web framework
- **Database**: PostgreSQL with GORM ORM
- **Cache**: Redis for caching and session storage
- **Object Storage**: MinIO for file storage
- **Health Checks**: Comprehensive health monitoring
- **Docker Support**: Full containerization with docker-compose
- **Structured Logging**: JSON-formatted logs with logrus
- **Environment Configuration**: Flexible configuration management

## Project Structure

```
server/
├── cmd/api/                 # Application entry point
├── config/                  # Configuration management
├── internal/                # Private application code
│   ├── handlers/            # HTTP handlers
│   ├── middleware/          # HTTP middleware
│   └── models/              # Data models
├── pkg/                     # Public library code
│   ├── cache/               # Redis client
│   ├── database/            # Database connection
│   └── storage/             # MinIO client
├── migrations/              # Database migrations
├── docker-compose.yml       # Docker services
├── Dockerfile              # Application container
├── Makefile                # Build commands
└── go.mod                  # Go module definition
```

## Quick Start

### Prerequisites

- Go 1.24+
- Docker and Docker Compose (optional)

### Using Docker Compose (Recommended)

1. Clone the repository
2. Copy environment file:
   ```bash
   cp env.example .env
   ```
3. Start all services:
   ```bash
   docker-compose up -d
   ```

This will start:
- PostgreSQL on port 5432
- Redis on port 6379
- MinIO on ports 9000 (API) and 9001 (Console)
- Go application on port 8080

### Manual Setup

1. Install dependencies:
   ```bash
   make deps
   ```

2. Set up environment variables:
   ```bash
   cp env.example .env
   # Edit .env with your configuration
   ```

3. Start PostgreSQL, Redis, and MinIO services

4. Run the application:
   ```bash
   make dev
   ```

## Configuration

The application uses environment variables for configuration. Copy `env.example` to `.env` and modify as needed:

```env
# Server Configuration
PORT=8080
GIN_MODE=debug

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=borderless_coding
DB_SSLMODE=disable

# Redis Configuration
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# MinIO Configuration
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_USE_SSL=false
MINIO_BUCKET_NAME=borderless-coding
```

## API Endpoints

### Health Checks

- `GET /health` - Basic health check
- `GET /health/ready` - Readiness check (includes dependency checks)
- `GET /health/live` - Liveness check

### API Routes

- `GET /` - Welcome message
- `GET /api/v1/ping` - Ping endpoint

## Development

### Available Commands

```bash
make help          # Show available commands
make build         # Build the application
make run           # Build and run
make dev           # Run in development mode
make test          # Run tests
make clean         # Clean build artifacts
make deps          # Download dependencies
make fmt           # Format code
make lint          # Run linter
```

### Hot Reload

For development with hot reload, install Air:

```bash
make install-tools
make dev-air
```

### Database Migrations

The application automatically runs database migrations on startup. To add new migrations:

1. Create a new model in `internal/models/`
2. Add it to the `AutoMigrate()` function in `pkg/database/database.go`

## Docker

### Build Image

```bash
make docker-build
```

### Run Container

```bash
make docker-run
```

### Full Stack with Docker Compose

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f app

# Stop all services
docker-compose down

# Stop and remove volumes
docker-compose down -v
```

## Monitoring

### Health Checks

The application provides comprehensive health checks:

- **Health**: Basic service health
- **Readiness**: Checks all dependencies (PostgreSQL, Redis, MinIO)
- **Liveness**: Service liveness probe

### Logging

All logs are structured in JSON format and include:
- Request/response information
- Error details
- Performance metrics
- Request tracing

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Run `make fmt` and `make lint`
6. Submit a pull request

## License

This project is licensed under the MIT License.
