# Makefile cho Address Parser Service với libpostal support

.PHONY: help build run test clean docker-build docker-run docker-stop build-libpostal run-libpostal test-libpostal

# Variables
BINARY_NAME=address-parser
MAIN_FILE=main.go
DOCKER_IMAGE=address-parser
DOCKER_TAG=latest

# Default target
help:
	@echo "🚀 Address Parser - Available commands:"
	@echo ""
	@echo "Standard commands:"
	@echo "  build        - Build the application"
	@echo "  run          - Run the application locally"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run with Docker Compose"
	@echo "  docker-stop  - Stop Docker Compose services"
	@echo ""
	@echo "Libpostal commands:"
	@echo "  build-libpostal - Build with libpostal support"
	@echo "  run-libpostal   - Run with libpostal support"
	@echo "  test-libpostal  - Test libpostal integration"
	@echo "  logs-libpostal  - Show libpostal service logs"
	@echo ""
	@echo "  fmt          - Format Go code"
	@echo "  lint         - Run linter"

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) $(MAIN_FILE)
	@echo "Build completed: $(BINARY_NAME)"

# Run the application locally
run:
	@echo "Running $(BINARY_NAME)..."
	go run $(MAIN_FILE)

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	rm -f *.out
	rm -f *.html
	@echo "Clean completed"

# Format Go code
fmt:
	@echo "Formatting Go code..."
	go fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

# Run with Docker Compose
docker-run:
	@echo "Starting services with Docker Compose..."
	docker-compose up -d
	@echo "Services started. Check status with: docker-compose ps"

# Stop Docker Compose services
docker-stop:
	@echo "Stopping Docker Compose services..."
	docker-compose down
	@echo "Services stopped"

# Show Docker Compose logs
docker-logs:
	docker-compose logs -f

# Show Docker Compose status
docker-status:
	docker-compose ps

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download
	@echo "Dependencies installed"

# Generate go.sum
go-sum:
	@echo "Generating go.sum..."
	go mod tidy
	@echo "go.sum generated"

# Development mode with hot reload (requires air)
dev:
	@echo "Starting development mode with hot reload..."

# Libpostal specific commands
download-data:
	@echo "📥 Downloading libpostal data..."
	@./download-libpostal-data.sh

build-libpostal:
	@echo "🔨 Building with libpostal support..."
	@make download-data
	docker-compose build --no-cache

run-libpostal:
	@echo "🌟 Starting services with libpostal..."
	@make download-data
	docker-compose up -d
	@echo "⏳ Waiting for services to initialize..."
	@sleep 30
	@make test-libpostal

test-libpostal:
	@echo "🧪 Testing libpostal integration..."
	@./test-libpostal.sh

logs-libpostal:
	@docker-compose logs -f app

stop-libpostal:
	@echo "🛑 Stopping libpostal services..."
	@docker-compose down

clean-libpostal: stop-libpostal
	@echo "🧹 Cleaning up libpostal containers..."
	@docker-compose down -v --rmi all
	air

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Development tools installed"
