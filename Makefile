# Makefile for abot

.PHONY: all agent server web clean test lint install docker-build docker-push help web-ui fmt tidy dev-agent dev-server dev-web test-coverage

# Variables
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT := $(shell git rev-parse --short HEAD)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)

# Build all binaries
all: agent server web

# Build abot-agent
agent:
	@echo "Building abot-agent..."
	@mkdir -p bin
	@CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/abot-agent ./cmd/abot-agent
	@echo "✓ abot-agent built successfully"

# Build abot-server
server:
	@echo "Building abot-server..."
	@mkdir -p bin
	@CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/abot-server ./cmd/abot-server
	@echo "✓ abot-server built successfully"

# Build abot-web
web:
	@echo "Building abot-web..."
	@mkdir -p bin
	@CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/abot-web ./cmd/abot-web
	@echo "✓ abot-web built successfully"

# Build web UI
web-ui:
	@echo "Building web UI..."
	@cd web && npm install && npm run build
	@echo "✓ Web UI built successfully"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -rf web/out/
	@rm -rf web/node_modules/
	@echo "✓ Clean complete"

# Run tests
test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@echo "✓ Tests passed"

# Run tests with coverage report
test-coverage: test
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"

# Run linters
lint:
	@echo "Running linters..."
	@golangci-lint run ./...
	@echo "✓ Linting passed"

# Install binaries to $GOPATH/bin
install: all
	@echo "Installing binaries..."
	@go install ./cmd/abot-agent
	@go install ./cmd/abot-server
	@go install ./cmd/abot-web
	@echo "✓ Binaries installed"

# Build Docker images
docker-build: docker-build-agent docker-build-server docker-build-web

docker-build-agent:
	@echo "Building abot-agent Docker image..."
	@docker build -f cmd/abot-agent/Dockerfile -t abot/agent:$(VERSION) -t abot/agent:latest .
	@echo "✓ abot-agent image built"

docker-build-server:
	@echo "Building abot-server Docker image..."
	@docker build -f cmd/abot-server/Dockerfile -t abot/server:$(VERSION) -t abot/server:latest .
	@echo "✓ abot-server image built"

docker-build-web:
	@echo "Building abot-web Docker image..."
	@docker build -f cmd/abot-web/Dockerfile -t abot/web:$(VERSION) -t abot/web:latest .
	@echo "✓ abot-web image built"

# Push Docker images
docker-push: docker-push-agent docker-push-server docker-push-web

docker-push-agent:
	@echo "Pushing abot-agent Docker image..."
	@docker push abot/agent:$(VERSION)
	@docker push abot/agent:latest
	@echo "✓ abot-agent image pushed"

docker-push-server:
	@echo "Pushing abot-server Docker image..."
	@docker push abot/server:$(VERSION)
	@docker push abot/server:latest
	@echo "✓ abot-server image pushed"

docker-push-web:
	@echo "Pushing abot-web Docker image..."
	@docker push abot/web:$(VERSION)
	@docker push abot/web:latest
	@echo "✓ abot-web image pushed"

# Development helpers
dev-agent:
	@go run ./cmd/abot-agent -config config.yaml

dev-server:
	@go run ./cmd/abot-server -config config.yaml

dev-web:
	@go run ./cmd/abot-web -config config.yaml

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "✓ Code formatted"

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	@go mod tidy
	@echo "✓ Dependencies tidied"

# Generate mocks (if using mockgen)
mocks:
	@echo "Generating mocks..."
	@go generate ./...
	@echo "✓ Mocks generated"

# Help
help:
	@echo "Available targets:"
	@echo "  all            - Build all binaries"
	@echo "  agent          - Build abot-agent"
	@echo "  server         - Build abot-server"
	@echo "  web            - Build abot-web"
	@echo "  web-ui         - Build web UI"
	@echo "  clean          - Clean build artifacts"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  lint           - Run linters"
	@echo "  install        - Install binaries to GOPATH/bin"
	@echo "  docker-build   - Build all Docker images"
	@echo "  docker-push    - Push all Docker images"
	@echo "  dev-agent      - Run abot-agent in development mode"
	@echo "  dev-server     - Run abot-server in development mode"
	@echo "  dev-web        - Run abot-web in development mode"
	@echo "  fmt            - Format code"
	@echo "  tidy           - Tidy dependencies"
	@echo "  mocks          - Generate mocks"
	@echo "  help           - Show this help message"
