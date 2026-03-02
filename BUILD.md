# Build and Development Guide

This document describes how to build, test, and develop abot.

## Prerequisites

- Go 1.21 or later
- Docker and Docker Compose (for containerized development)
- Make (for build automation)
- MySQL 8.0+ (for local development)
- Qdrant (optional, for vector storage)

## Quick Start

### Local Development

```bash
# Install dependencies
make deps

# Build all binaries
make all

# Run tests
make test

# Format code
make fmt

# Run linter
make lint
```

### Docker Development

```bash
# Start all services with Docker Compose
docker-compose -f docker-compose.dev.yml up

# The services will be available at:
# - abot-agent: http://localhost:8080
# - abot-server: http://localhost:8080
# - abot-web: http://localhost:3000
# - MySQL: localhost:3306
# - Qdrant: http://localhost:6333
```

## Build Commands

### Build Individual Binaries

```bash
make agent    # Build abot-agent
make server   # Build abot-server
make web      # Build abot-web
```

### Build for Multiple Platforms

```bash
# Build for Linux
GOOS=linux GOARCH=amd64 make all

# Build for macOS
GOOS=darwin GOARCH=arm64 make all

# Build for Windows
GOOS=windows GOARCH=amd64 make all
```

### Docker Images

```bash
# Build all Docker images
make docker-build

# Build specific image
docker build --build-arg BINARY=abot-agent -t abot-agent:latest .

# Push to registry
make docker-push
```

## Testing

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific test
go test -v ./tests/agent/...
```

## Code Quality

```bash
# Format code
make fmt

# Run linter
make lint

# Run all checks
make check
```

## Development Workflow

### Hot Reload Development

The project uses Air for hot reload during development:

```bash
# Start with hot reload
docker-compose -f docker-compose.dev.yml up

# Or run Air directly
air -c .air.toml
```

### Configuration

Create a `config.yaml` file:

```yaml
server:
  port: 8080
  host: 0.0.0.0

database:
  mysql_dsn: "root:password@tcp(localhost:3306)/abot?parseTime=true"

providers:
  - name: openai
    type: openai
    api_key: "your-api-key"
    model: "gpt-4"

channels:
  - type: telegram
    token: "your-bot-token"
```

### Environment Variables

You can also use environment variables:

```bash
export MYSQL_DSN="root:password@tcp(localhost:3306)/abot"
export OPENAI_API_KEY="your-api-key"
export JWT_SECRET="your-secret"
```

## CI/CD

### GitHub Actions

The project uses GitHub Actions for CI/CD:

- **build.yml**: Runs on every push and PR
  - Runs tests
  - Builds binaries
  - Builds Docker images

- **release.yml**: Runs on tag creation
  - Builds for multiple platforms
  - Creates GitHub release
  - Pushes Docker images to registry

### Creating a Release

```bash
# Tag a new version
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# GitHub Actions will automatically:
# 1. Build binaries for all platforms
# 2. Build and push Docker images
# 3. Create a GitHub release with artifacts
```

## Project Structure

```
abot/
├── cmd/                    # Command-line applications
│   ├── abot-agent/        # Agent service
│   ├── abot-server/       # API server
│   └── abot-web/          # Web console
├── pkg/                    # Shared packages
│   ├── agent/             # Agent logic
│   ├── api/               # API handlers
│   ├── channels/          # Channel integrations
│   ├── providers/         # LLM providers
│   ├── storage/           # Database and storage
│   └── tools/             # Agent tools
├── tests/                  # Test files
├── web/                    # Web frontend (if applicable)
├── Makefile               # Build automation
├── Dockerfile             # Multi-stage Docker build
├── docker-compose.yml     # Production compose
├── docker-compose.dev.yml # Development compose
└── .air.toml              # Hot reload config
```

## Troubleshooting

### Build Issues

```bash
# Clean build artifacts
make clean

# Update dependencies
go mod tidy
go mod download

# Rebuild everything
make clean && make all
```

### Docker Issues

```bash
# Remove all containers and volumes
docker-compose down -v

# Rebuild images
docker-compose build --no-cache

# Check logs
docker-compose logs -f
```

### Database Issues

```bash
# Reset database
docker-compose down -v
docker-compose up -d mysql

# Run migrations manually
go run cmd/abot-server/main.go -migrate
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests and linter
5. Submit a pull request

See [CONTRIBUTING.md](CONTRIBUTING.md) for more details.
