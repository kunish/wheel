.PHONY: build dev test clean docker

# Build the worker binary
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/wheel ./cmd/worker

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
