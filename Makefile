APP_NAME = url-shortener
BINARY = bin/$(APP_NAME)
GO = go
GOFLAGS = -mod=readonly

TEST_PACKAGES := $(shell go list ./internal/... | grep -v /mocks | grep -v /config$$ | grep -v /entity$$ | grep -v /dto$$ | grep -v /metrics$$)

.PHONY: build
build:
	@echo "Building Docker image for $(APP_NAME)..."
	docker-compose build

.PHONY: run
run: build
	@echo "Starting $(APP_NAME) with docker-compose..."
	docker-compose up -d
	@echo "$(APP_NAME) is running.

.PHONY: up
up:
	@echo "Starting services..."
	docker-compose up -d

.PHONY: down
down:
	@echo "Stopping services..."
	docker-compose down -v

.PHONY: test
test:
	@echo "Running unit tests..."
	go test -v -short ./...

.PHONY: test-coverage
test-coverage:
	@echo "Running unit tests with coverage..."
	go test -coverprofile=coverage.out -coverpkg=./internal/... $(TEST_PACKAGES)
	@echo "Coverage report:"
	go tool cover -func=coverage.out | grep total
	go tool cover -html=coverage.out -o coverage.html
	@echo "HTML report saved to coverage.html"

.PHONY: test-all
test-all:
	@echo "Running all tests (unit + integration) with coverage..."
	go test -coverprofile=coverage.out -tags=integration -coverpkg=./internal/... $(TEST_PACKAGES) 
	go tool cover -func=coverage.out | grep total
	go tool cover -html=coverage.out -o coverage.html

.PHONY: generate
generate:
	@echo "Generating mocks..."
	go generate ./...

.PHONY: clean
clean:
	@echo "Cleaning..."
	@rm -rf bin
	@rm -f coverage.out coverage.html
	@go clean -cache -testcache

.PHONY: lint
lint:
	@echo "Running linter..."
	golangci-lint run

.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make build            - Build the docker-compose"
	@echo "  make run              - Build and run the docker-compose"
	@echo "  make test             - Run unit tests (short mode, no integration)"
	@echo "  make test-coverage    - Run unit tests with coverage report"
	@echo "  make test-all         - Run all tests with coverage"
	@echo "  make generate         - Generate mocks (requires mockgen)"
	@echo "  make clean            - Remove build artifacts and coverage files"
	@echo "  make lint             - Run golangci-lint"
	@echo "  make help             - Show this help"