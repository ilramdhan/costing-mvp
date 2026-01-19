.PHONY: all build run test clean docker-up docker-down migrate-up migrate-down seed help

# Variables
BINARY_API=bin/api
BINARY_WORKER=bin/worker
BINARY_SEEDER=bin/seeder
BINARY_MIGRATE=bin/migrate

all: build

## build: Build all binaries
build:
	@echo "Building binaries..."
	@mkdir -p bin
	go build -o $(BINARY_API) ./cmd/api
	go build -o $(BINARY_WORKER) ./cmd/worker
	go build -o $(BINARY_SEEDER) ./cmd/seeder
	go build -o $(BINARY_MIGRATE) ./cmd/migrate
	@echo "Build complete!"

## run-api: Run the API server
run-api:
	go run ./cmd/api

## run-worker: Run the background worker
run-worker:
	go run ./cmd/worker

## test: Run all tests
test:
	go test -v -race -cover ./...

## test-formula: Run formula parser tests
test-formula:
	go test -v ./pkg/formula/...

## clean: Clean build artifacts
clean:
	rm -rf bin/
	go clean

## docker-up: Start all Docker services
docker-up:
	docker-compose up -d

## docker-down: Stop all Docker services
docker-down:
	docker-compose down

## docker-build: Build Docker images
docker-build:
	docker-compose build

## docker-logs: Follow Docker logs
docker-logs:
	docker-compose logs -f

## migrate-up: Run database migrations
migrate-up:
	go run ./cmd/migrate up

## migrate-down: Rollback database migrations
migrate-down:
	go run ./cmd/migrate down

## seed: Run the seeder with default values
seed:
	go run ./cmd/seeder --masters=1000 --children=100

## seed-full: Run the seeder with full MVP scale
seed-full:
	go run ./cmd/seeder --masters=500000 --children=500

## seed-stress: Run a medium stress test
seed-stress:
	go run ./cmd/seeder --masters=10000 --children=200

## recalc: Trigger recalculation via API
recalc:
	curl -X POST http://localhost:8080/api/v1/recalculate/all

## deps: Download dependencies
deps:
	go mod download
	go mod tidy

## lint: Run linter
lint:
	golangci-lint run ./...

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
