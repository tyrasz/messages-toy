# Messenger Backend Makefile

.PHONY: all build run test test-v test-cover clean lint fmt deps docker docker-build docker-run help

# Default target
all: build

# Build the application
build:
	CGO_ENABLED=1 go build -o bin/messenger ./cmd/server

# Build for production (static binary)
build-static:
	CGO_ENABLED=1 go build -a -ldflags '-linkmode external -extldflags "-static"' -o bin/messenger ./cmd/server

# Run the application
run:
	go run ./cmd/server

# Run with hot reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	air

# Run tests
test:
	go test ./...

# Run tests with verbose output
test-v:
	go test ./... -v

# Run tests with coverage
test-cover:
	go test ./... -cover

# Run tests with coverage report
test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run specific package tests
test-handlers:
	go test ./internal/api/handlers/... -v

test-middleware:
	go test ./internal/api/middleware/... -v

test-websocket:
	go test ./internal/websocket/... -v

test-models:
	go test ./internal/models/... -v

test-services:
	go test ./internal/services/... -v

# Run benchmarks
bench:
	go test ./... -bench=. -benchmem

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f messenger.db

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...
	gofmt -s -w .

# Tidy dependencies
deps:
	go mod tidy
	go mod verify

# Download dependencies
deps-download:
	go mod download

# Update dependencies
deps-update:
	go get -u ./...
	go mod tidy

# Docker targets
docker-build:
	docker build -t messenger-api .

docker-run:
	docker run -p 8080:8080 -v messenger-data:/app/data messenger-api

docker-compose-up:
	docker-compose up -d

docker-compose-down:
	docker-compose down

docker-compose-logs:
	docker-compose logs -f

# Database operations
db-migrate:
	go run ./cmd/server --migrate-only

# Generate VAPID keys for Web Push
gen-vapid:
	@go run -e 'package main; import ("fmt"; "github.com/SherClockHolmes/webpush-go"); func main() { priv, pub, _ := webpush.GenerateVAPIDKeys(); fmt.Printf("VAPID_PUBLIC_KEY=%s\nVAPID_PRIVATE_KEY=%s\n", pub, priv) }'

# Security check (requires govulncheck: go install golang.org/x/vuln/cmd/govulncheck@latest)
security:
	govulncheck ./...

# Show help
help:
	@echo "Messenger Backend - Available targets:"
	@echo ""
	@echo "  build          - Build the application"
	@echo "  build-static   - Build static binary for Docker"
	@echo "  run            - Run the application"
	@echo "  dev            - Run with hot reload (requires air)"
	@echo ""
	@echo "  test           - Run tests"
	@echo "  test-v         - Run tests with verbose output"
	@echo "  test-cover     - Run tests with coverage summary"
	@echo "  test-coverage  - Generate HTML coverage report"
	@echo "  test-handlers  - Test handlers package"
	@echo "  test-websocket - Test websocket package"
	@echo "  bench          - Run benchmarks"
	@echo ""
	@echo "  clean          - Remove build artifacts"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  deps           - Tidy dependencies"
	@echo "  deps-update    - Update dependencies"
	@echo "  security       - Run security vulnerability check"
	@echo ""
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container"
	@echo "  docker-compose-up   - Start with docker-compose"
	@echo "  docker-compose-down - Stop docker-compose"
	@echo ""
	@echo "  help           - Show this help"
