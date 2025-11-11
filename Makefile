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
migrate-create:
	@echo "Creating migration..."
	@powershell -Command "if ('$(name)' -eq '') { Write-Output 'Error: Please provide a migration name using name=<migration_name>'; Write-Output 'Example: make migrate-create name=add_username_to_users'; exit 1 }"
	@go run cmd/migrate/main.go -action=create $(name)

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

# Seed the database with default users
seed:
	@echo "Seeding database..."
	@go run cmd/seed/main.go

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

# Generate Swagger documentation
swagger:
	@echo "Generating Swagger documentation..."
	@powershell -ExecutionPolicy Bypass -Command "if (Get-Command swag -ErrorAction SilentlyContinue) { \
		swag init -g cmd/api/main.go --parseDependency --parseInternal; \
		Write-Output 'Swagger docs generated successfully!'; \
	} else { \
		Write-Output 'Installing swag...'; \
		go install github.com/swaggo/swag/cmd/swag@latest; \
		swag init -g cmd/api/main.go --parseDependency --parseInternal; \
		Write-Output 'Swagger docs generated successfully!'; \
	}"

.PHONY: all build run test clean watch docker-run docker-down itest migrate-create migrate-up-all migrate-down-all migrate-down migrate-up migrate-status migrate-refresh seed swagger
