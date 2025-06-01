.PHONY: build test clean run dev migrate-up migrate-down

# Build the application
build:
	go build -o bin/device-service cmd/server/main.go

# Run tests
test:
	go test -v ./...

# Run with race detection
test-race:
	go test -race -v ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Run in development mode
dev:
	go run cmd/server/main.go

# Run the application
run: build
	./bin/device-service

# Database migrations
migrate-up:
	migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/device_service?sslmode=disable" up

migrate-down:
	migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/device_service?sslmode=disable" down

# Docker commands
docker-build:
	docker build -t device-service:latest .

docker-run:
	docker-compose up -d

# Code quality
fmt:
	go fmt ./...

lint:
	golangci-lint run

# Generate mocks
generate-mocks:
	go generate ./...
```

## .gitignore
```gitignore
# Binaries
bin/
*.exe
*.exe~
*.dll
*.so
*.dylib

# Test binary
*.test

# Output of the go coverage tool
*.out

# Dependency directories
vendor/

# Go workspace file
go.work

# IDE files
.vscode/
.idea/
*.swp
*.swo

# OS files
.DS_Store
Thumbs.db

# Config files with secrets
config/local.yaml
config/secrets.yaml

# Logs
*.log
logs/

# Local database
*.db
*.sqlite

# Certificate files
*.pem
*.key
*.crt