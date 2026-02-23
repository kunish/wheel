.PHONY: build dev test clean docker docs

VERSION ?= dev
OUTPUT ?= bin/wheel

# Build the worker binary
build:
	CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/kunish/wheel/apps/worker/internal/config.Version=$(VERSION)" -o $(OUTPUT) ./cmd/worker

# Run in development mode with hot-reload
dev:
	go run github.com/air-verse/air@latest

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Docker build
docker:
	docker build -t wheel-worker .

# Generate OpenAPI docs
docs:
	go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/worker/main.go -o docs --parseDependency
