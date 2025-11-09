# Simple Makefile for a Go project

# Build the application
all: build test

build:
	@echo "Building..."
	
	
	@go build -o main.exe cmd/api/main.go

# Run the application
run:
	@go run cmd/api/main.go
# Create DB container
docker-run:
	@docker compose up --build

# Shutdown DB container
docker-down:
	@docker compose down

# Test the application
test:
	@echo "Testing..."
	@go test ./... -v
# Integrations Tests for the application
itest:
	@echo "Running integration tests..."
	@go test ./internal/database -v

# Database Migrations
migrate-up-all:
	@echo "Running migrations..."
	@go run cmd/migrate/main.go -action=up

migrate-down-all:
	@echo "Rolling back all migrations..."
	@go run cmd/migrate/main.go -action=down

migrate-down:
	@echo "Rolling back last migration..."
	@go run cmd/migrate/main.go -action=down -steps=1

migrate-up:
	@echo "Running next migration..."
	@go run cmd/migrate/main.go -action=up -steps=1

migrate-status:
	@echo "Checking migration status..."
	@go run cmd/migrate/main.go -action=status

migrate-refresh:
	@echo "Refreshing migrations..."
	@go run cmd/migrate/main.go -action=down
	@go run cmd/migrate/main.go -action=up

# Clean the binary
clean:
	@echo "Cleaning..."
	@rm -f main

# Live Reload
watch:
	@powershell -ExecutionPolicy Bypass -Command "if (Get-Command air -ErrorAction SilentlyContinue) { \
		air; \
		Write-Output 'Watching...'; \
	} else { \
		Write-Output 'Installing air...'; \
		go install github.com/air-verse/air@latest; \
		air; \
		Write-Output 'Watching...'; \
	}"

.PHONY: all build run test clean watch docker-run docker-down itest migrate-up migrate-down migrate-down-1 migrate-up-1 migrate-status migrate-refresh
