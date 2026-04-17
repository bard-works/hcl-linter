.PHONY: all build test lint clean install run help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Binary name
BINARY_NAME=hcl-linter
VERSION=$(shell git describe --always --dirty --tags 2>/dev/null || echo "dev")
BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE)"

# Directories
DIST_DIR=dist
CMD_DIR=cmd/hcl-linter

# Default target
all: test build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(DIST_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME) ./$(CMD_DIR)

# Build for multiple platforms
build-all: build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-windows-amd64

build-darwin-amd64:
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 ./$(CMD_DIR)

build-darwin-arm64:
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./$(CMD_DIR)

build-linux-amd64:
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./$(CMD_DIR)

build-windows-amd64:
	@mkdir -p $(DIST_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe ./$(CMD_DIR)

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# Vet code
vet:
	@echo "Vetting code..."
	$(GOVET) ./...

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(DIST_DIR)
	@rm -f coverage.out coverage.html

# Install binary to GOPATH
install:
	$(GOBUILD) $(LDFLAGS) -o $(GOPATH)/bin/$(BINARY_NAME) ./$(CMD_DIR)

# Run the binary
run:
	@./$(DIST_DIR)/$(BINARY_NAME)

# Show help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all           -- Run test and build (default)"
	@echo "  build         -- Build the binary to dist/"
	@echo "  build-all     -- Build for all platforms"
	@echo "  test          -- Run tests"
	@echo "  test-coverage -- Run tests with coverage report"
	@echo "  lint          -- Run golangci-lint"
	@echo "  fmt            -- Format code"
	@echo "  vet            -- Vet code"
	@echo "  tidy           -- Tidy dependencies"
	@echo "  deps           -- Download dependencies"
	@echo "  clean          -- Remove build artifacts"
	@echo "  install        -- Install binary to GOPATH/bin"
	@echo "  run            -- Run the built binary"
	@echo "  help           -- Show this help"
